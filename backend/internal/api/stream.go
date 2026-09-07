package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Broker fans out live events to SSE subscribers. Sessions and campaigns
// publish here on every mutation; /api/stream streams them to the war room.
type Broker struct {
	subs map[chan []byte]struct{}
}

func NewBroker() *Broker {
	return &Broker{subs: map[chan []byte]struct{}{}}
}

// Publish broadcasts one SSE frame (event name + JSON payload).
func (b *Broker) Publish(event string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	frame := []byte(fmt.Sprintf("event: %s\ndata: %s\n\n", event, data))
	for ch := range b.subs {
		select {
		case ch <- frame:
		default: // slow consumer: drop rather than block the pipeline
		}
	}
}

// ServeStream implements GET /api/stream (Server-Sent Events).
func (s *Server) serveStream(w http.ResponseWriter, r *http.Request) {
	// EventSource cannot send Authorization headers — accept the token as
	// a query parameter for this endpoint only.
	if s.IntakeToken != "" {
		got := r.URL.Query().Get("token")
		if got != s.IntakeToken {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	ch := make(chan []byte, 32)
	s.Stream.subs[ch] = struct{}{}
	defer delete(s.Stream.subs, ch)

	// hello frame so clients know the stream is live
	fmt.Fprint(w, "event: hello\ndata: {\"ok\":true}\n\n")
	flusher.Flush()

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case frame := <-ch:
			_, _ = w.Write(frame)
			flusher.Flush()
		case <-heartbeat.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}
