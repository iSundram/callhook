package callhookclient

import (
	"context"
	"testing"
)

func TestDryRunResultHonorsSchemaEnum(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"outcome": map[string]any{
				"type": "string",
				"enum": []string{"payment_promised", "no_answer"}, // []string, built in Go
			},
		},
	}
	res := DryRunResult(schema, "seed-1")
	if res == nil {
		t.Fatal("result is nil")
	}
	v, ok := res["outcome"].(string)
	if !ok {
		t.Fatalf("outcome not a string: %T", res["outcome"])
	}
	if v != "payment_promised" && v != "no_answer" {
		t.Errorf("outcome %q not from enum", v)
	}
}

func TestDryRunResultEnumFromJSONShape(t *testing.T) {
	// enums arriving via JSON decode are []any
	schema := map[string]any{
		"properties": map[string]any{
			"outcome": map[string]any{
				"enum": []any{"yes", "no", "unknown"},
			},
		},
	}
	for _, seed := range []string{"a", "b", "c", "d"} {
		res := DryRunResult(schema, seed)
		v, _ := res["outcome"].(string)
		if v != "yes" && v != "no" && v != "unknown" {
			t.Fatalf("seed %s: outcome %q not from enum", seed, v)
		}
	}
}

func TestDryRunResultPromiseDate(t *testing.T) {
	schema := map[string]any{
		"properties": map[string]any{
			"promise_date": map[string]any{"type": "string"},
		},
	}
	res := DryRunResult(schema, "s")
	d, ok := res["promise_date"].(string)
	if !ok || len(d) != 10 {
		t.Errorf("promise_date = %v, want YYYY-MM-DD", res["promise_date"])
	}
}

func TestDryRunCallShape(t *testing.T) {
	c := New("", "")
	c.DryRun = true
	call, err := c.CreateCall(context.Background(), &CreateCallRequest{
		Task:       "test task",
		Recipients: []Recipient{{Phones: []string{"+15550001"}}},
	}, "idem-1")
	if err != nil {
		t.Fatalf("dry-run create: %v", err)
	}
	if call.Status != "completed" || call.ID != "call_dry_idem-1" {
		t.Errorf("call: %+v", call)
	}
	if len(call.Attempts) == 0 || len(call.Attempts[0].TranscriptTurns) == 0 {
		t.Error("dry-run call must include a transcript")
	}
	if call.CompletionConfidence == nil || call.CompletionConfidence.Label != "high" {
		t.Error("dry-run call must carry confidence")
	}
}

func TestGoalRunAsCallTaskProjection(t *testing.T) {
	run := &GoalRun{Object: "goal_run", ID: "r1", GoalID: "g1", CallID: "call_x", Status: "completed",
		Result: map[string]any{"outcome": "accepted"}}
	task := run.AsCallTask()
	if task.ID != "call_x" || task.Status != "completed" {
		t.Errorf("projection: %+v", task)
	}
	if task.StructuredResult["outcome"] != "accepted" {
		t.Errorf("result not projected: %v", task.StructuredResult)
	}

	// errored run → failed status + failure code
	errRun := &GoalRun{Status: "completed", CallID: "call_y", Error: &GoalRunError{Code: "no_answer", Message: "no human answered"}}
	task2 := errRun.AsCallTask()
	if task2.Status != "failed" || task2.FailureCode != "no_answer" {
		t.Errorf("error projection: %+v", task2)
	}
}

func TestDryRunGoalCall(t *testing.T) {
	c := New("", "")
	c.DryRun = true
	run, err := c.CreateGoalCall(context.Background(), "goal_1", "+15550001", map[string]any{"a": 1}, "idem-2")
	if err != nil {
		t.Fatalf("dry-run goal: %v", err)
	}
	if run.Status != "completed" || run.Result["outcome"] != "unknown" {
		t.Errorf("dry-run goal run: %+v", run)
	}
}
