package integrations

// QuickBooks Online adapter — https://developer.intuit.com/app/developer/qbo/docs/develop/webhooks
//
// Signature: X-Intuit-Signature header, hex HMAC-SHA256 of the raw body
// with the webhook's verifier token (from the webhook settings in the
// Intuit Developer portal). The header may carry multiple
// comma-separated signatures; any match validates. Notifications are
// one-shot: respond 2xx fast and let the idempotency layer dedupe.
//
// Mapped events:
//   Invoice (update) → invoice.due
//   Payment (create) → skipped (positive signal, no call)
//   Customer (update) → skipped

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
)

// QuickBooksConfig configures the QuickBooks adapter.
type QuickBooksConfig struct {
	// Token is the verifier token. Falls back to QUICKBOOKS_VERIFIER_TOKEN.
	Token string
}

func (c QuickBooksConfig) token() string {
	if c.Token != "" {
		return c.Token
	}
	return os.Getenv("QUICKBOOKS_VERIFIER_TOKEN")
}

type quickbooksAdapter struct{ cfg QuickBooksConfig }

func (quickbooksAdapter) Platform() string { return "quickbooks" }

type qboNotification struct {
	EventNotifications []struct {
		RealmID         string `json:"realmId"`
		DataChangeEvent struct {
			Entities []struct {
				Name        string      `json:"name"`
				ID          json.Number `json:"id"`
				Operation   string      `json:"operation"`
				LastUpdated string      `json:"lastUpdated"`
			} `json:"entities"`
		} `json:"dataChangeEvent"`
	} `json:"eventNotifications"`
}

func (a quickbooksAdapter) Handle(r *http.Request, body []byte) ([]Event, error) {
	token := a.cfg.token()
	if token == "" {
		return nil, ErrNotConfigured("quickbooks", "QUICKBOOKS_VERIFIER_TOKEN")
	}
	header := r.Header.Get("X-Intuit-Signature")
	if header == "" {
		return nil, ErrUnauthorized("missing X-Intuit-Signature")
	}
	// Multiple signatures can be listed; any one matching validates.
	if !anyVerifyHexHMAC(token, body, header) {
		return nil, ErrUnauthorizedDoc("invalid intuit signature", "/pages/integrations.html#troubleshooting")
	}

	var qn qboNotification
	if err := jsonUnmarshalStrict(body, &qn); err != nil {
		return nil, err
	}

	var events []Event
	for _, notif := range qn.EventNotifications {
		for _, ent := range notif.DataChangeEvent.Entities {
			if ent.Name != "Invoice" || ent.Operation != "Update" {
				continue
			}
			events = append(events, Event{
				ID:         "qbo_" + notif.RealmID + "_inv_" + ent.ID.String(),
				Type:       "invoice.due",
				CustomerID: "qbo_" + notif.RealmID + "_inv_" + ent.ID.String(),
				Payload: payloadJSON(map[string]any{
					"invoice": ent.ID.String(),
					"realm":   notif.RealmID,
					"reason":  "invoice updated in QuickBooks",
				}),
			})
		}
	}
	return events, nil
}

// anyVerifyHexHMAC checks a comma-separated list of hex signatures.
func anyVerifyHexHMAC(secret string, body []byte, header string) bool {
	for _, sig := range strings.Split(header, ",") {
		if VerifyHexHMAC(secret, body, sig) {
			return true
		}
	}
	return false
}

// NewQuickBooks returns the QuickBooks Online adapter.
func NewQuickBooks(cfg QuickBooksConfig) Adapter { return quickbooksAdapter{cfg: cfg} }
