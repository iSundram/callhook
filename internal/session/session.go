package session

import (
	"sync"
	"time"

	"github.com/iSundram/calle/internal/business"
	"github.com/iSundram/calle/internal/calleclient"
	"github.com/iSundram/calle/internal/events"
)

// Session tracks one event's journey through the pipeline: received →
// prefetched → calling → terminal → outcome applied. Unanswered calls are
// retried up to MaxRetries times.
type Session struct {
	ID          string
	Event       events.Event
	Customer    *business.Customer
	Task        string
	Phone       string
	Locale      string
	Region      string
	ResultSchema map[string]any
	CallID      string
	CallStatus  string
	Outcome     map[string]any
	Actions     []Action
	RetryCount  int
	NextRetryAt *time.Time
	LastUpdate  time.Time
}

// Action is an audit-log entry — every business write or escalation the
// outcome engine performs is recorded here and shown live in the dashboard.
type Action struct {
	At    time.Time `json:"at"`
	Kind  string    `json:"kind"`  // e.g. prefetched, call_placed, write.mark_promise, escalate
	Detail string   `json:"detail"`
}

// Store is an in-memory session registry. (Swap for Postgres in production.)
type Store struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	seenKeys map[string]bool // idempotency
}

func NewStore() *Store {
	return &Store{
		sessions: map[string]*Session{},
		seenKeys: map[string]bool{},
	}
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

func (s *Store) Create(id string, e events.Event, c *business.Customer) *Session {
	sess := &Session{ID: id, Event: e, Customer: c, LastUpdate: time.Now().UTC()}
	s.mu.Lock()
	s.sessions[id] = sess
	s.mu.Unlock()
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
func (s *Store) SetCallTarget(id, phone, locale, region string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess, ok := s.sessions[id]; ok {
		sess.Phone, sess.Locale, sess.Region = phone, locale, region
	}
}

// SetResultSchema stores the blueprint schema for retries.
func (s *Store) SetResultSchema(id string, schema map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess, ok := s.sessions[id]; ok {
		sess.ResultSchema = schema
	}
}

// SetTask stores the composed task (used by retries).
func (s *Store) SetTask(id, task string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess, ok := s.sessions[id]; ok {
		sess.Task = task
	}
}

// RetryState returns the current retry count for a session.
func (s *Store) RetryState(id string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if sess, ok := s.sessions[id]; ok {
		return sess.RetryCount
	}
	return -1
}

// ScheduleRetry arms a retry: mark next attempt time if attempts remain.
func (s *Store) ScheduleRetry(id string, at time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess, ok := s.sessions[id]; ok && sess.RetryCount < MaxRetries {
		sess.NextRetryAt = &at
		return true
	}
	return false
}

// PopDueRetries returns sessions whose retry time has arrived, clears their
// timer, and advances their attempt counter. Callers place the new call.
func (s *Store) PopDueRetries(now time.Time) []*Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	var due []*Session
	for _, sess := range s.sessions {
		if sess.NextRetryAt != nil && !sess.NextRetryAt.After(now) && sess.RetryCount < MaxRetries {
			sess.NextRetryAt = nil
			sess.RetryCount++
			cp := *sess
			due = append(due, &cp)
		}
	}
	return due
}

// MaxRetries is the number of redials after the first attempt.
const MaxRetries = 2

// Log appends an audit action to a session.
func (s *Store) Log(id, kind, detail string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess, ok := s.sessions[id]; ok {
		sess.Actions = append(sess.Actions, Action{At: time.Now().UTC(), Kind: kind, Detail: detail})
		sess.LastUpdate = time.Now().UTC()
	}
}

// SetCall records the placed call on its session.
func (s *Store) SetCall(id string, task *calleclient.CallTask) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess, ok := s.sessions[id]; ok {
		sess.CallID = task.ID
		sess.CallStatus = task.Status
		sess.Task = task.Task
		sess.LastUpdate = time.Now().UTC()
	}
}

// SetTerminal records the terminal call state and structured result.
func (s *Store) SetTerminal(callID, status string, result map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess, ok := s.findByCallIDLocked(callID); ok {
		sess.CallStatus = status
		sess.Outcome = result
		sess.LastUpdate = time.Now().UTC()
	}
}

func (s *Store) findByCallIDLocked(callID string) (*Session, bool) {
	for _, sess := range s.sessions {
		if sess.CallID == callID {
			return sess, true
		}
	}
	return nil, false
}

// Snapshot returns a dashboard-ready copy of all sessions, newest first.
func (s *Store) Snapshot() []SessionView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]SessionView, 0, len(s.sessions))
	for _, sess := range s.sessions {
		out = append(out, SessionView{
			ID:         sess.ID,
			EventType:  sess.Event.Type,
			CustomerID: sess.Event.CustomerID,
			Customer:   derefName(sess.Customer),
			Phone:      sess.Phone,
			CallID:     sess.CallID,
			CallStatus: sess.CallStatus,
			Outcome:    sess.Outcome,
			Actions:    sess.Actions,
			Task:       sess.Task,
			RetryCount: sess.RetryCount,
			NextRetryAt: sess.NextRetryAt,
			UpdatedAt:  sess.LastUpdate,
		})
	}
	// newest first
	for i := len(out)/2 - 1; i >= 0; i-- {
		// simple insertion sort — session counts are small
		for j := i + 1; j < len(out); j++ {
			if out[j].UpdatedAt.After(out[j-1].UpdatedAt) {
				out[j], out[j-1] = out[j-1], out[j]
			} else {
				break
			}
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
	ID         string                `json:"id"`
	EventType  string                `json:"event_type"`
	CustomerID string                `json:"customer_id"`
	Customer   string                `json:"customer"`
	Phone      string                `json:"phone"`
	CallID     string                `json:"call_id"`
	CallStatus string                `json:"call_status"`
	Outcome    map[string]any        `json:"outcome"`
	Actions    []Action              `json:"actions"`
	Task       string                `json:"task"`
	RetryCount int                   `json:"retry_count"`
	NextRetryAt *time.Time           `json:"next_retry_at"`
	UpdatedAt  time.Time             `json:"updated_at"`
}
