// Package retry fires future call triggers: redials for unanswered
// customers, calls deferred to polite local hours, and caller-requested
// scheduled start times. The outcome engine arms a trigger; this scheduler
// executes it when due. Retry spacing is deliberately conservative — this
// is a courtesy redial, not harassment.
package retry

import (
	"context"
	"log"
	"time"

	"github.com/iSundram/calle/internal/calleclient"
	"github.com/iSundram/calle/internal/callwindow"
	"github.com/iSundram/calle/internal/session"
)

// Scheduler polls for due triggers and places the calls.
type Scheduler struct {
	Sessions *session.Store
	Place    func(*session.Session) (*calleclient.CallTask, bool, error)
	Tick     time.Duration // poll interval, default 15s
	// EnforceWindows re-defers calls that come due outside polite hours.
	EnforceWindows bool
}

func (s *Scheduler) Run(ctx context.Context) {
	tick := s.Tick
	if tick <= 0 {
		tick = 15 * time.Second
	}
	t := time.NewTicker(tick)
	defer t.Stop()
	log.Printf("scheduler armed (every %s): retries, calling-window deferrals, scheduled starts", tick)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			for _, sess := range s.Sessions.PopDue(time.Now().UTC()) {
				s.fire(sess)
			}
		}
	}
}

func (s *Scheduler) fire(sess *session.Session) {
	now := time.Now().UTC()

	// A trigger that arrives outside polite hours gets deferred to the next
	// window — without consuming a retry attempt.
	if s.EnforceWindows && !callwindow.IsOpen(sess.Region, sess.TZ, now) {
		next := callwindow.NextOpen(sess.Region, sess.TZ, now)
		s.Sessions.DeferUntil(sess.ID, next, session.KindWindow)
		s.Sessions.Log(sess.ID, "call_deferred", "outside calling hours — will dial at "+next.Format(time.RFC3339))
		return
	}

	// Only genuine redials consume attempts; window/scheduled triggers
	// place the call at whatever attempt number we're on.
	if sess.NextRetryKind == session.KindRetry && !s.Sessions.IncrRetry(sess.ID) {
		s.Sessions.Log(sess.ID, "retry_exhausted", "no attempts remaining")
		return
	}

	_, placed, err := s.Place(sess)
	if err != nil {
		log.Printf("trigger %s (%s): %v", sess.ID, sess.NextRetryKind, err)
		s.Sessions.Log(sess.ID, "trigger_call_failed", sess.NextRetryKind+": "+err.Error())
		return
	}
	if !placed {
		return // PlaceCall already deferred it (window check inside)
	}
	if sess.NextRetryKind == session.KindRetry {
		s.Sessions.Log(sess.ID, "retry_fired", "redial placed")
	} else if sess.NextRetryKind == session.KindScheduled {
		s.Sessions.Log(sess.ID, "scheduled_fired", "scheduled call placed")
	}
}
