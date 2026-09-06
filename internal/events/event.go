package events

import (
	"encoding/json"
	"fmt"
	"time"
)

// Event is the universal inbound payload. Any business system fires one of
// these at POST /api/events and calle handles all phone communication.
type Event struct {
	ID           string          `json:"id"`
	Type         string          `json:"type"`             // e.g. invoice.due, account.warning, promo.offer
	CustomerID   string          `json:"customer_id"`      // binds the call session to one customer
	Phone        string          `json:"phone"`            // E.164, optional if resolvable from customer
	Payload      json.RawMessage `json:"payload"`          // event-type-specific data
	CallbackURL  string          `json:"callback_url"`     // where the structured outcome is POSTed back
	Idempotency  string          `json:"idempotency_key"`  // dedupe key
	ReceivedAt   time.Time       `json:"received_at"`
}

func (e *Event) Validate() error {
	if e.Type == "" {
		return fmt.Errorf("type is required")
	}
	if e.CustomerID == "" {
		return fmt.Errorf("customer_id is required")
	}
	if e.ID == "" {
		return fmt.Errorf("id is required")
	}
	return nil
}

// DecodePayload unmarshals the event-type-specific payload into dst.
func (e *Event) DecodePayload(dst any) error {
	if len(e.Payload) == 0 {
		return nil
	}
	return json.Unmarshal(e.Payload, dst)
}
