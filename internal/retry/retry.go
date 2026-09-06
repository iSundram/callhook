// Package retry redials customers who did not answer. The outcome engine
// arms a retry (no_answer / failed call); this scheduler fires it when due.
// Retry spacing is deliberately conservative — this is a courtesy redial,
// not harassment.
package retry

import (
	"context"
	"log"
	"time"

	"github.com/iSundram/calle/internal/calleclient"
	"github.com/iSundram/calle/internal/session"
)

// Scheduler polls for due retries and places the redial.
type Scheduler struct {
	Sessions *session.Store
	Place    func(*session.Session) (*calleclient.CallTask, error)
	Tick     time.Duration // poll interval, default 15s
}

func (s *Scheduler) Run(ctx context.Context) {
	tick := s.Tick
	if tick <= 0 {
		tick = 15 * time.Second
	}
	t := time.NewTicker(tick)
	defer t.Stop()
	log.Printf("retry scheduler armed (every %s)", tick)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			for _, sess := range s.Sessions.PopDueRetries(time.Now().UTC()) {
				if _, err := s.Place(sess); err != nil {
					log.Printf("retry %s: %v", sess.ID, err)
					s.Sessions.Log(sess.ID, "retry_call_failed", err.Error())
				}
			}
		}
	}
}
