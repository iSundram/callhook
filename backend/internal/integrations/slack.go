package integrations

// Slack adapter — https://docs.slack.dev/authentication/verifying-requests-from-slack
//
// Signature: basestring "v0:TIMESTAMP:RAW_BODY" (X-Slack-Request-Timestamp
// + X-Slack-Signature headers), HMAC-SHA256 hex with the app signing
// secret, 5-minute replay window. (Verified against Slack docs.)
//
// Surface: the /call slash command.
//   /call invoice.due cus_1002 +15551234567
// The response is posted back into the channel the command came from.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// SlackConfig configures the Slack adapter.
type SlackConfig struct {
	// SigningSecret is the app signing secret. Falls back to SLACK_SIGNING_SECRET.
	SigningSecret string
}

func (c SlackConfig) secret() string {
	if c.SigningSecret != "" {
		return c.SigningSecret
	}
	return os.Getenv("SLACK_SIGNING_SECRET")
}

type slackAdapter struct{ cfg SlackConfig }

func (slackAdapter) Platform() string { return "slack" }

// Response answers the slash command in-channel.
func (a slackAdapter) Handle(r *http.Request, body []byte) ([]Event, error) {
	secret := a.cfg.secret()
	if secret == "" {
		return nil, ErrNotConfigured("slack", "SLACK_SIGNING_SECRET")
	}
	if !VerifySlack(secret, body, r.Header.Get("X-Slack-Request-Timestamp"), r.Header.Get("X-Slack-Signature")) {
		return nil, ErrUnauthorizedDoc("invalid slack signature", "/pages/integrations.html#troubleshooting")
	}

	form, err := url.ParseQuery(string(body))
	if err != nil {
		return nil, fmt.Errorf("invalid form body: %w", err)
	}
	if form.Get("command") != "/call" {
		return nil, nil
	}

	parts := strings.Fields(form.Get("text"))
	if len(parts) < 2 {
		// Not an event to fire, but a usage error worth surfacing.
		return nil, &slackUsageError{}
	}

	ev := Event{
		ID:         fmt.Sprintf("slack_%s_%d", form.Get("user_id"), time.Now().UnixNano()),
		Type:       parts[0],
		CustomerID: parts[1],
	}
	if len(parts) > 2 {
		ev.Phone = parts[2]
	}
	return []Event{ev}, nil
}

// slackUsageError signals a usage message instead of a hard error.
type slackUsageError struct{}

func (e *slackUsageError) Error() string     { return "usage" }
func (e *slackUsageError) usageJSON() string { return SlackUsageJSON() }

// NewSlack returns the Slack slash-command adapter.
func NewSlack(cfg SlackConfig) Adapter { return slackAdapter{cfg: cfg} }

// SlackUsageJSON is the in-channel response for a malformed /call.
func SlackUsageJSON() string {
	b, _ := json.Marshal(map[string]string{
		"text":          "Usage: `/call <event_type> <customer_id> [+E.164 phone]`",
		"response_type": "ephemeral",
	})
	return string(b)
}
