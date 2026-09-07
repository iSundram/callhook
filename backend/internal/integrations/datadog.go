package integrations

// Datadog adapter — https://docs.datadoghq.com/integrations/webhooks/
//
// Verification: Datadog does NOT sign webhook payloads. Their documented
// options are custom headers (JSON map defined in the integration tile),
// HTTP basic auth embedded in the URL, or OAuth 2.0. We verify:
//   1. A shared-secret custom header (DATADOG_SHARED_HEADER, default
//      "X-Callhook-Secret") equal to DATADOG_SHARED_SECRET, or
//   2. HTTP Basic auth with DATADOG_WEBHOOK_USER/PASSWORD.
//
// Payload: fully operator-defined via the Webhook integration tile
// (custom payload with $ALERT_TITLE, $ALERT_STATUS, $TAGS, ...). We accept
// the well-known monitor payload keys: title, alert_status, priority,
// event_msg, tags, plus a customer_id key the operator includes to route
// the call (e.g. "customer_id":"oncall_payments").
//
// Mapping: any payload with alert_status "Triggered" or transition
// "Triggered"/"Re-Triggered" fires account.warning; "Recovered" is skipped.

import (
	"net/http"
	"os"
	"strings"
)

// DatadogConfig configures the Datadog adapter.
type DatadogConfig struct {
	// SharedHeader is the custom-header name carrying the shared secret.
	SharedHeader string
	// SharedSecret is the expected secret value (a hidden custom variable
	// in Datadog). Falls back to DATADOG_SHARED_SECRET.
	SharedSecret string
	// Basic auth credentials (from the webhook URL user:pass@host form).
	User string
	Pass string
}

func (c DatadogConfig) sharedHeader() string {
	if c.SharedHeader != "" {
		return c.SharedHeader
	}
	return getenvDefault("DATADOG_SHARED_HEADER", "X-Callhook-Secret")
}

func (c DatadogConfig) sharedSecret() string {
	if c.SharedSecret != "" {
		return c.SharedSecret
	}
	return os.Getenv("DATADOG_SHARED_SECRET")
}

func (c DatadogConfig) user() string {
	if c.User != "" {
		return c.User
	}
	return os.Getenv("DATADOG_WEBHOOK_USER")
}

func (c DatadogConfig) pass() string {
	if c.Pass != "" {
		return c.Pass
	}
	return os.Getenv("DATADOG_WEBHOOK_PASSWORD")
}

type datadogAdapter struct{ cfg DatadogConfig }

func (datadogAdapter) Platform() string { return "datadog" }

type datadogWebhook struct {
	AlertTitle      string `json:"title"`
	AlertStatus     string `json:"alert_status"` // e.g. "Triggered" for log monitors
	AlertTransition string `json:"alert_transition"`
	AlertPriority   string `json:"alert_priority"`
	AlertType       string `json:"alert_type"`
	EventTitle      string `json:"event_title"`
	EventMsg        string `json:"event_msg"`
	Tags            string `json:"tags"`
	CustomerID      string `json:"customer_id"`
	DatePosix       int64  `json:"date_posix"`
	MsgText         string `json:"msg_text"`
	Body            string `json:"body"`
}

func (a datadogAdapter) Handle(r *http.Request, body []byte) ([]Event, error) {
	// Verify: shared-secret header or basic auth — at least one configured.
	secret := a.cfg.sharedSecret()
	wantUser, wantPass := a.cfg.user(), a.cfg.pass()
	if secret == "" && (wantUser == "" || wantPass == "") {
		return nil, ErrUnauthorized("DATADOG_SHARED_SECRET or DATADOG_WEBHOOK_USER/PASSWORD not configured")
	}
	authorized := false
	if secret != "" && hmacEqual(r.Header.Get(a.cfg.sharedHeader()), secret) {
		authorized = true
	}
	if !authorized && wantUser != "" && wantPass != "" {
		gotUser, gotPass, ok := parseBasicAuth(r.Header.Get("Authorization"))
		authorized = ok && VerifyBasic(gotUser, gotPass, wantUser, wantPass)
	}
	if !authorized {
		return nil, ErrUnauthorizedDoc("invalid datadog credentials", "/pages/integrations.html#troubleshooting")
	}

	var dw datadogWebhook
	if err := jsonUnmarshalStrict(body, &dw); err != nil {
		return nil, err
	}

	// Firing detection across payload shapes.
	transition := dw.AlertTransition
	if transition == "" {
		transition = dw.AlertStatus
	}
	status := strings.ToLower(transition)
	firing := strings.Contains(status, "triggered")
	if dw.AlertType == "error" && transition == "" {
		firing = true
	}
	if !firing {
		return nil, nil // recovered/no-data — no call
	}

	title := dw.AlertTitle
	if title == "" {
		title = dw.EventTitle
	}
	detail := dw.EventMsg
	if detail == "" {
		detail = dw.MsgText
	}
	if detail == "" {
		detail = dw.Body
	}
	customerID := dw.CustomerID
	if customerID == "" {
		// Fall back to the team tag: "env:prod,team:payments" → oncall_payments.
		for _, tag := range strings.Split(dw.Tags, ",") {
			tag = strings.TrimSpace(tag)
			if v, ok := strings.CutPrefix(tag, "team:"); ok {
				customerID = "oncall_" + v
				break
			}
		}
	}
	if customerID == "" {
		return nil, nil // no routing — nothing to do
	}

	return []Event{{
		ID:         "datadog_" + title + "_" + strconvFormat(dw.DatePosix),
		Type:       "account.warning",
		CustomerID: customerID,
		Payload: payloadJSON(map[string]any{
			"reason":   "Datadog monitor triggered",
			"detail":   title + " — " + detail,
			"priority": dw.AlertPriority,
			"tags":     dw.Tags,
		}),
	}}, nil
}

// NewDatadog returns the Datadog adapter.
func NewDatadog(cfg DatadogConfig) Adapter { return datadogAdapter{cfg: cfg} }

func getenvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
