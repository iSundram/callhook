// Package campaign implements goal-driven calling: a goal ("collect 25
// payment promises"), an audience, and a wave policy. The engine launches
// waves of calls through the standard event pipeline, tracks every outcome,
// and STOPS the moment the goal is met — never wasting a call.
package campaign

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Campaign statuses.
const (
	StatusRunning        = "running"
	StatusGoalMet        = "goal_met"
	StatusExhausted      = "exhausted"       // audience or waves spent, goal not met
	StatusBudgetSpent    = "budget_exhausted" // call budget hard stop
	StatusStopped        = "stopped"          // manually stopped
)

// Audience entry states.
const (
	EntryPending  = "pending"
	EntryInWave   = "in_wave"
	EntrySuccess  = "succeeded"
	EntryFailed   = "failed"
	EntryNoAnswer = "no_answer"
	EntrySkipped  = "skipped" // goal met / stopped before we called them
)

// Goal types: "count" needs N successes; "reach_all" needs every audience
// member to reach a terminal outcome.
const (
	GoalCount    = "count"
	GoalReachAll = "reach_all"
)

type Campaign struct {
	ID         string        `json:"id"`
	Name       string        `json:"name"`
	EventType  string        `json:"event_type"`
	Payload    json.RawMessage `json:"payload,omitempty"`
	Goal       GoalSpec      `json:"goal"`
	Audience   []AudienceEntry `json:"audience"`
	Waves      WavePolicy    `json:"waves"`
	Budget     Budget        `json:"budget"`
	Status     string        `json:"status"`
	Progress   Progress      `json:"progress"`
	WaveNumber int           `json:"wave_number"`
	WavePlaced int           `json:"wave_placed"`   // calls placed in the current wave
	WaveDone   int           `json:"wave_done"`     // terminal outcomes in the current wave
	NextWaveAt *time.Time    `json:"next_wave_at"`
	CreatedAt  time.Time     `json:"created_at"`
	CompletedAt *time.Time   `json:"completed_at,omitempty"`
	Log        []LogEntry    `json:"log,omitempty"`
}

type GoalSpec struct {
	Type            string   `json:"type"`              // count | reach_all
	Target          int      `json:"target,omitempty"`  // for count
	SuccessOutcomes []string `json:"success_outcomes"`  // which outcomes count as success
}

// Duration is a time.Duration that accepts JSON either as a string
// ("90s", "2h") or a number of seconds.
type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch x := v.(type) {
	case string:
		dd, err := time.ParseDuration(x)
		if err != nil {
			return err
		}
		d.Duration = dd
	case float64:
		d.Duration = time.Duration(x * float64(time.Second))
	default:
		return fmt.Errorf("duration must be a string like \"90s\" or seconds number")
	}
	return nil
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.Duration.String())
}

type WavePolicy struct {
	Size     int      `json:"size"`      // calls per wave
	Delay    Duration `json:"delay"`     // wait between waves ("90s", "2h")
	MaxWaves int      `json:"max_waves"` // 0 = unlimited (bounded by audience+budget)
}

type Budget struct {
	MaxCalls int `json:"max_calls"` // 0 = unlimited
}

type AudienceEntry struct {
	CustomerID string `json:"customer_id"`
	Phone      string `json:"phone,omitempty"`
	TZ         string `json:"tz,omitempty"`
	State      string `json:"state"`
	SessionID  string `json:"session_id,omitempty"` // latest session for this entry
	Rounds     int    `json:"rounds"`               // how many waves this entry has been in
}

type Progress struct {
	Successes   int `json:"successes"`
	Failures    int `json:"failures"`
	NoAnswer    int `json:"no_answer"`
	CallsPlaced int `json:"calls_placed"`
	Pending     int `json:"pending"`
}

type LogEntry struct {
	At    time.Time `json:"at"`
	Event string    `json:"event"`
	Note  string    `json:"note,omitempty"`
}

// IsSuccess reports whether an outcome counts toward the goal.
func (g GoalSpec) IsSuccess(outcome string) bool {
	for _, s := range g.SuccessOutcomes {
		if s == outcome {
			return true
		}
	}
	return false
}

// Met reports whether the goal is achieved given current progress and
// audience state.
func (c *Campaign) Met() bool {
	switch c.Goal.Type {
	case GoalCount:
		return c.Goal.Target > 0 && c.Progress.Successes >= c.Goal.Target
	case GoalReachAll:
		return c.Progress.Pending == 0
	}
	return false
}

// Store is the campaign registry with optional persistence (JSONL journal).
type Store struct {
	mu        sync.RWMutex
	campaigns map[string]*Campaign
	nextSeq   int
	onMutate  func(*Campaign)
	onNotify  func(*Campaign) // live subscribers (SSE) — fire-and-forget
}

func NewStore() *Store {
	return &Store{campaigns: map[string]*Campaign{}}
}

// SetNotifier installs a live-subscriber callback fired on every mutation.
func (s *Store) SetNotifier(fn func(*Campaign)) {
	s.mu.Lock()
	s.onNotify = fn
	s.mu.Unlock()
}

func (s *Store) SetPersister(fn func(*Campaign)) {
	s.mu.Lock()
	s.onMutate = fn
	s.mu.Unlock()
}

func (s *Store) Restore(cs []*Campaign) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range cs {
		cp := *c
		s.campaigns[c.ID] = &cp
		if c.Status == StatusRunning && c.CompletedAt != nil {
			cp.Status = StatusExhausted
		}
	}
}

func (s *Store) Create(name, eventType string, payload json.RawMessage, goal GoalSpec, audience []AudienceEntry, waves WavePolicy, budget Budget) *Campaign {
	s.mu.Lock()
	s.nextSeq++
	c := &Campaign{
		ID:        time.Now().UTC().Format("cmp_20060102_") + itoa(s.nextSeq),
		Name:      name, EventType: eventType, Payload: payload,
		Goal: goal, Audience: audience, Waves: waves, Budget: budget,
		Status: StatusRunning,
		Progress: Progress{Pending: len(audience)},
		CreatedAt: time.Now().UTC(),
	}
	if c.Waves.Size <= 0 {
		c.Waves.Size = 3
	}
	if c.Waves.Delay.Duration == 0 {
		c.Waves.Delay = Duration{10 * time.Minute}
	}
	s.campaigns[c.ID] = c
	fn := s.onMutate
	s.mu.Unlock()
	s.persist(fn, c)
	return c
}

func (s *Store) Get(id string) (*Campaign, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.campaigns[id]
	return c, ok
}

func (s *Store) List() []*Campaign {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Campaign, 0, len(s.campaigns))
	for _, c := range s.campaigns {
		out = append(out, c)
	}
	return out
}

// Stop manually halts a running campaign; remaining pending entries are
// marked skipped.
func (s *Store) Stop(id string) bool {
	ok := false
	s.mutate(id, func(c *Campaign) {
		if c.Status == StatusRunning {
			c.Status = StatusStopped
			c.skipPendingLocked()
			now := time.Now().UTC()
			c.CompletedAt = &now
			c.logLocked("stopped", "manually stopped")
			ok = true
		}
	})
	return ok
}

func (c *Campaign) skipPendingLocked() {
	for i := range c.Audience {
		if c.Audience[i].State == EntryPending {
			c.Audience[i].State = EntrySkipped
		}
	}
	c.Progress.Pending = 0
}

func (c *Campaign) logLocked(event, note string) {
	c.Log = append(c.Log, LogEntry{At: time.Now().UTC(), Event: event, Note: note})
}

func (s *Store) mutate(id string, fn func(*Campaign)) {
	s.mu.Lock()
	c, ok := s.campaigns[id]
	if ok {
		fn(c)
	}
	persist := s.onMutate
	s.mu.Unlock()
	if ok {
		s.persist(persist, c)
	}
}

func (s *Store) persist(fn func(*Campaign), c *Campaign) {
	if fn == nil {
		return
	}
	cp := *c
	fn(&cp)
	if s.onNotify != nil {
		s.onNotify(&cp)
	}
}

func itoa(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoa(n/10) + string(rune('0'+n%10))
}
