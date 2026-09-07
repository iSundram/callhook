package retry

import (
	"context"
	"testing"
	"time"

	"github.com/iSundram/callhook/internal/callhookclient"
	"github.com/iSundram/callhook/internal/events"
	"github.com/iSundram/callhook/internal/session"
)

// runScheduler starts the scheduler with a 1ms tick and returns a stop func.
func runScheduler(s *Scheduler) (stop func()) {
	ctx, cancel := context.WithCancel(context.Background())
	go s.Run(ctx)
	return cancel
}

func waitUntil(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition never met")
}

func TestSchedulerFiresDueRetry(t *testing.T) {
	sessions := session.NewStore()
	sessions.Create("r1", events.Event{ID: "r1", Type: "invoice.due"}, nil)
	sessions.SetCallID("r1", "call_r1")

	placed := 0
	s := &Scheduler{
		Sessions: sessions,
		Place: func(sess *session.Session) (*callhookclient.CallTask, bool, error) {
			placed++
			return &callhookclient.CallTask{ID: "call_r1_again", Status: "completed"}, true, nil
		},
		Tick: time.Millisecond,
	}

	// Arm a retry due immediately.
	sessions.ScheduleRetry("r1", time.Now().UTC().Add(-time.Minute))
	stop := runScheduler(s)
	defer stop()

	waitUntil(t, 2*time.Second, func() bool { return placed >= 1 })
	// RetryCount consumed by the fire.
	if got := sessions.RetryState("r1"); got != 1 {
		t.Errorf("retry count = %d, want 1", got)
	}
	// No double-fire: give it extra ticks and re-check.
	time.Sleep(50 * time.Millisecond)
	if placed != 1 {
		t.Errorf("double fire! placed = %d, want 1", placed)
	}
}

func TestSchedulerDefersOutsideWindows(t *testing.T) {
	sessions := session.NewStore()
	sessions.Create("w1", events.Event{ID: "w1", Type: "invoice.due"}, nil)
	sessions.SetCallTarget("w1", "+15550001", "en-US", "US", "")

	placed := 0
	s := &Scheduler{
		Sessions:       sessions,
		EnforceWindows: true,
		Place: func(sess *session.Session) (*callhookclient.CallTask, bool, error) {
			placed++
			return nil, true, nil
		},
		Tick: time.Millisecond,
	}
	// Sunday 10:00 UTC = night/weekend in New York → must defer.
	sessions.ScheduleRetry("w1", time.Now().UTC().Add(-time.Minute))
	stop := runScheduler(s)
	defer stop()

	waitUntil(t, 2*time.Second, func() bool {
		got, ok := sessions.Get("w1")
		return ok && got.NextRetryKind == session.KindWindow
	})
	if placed != 0 {
		t.Fatalf("placed = %d, want 0 (deferred outside window)", placed)
	}
	got, _ := sessions.Get("w1")
	if got.RetryCount != 0 {
		t.Errorf("deferral must not consume retries, got %d", got.RetryCount)
	}
}

func TestSchedulerRunStopsOnCancel(t *testing.T) {
	sessions := session.NewStore()
	s := &Scheduler{
		Sessions: sessions,
		Place:    func(*session.Session) (*callhookclient.CallTask, bool, error) { return nil, true, nil },
		Tick:     time.Millisecond,
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.Run(ctx); close(done) }()
	cancel()
	select {
	case <-done:
		// ok
	case <-time.After(time.Second):
		t.Fatal("Run did not stop on cancel")
	}
}
