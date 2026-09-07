package integrations

// Grafana adapter — https://grafana.com/docs/grafana/latest/alerting/configure-notifications/manage-contact-points/integrations/webhook-notifier/
//
// Signature (optional but supported): when a shared secret is configured
// in the contact point, Grafana signs the payload with HMAC-SHA256 and
// sends it in a configurable header (default X-Grafana-Alerting-Signature),
// hex-encoded, over "timestamp:body" if a timestamp header is configured,
// else the body alone. (Verified from Grafana's webhook-notifier docs.)
//
// Payload: Alertmanager-style JSON — status "firing"/"resolved", alerts[]
// with labels/annotations. Firing alerts map to account.warning (ops
// escalation); resolved alerts are skipped (nobody wants a "it's fine" call).
//
// The "customer" is the on-call engineer, resolved from the alert's team
// label via the business store directory.

import (
	"net/http"
	"os"
)

// GrafanaConfig configures the Grafana adapter.
type GrafanaConfig struct {
	// Secret, when set, requires signature verification. Falls back to
	// GRAFANA_WEBHOOK_SECRET. Empty = accept unsigned payloads (rely on
	// a bearer token or network controls instead).
	Secret string
	// Header is the signature header name; defaults to X-Grafana-Alerting-Signature.
	Header string
	// TimestampHeader, when set, names the header whose value is part of
	// the signed payload ("timestamp:body"). Defaults to none (body-only).
	TimestampHeader string
}

type grafanaAdapter struct{ cfg GrafanaConfig }

func (grafanaAdapter) Platform() string { return "grafana" }

func (c GrafanaConfig) secret() string {
	if c.Secret != "" {
		return c.Secret
	}
	return os.Getenv("GRAFANA_WEBHOOK_SECRET")
}

func (c GrafanaConfig) header() string {
	if c.Header != "" {
		return c.Header
	}
	return "X-Grafana-Alerting-Signature"
}

type grafanaWebhook struct {
	Receiver string `json:"receiver"`
	Status   string `json:"status"` // firing | resolved
	Alerts   []struct {
		Status       string            `json:"status"`
		Labels       map[string]string `json:"labels"`
		Annotations  map[string]string `json:"annotations"`
		StartsAt     string            `json:"startsAt"`
		GeneratorURL string            `json:"generatorURL"`
		Fingerprint  string            `json:"fingerprint"`
	} `json:"alerts"`
	CommonLabels map[string]string `json:"commonLabels"`
}

func (a grafanaAdapter) Handle(r *http.Request, body []byte) ([]Event, error) {
	secret := a.cfg.secret()
	if secret != "" {
		ts := ""
		if a.cfg.TimestampHeader != "" {
			ts = r.Header.Get(a.cfg.TimestampHeader)
		}
		if !VerifyGrafana(secret, body, ts, r.Header.Get(a.cfg.header())) {
			return nil, ErrUnauthorized("invalid grafana signature")
		}
	}

	var gw grafanaWebhook
	if err := jsonUnmarshalStrict(body, &gw); err != nil {
		return nil, err
	}

	var events []Event
	for _, alert := range gw.Alerts {
		if alert.Status != "firing" {
			continue
		}
		team := alert.Labels["team"]
		if team == "" {
			team = gw.CommonLabels["team"]
		}
		name := alert.Labels["alertname"]
		summary := alert.Annotations["summary"]
		if summary == "" {
			summary = alert.Annotations["description"]
		}
		events = append(events, Event{
			ID:         "grafana_" + alert.Fingerprint,
			Type:       "account.warning",
			CustomerID: "oncall_" + team,
			Payload: payloadJSON(map[string]any{
				"reason": "Grafana alert " + name + " firing",
				"detail": summary,
				"team":   team,
				"url":    alert.GeneratorURL,
			}),
		})
	}
	return events, nil
}

// NewGrafana returns the Grafana adapter.
func NewGrafana(cfg GrafanaConfig) Adapter { return grafanaAdapter{cfg: cfg} }
