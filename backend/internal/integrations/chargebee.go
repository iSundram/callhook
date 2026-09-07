package integrations

// Chargebee adapter — https://apidocs.chargebee.com/docs/api/events
// (Securing Your Webhook URL)
//
// Verification: HTTP Basic authentication — username and password
// configured in Chargebee's webhook settings ("My webhook URL is
// protected by basic authentication"). Verified from Chargebee's docs;
// Chargebee does NOT HMAC-sign webhooks. Operators should also note the
// optional random key path segment Chargebee supports.
//
// Mapped events:
//   payment_failed       → payment.failed
//   invoice_payment_failed → payment.failed
//   subscription_cancelled → account.warning (churn notice)

import (
	"encoding/base64"
	"net/http"
	"os"
	"strings"
)

// ChargebeeConfig configures the Chargebee adapter.
type ChargebeeConfig struct {
	// User/Pass are the basic-auth credentials from the webhook settings.
	// Fall back to CHARGEBEE_WEBHOOK_USER / CHARGEBEE_WEBHOOK_PASSWORD.
	User string
	Pass string
}

func (c ChargebeeConfig) user() string {
	if c.User != "" {
		return c.User
	}
	return os.Getenv("CHARGEBEE_WEBHOOK_USER")
}

func (c ChargebeeConfig) pass() string {
	if c.Pass != "" {
		return c.Pass
	}
	return os.Getenv("CHARGEBEE_WEBHOOK_PASSWORD")
}

type chargebeeAdapter struct{ cfg ChargebeeConfig }

func (chargebeeAdapter) Platform() string { return "chargebee" }

type chargebeeWebhook struct {
	ID         string `json:"id"` // event id, e.g. "ev_..."
	EventType  string `json:"event_type"`
	OccurredAt int64  `json:"occurred_at"`
	Content    struct {
		Customer struct {
			ID    string `json:"id"`
			Email string `json:"email"`
			Phone string `json:"phone"`
		} `json:"customer"`
		Invoice struct {
			ID           string `json:"id"`
			Total        int64  `json:"total"`
			CurrencyCode string `json:"currency_code"`
		} `json:"invoice"`
	} `json:"content"`
}

func (a chargebeeAdapter) Handle(r *http.Request, body []byte) ([]Event, error) {
	wantUser, wantPass := a.cfg.user(), a.cfg.pass()
	if wantUser == "" || wantPass == "" {
		return nil, ErrNotConfigured("chargebee", "CHARGEBEE_WEBHOOK_USER/PASSWORD")
	}

	// Verify HTTP Basic auth (Authorization: Basic base64(user:pass)).
	gotUser, gotPass, ok := parseBasicAuth(r.Header.Get("Authorization"))
	if !ok || !VerifyBasic(gotUser, gotPass, wantUser, wantPass) {
		return nil, ErrUnauthorizedDoc("invalid chargebee basic auth", "/pages/integrations.html#troubleshooting")
	}

	var cw chargebeeWebhook
	if err := jsonUnmarshalStrict(body, &cw); err != nil {
		return nil, err
	}

	content := cw.Content
	if content.Customer.ID == "" {
		return nil, nil
	}
	amount := content.Invoice.CurrencyCode + " " + moneyFormat(float64(content.Invoice.Total)/100.0)

	switch cw.EventType {
	case "payment_failed", "invoice_payment_failed":
		return []Event{{
			ID:         "chargebee_" + cw.ID,
			Type:       "payment.failed",
			CustomerID: "chargebee_" + content.Customer.ID,
			Phone:      content.Customer.Phone,
			Payload: payloadJSON(map[string]any{
				"amount":  amount,
				"reason":  "the Chargebee payment failed",
				"invoice": content.Invoice.ID,
			}),
		}}, nil
	case "subscription_cancelled":
		return []Event{{
			ID:         "chargebee_" + cw.ID,
			Type:       "account.warning",
			CustomerID: "chargebee_" + content.Customer.ID,
			Phone:      content.Customer.Phone,
			Payload: payloadJSON(map[string]any{
				"reason": "subscription cancelled",
				"detail": "They cancelled; offer a win-back.",
			}),
		}}, nil
	default:
		return nil, nil
	}
}

// parseBasicAuth decodes an Authorization: Basic header.
func parseBasicAuth(header string) (user, pass string, ok bool) {
	const prefix = "Basic "
	if !strings.HasPrefix(header, prefix) {
		return "", "", false
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(header, prefix))
	if err != nil {
		return "", "", false
	}
	parts := strings.SplitN(string(raw), ":", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// NewChargebee returns the Chargebee adapter.
func NewChargebee(cfg ChargebeeConfig) Adapter { return chargebeeAdapter{cfg: cfg} }
