package api

// integrations.go wires the platform webhook adapters (internal/integrations)
// into the API server: routes, the event → pipeline conversion, and the
// catalog endpoint the web UI renders.

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/iSundram/callhook/internal/events"
	"github.com/iSundram/callhook/internal/integrations"
)

// integrationsRoutes mounts all platform webhook adapters on the mux.
// Each adapter converts its platform's payload to an events.Event and
// pushes it through ProcessEvent — signature verification happens inside
// the adapter (see internal/integrations/verify.go for the per-platform
// schemes and their doc sources).
func (s *Server) integrationsRoutes(mux *http.ServeMux) {
	fire := func(iev integrations.Event) error {
		ev := events.Event{
			ID:         iev.ID,
			Type:       iev.Type,
			CustomerID: iev.CustomerID,
			Phone:      iev.Phone,
		}
		if iev.Payload != "" {
			ev.Payload = json.RawMessage(iev.Payload)
		}
		res := s.ProcessEvent(&ev)
		if res.Err != nil && res.Status == "error" {
			return res.Err
		}
		// duplicate/scheduled/deferred/call_placed are all success statuses
		// for a webhook: the platform did its job delivering the event.
		return nil
	}
	reg := integrations.NewRegistry(fire)
	reg.Register(mux)
	s.Integrations = reg
	mux.HandleFunc("GET /api/integrations", s.cors(s.handleIntegrationsCatalog))
}

// handleIntegrationsCatalog serves the platform catalog with live
// configured/unconfigured status per integration (secrets from env).
func (s *Server) handleIntegrationsCatalog(w http.ResponseWriter, _ *http.Request) {
	type entryOut struct {
		integrations.CatalogEntry
		EnvConfigured bool   `json:"env_configured"`
		Hint          string `json:"hint,omitempty"`
	}
	out := make([]entryOut, 0, 32)
	for _, e := range s.Integrations.Catalog() {
		eo := entryOut{CatalogEntry: e}
		eo.EnvConfigured = envVarsSet(e.EnvVars...)
		if !eo.EnvConfigured {
			eo.Hint = "set " + strings.Join(e.EnvVars, " / ") + " to enable"
		}
		out = append(out, eo)
	}
	writeJSON(w, http.StatusOK, map[string]any{"integrations": out})
}

// envVarsSet reports whether ALL of the given env vars are non-empty.
func envVarsSet(keys ...string) bool {
	for _, k := range keys {
		if os.Getenv(k) == "" {
			return false
		}
	}
	return true
}
