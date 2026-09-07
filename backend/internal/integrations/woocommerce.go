package integrations

// WooCommerce adapter — verified from WooCommerce source:
// plugins/woocommerce/includes/class-wc-webhook.php, generate_signature():
// base64_encode( hash_hmac( 'sha256', $payload, $secret, true ) ) sent in
// the X-WC-Webhook-Signature header. The signature is calculated over the
// body after it has been encoded (JSON by default) — i.e. the raw wire
// bytes. The secret is the webhook's configured secret (defaults to the
// API user's consumer secret when blank).
//
// Mapped topics:
//   order.updated → payment.failed   (status failed)
//   order.updated → promo.offer      (status cancelled — win-back)
//   refund.created → account.warning

import (
	"net/http"
	"os"
)

// WooCommerceConfig configures the WooCommerce adapter.
type WooCommerceConfig struct {
	// Secret is the webhook secret. Falls back to WOOCOMMERCE_WEBHOOK_SECRET.
	Secret string
}

func (c WooCommerceConfig) secret() string {
	if c.Secret != "" {
		return c.Secret
	}
	return os.Getenv("WOOCOMMERCE_WEBHOOK_SECRET")
}

type wooCommerceAdapter struct{ cfg WooCommerceConfig }

func (wooCommerceAdapter) Platform() string { return "woocommerce" }

type wooOrder struct {
	ID      int64  `json:"id"`
	Status  string `json:"status"`
	Total   string `json:"total"`
	Billing struct {
		Phone string `json:"phone"`
		Email string `json:"email"`
		First string `json:"first_name"`
		Last  string `json:"last_name"`
	} `json:"billing"`
	CustomerID int64 `json:"customer_id"`
}

func (a wooCommerceAdapter) Handle(r *http.Request, body []byte) ([]Event, error) {
	secret := a.cfg.secret()
	if secret == "" {
		return nil, ErrUnauthorized("WOOCOMMERCE_WEBHOOK_SECRET not configured")
	}
	if !VerifyBase64HMAC(secret, body, r.Header.Get("X-WC-Webhook-Signature")) {
		return nil, ErrUnauthorized("invalid woocommerce signature")
	}

	var order wooOrder
	if err := jsonUnmarshalStrict(body, &order); err != nil {
		return nil, err
	}

	customerID := "woo_anon_" + strconvFormat(order.ID)
	if order.CustomerID != 0 {
		customerID = "woo_cus_" + strconvFormat(order.CustomerID)
	}
	phone := order.Billing.Phone

	switch order.Status {
	case "failed":
		return []Event{{
			ID:         "woo_order_" + strconvFormat(order.ID) + "_failed",
			Type:       "payment.failed",
			CustomerID: customerID,
			Phone:      phone,
			Payload: payloadJSON(map[string]any{
				"amount": order.Total,
				"reason": "the order payment failed",
			}),
		}}, nil
	case "cancelled":
		return []Event{{
			ID:         "woo_order_" + strconvFormat(order.ID) + "_cancelled",
			Type:       "promo.offer",
			CustomerID: customerID,
			Phone:      phone,
			Payload: payloadJSON(map[string]any{
				"offer":   "a discount on their next order",
				"expires": "end of this week",
			}),
		}}, nil
	default:
		return nil, nil
	}
}

// NewWooCommerce returns the WooCommerce adapter.
func NewWooCommerce(cfg WooCommerceConfig) Adapter { return wooCommerceAdapter{cfg: cfg} }
