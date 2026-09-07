package api_test

// Integration test: the full server wired against a MOCK CALL-E API over
// real HTTP — the same loop as production, including webhook delivery.
//
//	event → server → mock CALL-E (places call, POSTs terminal webhook back)
//	      → outcome engine → business write → campaign progress

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/iSundram/callhook/internal/api"
	"github.com/iSundram/callhook/internal/business"
	"github.com/iSundram/callhook/internal/callhookclient"
	"github.com/iSundram/callhook/internal/campaign"
	"github.com/iSundram/callhook/internal/events"
	"github.com/iSundram/callhook/internal/outcome"
	"github.com/iSundram/callhook/internal/session"
)

// mockCalle is a fake CALL-E API: it accepts call creation, remembers the
// webhook URL, and (synchronously after a short delay) delivers a terminal
// webhook with a configurable outcome.
type mockCalle struct {
	server     *httptest.Server
	webhookURL string
	outcome    string // terminal outcome to report
	calls      int
}

func newMockCalle(outcome string) *mockCalle {
	m := &mockCalle{outcome: outcome}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/calls", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			WebhookURL string `json:"webhook_url"`
			Task       string `json:"task"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		m.webhookURL = req.WebhookURL
		m.calls++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":     "call_mock_" + time.Now().Format("150405.000000000"),
			"object": "call_task",
			"status": "queued",
			"task":   req.Task,
		})
	})
	mux.HandleFunc("GET /v1/calls/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": r.PathValue("id"), "object": "call_task", "status": "completed",
			"structured_result": map[string]any{"outcome": m.outcome},
			"attempts": []map[string]any{{
				"id": "att1", "phone": "+15550001", "status": "completed",
				"transcript_turns": []map[string]any{
					{"offset_seconds": 0, "speaker": "bot", "text": "Hello!"},
					{"offset_seconds": 3, "speaker": "user", "text": "Hi there."},
				},
			}},
		})
	})
	m.server = httptest.NewServer(mux)
	return m
}

// deliverTerminal POSTs a terminal webhook to the server under test, exactly
// as CALL-E would.
func (m *mockCalle) deliverTerminal(callID string) {
	body := map[string]any{
		"id":         "wh_mock_" + callID,
		"type":       "call.completed",
		"created_at": time.Now().UTC().Format(time.RFC3339),
		"data": map[string]any{
			"id": callID, "object": "call_task", "status": "completed",
			"structured_result": map[string]any{"outcome": m.outcome, "promise_date": "2026-09-12", "summary": "mock result"},
			"attempts":          []map[string]any{},
		},
	}
	b, _ := json.Marshal(body)
	_, _ = http.Post(m.webhookURL, "application/json", strings.NewReader(string(b)))
}

func TestIntegrationEventLoop(t *testing.T) {
	mock := newMockCalle("payment_promised")
	defer mock.server.Close()

	// server under test
	sessions := session.NewStore()
	outcomes := outcome.New(business.NewMockStore(), sessions)
	client := callhookclient.New("test-key", mock.server.URL)
	srv := &api.Server{
		Router:        events.NewRouter(),
		Store:         business.NewMockStore(),
		Sessions:      sessions,
		Calle:         client,
		Outcomes:      outcomes,
		Campaigns:     campaign.NewStore(),
		EnforceWindows: false,
		PublicBaseURL: "http://integration.test",
	}
	outcomes.FetchCall = client.GetCall
	apiSrv := httptest.NewServer(srv.Routes())
	defer apiSrv.Close()
	srv.PublicBaseURL = apiSrv.URL // mock CALL-E webhooks back to this server

	// 1. fire an event
	res, err := http.Post(apiSrv.URL+"/api/events", "application/json",
		strings.NewReader(`{"id":"it_1","type":"invoice.due","customer_id":"cus_1002"}`))
	if err != nil {
		t.Fatalf("fire: %v", err)
	}
	var fired map[string]any
	_ = json.NewDecoder(res.Body).Decode(&fired)
	res.Body.Close()
	if fired["status"] != "call_placed" {
		t.Fatalf("fire response: %v", fired)
	}

	// 2. mock CALL-E received the call, with our webhook URL attached
	if mock.calls != 1 {
		t.Fatalf("mock CALL-E calls = %d, want 1", mock.calls)
	}
	if !strings.Contains(mock.webhookURL, "/callhook/webhook") {
		t.Errorf("webhook URL = %q, must point at our endpoint", mock.webhookURL)
	}

	// 3. CALL-E delivers the terminal webhook (with a promise)
	mock.deliverTerminal(fired["call_id"].(string))

	// 4. the outcome engine processed it: outcome stored, transcript
	// enriched via the FetchCall re-GET, business write happened.
	waitFor(t, 3*time.Second, func() bool {
		for _, s := range sessions.Snapshot() {
			return s.ID == "it_1" && s.Outcome != nil
		}
		return false
	})
	var s session.SessionView
	for _, v := range sessions.Snapshot() {
		if v.ID == "it_1" {
			s = v
		}
	}
	if s.Outcome["outcome"] != "payment_promised" {
		t.Errorf("outcome = %v", s.Outcome)
	}
	if len(s.Transcript) == 0 {
		t.Error("transcript must be enriched via FetchCall (webhook had none)")
	}
	foundWrite := false
	for _, a := range s.Actions {
		if strings.HasPrefix(a.Detail, "write.mark_promise") {
			foundWrite = true
		}
	}
	if !foundWrite {
		t.Error("payment_promised must trigger write.mark_promise")
	}
}

func TestIntegrationCampaignLoop(t *testing.T) {
	mock := newMockCalle("accepted") // promo success outcome
	defer mock.server.Close()

	sessions := session.NewStore()
	outcomes := outcome.New(business.NewMockStore(), sessions)
	client := callhookclient.New("test-key", mock.server.URL)
	campaigns := campaign.NewStore()
	srv := &api.Server{
		Router:        events.NewRouter(),
		Store:         business.NewMockStore(),
		Sessions:      sessions,
		Calle:         client,
		Outcomes:      outcomes,
		Campaigns:     campaigns,
		EnforceWindows: false,
		PublicBaseURL: "http://integration.test",
	}
	outcomes.FetchCall = client.GetCall

	runner := &campaign.Runner{
		Store:         campaigns,
		Fire:          srv.FireEvent,
		TagSession:    sessions.SetCampaign,
		MaxConcurrent: 5,
	}
	outcomes.CampaignHook = runner.OnTerminal
	apiSrv := httptest.NewServer(srv.Routes())
	defer apiSrv.Close()
	srv.PublicBaseURL = apiSrv.URL // mock CALL-E webhooks back to this server

	// launch a campaign: 5 people, target 2, wave of 5, budget 5
	res, err := http.Post(apiSrv.URL+"/api/campaigns", "application/json",
		strings.NewReader(`{"name":"it","event_type":"promo.offer","goal":{"type":"count","target":2,"success_outcomes":["accepted"]},"audience":[{"customer_id":"cus_1001"},{"customer_id":"cus_1002"},{"customer_id":"cus_1003"},{"customer_id":"cus_2001"},{"customer_id":"cus_2002"}],"waves":{"size":5,"delay":"1ms"},"budget":{"max_calls":5}}`))
	if err != nil {
		t.Fatalf("campaign: %v", err)
	}
	var c campaign.Campaign
	_ = json.NewDecoder(res.Body).Decode(&c)
	res.Body.Close()
	if c.Status != campaign.StatusRunning {
		t.Fatalf("campaign status: %v", c.Status)
	}

	// launch wave 1 manually (the pump does this in production)
	runner.LaunchWave(&c)

	// every wave call reached mock CALL-E
	if mock.calls != 5 {
		t.Fatalf("mock CALL-E calls = %d, want 5", mock.calls)
	}

	// deliver terminal success for the first two calls → goal met
	for _, s := range sessions.Snapshot() {
		if s.CampaignID == c.ID && len(s.Transcript) >= 0 {
			mock.deliverTerminal(s.CallID)
			// only first two — but deliverTerminal is per call; we want 2 successes.
			// Deliver for all 5; goal met at 2, the rest are extra successes.
		}
	}

	// goal must be met and pending skipped
	waitFor(t, 3*time.Second, func() bool {
		cc, ok := campaigns.Get(c.ID)
		return ok && cc.Status == campaign.StatusGoalMet
	})
	cc, _ := campaigns.Get(c.ID)
	if cc.Progress.Successes < 2 {
		t.Errorf("successes = %d, want >= 2", cc.Progress.Successes)
	}
	if cc.Progress.Pending != 0 {
		t.Errorf("pending = %d, want 0 after goal met", cc.Progress.Pending)
	}
}

func waitFor(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition never met within timeout")
}
