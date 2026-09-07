package session

import (
	"testing"
	"time"

	"github.com/iSundram/callhook/internal/business"
	"github.com/iSundram/callhook/internal/events"
)

func newStoreWithSession(id string) *Store {
	s := NewStore()
	ev := events.Event{ID: id, Type: "invoice.due", CustomerID: "cus_1002"}
	s.Create(id, ev, &business.Customer{ID: "cus_1002", Name: "Daniel", Region: "US"})
	return s
}

func TestDedup(t *testing.T) {
	s := newStoreWithSession("e1")
	if s.AlreadySeen("e1") {
		t.Error("first sight should be unseen")
	}
	if !s.AlreadySeen("e1") {
		t.Error("second sight must be seen")
	}
}

func TestRetryLifecycle(t *testing.T) {
	s := newStoreWithSession("e1")
	at := time.Now().Add(time.Hour)

	if !s.ScheduleRetry("e1", at) {
		t.Fatal("retry should arm (attempts remain)")
	}
	got, ok := s.Get("e1")
	if !ok || got.NextRetryAt == nil || got.NextRetryKind != KindRetry {
		t.Fatal("retry not armed")
	}

	// PopDue before the time → nothing
	if due := s.PopDue(time.Now()); len(due) != 0 {
		t.Errorf("early PopDue returned %d, want 0", len(due))
	}
	// PopDue after → one, kind preserved
	due := s.PopDue(at.Add(time.Minute))
	if len(due) != 1 || due[0].NextRetryKind != KindRetry {
		t.Fatalf("PopDue: %+v", due)
	}
	// timer cleared
	got, _ = s.Get("e1")
	if got.NextRetryAt != nil {
		t.Error("timer must be cleared after PopDue")
	}
	// IncrRetry consumes the attempt and clears state
	if !s.IncrRetry("e1") {
		t.Error("IncrRetry should succeed")
	}
	got, _ = s.Get("e1")
	if got.RetryCount != 1 || got.NextRetryAt != nil {
		t.Errorf("after IncrRetry: count=%d next=%v", got.RetryCount, got.NextRetryAt)
	}
}

func TestRetryExhaustion(t *testing.T) {
	s := newStoreWithSession("e1")
	for i := 0; i < MaxRetries; i++ {
		s.IncrRetry("e1")
	}
	if s.ScheduleRetry("e1", time.Now()) {
		t.Error("ScheduleRetry must refuse when attempts are exhausted")
	}
	if s.IncrRetry("e1") {
		t.Error("IncrRetry must refuse when exhausted")
	}
}

func TestWindowDeferDoesNotConsumeRetry(t *testing.T) {
	s := newStoreWithSession("e1")
	at := time.Now().Add(time.Hour)
	s.DeferUntil("e1", at, KindWindow)
	due := s.PopDue(at.Add(time.Minute))
	if len(due) != 1 || due[0].NextRetryKind != KindWindow {
		t.Fatalf("window trigger: %+v", due)
	}
	got, _ := s.Get("e1")
	if got.RetryCount != 0 {
		t.Errorf("window deferral must not consume retries, got %d", got.RetryCount)
	}
}

func TestFindByCallID(t *testing.T) {
	s := newStoreWithSession("e1")
	s.SetCallID("e1", "call_x")
	if _, ok := s.FindByCallID("call_x"); !ok {
		t.Error("FindByCallID should find call_x")
	}
	if _, ok := s.FindByCallID("call_y"); ok {
		t.Error("FindByCallID must not find unknown ids")
	}
}

func TestCampaignTagging(t *testing.T) {
	s := newStoreWithSession("e1")
	s.SetCampaign("e1", "cmp_1")
	got, _ := s.Get("e1")
	if got.CampaignID != "cmp_1" {
		t.Error("campaign tag not stored")
	}
}

func TestRestoreRebuildsDedup(t *testing.T) {
	s := newStoreWithSession("e1")
	cp, _ := s.Get("e1")
	restored := []*Session{cp}
	s2 := NewStore()
	s2.Restore(restored)
	if !s2.AlreadySeen("e1") {
		t.Error("restored store must still dedupe the event id")
	}
	if _, ok := s2.Get("e1"); !ok {
		t.Error("restored store must contain the session")
	}
}
