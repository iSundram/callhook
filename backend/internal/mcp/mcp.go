// Package mcp implements a Model Context Protocol server over Streamable
// HTTP (POST /mcp), exposing callhook as agent tools: fire events, launch
// campaigns, inspect sessions. Any MCP client — Claude Code, Cursor, any
// agent — can operate the voice channel.
//
// JSON-RPC 2.0, stdlib only.
package mcp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

const protocolVersion = "2025-06-18"

// Server is the MCP endpoint. The host wires the tool implementations.
type Server struct {
	// FireEvent fires one event through the pipeline.
	FireEvent func(args json.RawMessage) (string, error)
	// LaunchCampaign creates and starts a goal-driven campaign.
	LaunchCampaign func(args json.RawMessage) (string, error)
	// GetCampaign returns a campaign's status as text.
	GetCampaign func(args json.RawMessage) (string, error)
	// ListSessions returns recent sessions as text.
	ListSessions func(args json.RawMessage) (string, error)
	// ListEventTypes returns the catalog with outcome schemas.
	ListEventTypes func() (string, error)
	// RunDemo fires demo events + a campaign (dry-run friendly).
	RunDemo func() (string, error)
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// tool is one MCP tool definition.
type tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

func (s *Server) tools() []tool {
	return []tool{
		{
			Name:        "fire_event",
			Description: "Fire one callhook event: places an intelligent AI phone call to the customer (via CALL-E) and returns the placed call. Event types: invoice.due, account.warning, promo.offer, delivery.window, appointment.reminder, payment.failed, subscription.expiring, feedback.request.",
			InputSchema: map[string]any{
				"type":     "object",
				"required": []string{"event_type", "customer_id"},
				"properties": map[string]any{
					"event_type":  map[string]any{"type": "string", "description": "One of the callhook event types."},
					"customer_id": map[string]any{"type": "string", "description": "Customer to call, e.g. cus_1002."},
					"phone":       map[string]any{"type": "string", "description": "Optional E.164 phone override."},
					"not_before":  map[string]any{"type": "string", "description": "Optional RFC3339 earliest call time."},
				},
			},
		},
		{
			Name:        "launch_campaign",
			Description: "Launch a goal-driven calling campaign: waves of calls that stop the moment the goal is met. E.g. 'collect 5 payment promises from all overdue customers'.",
			InputSchema: map[string]any{
				"type":     "object",
				"required": []string{"event_type", "target"},
				"properties": map[string]any{
					"event_type": map[string]any{"type": "string", "description": "Event type the campaign fires."},
					"target":     map[string]any{"type": "integer", "description": "Number of successes to reach (early-stop)."},
					"name":       map[string]any{"type": "string", "description": "Campaign name."},
					"budget":     map[string]any{"type": "integer", "description": "Max calls (hard stop). Default 15."},
					"wave_size":  map[string]any{"type": "integer", "description": "Calls per wave. Default 3."},
				},
			},
		},
		{
			Name:        "get_campaign",
			Description: "Get a campaign's live status: progress, waves, audience states, budget.",
			InputSchema: map[string]any{
				"type":     "object",
				"required": []string{"campaign_id"},
				"properties": map[string]any{
					"campaign_id": map[string]any{"type": "string"},
				},
			},
		},
		{
			Name:        "list_sessions",
			Description: "List recent call sessions with their statuses and outcomes.",
			InputSchema: map[string]any{
				"type":     "object",
				"properties": map[string]any{
					"limit": map[string]any{"type": "integer", "description": "Max sessions to return. Default 10."},
				},
			},
		},
		{
			Name:        "list_event_types",
			Description: "List all callhook event types with their structured outcome schemas.",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			Name:        "run_demo",
			Description: "Run the callhook demo: fires three events and launches a campaign. Great for a first look at the pipeline.",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
		},
	}
}

// HandleHTTP implements the Streamable HTTP transport: one JSON-RPC
// request per POST, one JSON response back.
func (s *Server) HandleHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "mcp: POST only", http.StatusMethodNotAllowed)
		return
	}
	var req rpcRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeRPC(w, rpcResponse{JSONRPC: "2.0", ID: json.RawMessage("null"),
			Error: &rpcError{Code: -32700, Message: "parse error"}})
		return
	}

	// Notifications (no id) get no response.
	if len(req.ID) == 0 {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	res := s.dispatch(&req)
	res.JSONRPC = "2.0"
	res.ID = req.ID
	writeRPC(w, res)
}

func (s *Server) dispatch(req *rpcRequest) rpcResponse {
	switch req.Method {
	case "initialize":
		return rpcResponse{Result: map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
			"serverInfo":      map[string]any{"name": "callhook", "version": "0.4.0"},
			"instructions":    "callhook is the voice channel as an API. Fire events to place intelligent calls; launch campaigns for goal-driven waves with early-stop.",
		}}
	case "ping":
		return rpcResponse{Result: map[string]any{}}
	case "tools/list":
		return rpcResponse{Result: map[string]any{"tools": s.tools()}}
	case "tools/call":
		return s.callTool(req.Params)
	default:
		return rpcResponse{Error: &rpcError{Code: -32601, Message: "method not found: " + req.Method}}
	}
}

func (s *Server) callTool(params json.RawMessage) rpcResponse {
	var p struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments,omitempty"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return rpcResponse{Error: &rpcError{Code: -32602, Message: "invalid params"}}
	}

	var text string
	var err error
	switch p.Name {
	case "fire_event":
		text, err = s.FireEvent(p.Arguments)
	case "launch_campaign":
		text, err = s.LaunchCampaign(p.Arguments)
	case "get_campaign":
		text, err = s.GetCampaign(p.Arguments)
	case "list_sessions":
		text, err = s.ListSessions(p.Arguments)
	case "list_event_types":
		text, err = s.ListEventTypes()
	case "run_demo":
		text, err = s.RunDemo()
	default:
		return rpcResponse{Error: &rpcError{Code: -32602, Message: "unknown tool: " + p.Name}}
	}

	if err != nil {
		// Tool execution errors are RESULTS with isError, per MCP.
		return rpcResponse{Result: map[string]any{
			"content": []any{map[string]any{"type": "text", "text": "error: " + err.Error()}},
			"isError": true,
		}}
	}
	return rpcResponse{Result: map[string]any{
		"content": []any{map[string]any{"type": "text", "text": text}},
	}}
}

func writeRPC(w http.ResponseWriter, res rpcResponse) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

// Helper used by tool impls: compact JSON or a readable error.
func MarshalArg(raw json.RawMessage, dst any) error {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "null" || strings.TrimSpace(string(raw)) == "{}" {
		return nil
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return fmt.Errorf("invalid arguments: %w", err)
	}
	return nil
}
