package events

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/iSundram/callhook/internal/business"
)

func testStore() business.Store { return business.NewMockStore() }

func TestRouterHasAllBlueprints(t *testing.T) {
	r := NewRouter()
	for _, typ := range []string{"invoice.due", "account.warning", "promo.offer"} {
		if _, ok := r.Blueprint(typ); !ok {
			t.Errorf("missing blueprint for %s", typ)
		}
	}
}

func TestInvoiceDueCompose(t *testing.T) {
	r := NewRouter()
	bp, _ := r.Blueprint("invoice.due")
	c := &business.Customer{ID: "cus_1002", Name: "Daniel", Plan: "Starter", Email: "d@example.com"}
	ev := &Event{ID: "e1", Type: "invoice.due", CustomerID: "cus_1002"}

	task, err := bp.Compose(c, ev, testStore())
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
	// The task must bake in the prefetched invoice data.
	for _, want := range []string{"Daniel", "Starter", "inv_5002", "USD 49.00"} {
		if !strings.Contains(task, want) {
			t.Errorf("task missing %q", want)
		}
	}
	// Behavioral rules must be present.
	if !strings.Contains(task, "do not argue") {
		t.Error("task must include the do-not-argue rule")
	}
}

func TestAccountWarningNeverAsksForSecrets(t *testing.T) {
	r := NewRouter()
	bp, _ := r.Blueprint("account.warning")
	c := &business.Customer{ID: "cus_1003", Name: "Mei", Plan: "Pro", Email: "m@example.com"}
	ev := &Event{ID: "e2", Type: "account.warning", CustomerID: "cus_1003",
		Payload: json.RawMessage(`{"reason":"login from a new country"}`)}

	task, err := bp.Compose(c, ev, testStore())
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
	if !strings.Contains(task, "Never ask them to read out credentials") {
		t.Error("security blueprint must forbid asking for credentials")
	}
	if !strings.Contains(task, "login from a new country") {
		t.Error("payload reason must be baked into the task")
	}
}

func TestPromoOfferNoDoublePush(t *testing.T) {
	r := NewRouter()
	bp, _ := r.Blueprint("promo.offer")
	c := &business.Customer{ID: "cus_1001", Name: "Priya", Plan: "Pro"}
	ev := &Event{ID: "e3", Type: "promo.offer", CustomerID: "cus_1001",
		Payload: json.RawMessage(`{"offer":"20% off","expires":"Friday"}`)}

	task, err := bp.Compose(c, ev, testStore())
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
	if !strings.Contains(task, "Never push a second time") {
		t.Error("promo blueprint must include the no-second-push rule")
	}
}

func TestBlueprintSchemasAreValid(t *testing.T) {
	r := NewRouter()
	for _, typ := range r.Supported() {
		bp, _ := r.Blueprint(typ)
		schema := bp.ResultSchema
		if schema["type"] != "object" {
			t.Errorf("%s: schema must be an object", typ)
		}
		props, ok := schema["properties"].(map[string]any)
		if !ok || props["outcome"] == nil {
			t.Errorf("%s: schema must define an outcome property", typ)
		}
		req, ok := schema["required"].([]string)
		if !ok || len(req) < 2 {
			t.Errorf("%s: schema must require outcome and summary", typ)
		}
	}
}

func TestEventValidation(t *testing.T) {
	cases := []struct {
		ev   Event
		want string // empty = valid
	}{
		{Event{ID: "a", Type: "invoice.due", CustomerID: "c"}, ""},
		{Event{Type: "invoice.due", CustomerID: "c"}, "id is required"},
		{Event{ID: "a", CustomerID: "c"}, "type is required"},
		{Event{ID: "a", Type: "invoice.due"}, "customer_id is required"},
	}
	for i, c := range cases {
		err := c.ev.Validate()
		if c.want == "" && err != nil {
			t.Errorf("case %d: unexpected error %v", i, err)
		}
		if c.want != "" && (err == nil || err.Error() != c.want) {
			t.Errorf("case %d: got %v, want %q", i, err, c.want)
		}
	}
}

func TestVariablesForEmptyWhenNil(t *testing.T) {
	bp := Blueprint{}
	if v := bp.VariablesFor(nil, nil); len(v) != 0 {
		t.Error("nil VariablesFor must return an empty map")
	}
}
