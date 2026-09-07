package api

import (
	"context"
	"encoding/json"
	"fmt"

	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/iSundram/callhook/internal/business"
	"github.com/iSundram/callhook/internal/callhookclient"
	"github.com/iSundram/callhook/internal/callwindow"
	"github.com/iSundram/callhook/internal/campaign"
	"github.com/iSundram/callhook/internal/events"
	"github.com/iSundram/callhook/internal/mcp"
	"github.com/iSundram/callhook/internal/outcome"
	"github.com/iSundram/callhook/internal/session"
)

// Server wires everything together: event intake, CALL-E client, outcome
// engine, session registry, campaign engine, and the web app.
type Server struct {
	Router   *events.Router
	Store    business.Store
	Sessions *session.Store
	Calle    *callhookclient.Client
	Outcomes *outcome.Engine
	// Campaigns is the campaign registry; nil disables campaign endpoints.
	Campaigns *campaign.Store
	// BusinessSource provides auto-audiences (e.g. all overdue customers).
	// Same as Store; separate field keeps the campaign API self-contained.
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
	// MCP, when set, serves the Model Context Protocol at POST /mcp —
	// callhook as agent tools.
	MCP *mcp.Server

	limiter     *rateLimiter
	routesCache http.Handler
}

// EventResult is the pipeline outcome for one fired event — shared by the
// HTTP intake and the campaign runner.
type EventResult struct {
	Status    string // call_placed | scheduled | deferred | duplicate | error
	SessionID string
	CallID    string
	Phone     string
	NotBefore string
	Err       error
	Body      map[string]any // full response body (error details etc.)
}

func (s *Server) Routes() http.Handler {
	if s.routesCache != nil {
		return s.routesCache
	}
	s.limiter = newRateLimiter(60, time.Minute) // 60 events/min per source IP
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/events", s.cors(s.authIntake(s.limiter.wrap(s.handleEvent))))
	mux.HandleFunc("POST /api/events/batch", s.cors(s.authIntake(s.handleEventBatch)))
	mux.HandleFunc("POST /callhook/webhook", s.authWebhook(s.handleCalleWebhook))
	mux.HandleFunc("GET /api/sessions", s.cors(s.authIntake(s.handleSessions)))
	mux.HandleFunc("GET /api/metrics", s.cors(s.authIntake(s.handleMetrics)))
	mux.HandleFunc("GET /api/health", s.cors(s.handleHealth)) // health is the connect probe
	if s.Campaigns != nil {
		mux.HandleFunc("POST /api/campaigns", s.cors(s.authIntake(s.handleCampaignCreate)))
		mux.HandleFunc("GET /api/campaigns", s.cors(s.authIntake(s.handleCampaignList)))
		mux.HandleFunc("GET /api/campaigns/{id}", s.cors(s.authIntake(s.handleCampaignGet)))
		mux.HandleFunc("POST /api/campaigns/{id}/stop", s.cors(s.authIntake(s.handleCampaignStop)))
	}
	if s.MCP != nil {
		mux.HandleFunc("POST /mcp", s.cors(s.authIntake(s.handleMCP)))
	}
	mux.HandleFunc("GET /", s.handleRoot)

	// OPTIONS preflight must be answered before Go's method-based routing
	// would 405 it.
	root := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions && strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Callhook-Secret")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		mux.ServeHTTP(w, r)
	})
	s.routesCache = root
	return root
}

// handleRoot serves the embedded web app.
func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	http.FileServer(http.FS(webAppFS())).ServeHTTP(w, r)
}

// cors allows the web app to talk to this API from any origin (the token is
// the real gate) and answers preflights.
func (s *Server) cors(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Callhook-Secret")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":               true,
		"dry_run":          s.Calle.DryRun,
		"windows_enforced": s.EnforceWindows,
		"auth_intake":      s.IntakeToken != "",
		"auth_webhook":     s.WebhookSecret != "",
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
	status, body := s.processEventHTTP(&ev)
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
		status, body := s.processEventHTTP(&batch.Events[i])
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

// ProcessEvent runs one event through the full pipeline. Shared by HTTP
// intake and the campaign runner.
func (s *Server) ProcessEvent(ev *events.Event) EventResult {
	ev.ReceivedAt = time.Now().UTC()

	if err := ev.Validate(); err != nil {
		return EventResult{Status: "error", Err: err, Body: map[string]any{"error": err.Error()}}
	}

	// Idempotency: the same event must never trigger two calls.
	key := ev.Idempotency
	if key == "" {
		key = ev.ID
	}
	if s.Sessions.AlreadySeen(key) {
		return EventResult{Status: "duplicate", SessionID: ev.ID, Body: map[string]any{"status": "duplicate", "session_id": ev.ID}}
	}

	blueprint, ok := s.Router.Blueprint(ev.Type)
	if !ok {
		err := fmt.Errorf("unsupported event type: %s", ev.Type)
		return EventResult{Status: "error", Err: err, Body: map[string]any{"error": err.Error(), "supported_types": s.Router.Supported()}}
	}

	// Prefetch: resolve the full customer record BEFORE dialing, so the call
	// task contains live business data and mid-call lookups hit cache.
	customer, err := s.Store.GetCustomer(ev.CustomerID)
	if err != nil {
		return EventResult{Status: "error", Err: err, Body: map[string]any{"error": err.Error()}}
	}
	phone := ev.Phone
	if phone == "" {
		phone = customer.Phone
	}
	if !strings.HasPrefix(phone, "+") {
		err := fmt.Errorf("phone must be E.164 format (start with +)")
		return EventResult{Status: "error", Err: err, Body: map[string]any{"error": err.Error()}}
	}

	task, err := blueprint.Compose(customer, ev, s.Store)
	if err != nil {
		return EventResult{Status: "error", Err: err, Body: map[string]any{"error": err.Error()}}
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
			return EventResult{Status: "error", Err: err, Body: map[string]any{"error": "not_before must be RFC3339"}}
		}
		s.Sessions.DeferUntil(sess.ID, nb.UTC(), session.KindScheduled)
		s.Sessions.Log(sess.ID, "call_scheduled", "not before "+nb.Format(time.RFC3339))
		return EventResult{Status: "scheduled", SessionID: sess.ID, NotBefore: ev.NotBefore,
			Body: map[string]any{"status": "scheduled", "session_id": sess.ID, "not_before": ev.NotBefore}}
	}

	call, placed, err := s.PlaceCall(sess)
	if err != nil {
		s.Sessions.Log(sess.ID, "call_failed", err.Error())
		return EventResult{Status: "error", SessionID: sess.ID, Err: err,
			Body: map[string]any{"error": "callhook: " + err.Error()}}
	}
	if !placed {
		return EventResult{Status: "deferred", SessionID: sess.ID, Phone: phone,
			Body: map[string]any{"status": "deferred", "session_id": sess.ID, "reason": "outside calling hours", "dry_run": s.Calle.DryRun}}
	}
	return EventResult{Status: "call_placed", SessionID: sess.ID, CallID: call.ID, Phone: phone,
		Body: map[string]any{"status": "call_placed", "session_id": sess.ID, "call_id": call.ID, "phone": phone, "dry_run": s.Calle.DryRun}}
}

// processEventHTTP maps a pipeline result to an HTTP status + body.
func (s *Server) processEventHTTP(ev *events.Event) (int, map[string]any) {
	res := s.ProcessEvent(ev)
	switch res.Status {
	case "call_placed", "scheduled", "deferred":
		return http.StatusAccepted, res.Body
	case "duplicate":
		return http.StatusOK, res.Body
	default:
		code := http.StatusBadRequest
		if _, isNotFound := res.Body["error"]; isNotFound && res.Err != nil && strings.Contains(res.Err.Error(), "not found") {
			code = http.StatusNotFound
		}
		if res.Status == "error" && res.CallID == "" && res.SessionID != "" && res.Err != nil && strings.HasPrefix(res.Err.Error(), "callhook:") {
			code = http.StatusBadGateway
		}
		return code, res.Body
	}
}

// FireEvent adapts ProcessEvent for the campaign runner.
func (s *Server) FireEvent(ev *events.Event) campaign.FireResult {
	res := s.ProcessEvent(ev)
	return campaign.FireResult{
		Status:    res.Status,
		SessionID: res.SessionID,
		CallID:    res.CallID,
		Err:       res.Err,
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

	// Dry-run: deliver the fabricated terminal result to the outcome engine
	// after a simulated call duration, so retries, campaigns and the
	// dashboard behave exactly as they will live.
	if s.Calle.DryRun && s.Outcomes != nil {
		ev := &callhookclient.WebhookEvent{
			ID:   "wh_dry_" + call.ID,
			Type: "call.completed",
			Data: *call,
		}
		go func(ev *callhookclient.WebhookEvent) {
			time.Sleep(2 * time.Second) // simulated call duration
			s.Outcomes.Apply(ev)
		}(ev)
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

// --- campaign endpoints ---

type campaignCreateReq struct {
	Name           string                   `json:"name"`
	EventType      string                   `json:"event_type"`
	Payload        json.RawMessage          `json:"payload,omitempty"`
	Goal           campaign.GoalSpec        `json:"goal"`
	Audience       []campaign.AudienceEntry `json:"audience"`
	AudienceSource string                   `json:"audience_source,omitempty"` // "all_overdue"
	Waves          campaign.WavePolicy      `json:"waves"`
	Budget         campaign.Budget          `json:"budget"`
}

func (s *Server) handleCampaignCreate(w http.ResponseWriter, r *http.Request) {
	var req campaignCreateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json: " + err.Error()})
		return
	}
	if req.EventType == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "event_type is required"})
		return
	}
	if req.Goal.Type == "" {
		req.Goal.Type = campaign.GoalCount
	}
	if req.Goal.Type == campaign.GoalCount && req.Goal.Target <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "goal.target must be > 0 for count goals"})
		return
	}
	if len(req.Goal.SuccessOutcomes) == 0 {
		// Default: the blueprint's success-flavored outcomes.
		req.Goal.SuccessOutcomes = []string{"payment_promised", "accepted", "acknowledged", "activity_confirmed_legitimate"}
	}

	// Audience: explicit list, or auto-query the business store.
	audience := req.Audience
	if len(audience) == 0 {
		switch req.AudienceSource {
		case "all_overdue", "":
			customers, err := s.Store.ListOverdueCustomers()
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			for _, c := range customers {
				audience = append(audience, campaign.AudienceEntry{CustomerID: c.ID, State: campaign.EntryPending})
			}
		default:
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown audience_source: " + req.AudienceSource})
			return
		}
	} else {
		for i := range audience {
			audience[i].State = campaign.EntryPending
		}
	}
	if len(audience) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "audience is empty"})
		return
	}

	if req.Budget.MaxCalls == 0 {
		req.Budget.MaxCalls = len(audience) // sane default: at most one call per person
	}
	if req.Name == "" {
		req.Name = req.EventType + " campaign"
	}

	c := s.Campaigns.Create(req.Name, req.EventType, req.Payload, req.Goal, audience, req.Waves, req.Budget)
	writeJSON(w, http.StatusCreated, c)
}

func (s *Server) handleCampaignList(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.Campaigns.List())
}

func (s *Server) handleCampaignGet(w http.ResponseWriter, r *http.Request) {
	c, ok := s.Campaigns.Get(r.PathValue("id"))
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "campaign not found"})
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) handleCampaignStop(w http.ResponseWriter, r *http.Request) {
	if !s.Campaigns.Stop(r.PathValue("id")) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "campaign not found or not running"})
		return
	}
	c, _ := s.Campaigns.Get(r.PathValue("id"))
	writeJSON(w, http.StatusOK, c)
}

// handleMetrics serves aggregate pipeline stats for monitoring.
func (s *Server) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	sessions := s.Sessions.Snapshot()
	m := map[string]any{
		"sessions":      len(sessions),
		"by_status":     map[string]int{},
		"by_outcome":    map[string]int{},
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

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// --- minimal per-IP rate limiter (stdlib only) ---

type rateLimiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	hits   map[string][]time.Time
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


// handleMCP serves the MCP transport.
func (s *Server) handleMCP(w http.ResponseWriter, r *http.Request) {
	s.MCP.HandleHTTP(w, r)
}

// BuildMCP wires the MCP server's tools to this API server.
func (s *Server) BuildMCP() *mcp.Server {
	s.MCP = &mcp.Server{
		FireEvent: func(args json.RawMessage) (string, error) {
			// MCP arguments use event_type/customer_id naming.
			var args_ struct {
				EventType  string `json:"event_type"`
				CustomerID string `json:"customer_id"`
				Phone      string `json:"phone"`
				NotBefore  string `json:"not_before"`
				Type       string `json:"type"`
			}
			if err := mcp.MarshalArg(args, &args_); err != nil {
				return "", err
			}
			ev := events.Event{
				Type:       args_.EventType,
				CustomerID: args_.CustomerID,
				Phone:      args_.Phone,
				NotBefore:  args_.NotBefore,
			}
			if ev.Type == "" {
				ev.Type = args_.Type
			}
			if ev.ID == "" {
				ev.ID = "mcp_" + time.Now().UTC().Format("20060102_150405.000000000")
			}
			res := s.ProcessEvent(&ev)
			b, _ := json.Marshal(res.Body)
			return string(b), nil
		},
		LaunchCampaign: func(args json.RawMessage) (string, error) {
			var req struct {
				Name      string `json:"name"`
				EventType string `json:"event_type"`
				Target    int    `json:"target"`
				Budget    int    `json:"budget"`
				WaveSize  int    `json:"wave_size"`
			}
			if err := mcp.MarshalArg(args, &req); err != nil {
				return "", err
			}
			if req.EventType == "" || req.Target <= 0 {
				return "", fmt.Errorf("event_type and target>0 are required")
			}
			success := []string{"payment_promised", "accepted", "acknowledged", "confirmed"}
			audience, err := s.Store.ListOverdueCustomers()
			if err != nil {
				return "", err
			}
			budget := req.Budget
			if budget == 0 {
				budget = 15
			}
			size := req.WaveSize
			if size == 0 {
				size = 3
			}
			name := req.Name
			if name == "" {
				name = "MCP campaign — " + req.EventType
			}
			var entries []campaign.AudienceEntry
			for _, c := range audience {
				entries = append(entries, campaign.AudienceEntry{CustomerID: c.ID, State: campaign.EntryPending})
			}
			cc := s.Campaigns.Create(name, req.EventType, nil,
				campaign.GoalSpec{Type: campaign.GoalCount, Target: req.Target, SuccessOutcomes: success},
				entries, campaign.WavePolicy{Size: size, Delay: campaign.Duration{Duration: 30 * time.Second}},
				campaign.Budget{MaxCalls: budget})
			b, _ := json.Marshal(map[string]any{"campaign_id": cc.ID, "status": cc.Status, "audience": len(entries), "budget": budget})
			return string(b), nil
		},
		GetCampaign: func(args json.RawMessage) (string, error) {
			var req struct {
				CampaignID string `json:"campaign_id"`
			}
			if err := mcp.MarshalArg(args, &req); err != nil {
				return "", err
			}
			c, ok := s.Campaigns.Get(req.CampaignID)
			if !ok {
				return "", fmt.Errorf("campaign not found: %s", req.CampaignID)
			}
			b, _ := json.MarshalIndent(c, "", "  ")
			return string(b), nil
		},
		ListSessions: func(args json.RawMessage) (string, error) {
			var req struct {
				Limit int `json:"limit"`
			}
			if err := mcp.MarshalArg(args, &req); err != nil {
				return "", err
			}
			if req.Limit <= 0 {
				req.Limit = 10
			}
			views := s.Sessions.Snapshot()
			if len(views) > req.Limit {
				views = views[:req.Limit]
			}
			out := make([]map[string]any, 0, len(views))
			for _, v := range views {
				out = append(out, map[string]any{
					"id": v.ID, "customer": v.Customer, "event_type": v.EventType,
					"status": v.CallStatus, "outcome": v.Outcome["outcome"], "phone": v.Phone,
				})
			}
			b, _ := json.MarshalIndent(out, "", "  ")
			return string(b), nil
		},
		ListEventTypes: func() (string, error) {
			out := map[string]any{}
			for _, typ := range s.Router.Supported() {
				bp, _ := s.Router.Blueprint(typ)
				out[typ] = bp.ResultSchema
			}
			b, _ := json.MarshalIndent(out, "", "  ")
			return string(b), nil
		},
		RunDemo: func() (string, error) {
			for i, cid := range []string{"cus_1002", "cus_1003", "cus_1001"} {
				ev := events.Event{ID: fmt.Sprintf("mcp_demo_%d_%d", time.Now().UnixNano(), i), Type: "invoice.due", CustomerID: cid}
				if i == 1 {
					ev.Type = "account.warning"
				}
				if i == 2 {
					ev.Type = "promo.offer"
				}
				s.ProcessEvent(&ev)
			}
			audience, _ := s.Store.ListOverdueCustomers()
			var entries []campaign.AudienceEntry
			for _, c := range audience {
				entries = append(entries, campaign.AudienceEntry{CustomerID: c.ID, State: campaign.EntryPending})
			}
			cc := s.Campaigns.Create("MCP demo campaign", "invoice.due", nil,
				campaign.GoalSpec{Type: campaign.GoalCount, Target: 5, SuccessOutcomes: []string{"payment_promised"}},
				entries, campaign.WavePolicy{Size: 3, Delay: campaign.Duration{Duration: 15 * time.Second}},
				campaign.Budget{MaxCalls: 15})
			return fmt.Sprintf("demo running: 3 events fired + campaign %s launched (goal: 5 payment promises, budget 15)", cc.ID), nil
		},
	}
	return s.MCP
}
