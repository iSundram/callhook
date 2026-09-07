package integrations

// HubSpot adapter — https://developers.hubspot.com/docs/apps/developer-platform/build-apps/authentication/request-validation
//
// Signature (v3, recommended): X-HubSpot-Signature-V3 header, base64
// HMAC-SHA256 where the message is requestMethod + requestURI + requestBody
// + timestamp (milliseconds, from X-HubSpot-Request-Timestamp), keyed with
// the app client secret; reject timestamps older than 5 minutes. The URI
// must exactly match the original request including protocol and query.
// (Verified from HubSpot's request-validation guide.)
//
// Mapped subscriptions (to the 8 supported blueprints):
//   deal.propertyChange (dealstage)          → appointment.reminder (follow-up)
//   contact.propertyChange (hs_lead_status)  → appointment.reminder
// HubSpot payload carries object ids, not phone numbers — the customer
// record in the business store is expected to resolve phone (HUBSPOT fetch
// is documented in integrations/hubspot/README.md).

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
)

// HubSpotConfig configures the HubSpot adapter.
type HubSpotConfig struct {
	// ClientSecret is the app's client secret. Falls back to HUBSPOT_CLIENT_SECRET.
	ClientSecret string
	// ExternalBaseURL, when set, is used to reconstruct the request URI for
	// signature verification behind a proxy/tunnel (the URI HubSpot signed
	// is the public one, not the internal one). Falls back to the Host header.
	ExternalBaseURL string
}

func (c HubSpotConfig) secret() string {
	if c.ClientSecret != "" {
		return c.ClientSecret
	}
	return os.Getenv("HUBSPOT_CLIENT_SECRET")
}

type hubspotAdapter struct{ cfg HubSpotConfig }

func (hubspotAdapter) Platform() string { return "hubspot" }

type hubspotPayload struct {
	ObjectID         json.Number `json:"objectId"`
	SubscriptionType string      `json:"subscriptionType"`
	PropertyName     string      `json:"propertyName"`
	PropertyValue    string      `json:"propertyValue"`
	ChangeSource     string      `json:"changeSource"`
	EventID          int64       `json:"eventId"`
	PortalID         int64       `json:"portalId"`
	EventType        string      `json:"eventType"`
}

func (a hubspotAdapter) Handle(r *http.Request, body []byte) ([]Event, error) {
	secret := a.cfg.secret()
	if secret == "" {
		return nil, ErrUnauthorized("HUBSPOT_CLIENT_SECRET not configured")
	}

	sig := r.Header.Get("X-HubSpot-Signature-V3")
	if sig == "" {
		// Legacy v1 fallback: plain SHA-256 hex of secret + body.
		v1 := r.Header.Get("X-HubSpot-Signature")
		if v1 == "" || !VerifyHubSpotV1(secret, body, v1) {
			return nil, ErrUnauthorized("invalid hubspot signature")
		}
	} else {
		// Reconstruct the exact URI HubSpot signed: scheme://host + path.
		// Behind a tunnel the public URL differs from what the server sees,
		// so operators can pin HUBSPOT_EXTERNAL_BASE_URL.
		scheme := "https"
		if r.TLS == nil {
			scheme = "http"
		}
		uri := scheme + "://" + r.Host + r.URL.RequestURI()
		if base := a.cfg.externalBase(); base != "" {
			uri = strings.TrimSuffix(base, "/") + r.URL.RequestURI()
		}
		if !VerifyHubSpotV3(secret, r.Method, uri, body,
			r.Header.Get("X-HubSpot-Request-Timestamp"), sig) {
			return nil, ErrUnauthorized("invalid hubspot signature")
		}
	}

	var p hubspotPayload
	if err := jsonUnmarshalStrict(body, &p); err != nil {
		return nil, err
	}

	customerID := "hs_" + p.ObjectID.String()
	var ev Event
	switch {
	case p.SubscriptionType == "deal.propertyChange" && p.PropertyName == "dealstage":
		ev = Event{
			ID:         "hs_deal_" + p.ObjectID.String(),
			Type:       "appointment.reminder", // deal motion → follow-up call
			CustomerID: customerID,
			Payload: payloadJSON(map[string]any{
				"what":    "your open proposal",
				"when":    "this week",
				"deal_id": p.ObjectID.String(),
				"stage":   p.PropertyValue,
			}),
		}
	case p.SubscriptionType == "contact.propertyChange" && p.PropertyName == "hs_lead_status":
		ev = Event{
			ID:         "hs_contact_" + p.ObjectID.String(),
			Type:       "appointment.reminder", // lead follow-up motion → callback request
			CustomerID: customerID,
			Payload: payloadJSON(map[string]any{
				"what":    "a follow-up about your inquiry",
				"when":    "at your convenience",
				"lead_id": p.ObjectID.String(),
			}),
		}
	default:
		return nil, nil
	}
	return []Event{ev}, nil
}

func (c HubSpotConfig) externalBase() string {
	if c.ExternalBaseURL != "" {
		return c.ExternalBaseURL
	}
	return os.Getenv("HUBSPOT_EXTERNAL_BASE_URL")
}

// NewHubSpot returns the HubSpot adapter.
func NewHubSpot(cfg HubSpotConfig) Adapter { return hubspotAdapter{cfg: cfg} }
