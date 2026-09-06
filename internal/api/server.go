package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/iSundram/callhook/internal/business"
	"github.com/iSundram/callhook/internal/callhookclient"
	"github.com/iSundram/callhook/internal/callwindow"
	"github.com/iSundram/callhook/internal/events"
	"github.com/iSundram/callhook/internal/outcome"
	"github.com/iSundram/callhook/internal/session"
)

// Server wires everything together: event intake, CALL-E client, outcome
// engine, session registry, and the live dashboard.
type Server struct {
	Router   *events.Router
	Store    business.Store
	Sessions *session.Store
	Calle    *callhookclient.Client
	Outcomes *outcome.Engine
	// PublicBaseURL is where CALL-E can reach our webhook (e.g. a tunnel URL).
	PublicBaseURL string
	// IntakeToken, when set, is required as "Authorization: Bearer <token>"
	// on POST /api/events. Empty = open (development only).
	IntakeToken string
	// WebhookSecret, when set, is required as "X-Callhook-Secret" on
	// POST /callhook/webhook so nobody can forge terminal results.
	WebhookSecret string
	// EnforceWindows defers calls placed outside polite local hours.
	EnforceWindows bool

	limiter *rateLimiter
}

func (s *Server) Routes() *http.ServeMux {
	s.limiter = newRateLimiter(60, time.Minute) // 60 events/min per source IP
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/events", s.authIntake(s.limiter.wrap(s.handleEvent)))
	mux.HandleFunc("POST /api/events/batch", s.authIntake(s.handleEventBatch))
	mux.HandleFunc("POST /callhook/webhook", s.authWebhook(s.handleCalleWebhook))
	mux.HandleFunc("GET /api/sessions", s.handleSessions)
	mux.HandleFunc("GET /api/metrics", s.handleMetrics)
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /", s.handleDashboard)
	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":              true,
		"dry_run":         s.Calle.DryRun,
		"windows_enforced": s.EnforceWindows,
		"auth_intake":     s.IntakeToken != "",
		"auth_webhook":    s.WebhookSecret != "",
	})
}

func (s *Server) authIntake(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.IntakeToken != "" {
			got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if got != s.IntakeToken {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid or missing bearer token"})
				return
			}
		}
		next(w, r)
	}
}

func (s *Server) authWebhook(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.WebhookSecret != "" && r.Header.Get("X-Callhook-Secret") != s.WebhookSecret {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid webhook secret"})
			return
		}
		next(w, r)
	}
}

// handleEvent is THE product: a business system POSTs an event, and callhook
// takes over all phone communication for it.
func (s *Server) handleEvent(w http.ResponseWriter, r *http.Request) {
	var ev events.Event
	if err := json.NewDecoder(r.Body).Decode(&ev); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json: " + err.Error()})
		return
	}
	status, body := s.processEvent(&ev)
	writeJSON(w, status, body)
}

// handleEventBatch accepts {events: [...]} and processes each
// independently — one bad event does not sink the batch.
func (s *Server) handleEventBatch(w http.ResponseWriter, r *http.Request) {
	var batch struct {
		Events []events.Event `json:"events"`
	}
	if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json: " + err.Error()})
		return
	}
	results := make([]map[string]any, 0, len(batch.Events))
	anyOK := false
	for i := range batch.Events {
		status, body := s.processEvent(&batch.Events[i])
		body["http_status"] = status
		if status < 300 {
			anyOK = true
		}
		results = append(results, body)
	}
	code := http.StatusOK
	if !anyOK && len(results) > 0 {
		code = http.StatusBadRequest
	}
	writeJSON(w, code, map[string]any{"results": results})
}

// processEvent runs one event through the pipeline and returns the HTTP
// status plus the response body (shared by single and batch intake).
func (s *Server) processEvent(ev *events.Event) (int, map[string]any) {
	ev.ReceivedAt = time.Now().UTC()

	if err := ev.Validate(); err != nil {
		return http.StatusBadRequest, map[string]any{"error": err.Error()}
	}

	// Idempotency: the same event must never trigger two calls.
	key := ev.Idempotency
	if key == "" {
		key = ev.ID
	}
	if s.Sessions.AlreadySeen(key) {
		return http.StatusOK, map[string]any{"status": "duplicate", "session_id": ev.ID}
	}

	blueprint, ok := s.Router.Blueprint(ev.Type)
	if !ok {
		return http.StatusBadRequest, map[string]any{
			"error":           "unsupported event type: " + ev.Type,
			"supported_types": s.Router.Supported(),
		}
	}

	// Prefetch: resolve the full customer record BEFORE dialing, so the call
	// task contains live business data and mid-call lookups hit cache.
	customer, err := s.Store.GetCustomer(ev.CustomerID)
	if err != nil {
		return http.StatusNotFound, map[string]any{"error": err.Error()}
	}
	phone := ev.Phone
	if phone == "" {
		phone = customer.Phone
	}
	if !strings.HasPrefix(phone, "+") {
		return http.StatusBadRequest, map[string]any{"error": "phone must be E.164 format (start with +)"}
	}

	task, err := blueprint.Compose(customer, ev, s.Store)
	if err != nil {
		return http.StatusInternalServerError, map[string]any{"error": err.Error()}
	}

	sess := s.Sessions.Create(ev.ID, *ev, customer)
	s.Sessions.Log(sess.ID, "event_received", ev.Type+" for "+customer.Name)
	s.Sessions.Log(sess.ID, "prefetched", "customer="+customer.ID+" invoice/payment history loaded")
	s.Sessions.SetCallTarget(sess.ID, phone, customer.Locale, customer.Region, ev.TZ)
	s.Sessions.SetResultSchema(sess.ID, blueprint.ResultSchema)
	s.Sessions.SetTask(sess.ID, task)
	if blueprint.GoalID != "" {
		s.Sessions.SetGoal(sess.ID, blueprint.GoalID, blueprint.VariablesFor(customer, ev))
	}

	// Caller-requested start time: park the session until then.
	if ev.NotBefore != "" {
		nb, err := time.Parse(time.RFC3339, ev.NotBefore)
		if err != nil {
			return http.StatusBadRequest, map[string]any{"error": "not_before must be RFC3339"}
		}
		s.Sessions.DeferUntil(sess.ID, nb.UTC(), session.KindScheduled)
		s.Sessions.Log(sess.ID, "call_scheduled", "not before "+nb.Format(time.RFC3339))
		return http.StatusAccepted, map[string]any{
			"status": "scheduled", "session_id": sess.ID, "not_before": ev.NotBefore,
		}
	}

	call, placed, err := s.PlaceCall(sess)
	if err != nil {
		s.Sessions.Log(sess.ID, "call_failed", err.Error())
		return http.StatusBadGateway, map[string]any{"error": "callhook: " + err.Error()}
	}
	if !placed {
		return http.StatusAccepted, map[string]any{
			"status": "deferred", "session_id": sess.ID,
			"reason": "outside calling hours", "dry_run": s.Calle.DryRun,
		}
	}
	return http.StatusAccepted, map[string]any{
		"status": "call_placed", "session_id": sess.ID, "call_id": call.ID,
		"phone": phone, "dry_run": s.Calle.DryRun,
	}
}

// PlaceCall places (or re-places, on retry) the call for a session. Shared
// by the intake handler and the scheduler. Each attempt gets its own
// idempotency key so a crash mid-retry never double-dials. Returns
// placed=false when the call was deferred to the next open calling window.
func (s *Server) PlaceCall(sess *session.Session) (*callhookclient.CallTask, bool, error) {
	// Polite-hours gate: defer instead of dialing at night.
	if s.EnforceWindows && !callwindow.IsOpen(sess.Region, sess.TZ, time.Now().UTC()) {
		next := callwindow.NextOpen(sess.Region, sess.TZ, time.Now().UTC())
		s.Sessions.DeferUntil(sess.ID, next, session.KindWindow)
		s.Sessions.Log(sess.ID, "call_deferred", "outside calling hours — will dial at "+next.Format(time.RFC3339))
		return nil, false, nil
	}

	attempt := sess.RetryCount + 1
	idem := fmt.Sprintf("%s-a%d", sess.ID, attempt)

	var call *callhookclient.CallTask
	var err error
	if sess.GoalID != "" {
		// Goals path: execute a published, versioned workflow with typed
		// variables instead of a free-text task.
		var run *callhookclient.GoalRun
		run, err = s.Calle.CreateGoalCall(context.Background(), sess.GoalID, sess.Phone, sess.GoalVariables, idem)
		if err == nil {
			call = run.AsCallTask()
		}
	} else {
		callReq := &callhookclient.CreateCallRequest{
			Task: sess.Task,
			Recipients: []callhookclient.Recipient{{
				Phones: []string{sess.Phone},
				Locale: sess.Locale,
				Region: sess.Region,
			}},
			ResultSchema: sess.ResultSchema,
			Metadata: map[string]any{
				"event_id":    sess.Event.ID,
				"event_type":  sess.Event.Type,
				"customer_id": sess.Event.CustomerID,
				"attempt":     attempt,
			},
		}
		if s.PublicBaseURL != "" {
			callReq.WebhookURL = strings.TrimSuffix(s.PublicBaseURL, "/") + "/callhook/webhook"
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		call, err = s.Calle.CreateCall(ctx, callReq, idem)
		cancel()
	}
	if err != nil {
		return nil, false, err
	}

	s.Sessions.SetCall(sess.ID, call)
	if attempt > 1 {
		s.Sessions.Log(sess.ID, "call_placed", fmt.Sprintf("%s → %s (retry attempt %d/%d)", call.ID, sess.Phone, attempt, session.MaxRetries+1))
	} else {
		s.Sessions.Log(sess.ID, "call_placed", call.ID+" → "+sess.Phone)
	}
	return call, true, nil
}

// handleCalleWebhook receives terminal call results from CALL-E.
func (s *Server) handleCalleWebhook(w http.ResponseWriter, r *http.Request) {
	var ev callhookclient.WebhookEvent
	if err := json.NewDecoder(r.Body).Decode(&ev); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	go s.Outcomes.Apply(&ev)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleSessions(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.Sessions.Snapshot())
}

// handleMetrics serves aggregate pipeline stats for monitoring.
func (s *Server) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	sessions := s.Sessions.Snapshot()
	m := map[string]any{
		"sessions":     len(sessions),
		"by_status":    map[string]int{},
		"by_outcome":   map[string]int{},
		"retries_armed": 0,
	}
	byStatus := m["by_status"].(map[string]int)
	byOutcome := m["by_outcome"].(map[string]int)
	for _, s := range sessions {
		status := s.CallStatus
		if status == "" {
			status = "intake"
		}
		byStatus[status]++
		if s.NextRetryAt != nil {
			m["retries_armed"] = m["retries_armed"].(int) + 1
		}
		if v, ok := s.Outcome["outcome"].(string); ok && v != "" {
			byOutcome[v]++
		}
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := fmt.Fprint(w, dashboardHTML); err != nil {
		log.Printf("dashboard render: %v", err)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// --- minimal per-IP rate limiter (stdlib only) ---

type rateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	hits    map[string][]time.Time
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{limit: limit, window: window, hits: map[string][]time.Time{}}
}

func (rl *rateLimiter) wrap(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			ip = strings.TrimSpace(strings.Split(fwd, ",")[0])
		}
		if !rl.allow(ip, time.Now()) {
			writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate limit exceeded"})
			return
		}
		next(w, r)
	}
}

func (rl *rateLimiter) allow(key string, now time.Time) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	cutoff := now.Add(-rl.window)
	kept := rl.hits[key][:0]
	for _, t := range rl.hits[key] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= rl.limit {
		rl.hits[key] = kept
		return false
	}
	rl.hits[key] = append(kept, now)
	return true
}
