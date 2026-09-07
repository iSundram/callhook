package integrations

// Stripe adapter — https://docs.stripe.com/webhooks
//
// Signature: Stripe-Signature header with t= and v1= entries; the signed
// payload is timestamp + "." + raw body, HMAC-SHA256 hex, 5-minute replay
// tolerance. (Verified: docs.stripe.com/webhooks#verify-signature.)
//
// Mapped events (per integrations/README.md):
//   invoice.payment_failed        → invoice.due      (dunning call)
//   invoice.payment_action_required → invoice.due    (3DS needed)
//   customer.subscription.deleted   → account.warning (churn notice)

import (
	"fmt"
	"net/http"
	"os"
)

// StripeConfig is the Stripe adapter configuration.
type StripeConfig struct {
	// Secret is the endpoint signing secret (whsec_...). If empty the
	// STRIPE_WEBHOOK_SECRET env var is read at request time.
	Secret string
}

func (c StripeConfig) secret() string {
	if c.Secret != "" {
		return c.Secret
	}
	return os.Getenv("STRIPE_WEBHOOK_SECRET")
}

type stripeAdapter struct{ cfg StripeConfig }

func (stripeAdapter) Platform() string { return "stripe" }

type stripeEvent struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Data struct {
		Object struct {
			Customer      string `json:"customer"`
			CustomerEmail string `json:"customer_email"`
			CustomerPhone string `json:"customer_phone"`
			AmountDue     int64  `json:"amount_due"`
			AmountPaid    int64  `json:"amount_paid"`
			Currency      string `json:"currency"`
			AttemptCount  int    `json:"attempt_count"`
			InvoiceID     string `json:"invoice"`
		} `json:"object"`
	} `json:"data"`
}

func (a stripeAdapter) Handle(r *http.Request, body []byte) ([]Event, error) {
	secret := a.cfg.secret()
	if secret == "" {
		return nil, ErrNotConfigured("stripe", "STRIPE_WEBHOOK_SECRET")
	}
	if !VerifyStripe(secret, body, r.Header.Get("Stripe-Signature")) {
		return nil, ErrUnauthorizedDoc("invalid stripe signature", "/pages/integrations.html#troubleshooting")
	}

	var se stripeEvent
	if err := jsonUnmarshalStrict(body, &se); err != nil {
		return nil, err
	}

	obj := se.Data.Object
	amount := fmt.Sprintf("%s %.2f", obj.Currency, float64(obj.AmountDue)/100.0)
	customerID := obj.Customer
	if customerID == "" {
		return nil, nil // some events carry no customer — skip quietly
	}

	switch se.Type {
	case "invoice.payment_failed", "invoice.payment_action_required":
		return []Event{{
			ID:         "stripe_" + se.ID,
			Type:       "invoice.due",
			CustomerID: customerID,
			Phone:      obj.CustomerPhone,
			Payload: payloadJSON(map[string]any{
				"amount":  amount,
				"attempt": obj.AttemptCount,
				"reason":  "payment failed",
				"invoice": obj.InvoiceID,
			}),
		}}, nil
	case "customer.subscription.deleted":
		return []Event{{
			ID:         "stripe_" + se.ID,
			Type:       "account.warning",
			CustomerID: customerID,
			Phone:      obj.CustomerPhone,
			Payload: payloadJSON(map[string]any{
				"reason": "subscription cancelled",
				"detail": "Their subscription was deleted; win-back conversation.",
			}),
		}}, nil
	default:
		return nil, nil // not subscribed — 200 fast so Stripe doesn't retry
	}
}

// NewStripe returns the Stripe adapter.
func NewStripe(cfg StripeConfig) Adapter { return stripeAdapter{cfg: cfg} }
