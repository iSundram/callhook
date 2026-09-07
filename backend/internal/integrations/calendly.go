package integrations

// Calendly adapter — https://developer.calendly.com/api-docs/overview/webhooks/webhook-signatures
//
// Signature: Calendly-Webhook-Signature header (no X- prefix) with
// t=UNIX,v1=HEX entries; signed payload = timestamp + "." + raw JSON body,
// HMAC-SHA256 hex; reject stale timestamps (~3 min tolerance recommended,
// we allow 5). (Verified from Calendly's webhook-signatures page.)
//
// Mapped events:
//   invitee.no_show      → appointment.reminder (reschedule)
//   invitee.canceled     → appointment.reminder (confirm new time)
//   invitee.created      → appointment.reminder (confirmation)

import (
	"net/http"
	"os"
)

// CalendlyConfig configures the Calendly adapter.
type CalendlyConfig struct {
	// Secret is the webhook signing key. Falls back to CALENDLY_WEBHOOK_SECRET.
	Secret string
}

func (c CalendlyConfig) secret() string {
	if c.Secret != "" {
		return c.Secret
	}
	return os.Getenv("CALENDLY_WEBHOOK_SECRET")
}

type calendlyAdapter struct{ cfg CalendlyConfig }

func (calendlyAdapter) Platform() string { return "calendly" }

type calendlyWebhook struct {
	Event   string `json:"event"` // invitee.created etc.
	Created string `json:"created_at"`
	Payload struct {
		Email               string `json:"email"`
		Name                string `json:"name"`
		URI                 string `json:"uri"` // invitee URI, ends with invitee id
		TextReminderNumber  string `json:"text_reminder_number"`
		QuestionsAndAnswers []struct {
			Question string `json:"question"`
			Answer   string `json:"answer"`
		} `json:"questions_and_answers"`
		ScheduledEvent struct {
			StartTime string `json:"start_time"`
			URI       string `json:"uri"`
		} `json:"scheduled_event"`
		EventType struct {
			Name string `json:"name"`
		} `json:"event_type"`
	} `json:"payload"`
}

func (a calendlyAdapter) Handle(r *http.Request, body []byte) ([]Event, error) {
	secret := a.cfg.secret()
	if secret == "" {
		return nil, ErrUnauthorized("CALENDLY_WEBHOOK_SECRET not configured")
	}
	if !VerifyCalendly(secret, body, r.Header.Get("Calendly-Webhook-Signature")) {
		return nil, ErrUnauthorized("invalid calendly signature")
	}

	var cw calendlyWebhook
	if err := jsonUnmarshalStrict(body, &cw); err != nil {
		return nil, err
	}

	p := cw.Payload
	if p.URI == "" {
		return nil, nil
	}
	// invitee URI: https://api.calendly.com/invitees/XXXX — take the tail.
	inviteeID := lastPathSegment(p.URI)
	when := shortWhen(p.ScheduledEvent.StartTime)

	var detail string
	switch cw.Event {
	case "invitee.no_show":
		detail = "They missed their " + p.EventType.Name + " — offer a new time."
	case "invitee.canceled":
		detail = "They cancelled their " + p.EventType.Name + " — offer a new time."
	case "invitee.created":
		detail = "Confirm their upcoming " + p.EventType.Name + "."
	default:
		return nil, nil
	}

	return []Event{{
		ID:         "calendly_" + inviteeID + "_" + sanitizeEvent(cw.Event),
		Type:       "appointment.reminder",
		CustomerID: "calendly_" + inviteeID,
		Phone:      p.TextReminderNumber,
		Payload: payloadJSON(map[string]any{
			"what":      p.EventType.Name,
			"when":      when,
			"invitee":   inviteeID,
			"situation": detail,
			"email":     p.Email,
		}),
	}}, nil
}

func lastPathSegment(uri string) string {
	for i := len(uri) - 1; i >= 0; i-- {
		if uri[i] == '/' {
			return uri[i+1:]
		}
	}
	return uri
}

func sanitizeEvent(e string) string {
	out := make([]rune, 0, len(e))
	for _, r := range e {
		if r == '.' || r == '_' {
			out = append(out, '_')
		}
	}
	return string(out)
}

// shortWhen trims an RFC3339 timestamp to date + HH:MM for the voice task.
func shortWhen(ts string) string {
	if len(ts) >= 16 && ts[4] == '-' {
		return ts[:16] + "Z"
	}
	return ts
}

// NewCalendly returns the Calendly adapter.
func NewCalendly(cfg CalendlyConfig) Adapter { return calendlyAdapter{cfg: cfg} }
