package mcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testServer() *Server {
	return &Server{
		FireEvent: func(args json.RawMessage) (string, error) {
			var a struct {
				EventType  string `json:"event_type"`
				CustomerID string `json:"customer_id"`
			}
			_ = json.Unmarshal(args, &a)
			if a.EventType == "" {
				return "", errString("event_type is required")
			}
			return `{"status":"call_placed"}`, nil
		},
		LaunchCampaign: func(args json.RawMessage) (string, error) { return `{"campaign_id":"c1"}`, nil },
		GetCampaign:    func(args json.RawMessage) (string, error) { return `{"status":"goal_met"}`, nil },
		ListSessions:   func(args json.RawMessage) (string, error) { return `[]`, nil },
		ListEventTypes: func() (string, error) { return `{}`, nil },
		RunDemo:        func() (string, error) { return "demo running", nil },
	}
}

func errString(s string) error { return &testErr{s} }

type testErr struct{ msg string }

func (e *testErr) Error() string { return e.msg }

func callRPC(s *Server, body string) (int, map[string]any) {
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	w := httptest.NewRecorder()
	s.HandleHTTP(w, req)
	var res map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	return w.Code, res
}

func TestInitialize(t *testing.T) {
	code, res := callRPC(testServer(), `{"jsonrpc":"2.0","id":1,"method":"initialize"}`)
	if code != 200 || res["result"] == nil {
		t.Fatalf("initialize: %d %v", code, res)
	}
	result := res["result"].(map[string]any)
	if result["protocolVersion"] != protocolVersion {
		t.Errorf("protocol version: %v", result["protocolVersion"])
	}
	info := result["serverInfo"].(map[string]any)
	if info["name"] != "callhook" {
		t.Errorf("serverInfo: %v", info)
	}
}

func TestToolsList(t *testing.T) {
	_, res := callRPC(testServer(), `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	tools := res["result"].(map[string]any)["tools"].([]any)
	if len(tools) != 6 {
		t.Fatalf("tools = %d, want 6", len(tools))
	}
	for _, tt := range tools {
		tool := tt.(map[string]any)
		if tool["inputSchema"] == nil || tool["description"] == "" {
			t.Errorf("tool %v missing schema or description", tool["name"])
		}
	}
}

func TestToolsCallFireEvent(t *testing.T) {
	_, res := callRPC(testServer(), `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"fire_event","arguments":{"event_type":"invoice.due","customer_id":"cus_1002"}}}`)
	result := res["result"].(map[string]any)
	content := result["content"].([]any)[0].(map[string]any)
	if content["text"] != `{"status":"call_placed"}` {
		t.Errorf("content: %v", content)
	}
}

func TestToolsCallErrorIsResult(t *testing.T) {
	// tool execution errors must be isError RESULTS, not RPC errors
	_, res := callRPC(testServer(), `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"fire_event","arguments":{"customer_id":"cus_1002"}}}`)
	if res["error"] != nil {
		t.Fatalf("tool error surfaced as RPC error: %v", res["error"])
	}
	result := res["result"].(map[string]any)
	if result["isError"] != true {
		t.Errorf("expected isError result, got %v", result)
	}
}

func TestUnknownMethodAndTool(t *testing.T) {
	_, res := callRPC(testServer(), `{"jsonrpc":"2.0","id":5,"method":"nope/xyz"}`)
	if res["error"] == nil {
		t.Error("unknown method must be an RPC error")
	}
	_, res = callRPC(testServer(), `{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"nope_tool"}}`)
	if res["error"] == nil {
		t.Error("unknown tool must be an RPC error")
	}
}

func TestNotificationNoResponse(t *testing.T) {
	s := testServer()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","method":"notifications/initialized"}`))
	w := httptest.NewRecorder()
	s.HandleHTTP(w, req)
	if w.Body.Len() != 0 {
		t.Errorf("notification must not produce a response body, got %q", w.Body)
	}
}

func TestGetOnlyPost(t *testing.T) {
	s := testServer()
	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	w := httptest.NewRecorder()
	s.HandleHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET /mcp: %d, want 405", w.Code)
	}
}

func TestMarshalArgEmpty(t *testing.T) {
	var dst struct{ A int }
	if err := MarshalArg(nil, &dst); err != nil {
		t.Errorf("nil args: %v", err)
	}
	if err := MarshalArg(json.RawMessage(`{}`), &dst); err != nil {
		t.Errorf("empty args: %v", err)
	}
	if err := MarshalArg(json.RawMessage(`{"A":1}`), &dst); err != nil || dst.A != 1 {
		t.Errorf("real args: %v %+v", err, dst)
	}
}
