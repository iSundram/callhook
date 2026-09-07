package integrations

// Pipedrive adapter — https://pipedrive.readme.io/docs/guide-for-webhooks
//
// Verification: Pipedrive webhooks do NOT use HMAC signing. Two documented
// options (both verified from the guide):
//   1. HTTP Basic auth on the webhook (http_auth_user/http_auth_password
//      set when creating the webhook) — the recommended path, what we use.
//   2. A secret token embedded in the subscription URL path.
// We verify basic auth when configured; otherwise a URL token via
// PIPEDRIVE_URL_TOKEN.
//
// Payload: meta (action, object, webhook_id) + data (object fields).
// Mapped:
//   updated.deal (status lost)  → promo.offer (win-back)
//   updated.deal (rotting)      → appointment.reminder (nudge)

import (
	"encoding/json"
	"net/http"
	"os"
)

// PipedriveConfig configures the Pipedrive adapter.
type PipedriveConfig struct {
	// User/Pass for webhook basic auth. Fall back to PIPEDRIVE_WEBHOOK_USER
	// / PIPEDRIVE_WEBHOOK_PASSWORD.
	User string
	Pass string
	// URLToken, when set, must appear as the last path segment instead of
	// basic auth (PIPEDRIVE_URL_TOKEN).
	URLToken string
}

func (c PipedriveConfig) user() string {
	if c.User != "" {
		return c.User
	}
	return os.Getenv("PIPEDRIVE_WEBHOOK_USER")
}

func (c PipedriveConfig) pass() string {
	if c.Pass != "" {
		return c.Pass
	}
	return os.Getenv("PIPEDRIVE_WEBHOOK_PASSWORD")
}

func (c PipedriveConfig) urlToken() string {
	if c.URLToken != "" {
		return c.URLToken
	}
	return os.Getenv("PIPEDRIVE_URL_TOKEN")
}

type pipedriveAdapter struct{ cfg PipedriveConfig }

func (pipedriveAdapter) Platform() string { return "pipedrive" }

type pipedriveWebhook struct {
	Meta struct {
		Action    string      `json:"action"` // updated, added, deleted
		Object    string      `json:"object"` // deal, person, activity
		WebhookID json.Number `json:"webhook_id"`
	} `json:"meta"`
	Data struct {
		ID           int64  `json:"id"`
		Title        string `json:"title"`
		Status       string `json:"status"` // open, lost, won
		PersonID     int64  `json:"person_id"`
		RottingSince string `json:"rotting_since"` // set when deal went stale
	} `json:"data"`
	Previous struct {
		Status string `json:"status"`
	} `json:"previous"`
}

func (a pipedriveAdapter) Handle(r *http.Request, body []byte) ([]Event, error) {
	// Either basic auth or URL token must be configured.
	wantUser, wantPass := a.cfg.user(), a.cfg.pass()
	urlToken := a.cfg.urlToken()
	if wantUser != "" && wantPass != "" {
		gotUser, gotPass, ok := parseBasicAuth(r.Header.Get("Authorization"))
		if !ok || !VerifyBasic(gotUser, gotPass, wantUser, wantPass) {
			return nil, ErrUnauthorized("invalid pipedrive basic auth")
		}
	} else if urlToken != "" {
		if !stringsHasSuffixPath(r.URL.Path, urlToken) {
			return nil, ErrUnauthorized("invalid pipedrive url token")
		}
	} else {
		return nil, ErrUnauthorized("PIPEDRIVE_WEBHOOK_USER/PASSWORD or PIPEDRIVE_URL_TOKEN not configured")
	}

	var pw pipedriveWebhook
	if err := jsonUnmarshalStrict(body, &pw); err != nil {
		return nil, err
	}

	if pw.Meta.Object != "deal" || pw.Meta.Action != "updated" {
		return nil, nil
	}
	if pw.Data.PersonID == 0 && pw.Data.ID == 0 {
		return nil, nil
	}
	customerID := "pipedrive_deal_" + strconvFormat(pw.Data.ID)

	switch {
	case pw.Data.Status == "lost" && pw.Previous.Status != "lost":
		return []Event{{
			ID:         "pipedrive_" + strconvFormat(pw.Data.ID) + "_lost",
			Type:       "promo.offer",
			CustomerID: customerID,
			Payload: payloadJSON(map[string]any{
				"offer":   "an alternative that fits better",
				"expires": "end of this week",
				"deal":    pw.Data.Title,
			}),
		}}, nil
	case pw.Data.RottingSince != "":
		return []Event{{
			ID:         "pipedrive_" + strconvFormat(pw.Data.ID) + "_rotting",
			Type:       "appointment.reminder",
			CustomerID: customerID,
			Payload: payloadJSON(map[string]any{
				"what": "their open proposal (" + pw.Data.Title + ")",
				"when": "this week",
			}),
		}}, nil
	default:
		return nil, nil
	}
}

// stringsHasSuffixPath checks that the token appears as a full path segment.
func stringsHasSuffixPath(path, token string) bool {
	if token == "" {
		return false
	}
	for i := 0; i+len(token) <= len(path); i++ {
		if path[i:i+len(token)] != token {
			continue
		}
		beforeOK := i == 0 || path[i-1] == '/'
		after := i + len(token)
		afterOK := after == len(path) || path[after] == '/'
		if beforeOK && afterOK {
			return true
		}
	}
	return false
}

// NewPipedrive returns the Pipedrive adapter.
func NewPipedrive(cfg PipedriveConfig) Adapter { return pipedriveAdapter{cfg: cfg} }
