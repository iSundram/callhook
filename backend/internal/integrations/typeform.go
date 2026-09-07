package integrations

// Typeform adapter — https://www.typeform.com/developers/webhooks/secure-your-webhooks
//
// Signature: Typeform-Signature header = "sha256=" + base64(HMAC-SHA256(
// raw body, webhook secret)). (Verified from Typeform's secure-your-webhooks
// page, including their Ruby/Node/Python examples.)
//
// Mapping: any form response fires a feedback.request event; answers are
// flattened into the payload. Fields carrying a phone number (phone field
// or a "phone" key) are picked up automatically.

import (
	"net/http"
	"os"
)

// TypeformConfig configures the Typeform adapter.
type TypeformConfig struct {
	// Secret is the webhook secret. Falls back to TYPEFORM_WEBHOOK_SECRET.
	Secret string
}

func (c TypeformConfig) secret() string {
	if c.Secret != "" {
		return c.Secret
	}
	return os.Getenv("TYPEFORM_WEBHOOK_SECRET")
}

type typeformAdapter struct{ cfg TypeformConfig }

func (typeformAdapter) Platform() string { return "typeform" }

type typeformWebhook struct {
	EventID      string `json:"event_id"`
	EventType    string `json:"event_type"`
	FormResponse struct {
		FormID    string `json:"form_id"`
		Token     string `json:"token"`
		Submitted string `json:"submitted_at"`
		Answers   []struct {
			Field struct {
				ID   string `json:"id"`
				Type string `json:"type"`
				Ref  string `json:"ref"`
			} `json:"field"`
			Type   string `json:"type"`
			Phone  string `json:"phone"`
			Text   string `json:"text"`
			Email  string `json:"email"`
			Choice struct {
				Label string `json:"label"`
			} `json:"choice"`
		} `json:"answers"`
		Hidden map[string]string `json:"hidden"`
	} `json:"form_response"`
}

func (a typeformAdapter) Handle(r *http.Request, body []byte) ([]Event, error) {
	secret := a.cfg.secret()
	if secret == "" {
		return nil, ErrUnauthorized("TYPEFORM_WEBHOOK_SECRET not configured")
	}
	if !VerifyBase64HMAC(secret, body, r.Header.Get("Typeform-Signature")) {
		return nil, ErrUnauthorized("invalid typeform signature")
	}

	var tw typeformWebhook
	if err := jsonUnmarshalStrict(body, &tw); err != nil {
		return nil, err
	}
	if tw.EventType != "form_response" {
		return nil, nil
	}

	fr := tw.FormResponse
	phone := ""
	answers := map[string]any{}
	for _, ans := range fr.Answers {
		val := answerValue(ans)
		key := ans.Field.Ref
		if key == "" {
			key = ans.Field.ID
		}
		answers[key] = val
		if ans.Type == "phone" && ans.Phone != "" {
			phone = ans.Phone
		}
	}
	// Hidden fields may carry a customer mapping (set in the form's share URL).
	customerID := fr.Hidden["customer_id"]
	if customerID == "" {
		customerID = "typeform_" + fr.FormID
	}
	if h := fr.Hidden["phone"]; h != "" {
		phone = h
	}

	return []Event{{
		ID:         "typeform_" + tw.EventID,
		Type:       "feedback.request",
		CustomerID: customerID,
		Phone:      phone,
		Payload: payloadJSON(map[string]any{
			"topic":   "their recent form submission",
			"form_id": fr.FormID,
			"answers": answers,
		}),
	}}, nil
}

func answerValue(ans struct {
	Field struct {
		ID   string `json:"id"`
		Type string `json:"type"`
		Ref  string `json:"ref"`
	} `json:"field"`
	Type   string `json:"type"`
	Phone  string `json:"phone"`
	Text   string `json:"text"`
	Email  string `json:"email"`
	Choice struct {
		Label string `json:"label"`
	} `json:"choice"`
}) string {
	switch ans.Type {
	case "phone":
		return ans.Phone
	case "email":
		return ans.Email
	case "choice":
		return ans.Choice.Label
	default:
		return ans.Text
	}
}

// NewTypeform returns the Typeform adapter.
func NewTypeform(cfg TypeformConfig) Adapter { return typeformAdapter{cfg: cfg} }
