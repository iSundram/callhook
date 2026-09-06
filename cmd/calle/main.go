// calle — event-driven AI phone calls. A business system fires a webhook;
// calle places an intelligent phone call via CALL-E and POSTs a structured
// outcome back.
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/iSundram/calle/internal/api"
	"github.com/iSundram/calle/internal/business"
	"github.com/iSundram/calle/internal/calleclient"
	"github.com/iSundram/calle/internal/events"
	"github.com/iSundram/calle/internal/outcome"
	"github.com/iSundram/calle/internal/session"
)

func main() {
	addr := getenv("CALLE_ADDR", ":8080")
	apiKey := os.Getenv("CALLE_API_KEY")
	baseURL := os.Getenv("CALLE_API_BASE") // optional override, e.g. a proxy
	publicURL := os.Getenv("CALLE_PUBLIC_URL") // where CALL-E reaches our webhook

	// Dry-run mode: no API key → fabricate results, burn no call balance.
	dryRun := apiKey == ""
	if dryRun {
		log.Printf("CALLE_API_KEY not set — running in DRY-RUN mode (no real calls placed)")
	}

	store := business.NewMockStore()
	sessions := session.NewStore()
	client := calleclient.New(apiKey, baseURL)
	client.DryRun = dryRun
	outcomes := outcome.New(store, sessions)

	srv := &api.Server{
		Router:        events.NewRouter(),
		Store:         store,
		Sessions:      sessions,
		Calle:         client,
		Outcomes:      outcomes,
		PublicBaseURL: publicURL,
	}

	log.Printf("calle listening on %s (dashboard: http://localhost%s/)", addr, addr)
	log.Printf("supported events: invoice.due, account.warning, promo.offer")
	log.Printf("intake:  POST /api/events   webhook-in from CALL-E: POST /calle/webhook")
	if err := http.ListenAndServe(addr, srv.Routes()); err != nil {
		log.Fatal(err)
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
