// Package store provides crash-safe session persistence with zero external
// dependencies: an append-only JSONL journal holding a full session snapshot
// per mutation. Replay on boot keeps the last snapshot per session. Good for
// hackathon scale; swap for Postgres when volume demands it.
package store

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/iSundram/calle/internal/session"
)

// Journal appends session snapshots to a JSONL file.
type Journal struct {
	mu   sync.Mutex
	path string
	f    *os.File
}

// Open creates the journal file (and its parent directory).
func Open(path string) (*Journal, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	return &Journal{path: path, f: f}, nil
}

// Persist marshals the session and appends one line. Errors are returned to
// the caller; the store treats persistence as best-effort.
func (j *Journal) Persist(sess *session.Session) error {
	if j == nil {
		return nil
	}
	b, err := json.Marshal(sess)
	if err != nil {
		return err
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	_, err = j.f.Write(append(b, '\n'))
	return err
}

// Replay returns the last snapshot for each persisted session, in the order
// they first appeared.
func Replay(path string) ([]*session.Session, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	order := []string{}
	last := map[string]*session.Session{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var sess session.Session
		if err := json.Unmarshal(line, &sess); err != nil {
			continue // tolerate a torn final line after a crash
		}
		if _, seen := last[sess.ID]; !seen {
			order = append(order, sess.ID)
		}
		last[sess.ID] = &sess
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	out := make([]*session.Session, 0, len(order))
	for _, id := range order {
		out = append(out, last[id])
	}
	return out, nil
}

// Close flushes and closes the journal.
func (j *Journal) Close() error {
	if j == nil {
		return nil
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.f.Close()
}
