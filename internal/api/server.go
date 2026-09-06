package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/iSundram/calle/internal/business"
	"github.com/iSundram/calle/internal/calleclient"
	"github.com/iSundram/calle/internal/events"
	"github.com/iSundram/calle/internal/outcome"
	"github.com/iSundram/calle/internal/session"
)

// Server wires everything together: event intake, CALL-E client, outcome
// engine, session registry, and the live dashboard.
type Server struct {
	Router   *events.Router
	Store    business.Store
	Sessions *session.Store
	Calle    *calleclient.Client
	Outcomes *outcome.Engine
	// PublicBaseURL is where CALL-E can reach our webhook (e.g. an ngrok URL).
	PublicBaseURL string
}

func (s *Server) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/events", s.handleEvent)
	mux.HandleFunc("POST /calle/webhook", s.handleCalleWebhook)
	mux.HandleFunc("GET /api/sessions", s.handleSessions)
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "dry_run": s.Calle.DryRun})
	})
	mux.HandleFunc("GET /", s.handleDashboard)
	return mux
}

// handleEvent is THE product: a business system POSTs an event, and calle
// takes over all phone communication for it.
func (s *Server) handleEvent(w http.ResponseWriter, r *http.Request) {
	var ev events.Event
	if err := json.NewDecoder(r.Body).Decode(&ev); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json: " + err.Error()})
		return
	}
	ev.ReceivedAt = time.Now().UTC()

	if err := ev.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// Idempotency: the same event must never trigger two calls.
	key := ev.Idempotency
	if key == "" {
		key = ev.ID
	}
	if s.Sessions.AlreadySeen(key) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "duplicate", "session_id": ev.ID})
		return
	}

	blueprint, ok := s.Router.Blueprint(ev.Type)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":              "unsupported event type: " + ev.Type,
			"supported_types":    s.Router.Supported(),
		})
		return
	}

	// Prefetch: resolve the full customer record BEFORE dialing, so the call
	// task contains live business data and mid-call lookups hit cache.
	customer, err := s.Store.GetCustomer(ev.CustomerID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	phone := ev.Phone
	if phone == "" {
		phone = customer.Phone
	}
	if !strings.HasPrefix(phone, "+") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "phone must be E.164 format (start with +)"})
		return
	}

	task, err := blueprint.Compose(customer, &ev, s.Store)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	sess := s.Sessions.Create(ev.ID, ev, customer)
	s.Sessions.Log(sess.ID, "event_received", ev.Type+" for "+customer.Name)
	s.Sessions.Log(sess.ID, "prefetched", "customer="+customer.ID+" invoice/payment history loaded")
	s.Sessions.SetCallTarget(sess.ID, phone, customer.Locale, customer.Region)
	s.Sessions.SetResultSchema(sess.ID, blueprint.ResultSchema)
	s.Sessions.SetTask(sess.ID, task)

	call, err := s.PlaceCall(sess)
	if err != nil {
		s.Sessions.Log(sess.ID, "call_failed", err.Error())
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "calle: " + err.Error()})
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"status":     "call_placed",
		"session_id": sess.ID,
		"call_id":    call.ID,
		"phone":      phone,
		"dry_run":    s.Calle.DryRun,
	})
}

// PlaceCall places (or re-places, on retry) the call for a session. Shared
// by the intake handler and the retry scheduler. Each attempt gets its own
// idempotency key so a crash mid-retry never double-dials.
func (s *Server) PlaceCall(sess *session.Session) (*calleclient.CallTask, error) {
	attempt := sess.RetryCount + 1
	callReq := &calleclient.CreateCallRequest{
		Task: sess.Task,
		Recipients: []calleclient.Recipient{{
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
		callReq.WebhookURL = strings.TrimSuffix(s.PublicBaseURL, "/") + "/calle/webhook"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	call, err := s.Calle.CreateCall(ctx, callReq, fmt.Sprintf("%s-a%d", sess.ID, attempt))
	if err != nil {
		return nil, err
	}
	s.Sessions.SetCall(sess.ID, call)
	if attempt > 1 {
		s.Sessions.Log(sess.ID, "call_placed", fmt.Sprintf("%s → %s (retry attempt %d/%d)", call.ID, sess.Phone, attempt, session.MaxRetries+1))
	} else {
		s.Sessions.Log(sess.ID, "call_placed", call.ID+" → "+sess.Phone)
	}
	return call, nil
}

// handleCalleWebhook receives terminal call results from CALL-E.
func (s *Server) handleCalleWebhook(w http.ResponseWriter, r *http.Request) {
	var ev calleclient.WebhookEvent
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
