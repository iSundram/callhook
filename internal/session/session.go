package session

import (
	"sync"
	"time"

	"github.com/iSundram/callhook/internal/business"
	"github.com/iSundram/callhook/internal/callhookclient"
	"github.com/iSundram/callhook/internal/events"
)

// MaxRetries is the number of redials after the first attempt.
const MaxRetries = 2

// Defer kinds: why a session is waiting for a future trigger.
const (
	KindRetry     = "retry"     // no answer — redial later
	KindWindow    = "window"    // polite calling hours — dial when they open
	KindScheduled = "scheduled" // caller asked the call to start not before a time
)

// Session tracks one event's journey through the pipeline: received →
// (deferred) → prefetched → calling → terminal → outcome applied.
type Session struct {
	ID           string
	Event        events.Event
	Customer     *business.Customer
	Task         string
	Phone        string
	Locale       string
	Region       string
	TZ           string // timezone override; empty = region default
	ResultSchema map[string]any
	GoalID       string
	GoalVariables map[string]any
	CallID       string
	CallStatus   string
	Outcome      map[string]any
	Transcript   []callhookclient.Turn
	Actions      []Action
	RetryCount   int
	NextRetryAt  *time.Time
	NextRetryKind string
	LastUpdate   time.Time
}

// Action is an audit-log entry — every business write or escalation the
// outcome engine performs is recorded here and shown live in the dashboard.
type Action struct {
	At     time.Time `json:"at"`
	Kind   string    `json:"kind"`
	Detail string    `json:"detail"`
}

// Store is the session registry. If a persist callback is set, every
// mutation is journaled so state survives restarts (see internal/store).
type Store struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	seenKeys map[string]bool
	onMutate func(*Session)
}

func NewStore() *Store {
	return &Store{
		sessions: map[string]*Session{},
		seenKeys: map[string]bool{},
	}
}

// SetPersister installs a callback invoked (outside the store lock) after
// every mutation, with a snapshot of the changed session.
func (s *Store) SetPersister(fn func(*Session)) {
	s.mu.Lock()
	s.onMutate = fn
	s.mu.Unlock()
}

// AlreadySeen reports whether an idempotency key was processed, and marks it.
func (s *Store) AlreadySeen(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.seenKeys[key] {
		return true
	}
	s.seenKeys[key] = true
	return false
}

// Restore loads persisted sessions at boot. Keys are re-marked as seen so
// replayed events stay deduplicated across restarts.
func (s *Store) Restore(sesss []*Session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sess := range sesss {
		cp := *sess
		s.sessions[sess.ID] = &cp
		s.seenKeys[sess.Event.ID] = true
		if sess.Event.Idempotency != "" {
			s.seenKeys[sess.Event.Idempotency] = true
		}
	}
}

func (s *Store) Create(id string, e events.Event, c *business.Customer) *Session {
	sess := &Session{ID: id, Event: e, Customer: c, LastUpdate: time.Now().UTC()}
	s.mu.Lock()
	s.sessions[id] = sess
	fn := s.onMutate
	s.mu.Unlock()
	s.persist(fn, sess)
	return sess
}

func (s *Store) Get(id string) (*Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.sessions[id]
	return sess, ok
}

// FindByCallID locates a session by its CALL-E call id (webhook path).
func (s *Store) FindByCallID(callID string) (*Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, sess := range s.sessions {
		if sess.CallID == callID {
			return sess, true
		}
	}
	return nil, false
}

// SetCallTarget records who gets called and how.
func (s *Store) SetCallTarget(id, phone, locale, region, tz string) {
	s.mutate(id, func(sess *Session) {
		sess.Phone, sess.Locale, sess.Region, sess.TZ = phone, locale, region, tz
	})
}

// SetResultSchema stores the blueprint schema for retries.
func (s *Store) SetResultSchema(id string, schema map[string]any) {
	s.mutate(id, func(sess *Session) { sess.ResultSchema = schema })
}

// SetGoal pins this session to a published CALL-E goal (versioned workflow)
// instead of a free-text task.
func (s *Store) SetGoal(id, goalID string, vars map[string]any) {
	s.mutate(id, func(sess *Session) {
		sess.GoalID = goalID
		sess.GoalVariables = vars
	})
}

// SetTask stores the composed task (used by retries).
func (s *Store) SetTask(id, task string) {
	s.mutate(id, func(sess *Session) { sess.Task = task })
}

// RetryState returns the current retry count for a session (-1 if missing).
func (s *Store) RetryState(id string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if sess, ok := s.sessions[id]; ok {
		return sess.RetryCount
	}
	return -1
}

// ScheduleRetry arms a redial if attempts remain.
func (s *Store) ScheduleRetry(id string, at time.Time) bool {
	ok := false
	s.mutate(id, func(sess *Session) {
		if sess.RetryCount < MaxRetries {
			sess.NextRetryAt = &at
			sess.NextRetryKind = KindRetry
			ok = true
		}
	})
	return ok
}

// DeferUntil schedules a future trigger that does not consume a retry
// attempt (calling-window deferral or caller-requested start time).
func (s *Store) DeferUntil(id string, at time.Time, kind string) {
	s.mutate(id, func(sess *Session) {
		sess.NextRetryAt = &at
		sess.NextRetryKind = kind
	})
}

// PopDue returns sessions whose trigger time has arrived, clearing the
// timer without advancing the attempt counter. Callers decide whether the
// trigger actually fires (e.g. window still closed → defer again).
func (s *Store) PopDue(now time.Time) []*Session {
	s.mu.Lock()
	var due []*Session
	for _, sess := range s.sessions {
		if sess.NextRetryAt != nil && !sess.NextRetryAt.After(now) {
			kind := sess.NextRetryKind
			sess.NextRetryAt = nil
			sess.NextRetryKind = ""
			cp := *sess
			cp.NextRetryKind = kind // scheduler needs to know why it fired
			due = append(due, &cp)
		}
	}
	fn := s.onMutate
	s.mu.Unlock()
	for _, sess := range due {
		s.persist(fn, sess)
	}
	return due
}

// IncrRetry consumes one retry attempt and clears the armed trigger.
// Returns false when attempts are exhausted.
func (s *Store) IncrRetry(id string) bool {
	ok := false
	s.mutate(id, func(sess *Session) {
		if sess.RetryCount < MaxRetries {
			sess.RetryCount++
			sess.NextRetryAt = nil
			sess.NextRetryKind = ""
			ok = true
		}
	})
	return ok
}

// SetCallID records the placed call id (test/helper path).
func (s *Store) SetCallID(id, callID string) {
	s.mutate(id, func(sess *Session) { sess.CallID = callID })
}

// Log appends an audit action to a session.
func (s *Store) Log(id, kind, detail string) {
	s.mutate(id, func(sess *Session) {
		sess.Actions = append(sess.Actions, Action{At: time.Now().UTC(), Kind: kind, Detail: detail})
		sess.LastUpdate = time.Now().UTC()
	})
}

// SetCall records the placed call on its session.
func (s *Store) SetCall(id string, task *callhookclient.CallTask) {
	s.mutate(id, func(sess *Session) {
		sess.CallID = task.ID
		sess.CallStatus = task.Status
		sess.Task = task.Task
		sess.LastUpdate = time.Now().UTC()
	})
}

// SetTerminal records the terminal call state, structured result and
// transcript.
func (s *Store) SetTerminal(callID, status string, result map[string]any, transcript []callhookclient.Turn) {
	s.mu.Lock()
	for _, sess := range s.sessions {
		if sess.CallID == callID {
			sess.CallStatus = status
			sess.Outcome = result
			sess.Transcript = transcript
			sess.LastUpdate = time.Now().UTC()
			fn := s.onMutate
			s.mu.Unlock()
			s.persist(fn, sess)
			return
		}
	}
	fn := s.onMutate
	s.mu.Unlock()
	_ = fn
}

func (s *Store) mutate(id string, fn func(*Session)) {
	s.mu.Lock()
	sess, ok := s.sessions[id]
	if ok {
		fn(sess)
		sess.LastUpdate = time.Now().UTC()
	}
	persist := s.onMutate
	s.mu.Unlock()
	if ok {
		s.persist(persist, sess)
	}
}

// persist invokes the journal callback with a copy, outside the lock.
func (s *Store) persist(fn func(*Session), sess *Session) {
	if fn == nil {
		return
	}
	cp := *sess
	fn(&cp)
}

// Snapshot returns a dashboard-ready copy of all sessions, newest first.
func (s *Store) Snapshot() []SessionView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]SessionView, 0, len(s.sessions))
	for _, sess := range s.sessions {
		out = append(out, SessionView{
			ID:            sess.ID,
			EventType:     sess.Event.Type,
			CustomerID:    sess.Event.CustomerID,
			Customer:      derefName(sess.Customer),
			Phone:         sess.Phone,
			CallID:        sess.CallID,
			CallStatus:    sess.CallStatus,
			Outcome:       sess.Outcome,
			Actions:       sess.Actions,
			Task:          sess.Task,
			Transcript:    sess.Transcript,
			RetryCount:    sess.RetryCount,
			NextRetryAt:   sess.NextRetryAt,
			NextRetryKind: sess.NextRetryKind,
			UpdatedAt:     sess.LastUpdate,
		})
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].UpdatedAt.After(out[j-1].UpdatedAt); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

func derefName(c *business.Customer) string {
	if c == nil {
		return ""
	}
	return c.Name
}

// SessionView is the JSON shape served to the dashboard.
type SessionView struct {
	ID            string         `json:"id"`
	EventType     string         `json:"event_type"`
	CustomerID    string         `json:"customer_id"`
	Customer      string         `json:"customer"`
	Phone         string         `json:"phone"`
	CallID        string         `json:"call_id"`
	CallStatus    string         `json:"call_status"`
	Outcome       map[string]any `json:"outcome"`
	Actions       []Action       `json:"actions"`
	Task          string         `json:"task"`
	Transcript    []callhookclient.Turn `json:"transcript"`
	RetryCount    int            `json:"retry_count"`
	NextRetryAt   *time.Time     `json:"next_retry_at"`
	NextRetryKind string         `json:"next_retry_kind"`
	UpdatedAt     time.Time      `json:"updated_at"`
}
