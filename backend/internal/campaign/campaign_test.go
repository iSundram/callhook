package campaign

import (
	"testing"
	"time"

	"github.com/iSundram/callhook/internal/events"
)

func newTestRunner(t *testing.T) (*Store, *Runner) {
	t.Helper()
	st := NewStore()
	fired := map[string]string{} // sessionID → event id
	r := &Runner{
		Store: st,
		Fire: func(ev *events.Event) FireResult {
			fired[ev.ID] = ev.ID
			return FireResult{Status: "call_placed", SessionID: ev.ID}
		},
		TagSession: func(sessionID, campaignID string) {},
	}
	return st, r
}

func audience(n int) []AudienceEntry {
	a := make([]AudienceEntry, n)
	for i := range a {
		a[i] = AudienceEntry{CustomerID: "cus", State: EntryPending}
	}
	return a
}

func TestGoalMetEarlyStopsAndSkips(t *testing.T) {
	st, r := newTestRunner(t)
	c := st.Create("t", "invoice.due", nil,
		GoalSpec{Type: GoalCount, Target: 2, SuccessOutcomes: []string{"payment_promised"}},
		audience(10), WavePolicy{Size: 3, Delay: Duration{time.Millisecond}, MaxWaves: 10}, Budget{})

	r.LaunchWave(c)
	if c.WavePlaced != 3 {
		t.Fatalf("wave 1 placed %d, want 3", c.WavePlaced)
	}

	// Two successes arrive — goal met, remaining 7 must be skipped.
	r.OnTerminal(c.ID, c.Audience[0].SessionID, "payment_promised")
	r.OnTerminal(c.ID, c.Audience[1].SessionID, "payment_promised")

	if c.Status != StatusGoalMet {
		t.Errorf("status = %s, want goal_met", c.Status)
	}
	if c.Progress.Pending != 0 {
		t.Errorf("pending = %d, want 0 (all skipped)", c.Progress.Pending)
	}
	skipped := 0
	for _, e := range c.Audience {
		if e.State == EntrySkipped {
			skipped++
		}
	}
	// 7 never-called entries skipped; the third in-flight entry stays in_wave.
	if skipped != 7 {
		t.Errorf("skipped = %d, want 7", skipped)
	}
}

func TestNoAnswerRequeuedForNextWave(t *testing.T) {
	st, r := newTestRunner(t)
	c := st.Create("t", "invoice.due", nil,
		GoalSpec{Type: GoalCount, Target: 3, SuccessOutcomes: []string{"payment_promised"}},
		audience(6), WavePolicy{Size: 3, Delay: Duration{time.Millisecond}}, Budget{})

	r.LaunchWave(c)
	// One no-answer, two failures.
	r.OnTerminal(c.ID, c.Audience[0].SessionID, "no_answer")
	r.OnTerminal(c.ID, c.Audience[1].SessionID, "refused")
	r.OnTerminal(c.ID, c.Audience[2].SessionID, "disputed")

	if c.Status != StatusRunning {
		t.Fatalf("status = %s, want running (wave armed)", c.Status)
	}
	if c.NextWaveAt == nil {
		t.Fatal("next wave not armed")
	}
	if c.Audience[0].State != EntryPending {
		t.Errorf("no_answer entry should requeue to pending, got %s", c.Audience[0].State)
	}
	if c.Audience[1].State != EntryFailed {
		t.Errorf("refused entry should be failed, got %s", c.Audience[1].State)
	}
}

func TestBudgetHardStop(t *testing.T) {
	st, r := newTestRunner(t)
	c := st.Create("t", "invoice.due", nil,
		GoalSpec{Type: GoalCount, Target: 5, SuccessOutcomes: []string{"payment_promised"}},
		audience(9), WavePolicy{Size: 3, Delay: Duration{time.Millisecond}}, Budget{MaxCalls: 3})

	r.LaunchWave(c)
	r.OnTerminal(c.ID, c.Audience[0].SessionID, "refused")
	r.OnTerminal(c.ID, c.Audience[1].SessionID, "refused")
	r.OnTerminal(c.ID, c.Audience[2].SessionID, "refused")

	if c.Status != StatusBudgetSpent {
		t.Errorf("status = %s, want budget_exhausted", c.Status)
	}
	if c.Progress.CallsPlaced != 3 {
		t.Errorf("calls placed = %d, want 3", c.Progress.CallsPlaced)
	}
}

func TestAudienceExhausted(t *testing.T) {
	st, r := newTestRunner(t)
	c := st.Create("t", "promo.offer", nil,
		GoalSpec{Type: GoalCount, Target: 5, SuccessOutcomes: []string{"accepted"}},
		audience(2), WavePolicy{Size: 3, Delay: Duration{time.Millisecond}}, Budget{})

	r.LaunchWave(c)
	r.OnTerminal(c.ID, c.Audience[0].SessionID, "declined")
	r.OnTerminal(c.ID, c.Audience[1].SessionID, "declined")

	if c.Status != StatusExhausted {
		t.Errorf("status = %s, want exhausted", c.Status)
	}
}

func TestMaxWavesStops(t *testing.T) {
	st, r := newTestRunner(t)
	c := st.Create("t", "promo.offer", nil,
		GoalSpec{Type: GoalCount, Target: 9, SuccessOutcomes: []string{"accepted"}},
		audience(9), WavePolicy{Size: 3, Delay: Duration{time.Millisecond}, MaxWaves: 1}, Budget{})

	r.LaunchWave(c)
	r.OnTerminal(c.ID, c.Audience[0].SessionID, "declined")
	r.OnTerminal(c.ID, c.Audience[1].SessionID, "declined")
	r.OnTerminal(c.ID, c.Audience[2].SessionID, "declined")

	// Next pump should refuse to launch wave 2.
	r.Pump(time.Now().UTC().Add(time.Hour))
	if c.Status != StatusExhausted {
		t.Errorf("status = %s, want exhausted after max waves", c.Status)
	}
}

func TestStopSkipsPending(t *testing.T) {
	st, _ := newTestRunner(t)
	c := st.Create("t", "promo.offer", nil,
		GoalSpec{Type: GoalCount, Target: 9, SuccessOutcomes: []string{"accepted"}},
		audience(9), WavePolicy{Size: 3, Delay: Duration{time.Second}}, Budget{})
	if !st.Stop(c.ID) {
		t.Fatal("Stop should succeed on a running campaign")
	}
	if c.Status != StatusStopped || c.Progress.Pending != 0 {
		t.Errorf("status=%s pending=%d, want stopped/0", c.Status, c.Progress.Pending)
	}
}
