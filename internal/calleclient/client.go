// Package calleclient is a Go client for the CALL-E Developer API
// (https://api.heycall-e.com). Contract: docs/calle.openapi.yaml.
package calleclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const defaultBaseURL = "https://api.heycall-e.com"

// Client talks to the CALL-E Developer API.
type Client struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
	// DryRun, when true, fabricates terminal call results instead of placing
	// real phone calls. Used for local development and demos without balance.
	DryRun bool
}

func New(apiKey, baseURL string) *Client {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Client{
		BaseURL: baseURL,
		APIKey:  apiKey,
		HTTP:    &http.Client{Timeout: 30 * time.Second},
	}
}

// CreateCallRequest mirrors the OpenAPI CreateCallRequest schema.
type CreateCallRequest struct {
	Task                 string            `json:"task"`
	Recipients           []Recipient       `json:"recipients,omitempty"`
	ResultSchema         map[string]any    `json:"result_schema,omitempty"`
	RecipientResultSchema map[string]any   `json:"recipient_result_schema,omitempty"`
	Metadata             map[string]any    `json:"metadata,omitempty"`
	WebhookURL           string            `json:"webhook_url,omitempty"`
}

// Recipient is one call target.
type Recipient struct {
	Phones []string `json:"phones"`
	Locale string   `json:"locale,omitempty"`
	Region string   `json:"region,omitempty"`
}

// CallTask is the call object returned by GET /v1/calls/{id} and embedded in
// terminal webhook events.
type CallTask struct {
	ID                 string             `json:"id"`
	Object             string             `json:"object"`
	Status             string             `json:"status"` // queued | in_progress | completed | failed | canceled
	Task               string             `json:"task"`
	Recipients         []RecipientResult  `json:"recipients"`
	Attempts           []Attempt          `json:"attempts"`
	StructuredResult   map[string]any     `json:"structured_result"`
	CompletionConfidence *Confidence      `json:"completion_confidence"`
	Evidence           []string           `json:"evidence"`
	CreatedAt          time.Time          `json:"created_at"`
}

// RecipientResult is per-recipient state in a call task.
type RecipientResult struct {
	ID            string         `json:"id"`
	Status        string         `json:"status"`
	StructuredResult map[string]any `json:"structured_result"`
}

// Attempt is one outbound dial attempt, including the transcript.
type Attempt struct {
	ID             string    `json:"id"`
	Phone          string    `json:"phone"`
	Status         string    `json:"status"`
	StartedAt      *time.Time `json:"started_at"`
	CompletedAt    *time.Time `json:"completed_at"`
	Summary        *string    `json:"summary"`
	TranscriptTurns []Turn    `json:"transcript_turns"`
}

// Turn is one transcript line.
type Turn struct {
	OffsetSeconds *int   `json:"offset_seconds"`
	Speaker       string `json:"speaker"` // bot | user | unknown
	Text          string `json:"text"`
}

// Confidence is CALL-E's task-completion judgment.
type Confidence struct {
	Score float64 `json:"score"`
	Label string  `json:"label"` // low | medium | high
}

// WebhookEvent is the terminal event payload CALL-E POSTs to our webhook.
type WebhookEvent struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"` // call.completed | call.failed | call.result_validation_failed
	CreatedAt time.Time `json:"created_at"`
	Data      CallTask  `json:"data"`
}

// CreateCall places a call task. Returns the created call.
func (c *Client) CreateCall(ctx context.Context, req *CreateCallRequest, idempotencyKey string) (*CallTask, error) {
	if c.DryRun {
		return c.dryRunCall(req, idempotencyKey), nil
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/v1/calls", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	c.auth(httpReq, idempotencyKey)
	httpReq.Header.Set("Content-Type", "application/json")
	return c.doCall(httpReq)
}

// GetCall fetches current call state.
func (c *Client) GetCall(ctx context.Context, callID string) (*CallTask, error) {
	if c.DryRun {
		return nil, fmt.Errorf("getcall: not supported in dry-run mode")
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/v1/calls/"+callID, nil)
	if err != nil {
		return nil, err
	}
	c.auth(httpReq, "")
	return c.doCall(httpReq)
}

func (c *Client) auth(req *http.Request, idempotencyKey string) {
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
}

func (c *Client) doCall(req *http.Request) (*CallTask, error) {
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("calle api %s: %s", resp.Status, truncate(raw, 400))
	}
	var task CallTask
	if err := json.Unmarshal(raw, &task); err != nil {
		return nil, fmt.Errorf("decode call task: %w", err)
	}
	return &task, nil
}

func truncate(b []byte, n int) string {
	if len(b) > n {
		return string(b[:n]) + "..."
	}
	return string(b)
}

// dryRunCall fabricates a plausible completed call so the whole pipeline —
// event intake, task composition, outcome engine, dashboard — can be
// exercised end to end without spending call balance.
func (c *Client) dryRunCall(req *CreateCallRequest, idempotencyKey string) *CallTask {
	now := time.Now().UTC()
	phone := ""
	if len(req.Recipients) > 0 && len(req.Recipients[0].Phones) > 0 {
		phone = req.Recipients[0].Phones[0]
	}
	summary := "DRY RUN — simulated completed call for local development."
	return &CallTask{
		ID:     "call_dry_" + idempotencyKey,
		Object: "call_task",
		Status: "completed",
		Task:   req.Task,
		Recipients: []RecipientResult{{
			ID:     "rcp_dry",
			Status: "completed",
		}},
		Attempts: []Attempt{{
			ID:         "att_dry",
			Phone:      phone,
			Status:     "completed",
			StartedAt:  &now,
			CompletedAt: &now,
			Summary:    &summary,
			TranscriptTurns: []Turn{
				{OffsetSeconds: ptr(0), Speaker: "bot", Text: "Hello! This is a dry-run call — no real phone was dialed."},
				{OffsetSeconds: ptr(3), Speaker: "user", Text: "Got it, this is a test."},
				{OffsetSeconds: ptr(5), Speaker: "bot", Text: "Perfect, terminating the simulated call. Goodbye!"},
			},
		}},
		CompletionConfidence: &Confidence{Score: 0.99, Label: "high"},
		Evidence:             []string{"Dry-run mode: fabricated terminal result."},
		CreatedAt:            now,
	}
}

func ptr[T any](v T) *T { return &v }
