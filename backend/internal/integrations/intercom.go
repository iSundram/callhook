package integrations

// Intercom adapter — https://developers.intercom.com (Webhook Notifications)
//
// Signature: X-Hub-Signature header = "sha1=" + hex HMAC-SHA1(raw body,
// app client secret). Verified from Intercom's webhook-models reference:
// "The signature is the hexadecimal (40-byte) representation of a SHA-1
// signature computed using the HMAC algorithm", value "starts with the
// string sha1=".
//
// Mapped topics:
//   conversation.admin.closed (unresolved) → feedback.request
//   user.tag.created (at-risk)             → account.warning
//   user.deleted                           → never called (skipped)

import (
	"net/http"
	"os"
	"strings"
)

// IntercomConfig configures the Intercom adapter.
type IntercomConfig struct {
	// ClientSecret is the app's client secret (Basic Info page).
	// Falls back to INTERCOM_CLIENT_SECRET.
	ClientSecret string
}

func (c IntercomConfig) secret() string {
	if c.ClientSecret != "" {
		return c.ClientSecret
	}
	return os.Getenv("INTERCOM_CLIENT_SECRET")
}

type intercomAdapter struct{ cfg IntercomConfig }

func (intercomAdapter) Platform() string { return "intercom" }

type intercomWebhook struct {
	Type  string `json:"type"` // notification_event
	ID    string `json:"id"`
	AppID string `json:"app_id"`
	Topic string `json:"topic"`
	Data  struct {
		Item struct {
			Type string `json:"type"`
			ID   string `json:"id"`
			User struct {
				ID    string `json:"id"`
				Email string `json:"email"`
				Phone string `json:"phone"`
			} `json:"user"`
			Assignee struct {
				Type string `json:"type"`
				ID   string `json:"id"`
			} `json:"assignee"`
			ConversationParts struct {
				ConversationParts []struct {
					PartType string `json:"part_type"`
				} `json:"conversation_parts"`
			} `json:"conversation_parts"`
			Tag struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"tag"`
		} `json:"item"`
	} `json:"data"`
}

func (a intercomAdapter) Handle(r *http.Request, body []byte) ([]Event, error) {
	secret := a.cfg.secret()
	if secret == "" {
		return nil, ErrNotConfigured("intercom", "INTERCOM_CLIENT_SECRET")
	}
	if !VerifySHA1HexHMAC(secret, body, r.Header.Get("X-Hub-Signature")) {
		return nil, ErrUnauthorizedDoc("invalid intercom signature", "/pages/integrations.html#troubleshooting")
	}

	var iw intercomWebhook
	if err := jsonUnmarshalStrict(body, &iw); err != nil {
		return nil, err
	}

	item := iw.Data.Item
	switch {
	case strings.HasPrefix(iw.Topic, "user.tag.created"):
		tag := item.Tag.Name
		if tag == "" || (tag != "at-risk" && tag != "At Risk" && tag != "churn-risk") {
			return nil, nil
		}
		return []Event{{
			ID:         "intercom_" + iw.ID,
			Type:       "account.warning",
			CustomerID: "intercom_" + item.User.ID,
			Phone:      item.User.Phone,
			Payload: payloadJSON(map[string]any{
				"reason": "flagged at-risk in Intercom",
				"detail": "Tagged '" + tag + "' — check in before they churn.",
			}),
		}}, nil
	case iw.Topic == "conversation.admin.closed":
		// Unresolved closed conversations get a satisfaction call.
		return []Event{{
			ID:         "intercom_" + iw.ID,
			Type:       "feedback.request",
			CustomerID: "intercom_" + item.User.ID,
			Phone:      item.User.Phone,
			Payload: payloadJSON(map[string]any{
				"topic": "their recent support conversation",
				"email": item.User.Email,
			}),
		}}, nil
	default:
		return nil, nil
	}
}

// NewIntercom returns the Intercom adapter.
func NewIntercom(cfg IntercomConfig) Adapter { return intercomAdapter{cfg: cfg} }
