package integrations

// PagerDuty adapter — https://developer.pagerduty.com/docs/webhooks/webhook-v3-overview
//
// Signature: X-PagerDuty-Signature header, HMAC-SHA256 of the raw request
// body with the signing secret configured on the webhook (timing-safe
// compare; computed over the raw body exactly as sent). Every delivery
// also carries X-PagerDuty-Event (event type) and X-PagerDuty-Event-Id.
//
// Mapped events:
//   incident.triggered / incident.escalated (high urgency) → account.warning
// The "customer" is the on-call engineer, resolved from the assignee email
// via the business store (e.g. gh_/slack-style directory entries).

import (
	"net/http"
	"os"
)

// PagerDutyConfig configures the PagerDuty adapter.
type PagerDutyConfig struct {
	// Secret is the webhook signing secret. Falls back to PAGERDUTY_WEBHOOK_SECRET.
	Secret string
}

func (c PagerDutyConfig) secret() string {
	if c.Secret != "" {
		return c.Secret
	}
	return os.Getenv("PAGERDUTY_WEBHOOK_SECRET")
}

type pagerdutyAdapter struct{ cfg PagerDutyConfig }

func (pagerdutyAdapter) Platform() string { return "pagerduty" }

type pagerdutyWebhook struct {
	ID        string `json:"id"`
	CreatedOn string `json:"created_on"`
	EventType string `json:"event_type"`
	Resource  struct {
		Type      string `json:"type"`
		ID        string `json:"id"`
		Title     string `json:"title"`
		Urgency   string `json:"urgency"`
		Status    string `json:"status"`
		HTMLURL   string `json:"html_url"`
		Assignees []struct {
			Assignee struct {
				ID      string `json:"id"`
				Summary string `json:"summary"` // e.g. "Jane Doe (jane@corp.com)"
				Email   string `json:"email"`
				Type    string `json:"type"` // user or team
			} `json:"assignee"`
		} `json:"assignees"`
	} `json:"resource"`
}

func (a pagerdutyAdapter) Handle(r *http.Request, body []byte) ([]Event, error) {
	secret := a.cfg.secret()
	if secret == "" {
		return nil, ErrUnauthorized("PAGERDUTY_WEBHOOK_SECRET not configured")
	}
	sig := r.Header.Get("X-PagerDuty-Signature")
	if sig == "" {
		return nil, ErrUnauthorized("missing X-PagerDuty-Signature")
	}
	if !VerifyHexHMAC(secret, body, sig) {
		return nil, ErrUnauthorized("invalid pagerduty signature")
	}

	var pwr pagerdutyWebhook
	if err := jsonUnmarshalStrict(body, &pwr); err != nil {
		return nil, err
	}

	switch pwr.EventType {
	case "incident.triggered", "incident.escalated", "incident.reopened":
		// high-urgency pages are the escalation events
	default:
		return nil, nil
	}

	inc := pwr.Resource
	customerID := pagerdutyCustomer(inc.Assignees)
	if customerID == "" {
		return nil, nil // no assignee yet — nothing to do
	}

	return []Event{{
		ID:         "pd_" + pwr.ID,
		Type:       "account.warning",
		CustomerID: customerID,
		Payload: payloadJSON(map[string]any{
			"reason":      "PagerDuty incident " + inc.Urgency + " urgency",
			"detail":      inc.Title + " — " + inc.HTMLURL,
			"incident_id": inc.ID,
			"urgency":     inc.Urgency,
		}),
	}}, nil
}

// pagerdutyCustomer picks the first user assignee and normalizes their
// summary/email into a directory customer id.
func pagerdutyCustomer(assignees []struct {
	Assignee struct {
		ID      string `json:"id"`
		Summary string `json:"summary"`
		Email   string `json:"email"`
		Type    string `json:"type"`
	} `json:"assignee"`
}) string {
	for _, a := range assignees {
		if a.Assignee.Type == "user" || a.Assignee.Email != "" {
			if a.Assignee.Email != "" {
				return "pd_" + a.Assignee.Email
			}
			return "pd_" + a.Assignee.ID
		}
	}
	return ""
}

// NewPagerDuty returns the PagerDuty adapter.
func NewPagerDuty(cfg PagerDutyConfig) Adapter { return pagerdutyAdapter{cfg: cfg} }
