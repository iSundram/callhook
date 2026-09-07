package integrations

// Xero adapter — https://developer.xero.com/documentation/guides/webhooks/overview
//
// Signature: X-Xero-Signature header = base64(HMAC-SHA256(raw body,
// webhook key)), constant-time compare. Xero's "intent to receive"
// handshake: answer 200 for a valid signature, 401 for invalid — Xero
// requires the 200/401 semantics exactly, which our Handler provides.
//
// Mapped events (webhook event list):
//   INVOICE { UPDATE (status OVERDUE) } → invoice.due
//   INVOICE { DELETE }                  → skipped

import (
	"net/http"
	"os"
)

// XeroConfig configures the Xero adapter.
type XeroConfig struct {
	// Key is the webhook signing key. Falls back to XERO_WEBHOOK_KEY.
	Key string
}

func (c XeroConfig) key() string {
	if c.Key != "" {
		return c.Key
	}
	return os.Getenv("XERO_WEBHOOK_KEY")
}

type xeroAdapter struct{ cfg XeroConfig }

func (xeroAdapter) Platform() string { return "xero" }

type xeroWebhook struct {
	Events []struct {
		EventID       string `json:"eventId"`
		EventDate     string `json:"eventDateUtc"`
		EventType     string `json:"eventType"`
		EventCategory string `json:"eventCategory"`
		TenantID      string `json:"tenantId"`
		ResourceID    string `json:"resourceId"`
		Payload       struct {
			Invoice struct {
				InvoiceID     string  `json:"InvoiceID"`
				InvoiceNumber string  `json:"InvoiceNumber"`
				Type          string  `json:"Type"`
				Status        string  `json:"Status"` // OVERDUE is derived; ACCREC + DUE
				Total         float64 `json:"Total"`
				CurrencyCode  string  `json:"CurrencyCode"`
				Contact       struct {
					ContactID    string `json:"ContactID"`
					Name         string `json:"Name"`
					EmailAddress string `json:"EmailAddress"`
				} `json:"Contact"`
			} `json:"Invoice"`
		} `json:"payload"`
	} `json:"events"`
}

func (a xeroAdapter) Handle(r *http.Request, body []byte) ([]Event, error) {
	key := a.cfg.key()
	if key == "" {
		return nil, ErrNotConfigured("xero", "XERO_WEBHOOK_KEY")
	}
	// Xero requires 401 on bad signature, 200 otherwise — matches our Handler.
	if !VerifyBase64HMAC(key, body, r.Header.Get("X-Xero-Signature")) {
		return nil, ErrUnauthorizedDoc("invalid xero signature", "/pages/integrations.html#troubleshooting")
	}

	var xw xeroWebhook
	if err := jsonUnmarshalStrict(body, &xw); err != nil {
		return nil, err
	}

	var events []Event
	for _, e := range xw.Events {
		if e.EventCategory != "INVOICE" || e.EventType != "UPDATE" {
			continue
		}
		inv := e.Payload.Invoice
		if inv.Type != "ACCREC" {
			continue // payables are not customer invoices
		}
		events = append(events, Event{
			ID:         "xero_" + e.EventID,
			Type:       "invoice.due",
			CustomerID: "xero_" + inv.Contact.ContactID,
			Payload: payloadJSON(map[string]any{
				"amount":  currencyAmount(inv.CurrencyCode, inv.Total),
				"invoice": inv.InvoiceNumber,
				"due":     "see Xero",
			}),
		})
	}
	return events, nil
}

func currencyAmount(code string, total float64) string {
	return code + " " + moneyFormat(total)
}

// NewXero returns the Xero adapter.
func NewXero(cfg XeroConfig) Adapter { return xeroAdapter{cfg: cfg} }
