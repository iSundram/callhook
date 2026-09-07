package integrations

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
)

// maxBody bounds webhook payload reads (defense against memory abuse).
const maxBody = 5 << 20 // 5 MiB

// Adapter is one platform's webhook endpoint: it receives the raw request
// (with body already buffered) and returns the events to fire, or an error
// which the wrapper translates to a 4xx/5xx response.
//
// Adapters only parse and map — they never touch the pipeline. The wiring
// layer invokes Fire for each returned event.
type Adapter interface {
	// Platform name, e.g. "stripe" — used in routes and docs.
	Platform() string
	// Handle verifies and maps. It MUST return nil events with a nil error
	// for benign skippable webhooks (e.g. events the adapter is not
	// subscribed to) so the wrapper can still answer 200 quickly.
	Handle(r *http.Request, body []byte) ([]Event, error)
}

// Handler wraps an adapter into an http.HandlerFunc: buffers the raw body,
// delegates verification+mapping, fires events, and answers 2xx fast so the
// platform's retry policy never kicks in for slow pipelines.
//
// The Fire function returning an error yields a 500 — for platforms that
// retry (Stripe, GitHub, Slack all do), that is the correct "try me again"
// signal. Verification failures yield 401/400, which most platforms treat
// as permanent failures (also correct: a forged payload will never verify).
func Handler(a Adapter, fire Fire) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, maxBody))
		if err != nil {
			writeErr(w, http.StatusBadRequest, "read body: "+err.Error())
			return
		}

		events, err := a.Handle(r, body)
		if err != nil {
			// Slash-command adapters may want a friendly usage message
			// instead of a raw error status.
			if u, ok := err.(interface{ usageJSON() string }); ok {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(u.usageJSON()))
				return
			}
			// Adapters may send a *httpError for precise statuses.
			if he, ok := err.(*httpError); ok {
				if he.docs != "" {
					writeErrDoc(w, he.code, he.msg, he.docs)
				} else {
					writeErr(w, he.code, he.msg)
				}
				return
			}
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}

		for _, ev := range events {
			if err := fire(ev); err != nil {
				writeErr(w, http.StatusInternalServerError, "fire: "+err.Error())
				return
			}
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}
}

// httpError lets adapters choose the exact response status, and attach
// the docs page that explains how to fix the problem.
type httpError struct {
	code int
	msg  string
	// docs, when set, is sent as a "docs" field in the error body.
	docs string
}

func (e *httpError) Error() string { return e.msg }

// ErrUnauthorized is the canonical "signature verification failed" error.
func ErrUnauthorized(msg string) error { return &httpError{code: 401, msg: msg} }

// ErrUnauthorizedDoc is a 401 that points at the doc that fixes it.
func ErrUnauthorizedDoc(msg, docsURL string) error {
	return &httpError{code: 401, msg: msg, docs: docsURL}
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// writeErrDoc writes an error with a docs URL that explains the fix.
// All paths are on the public docs site; callers pass the anchor.
func writeErrDoc(w http.ResponseWriter, code int, msg, docsPath string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": msg,
		"docs":  docsURL(docsPath),
	})
}

// docsBase is the public docs site. Overridable for self-hosted docs.
var docsBase = func() string {
	if v := os.Getenv("CALLHOOK_DOCS_URL"); v != "" {
		return v
	}
	return "https://callhook.github.io"
}()

func docsURL(path string) string {
	if path == "" || strings.HasPrefix(path, "http") {
		return path
	}
	return docsBase + path
}

// payloadJSON wraps a map[string]any as JSON payload bytes, or nil when empty.
func payloadJSON(m map[string]any) string {
	if len(m) == 0 {
		return ""
	}
	b, _ := json.Marshal(m)
	return string(b)
}

// platformGuide is the docs path for one platform's setup guide.
// Every "not configured" error carries it, so a failed delivery
// tells the operator exactly where the fix lives.
func platformGuide(platform string) string {
	return "/integrations/" + platform + "/#setup"
}

// ErrNotConfigured is the loud 401 every unconfigured adapter returns,
// pointing at the guide that explains the env var.
func ErrNotConfigured(platform, envVar string) error {
	return &httpError{
		code: 401,
		msg:  envVar + " not configured",
		docs: "/pages/integrations.html#native-adapters",
	}
}
