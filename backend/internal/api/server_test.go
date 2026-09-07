package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/iSundram/callhook/internal/business"
	"github.com/iSundram/callhook/internal/callhookclient"
	"github.com/iSundram/callhook/internal/campaign"
	"github.com/iSundram/callhook/internal/events"
	"github.com/iSundram/callhook/internal/outcome"
	"github.com/iSundram/callhook/internal/session"
)

// newTestServer builds a full API server in dry-run mode.
func newTestServer(token string) *Server {
	sessions := session.NewStore()
	outcomes := outcome.New(business.NewMockStore(), sessions)
	srv := &Server{
		Router:     events.NewRouter(),
		Store:      business.NewMockStore(),
		Sessions:   sessions,
		Calle:      &callhookclient.Client{DryRun: true},
		Outcomes:   outcomes,
		Campaigns:  campaign.NewStore(),
		EnforceWindows: false,
		IntakeToken: token,
	}
	return srv
}

func post(srv *Server, path, body, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	srv.Routes().ServeHTTP(w, req)
	return w
}

func get(srv *Server, path, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	srv.Routes().ServeHTTP(w, req)
	return w
}

func TestEventHappyPath(t *testing.T) {
	srv := newTestServer("")
	w := post(srv, "/api/events", `{"id":"t1","type":"invoice.due","customer_id":"cus_1002"}`, "")
	if w.Code != http.StatusAccepted {
		t.Fatalf("code = %d, want 202; body: %s", w.Code, w.Body)
	}
	var res map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	if res["status"] != "call_placed" {
		t.Errorf("status = %v, want call_placed", res["status"])
	}
}

func TestEventIdempotentReplay(t *testing.T) {
	srv := newTestServer("")
	body := `{"id":"t1","type":"invoice.due","customer_id":"cus_1002"}`
	_ = post(srv, "/api/events", body, "")
	w := post(srv, "/api/events", body, "")
	if w.Code != http.StatusOK {
		t.Fatalf("replay code = %d, want 200", w.Code)
	}
	var res map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	if res["status"] != "duplicate" {
		t.Errorf("replay status = %v, want duplicate", res["status"])
	}
	// Only one session must exist.
	if got := len(srv.Sessions.Snapshot()); got != 1 {
		t.Errorf("sessions = %d, want 1 (never double-dial)", got)
	}
}

func TestEventBadTypeRejected(t *testing.T) {
	srv := newTestServer("")
	w := post(srv, "/api/events", `{"id":"t1","type":"nope.nope","customer_id":"cus_1002"}`, "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400", w.Code)
	}
}

func TestEventUnknownCustomer(t *testing.T) {
	srv := newTestServer("")
	w := post(srv, "/api/events", `{"id":"t1","type":"invoice.due","customer_id":"ghost"}`, "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("code = %d, want 404 (unknown customer)", w.Code)
	}
}

func TestEventInvalidPhone(t *testing.T) {
	srv := newTestServer("")
	w := post(srv, "/api/events", `{"id":"t1","type":"invoice.due","customer_id":"cus_1002","phone":"555"}`, "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400 (non-E.164 phone)", w.Code)
	}
}

func TestBatchMixedResults(t *testing.T) {
	srv := newTestServer("")
	w := post(srv, "/api/events/batch",
		`{"events":[{"id":"b1","type":"invoice.due","customer_id":"cus_1002"},{"id":"b2","type":"nope","customer_id":"cus_1002"}]}`, "")
	if w.Code != http.StatusOK {
		t.Fatalf("batch code = %d, want 200 (one good event is enough)", w.Code)
	}
	var res struct {
		Results []map[string]any `json:"results"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	if len(res.Results) != 2 {
		t.Fatalf("results = %d, want 2", len(res.Results))
	}
	if res.Results[0]["status"] != "call_placed" || res.Results[1]["error"] == nil {
		t.Errorf("batch results: %+v", res.Results)
	}
}

func TestAuthRequired(t *testing.T) {
	srv := newTestServer("sekrit")
	// no token → 401
	if w := get(srv, "/api/sessions", ""); w.Code != http.StatusUnauthorized {
		t.Errorf("no token: %d, want 401", w.Code)
	}
	// wrong token → 401
	if w := get(srv, "/api/sessions", "wrong"); w.Code != http.StatusUnauthorized {
		t.Errorf("wrong token: %d, want 401", w.Code)
	}
	// right token → 200
	if w := get(srv, "/api/sessions", "sekrit"); w.Code != http.StatusOK {
		t.Errorf("right token: %d, want 200", w.Code)
	}
	// health stays open (connect probe)
	if w := get(srv, "/api/health", ""); w.Code != http.StatusOK {
		t.Errorf("health open: %d, want 200", w.Code)
	}
}

func TestRateLimit(t *testing.T) {
	srv := newTestServer("")
	// default limiter: 60/min — hammer it
	var last int
	for i := 0; i < 65; i++ {
		last = post(srv, "/api/events", `{"id":"rl","type":"nope","customer_id":"x"}`, "").Code
	}
	if last != http.StatusTooManyRequests {
		t.Errorf("65th request: %d, want 429", last)
	}
}

func TestCORSPreflight(t *testing.T) {
	srv := newTestServer("")
	req := httptest.NewRequest(http.MethodOptions, "/api/sessions", nil)
	w := httptest.NewRecorder()
	srv.Routes().ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("preflight code = %d, want 204", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("missing CORS header")
	}
}

func TestCampaignLifecycle(t *testing.T) {
	srv := newTestServer("")
	w := post(srv, "/api/campaigns",
		`{"name":"t","event_type":"promo.offer","goal":{"type":"count","target":2,"success_outcomes":["accepted"]},"audience":[{"customer_id":"cus_1001"},{"customer_id":"cus_1002"},{"customer_id":"cus_1003"}],"waves":{"size":2,"delay":"1ms"},"budget":{"max_calls":4}}`, "")
	if w.Code != http.StatusCreated {
		t.Fatalf("create: %d body=%s", w.Code, w.Body)
	}
	var c campaign.Campaign
	_ = json.Unmarshal(w.Body.Bytes(), &c)
	if c.Status != campaign.StatusRunning || len(c.Audience) != 3 {
		t.Fatalf("campaign: %+v", c)
	}

	// GET list + by id
	if w := get(srv, "/api/campaigns", ""); w.Code != http.StatusOK {
		t.Errorf("list: %d", w.Code)
	}
	if w := get(srv, "/api/campaigns/"+c.ID, ""); w.Code != http.StatusOK {
		t.Errorf("get: %d", w.Code)
	}
	// unknown id
	if w := get(srv, "/api/campaigns/nope", ""); w.Code != http.StatusNotFound {
		t.Errorf("unknown: %d, want 404", w.Code)
	}
	// stop
	if w := post(srv, "/api/campaigns/"+c.ID+"/stop", "", ""); w.Code != http.StatusOK {
		t.Errorf("stop: %d", w.Code)
	}
	// second stop fails (not running)
	if w := post(srv, "/api/campaigns/"+c.ID+"/stop", "", ""); w.Code != http.StatusNotFound {
		t.Errorf("double stop: %d, want 404", w.Code)
	}
}

func TestWebhookDedup(t *testing.T) {
	srv := newTestServer("")
	_ = post(srv, "/api/events", `{"id":"wh1","type":"invoice.due","customer_id":"cus_1002"}`, "")
	var callID string
	for _, s := range srv.Sessions.Snapshot() {
		callID = s.CallID
	}
	if callID == "" {
		t.Fatal("no call placed (dry-run should place one)")
	}
	wh := `{"id":"evt_wh_1","type":"call.completed","data":{"id":"` + callID + `","object":"call_task","status":"completed","structured_result":{"outcome":"payment_promised","promise_date":"2026-09-12"},"attempts":[]}}`
	// deliver the same event twice — must apply exactly once
	_ = post(srv, "/callhook/webhook", wh, "")
	_ = post(srv, "/callhook/webhook", wh, "")

	// Apply is async — poll for it
	for i := 0; i < 100; i++ {
		for _, s := range srv.Sessions.Snapshot() {
			if s.ID == "wh1" && s.Outcome != nil {
				n := 0
				for _, a := range s.Actions {
					if a.Kind == "call_terminal" {
						n++
					}
				}
				if n != 1 {
					t.Errorf("call_terminal count = %d, want 1 (event-id dedup)", n)
				}
				if s.Outcome["outcome"] != "payment_promised" {
					t.Errorf("outcome = %v", s.Outcome)
				}
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("webhook outcome never applied")
}
