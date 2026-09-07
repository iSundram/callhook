package integrations

// Shopify adapter — https://shopify.dev/docs/apps/webhooks
//
// Signature: X-Shopify-Hmac-Sha256 header, base64-encoded HMAC-SHA256 of
// the raw request body with the app's API secret key. (Verified from
// Shopify docs; note the digest is base64, not hex.)
//
// Mapped topics (per integrations/README.md):
//   orders/payment_failure → invoice.due
//   refunds/create         → account.warning
//   orders/cancelled       → promo.offer (win-back)

import (
	"net/http"
	"os"
	"strconv"
)

func strconvFormat(n int64) string { return strconv.FormatInt(n, 10) }

// ShopifyConfig configures the Shopify adapter.
type ShopifyConfig struct {
	// Secret is the API secret key. Falls back to SHOPIFY_API_SECRET.
	Secret string
}

func (c ShopifyConfig) secret() string {
	if c.Secret != "" {
		return c.Secret
	}
	return os.Getenv("SHOPIFY_API_SECRET")
}

type shopifyAdapter struct{ cfg ShopifyConfig }

func (shopifyAdapter) Platform() string { return "shopify" }

type shopifyOrder struct {
	ID              int64  `json:"id"`
	Phone           string `json:"phone"`
	TotalPrice      string `json:"total_price"`
	Currency        string `json:"currency"`
	FinancialStatus string `json:"financial_status"`
	Customer        struct {
		ID    int64  `json:"id"`
		Phone string `json:"phone"`
		Email string `json:"email"`
	} `json:"customer"`
}

func (a shopifyAdapter) Handle(r *http.Request, body []byte) ([]Event, error) {
	secret := a.cfg.secret()
	if secret == "" {
		return nil, ErrNotConfigured("shopify", "SHOPIFY_API_SECRET")
	}
	if !VerifyBase64HMAC(secret, body, r.Header.Get("X-Shopify-Hmac-Sha256")) {
		return nil, ErrUnauthorizedDoc("invalid shopify signature", "/pages/integrations.html#troubleshooting")
	}

	// The webhook topic is in the X-Shopify-Topic header; body is the object.
	topic := r.Header.Get("X-Shopify-Topic")
	var order shopifyOrder
	if err := jsonUnmarshalStrict(body, &order); err != nil {
		return nil, err
	}

	customerID := ""
	if order.Customer.ID != 0 {
		customerID = shopifyCustomerID(order.Customer.ID)
	}
	phone := order.Phone
	if phone == "" {
		phone = order.Customer.Phone
	}

	switch topic {
	case "orders/payment_failure":
		return []Event{{
			ID:         shopifyEventID("order", order.ID),
			Type:       "invoice.due",
			CustomerID: customerID,
			Phone:      phone,
			Payload: payloadJSON(map[string]any{
				"amount":  order.TotalPrice + " " + order.Currency,
				"reason":  "order payment failed",
				"invoice": fmtShopifyOrder(order.ID),
			}),
		}}, nil
	case "refunds/create":
		return []Event{{
			ID:         shopifyEventID("refund", order.ID),
			Type:       "account.warning",
			CustomerID: customerID,
			Phone:      phone,
			Payload: payloadJSON(map[string]any{
				"reason": "refund processing",
				"detail": "Refund status call for order " + fmtShopifyOrder(order.ID),
			}),
		}}, nil
	case "orders/cancelled":
		return []Event{{
			ID:         shopifyEventID("order", order.ID),
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

func shopifyCustomerID(id int64) string { return "shopify_cus_" + itoa(id) }
func shopifyEventID(kind string, id int64) string {
	return "shopify_" + kind + "_" + itoa(id)
}
func fmtShopifyOrder(id int64) string { return "order_" + itoa(id) }

func itoa(n int64) string {
	return strconvFormat(n)
}

// NewShopify returns the Shopify adapter.
func NewShopify(cfg ShopifyConfig) Adapter { return shopifyAdapter{cfg: cfg} }

// moneyFormat renders a float as a 2-decimal string.
func moneyFormat(f float64) string { return strconv.FormatFloat(f, 'f', 2, 64) }
