package integrations

// Generic HTTP adapter — the universal fallback.
//
// Accepts the full callhook event JSON directly (no translation needed):
// POST /integrations/generic/webhook with the callhook Event body.
// Verification: optional shared secret via X-Callhook-Secret header or
// Authorization: Bearer (GENERIC_WEBHOOK_SECRET / GENERIC_BEARER_TOKEN).

import (
	"encoding/json"
	"net/http"
	"os"
)

// GenericConfig configures the generic adapter.
type GenericConfig struct {
	// Secret, when set, is checked against the X-Callhook-Secret header.
	Secret string
	// Token, when set, is checked as Authorization: Bearer.
	Token string
}

func (c GenericConfig) secret() string {
	if c.Secret != "" {
		return c.Secret
	}
	return os.Getenv("GENERIC_WEBHOOK_SECRET")
}

func (c GenericConfig) token() string {
	if c.Token != "" {
		return c.Token
	}
	return os.Getenv("GENERIC_BEARER_TOKEN")
}

type genericAdapter struct{ cfg GenericConfig }

func (genericAdapter) Platform() string { return "generic" }

// genericEvent mirrors events.Event's wire shape.
type genericEvent struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	CustomerID string          `json:"customer_id"`
	Phone      string          `json:"phone"`
	RawPayload json.RawMessage `json:"payload"`
}

func (a genericAdapter) Handle(r *http.Request, body []byte) ([]Event, error) {
	secret, token := a.cfg.secret(), a.cfg.token()
	if secret == "" && token == "" {
		// Unauthenticated generic intake is allowed only when the operator
		// deliberately configured nothing — logged loudly in the README.
		return decodeGeneric(body)
	}
	if token != "" && VerifyBearer(r.Header.Get("Authorization"), token) {
		return decodeGeneric(body)
	}
	if secret != "" && hmacEqual(r.Header.Get("X-Callhook-Secret"), secret) {
		return decodeGeneric(body)
	}
	return nil, ErrUnauthorized("invalid generic webhook credentials")
}

func decodeGeneric(body []byte) ([]Event, error) {
	var ge genericEvent
	if err := jsonUnmarshalStrict(body, &ge); err != nil {
		return nil, err
	}
	if ge.ID == "" || ge.Type == "" || ge.CustomerID == "" {
		return nil, &httpError{code: 400, msg: "event requires id, type and customer_id"}
	}
	return []Event{{
		ID:         ge.ID,
		Type:       ge.Type,
		CustomerID: ge.CustomerID,
		Phone:      ge.Phone,
		Payload:    string(ge.RawPayload),
	}}, nil
}

// NewGeneric returns the generic webhook adapter.
func NewGeneric(cfg GenericConfig) Adapter { return genericAdapter{cfg: cfg} }
