package integrations

// Paddle (Billing) adapter — https://developer.paddle.com/webhooks/about/signature-verification
//
// Signature: Paddle-Signature header "ts=UNIX;h1=HEX" — the signed payload
// is "timestamp:rawBody", HMAC-SHA256 hex with the endpoint secret key.
// Paddle's SDK default tolerance is 5 seconds; we accept a configurable
// lenient window for clock skew (default 60s). (Verified from Paddle's
// signature-verification page.)
//
// Mapped events:
//   transaction.payment_failed → payment.failed
//   customer.subscription.canceled → account.warning (churn notice)
//   transaction.ready          → skipped

import (
	"fmt"
	"net/http"
	"os"
	"time"
)

// PaddleConfig configures the Paddle adapter.
type PaddleConfig struct {
	// Secret is the endpoint secret key. Falls back to PADDLE_WEBHOOK_SECRET.
	Secret string
	// Tolerance widens the 5s default replay window (clock skew). 0 → 60s.
	Tolerance time.Duration
}

func (c PaddleConfig) secret() string {
	if c.Secret != "" {
		return c.Secret
	}
	return os.Getenv("PADDLE_WEBHOOK_SECRET")
}

func (c PaddleConfig) tolerance() time.Duration {
	if c.Tolerance > 0 {
		return c.Tolerance
	}
	return 60 * time.Second
}

type paddleAdapter struct{ cfg PaddleConfig }

func (paddleAdapter) Platform() string { return "paddle" }

type paddleWebhook struct {
	EventID   string `json:"event_id"`   // e.g. evt_01h...
	EventType string `json:"event_type"` // e.g. transaction.payment_failed
	Data      struct {
		CustomerID  string `json:"customer_id"`
		Transaction struct {
			ID             string `json:"id"`
			BillingDetails struct {
				Totals []struct {
					Total        string `json:"total"`
					CurrencyCode string `json:"currency_code"`
				} `json:"totals"`
			} `json:"billing_details"`
		} `json:"transaction"`
	} `json:"data"`
}

func (a paddleAdapter) Handle(r *http.Request, body []byte) ([]Event, error) {
	secret := a.cfg.secret()
	if secret == "" {
		return nil, ErrUnauthorized("PADDLE_WEBHOOK_SECRET not configured")
	}
	if !VerifyPaddle(secret, body, r.Header.Get("Paddle-Signature"), a.cfg.tolerance()) {
		return nil, ErrUnauthorized("invalid paddle signature")
	}

	var pw paddleWebhook
	if err := jsonUnmarshalStrict(body, &pw); err != nil {
		return nil, err
	}
	if pw.Data.CustomerID == "" {
		return nil, nil
	}

	amount := ""
	for _, t := range pw.Data.Transaction.BillingDetails.Totals {
		if t.CurrencyCode != "" {
			amount = fmt.Sprintf("%s %s", t.CurrencyCode, t.Total)
			break
		}
	}

	switch pw.EventType {
	case "transaction.payment_failed":
		return []Event{{
			ID:         "paddle_" + pw.EventID,
			Type:       "payment.failed",
			CustomerID: "paddle_" + pw.Data.CustomerID,
			Payload: payloadJSON(map[string]any{
				"amount":      amount,
				"reason":      "the Paddle transaction failed",
				"transaction": pw.Data.Transaction.ID,
			}),
		}}, nil
	case "customer.subscription.canceled":
		return []Event{{
			ID:         "paddle_" + pw.EventID,
			Type:       "account.warning",
			CustomerID: "paddle_" + pw.Data.CustomerID,
			Payload: payloadJSON(map[string]any{
				"reason": "subscription cancelled",
				"detail": "Their subscription was cancelled; offer a win-back.",
			}),
		}}, nil
	default:
		return nil, nil
	}
}

// NewPaddle returns the Paddle Billing adapter.
func NewPaddle(cfg PaddleConfig) Adapter { return paddleAdapter{cfg: cfg} }
