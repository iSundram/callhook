package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/iSundram/callhook/internal/business"
	"github.com/iSundram/callhook/internal/events"
	"github.com/iSundram/callhook/internal/session"
)

func TestJournalRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.jsonl")
	j, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}

	s := session.NewStore()
	s.SetPersister(func(sess *session.Session) { _ = j.Persist(sess) })

	ev := events.Event{ID: "evt_1", Type: "invoice.due", CustomerID: "cus_1002"}
	cust := &business.Customer{ID: "cus_1002", Name: "Daniel", Locale: "en-US", Region: "US"}
	s.Create("evt_1", ev, cust)
	s.SetCallTarget("evt_1", "+14155550002", "en-US", "US", "")
	s.Log("evt_1", "event_received", "test")

	if err := j.Close(); err != nil {
		t.Fatal(err)
	}

	restored, err := Replay(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(restored) != 1 {
		t.Fatalf("restored %d sessions, want 1", len(restored))
	}
	got := restored[0]
	if got.ID != "evt_1" || got.Phone != "+14155550002" || got.Customer.Name != "Daniel" {
		t.Errorf("restored session mismatch: %+v", got)
	}
	if len(got.Actions) == 0 {
		t.Error("audit actions were not persisted")
	}

	// Dedup keys survive restart: the same event must not call twice.
	s2 := session.NewStore()
	s2.Restore(restored)
	if !s2.AlreadySeen("evt_1") {
		t.Error("restored store should still dedupe evt_1")
	}
}

func TestReplayToleratesTornLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.jsonl")
	line := `{"ID":"evt_1","Event":{"ID":"evt_1"},"Customer":{"ID":"c1"}}` + "\n"
	torn := `{"ID":"evt_1","Even`
	if err := os.WriteFile(path, []byte(line+torn), 0o644); err != nil {
		t.Fatal(err)
	}
	restored, err := Replay(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(restored) != 1 || restored[0].ID != "evt_1" {
		t.Errorf("torn final line should be dropped, got %+v", restored)
	}
}

func TestReplayKeepsLastSnapshotPerSession(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.jsonl")
	j, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	ev := events.Event{ID: "evt_1", Type: "promo.offer", CustomerID: "cus_1001"}
	s := session.NewStore()
	s.SetPersister(func(sess *session.Session) { _ = j.Persist(sess) })
	s.Create("evt_1", ev, &business.Customer{ID: "cus_1001", Name: "Priya"})
	s.Log("evt_1", "call_placed", "call_1")
	s.Log("evt_1", "call_terminal", "completed")
	j.Close()

	restored, _ := Replay(path)
	if len(restored) != 1 {
		t.Fatalf("want 1 session, got %d", len(restored))
	}
	if len(restored[0].Actions) != 2 {
		t.Errorf("want latest snapshot with 2 actions, got %d", len(restored[0].Actions))
	}
}
