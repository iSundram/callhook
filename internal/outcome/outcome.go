// Package outcome maps a terminal CALL-E structured result to concrete
// business actions: writes to the business store, escalation to humans, and
// the outcome webhook POSTed back to the originating system.
package outcome

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/iSundram/calle/internal/business"
	"github.com/iSundram/calle/internal/calleclient"
	"github.com/iSundram/calle/internal/session"
)

// Engine applies outcomes. Writes are policy-gated: confident, unambiguous
// results write to the business store; anything uncertain escalates to a
// human instead of acting.
type Engine struct {
	Store    business.Store
	Sessions *session.Store
	Client   *http.Client
}

func New(st business.Store, sessions *session.Store) *Engine {
	return &Engine{Store: st, Sessions: sessions, Client: &http.Client{Timeout: 10 * time.Second}}
}

// Apply processes one terminal webhook event from CALL-E.
func (e *Engine) Apply(ev *calleclient.WebhookEvent) {
	sess, ok := e.Sessions.FindByCallID(ev.Data.ID)
	if !ok {
		log.Printf("outcome: no session for call %s — ignoring", ev.Data.ID)
		return
	}

	result := ev.Data.StructuredResult
	e.Sessions.SetTerminal(ev.Data.ID, ev.Data.Status, result)
	e.Sessions.Log(sess.ID, "call_terminal", "status="+ev.Data.Status+" type="+ev.Type)

	// The action ladder: unambiguous → write, uncertain → escalate.
	var actions []string
	outcomeVal, _ := result["outcome"].(string)

	switch outcomeVal {
	case "payment_promised":
		if dateStr, ok := result["promise_date"].(string); ok && dateStr != "" {
			if date, err := time.Parse("2006-01-02", dateStr); err == nil {
				if inv, err := e.Store.GetOpenInvoice(sess.Event.CustomerID); err == nil {
					if err := e.Store.MarkPromise(sess.Event.CustomerID, inv.ID, date); err == nil {
						actions = append(actions, "write.mark_promise("+inv.ID+" → "+dateStr+")")
					}
				}
			}
		}
	case "claims_already_paid", "disputed":
		if err := e.Store.Escalate(sess.Event.CustomerID, "invoice "+outcomeVal+" — needs billing review"); err == nil {
			actions = append(actions, "escalate.billing_review")
		}
	case "callback_requested", "needs_human":
		if err := e.Store.Escalate(sess.Event.CustomerID, outcomeVal); err == nil {
			actions = append(actions, "escalate.human_callback")
		}
	case "acknowledged", "activity_confirmed_legitimate", "accepted", "declined", "refused", "no_answer", "unknown", "":
		// informational — no write, no escalation
	}

	channel := "phone"
	outcomeLabel := outcomeVal
	if outcomeLabel == "" {
		outcomeLabel = ev.Data.Status
	}
	_ = e.Store.RecordContact(sess.Event.CustomerID, channel, outcomeLabel)
	actions = append(actions, "audit.record_contact")

	for _, a := range actions {
		e.Sessions.Log(sess.ID, "action", a)
	}

	// Close the loop: POST the structured outcome back to the business.
	if sess.Event.CallbackURL != "" {
		e.postBack(sess, ev, actions)
	}
}

func (e *Engine) postBack(sess *session.Session, ev *calleclient.WebhookEvent, actions []string) {
	payload := map[string]any{
		"event_id":     sess.Event.ID,
		"event_type":   sess.Event.Type,
		"customer_id":  sess.Event.CustomerID,
		"call_id":      ev.Data.ID,
		"call_status":  ev.Data.Status,
		"outcome":      ev.Data.StructuredResult,
		"actions":      actions,
		"confidence":   ev.Data.CompletionConfidence,
		"transcript":   transcriptOf(ev.Data),
		"processed_at": time.Now().UTC(),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("outcome: marshal callback: %v", err)
		return
	}
	resp, err := e.Client.Post(sess.Event.CallbackURL, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("outcome: callback to %s failed: %v", sess.Event.CallbackURL, err)
		e.Sessions.Log(sess.ID, "callback_failed", err.Error())
		return
	}
	resp.Body.Close()
	e.Sessions.Log(sess.ID, "callback_sent", sess.Event.CallbackURL+" → "+resp.Status)
}

func transcriptOf(task calleclient.CallTask) []calleclient.Turn {
	for _, att := range task.Attempts {
		if len(att.TranscriptTurns) > 0 {
			return att.TranscriptTurns
		}
	}
	return nil
}
