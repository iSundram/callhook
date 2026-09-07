package campaign

import (
	"fmt"
	"log"
	"time"

	"github.com/iSundram/callhook/internal/events"
)

// FireResult is what the pipeline reports back for one fired event.
type FireResult struct {
	Status    string // call_placed | scheduled | deferred | duplicate | error
	SessionID string
	CallID    string
	Err       error
}

// Runner launches waves and reacts to terminal outcomes. The wave lifecycle:
// launch → sessions fire through the standard pipeline → terminal outcomes
// arrive via OnTerminal → when the wave is fully terminal, evaluate the goal:
// met → stop (skip the rest, save the budget); otherwise arm the next wave
// after the policy delay.
type Runner struct {
	Store *Store
	// Fire pushes one event through the standard pipeline.
	Fire func(ev *events.Event) FireResult
	// TagSession marks a session as belonging to a campaign.
	TagSession func(sessionID, campaignID string)
	// MaxConcurrent caps in-flight calls per wave (CALL-E rejects bursts
	// with call_not_ready — learned the hard way in live testing).
	MaxConcurrent int
	// Stagger between individual calls inside a wave.
	Stagger time.Duration
}

// LaunchWave starts the next wave for a running campaign whose previous
// wave is complete.
func (r *Runner) LaunchWave(c *Campaign) {
	if c.Status != StatusRunning {
		return
	}
	if c.WavePlaced > c.WaveDone {
		return // wave still in flight
	}

	size := c.Waves.Size
	if r.MaxConcurrent > 0 && size > r.MaxConcurrent {
		size = r.MaxConcurrent
	}
	if c.Waves.MaxWaves > 0 && c.WaveNumber >= c.Waves.MaxWaves {
		r.finish(c, StatusExhausted, "max waves reached")
		return
	}

	placed := 0
	var idxs []int
	r.Store.mutate(c.ID, func(cc *Campaign) {
		for i := range cc.Audience {
			if placed >= size {
				break
			}
			if cc.Audience[i].State != EntryPending {
				continue
			}
			if cc.Budget.MaxCalls > 0 && cc.Progress.CallsPlaced+placed >= cc.Budget.MaxCalls {
				break
			}
			cc.Audience[i].State = EntryInWave
			cc.Audience[i].Rounds++
			idxs = append(idxs, i)
			placed++
		}
	})
	if placed == 0 {
		// Nothing pending: either done or nothing left to do.
		r.Store.mutate(c.ID, func(cc *Campaign) {
			if cc.Progress.Pending == 0 {
				r.finish(cc, StatusExhausted, "audience exhausted")
			} else {
				r.finish(cc, StatusBudgetSpent, "call budget exhausted")
			}
		})
		return
	}

	r.Store.mutate(c.ID, func(cc *Campaign) {
		cc.WaveNumber++
		cc.WavePlaced = placed
		cc.WaveDone = 0
		cc.NextWaveAt = nil
		cc.logLocked("wave_launched", fmt.Sprintf("wave %d: %d calls", cc.WaveNumber, placed))
	})

	// Fire each audience entry through the standard pipeline.
	succeeded := 0
	for _, i := range idxs {
		entry := c.Audience[i]
		ev := &events.Event{
			ID:         fmt.Sprintf("%s_w%d_e%d", c.ID, c.WaveNumber, i),
			Type:       c.EventType,
			CustomerID: entry.CustomerID,
			Phone:      entry.Phone,
			TZ:         entry.TZ,
			Payload:    c.Payload,
		}
		res := r.Fire(ev)
		if res.Err != nil || res.Status == "error" {
			// Fire failed — return the entry to pending for a later wave.
			r.Store.mutate(c.ID, func(cc *Campaign) {
				cc.Audience[i].State = EntryPending
				cc.logLocked("fire_failed", entry.CustomerID+": "+errString(res.Err))
			})
			continue
		}
		succeeded++
		if res.SessionID != "" && r.TagSession != nil {
			r.TagSession(res.SessionID, c.ID)
		}
		r.Store.mutate(c.ID, func(cc *Campaign) {
			cc.Audience[i].SessionID = res.SessionID
			cc.Progress.CallsPlaced++
		})
		if r.Stagger > 0 {
			time.Sleep(r.Stagger)
		}
	}
	r.Store.mutate(c.ID, func(cc *Campaign) {
		cc.WavePlaced = succeeded
		if succeeded == 0 {
			// Every fire failed (e.g. API down) — re-arm a wave later.
			next := time.Now().UTC().Add(2 * time.Minute)
			cc.NextWaveAt = &next
			cc.WaveNumber--
			cc.logLocked("wave_retried", "all fires failed — retrying in 2m")
		}
	})
}

// OnTerminal records one terminal outcome (called by the outcome engine)
// and evaluates the goal. This is where early-stop happens: the moment the
// goal is met, pending entries are skipped — no further calls are placed.
func (r *Runner) OnTerminal(campaignID, sessionID, outcome string) {
	c, ok := r.Store.Get(campaignID)
	if !ok || c.Status != StatusRunning {
		return
	}

	r.Store.mutate(campaignID, func(cc *Campaign) {
		for i := range cc.Audience {
			if cc.Audience[i].SessionID != sessionID {
				continue
			}
			switch {
			case cc.Goal.IsSuccess(outcome):
				cc.Audience[i].State = EntrySuccess
				cc.Progress.Successes++
			case outcome == "no_answer" || outcome == "":
				cc.Audience[i].State = EntryNoAnswer
				cc.Progress.NoAnswer++
			default:
				cc.Audience[i].State = EntryFailed
				cc.Progress.Failures++
			}
			cc.Progress.Pending = cc.countLocked(EntryPending)
			cc.WaveDone++
			break
		}

		// Early stop: goal met — skip everyone we haven't called yet.
		if cc.Met() {
			cc.skipPendingLocked()
			cc.Status = StatusGoalMet
			now := time.Now().UTC()
			cc.CompletedAt = &now
			cc.logLocked("goal_met", fmt.Sprintf("%d successes in %d call(s)", cc.Progress.Successes, cc.Progress.CallsPlaced))
			return
		}

		// Wave complete → arm the next one (or finish).
		if cc.WaveDone >= cc.WavePlaced && cc.WavePlaced > 0 {
			pending := cc.countLocked(EntryPending)
			if pending == 0 {
				cc.Status = StatusExhausted
				now := time.Now().UTC()
				cc.CompletedAt = &now
				cc.skipPendingLocked()
				cc.logLocked("finished", "audience exhausted before goal")
				return
			}
			if cc.Budget.MaxCalls > 0 && cc.Progress.CallsPlaced >= cc.Budget.MaxCalls {
				cc.Status = StatusBudgetSpent
				now := time.Now().UTC()
				cc.CompletedAt = &now
				cc.skipPendingLocked()
				cc.logLocked("finished", "call budget exhausted")
				return
			}
			next := time.Now().UTC().Add(cc.Waves.Delay.Duration)
			cc.NextWaveAt = &next
			// no_answer entries go back to pending for the next wave
			// (bounded by Rounds: at most 2 rounds per person).
			for i := range cc.Audience {
				if cc.Audience[i].State == EntryNoAnswer && cc.Audience[i].Rounds < 2 {
					cc.Audience[i].State = EntryPending
				}
			}
			cc.Progress.Pending = cc.countLocked(EntryPending)
			cc.logLocked("wave_complete", fmt.Sprintf("wave %d done — next wave at %s", cc.WaveNumber, next.Format(time.Kitchen)))
		}
	})
}

func (r *Runner) finish(c *Campaign, status, note string) {
	r.Store.mutate(c.ID, func(cc *Campaign) {
		if cc.Status != StatusRunning {
			return
		}
		cc.Status = status
		now := time.Now().UTC()
		cc.CompletedAt = &now
		cc.skipPendingLocked()
		cc.logLocked("finished", note)
	})
}

func (c *Campaign) countLocked(state string) int {
	n := 0
	for i := range c.Audience {
		if c.Audience[i].State == state {
			n++
		}
	}
	return n
}

func errString(err error) string {
	if err == nil {
		return "unknown"
	}
	return err.Error()
}

// Pump is the scheduler entry: launches waves for every running campaign
// whose wave is due (initial start or post-delay). Call it on a ticker.
func (r *Runner) Pump(now time.Time) {
	for _, c := range r.Store.List() {
		if c.Status != StatusRunning {
			continue
		}
		if c.NextWaveAt != nil && c.NextWaveAt.After(now) {
			continue
		}
		r.LaunchWave(c)
	}
	log.Printf("campaign pump: %d running", r.countRunning())
}

func (r *Runner) countRunning() int {
	n := 0
	for _, c := range r.Store.List() {
		if c.Status == StatusRunning {
			n++
		}
	}
	return n
}
