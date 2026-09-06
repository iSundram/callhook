package outcome

import (
	"testing"
	"time"

	"github.com/iSundram/calle/internal/business"
	"github.com/iSundram/calle/internal/calleclient"
	"github.com/iSundram/calle/internal/events"
	"github.com/iSundram/calle/internal/session"
)

func newTestEngine() (*Engine, *session.Store) {
	sessions := session.NewStore()
	return New(business.NewMockStore(), sessions), sessions
}

func placeCall(t *testing.T, sessions *session.Store, id string) {
	t.Helper()
	ev := events.Event{ID: id, Type: "invoice.due", CustomerID: "cus_1002"}
	sessions.Create(id, ev, &business.Customer{ID: "cus_1002", Name: "Daniel", Region: "US"})
	sessions.SetCallID(id, "call_"+id)
}

func terminalEvent(callID, outcome string) *calleclient.WebhookEvent {
	return &calleclient.WebhookEvent{
		ID:   "wh_" + callID,
		Type: "call.completed",
		Data: calleclient.CallTask{
			ID: callID, Status: "completed",
			StructuredResult: map[string]any{"outcome": outcome},
		},
	}
}

func TestPaymentPromiseWrites(t *testing.T) {
	e, sessions := newTestEngine()
	placeCall(t, sessions, "evt_p1")
	ev := terminalEvent("call_evt_p1", "payment_promised")
	ev.Data.StructuredResult["promise_date"] = "2026-09-12"
	e.Apply(ev)

	sess, ok := sessions.Get("evt_p1")
	if !ok {
		t.Fatal("session missing")
	}
	found := false
	for _, a := range sess.Actions {
		if a.Kind == "action" && len(a.Detail) > 5 && a.Detail[:5] == "write" {
			found = true
		}
	}
	if !found {
		t.Error("payment_promised should produce a write action")
	}
}

func TestNoAnswerSchedulesRetry(t *testing.T) {
	e, sessions := newTestEngine()
	placeCall(t, sessions, "evt_na")
	e.Apply(terminalEvent("call_evt_na", "no_answer"))

	sess, _ := sessions.Get("evt_na")
	if sess.NextRetryAt == nil || sess.NextRetryKind != session.KindRetry {
		t.Error("no_answer should arm a retry trigger")
	}
}

func TestRefusalNeverRetries(t *testing.T) {
	e, sessions := newTestEngine()
	placeCall(t, sessions, "evt_ref")
	e.Apply(terminalEvent("call_evt_ref", "refused"))

	sess, _ := sessions.Get("evt_ref")
	if sess.NextRetryAt != nil {
		t.Error("an explicit refusal must never be redialed")
	}
}

func TestFailureCodeBlocksRetry(t *testing.T) {
	e, sessions := newTestEngine()
	placeCall(t, sessions, "evt_blk")
	ev := terminalEvent("call_evt_blk", "")
	ev.Data.Status = "failed"
	ev.Data.FailureCode = "invalid_phone"
	e.Apply(ev)

	sess, _ := sessions.Get("evt_blk")
	if sess.NextRetryAt != nil {
		t.Error("invalid_phone must not be retried")
	}
}

func TestTransientFailureRetries(t *testing.T) {
	e, sessions := newTestEngine()
	placeCall(t, sessions, "evt_prov")
	ev := terminalEvent("call_evt_prov", "")
	ev.Data.Status = "failed"
	ev.Data.FailureCode = "provider_unavailable"
	e.Apply(ev)

	sess, _ := sessions.Get("evt_prov")
	if sess.NextRetryAt == nil {
		t.Error("trans provider failure should arm a retry")
	}
}

func TestRetryExhaustionStops(t *testing.T) {
	e, sessions := newTestEngine()
	placeCall(t, sessions, "evt_max")
	// Burn all attempts.
	e.Apply(terminalEvent("call_evt_max", "no_answer"))
	for i := 0; i < session.MaxRetries; i++ {
		sessions.IncrRetry("evt_max")
		sessions.Log("evt_max", "retry_fired", "test")
		placeCallID := "call_evt_max_r" + time.Now().Format("150405.000000000")
		sessions.SetCallID("evt_max", placeCallID)
		e.Apply(terminalEvent(placeCallID, "no_answer"))
	}
	sess, _ := sessions.Get("evt_max")
	if sess.NextRetryAt != nil {
		t.Error("retries must stop after MaxRetries")
	}
}
