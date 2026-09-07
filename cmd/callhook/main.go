// callhook — event-driven AI phone calls. A business system fires a webhook;
// callhook places an intelligent phone call via CALL-E and POSTs a structured
// outcome back.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/iSundram/callhook/internal/api"
	"github.com/iSundram/callhook/internal/business"
	"github.com/iSundram/callhook/internal/callhookclient"
	"github.com/iSundram/callhook/internal/events"
	"github.com/iSundram/callhook/internal/outcome"
	"github.com/iSundram/callhook/internal/retry"
	"github.com/iSundram/callhook/internal/session"
	"github.com/iSundram/callhook/internal/store"
)

// version is stamped at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	addr := getenv("CALLHOOK_ADDR", ":8080")
	apiKey := os.Getenv("CALLHOOK_API_KEY")
	baseURL := os.Getenv("CALLHOOK_API_BASE")  // optional override, e.g. a proxy
	publicURL := os.Getenv("CALLHOOK_PUBLIC_URL") // where CALL-E reaches our webhook
	journalPath := getenv("CALLHOOK_JOURNAL", "data/sessions.jsonl")

	// Security: intake bearer token + webhook shared secret. Both optional
	// for local development, both required the moment you tunnel publicly.
	intakeToken := os.Getenv("CALLHOOK_INTAKE_TOKEN")
	webhookSecret := os.Getenv("CALLHOOK_WEBHOOK_SECRET")

	// Dry-run mode: no API key → fabricate results, burn no call balance.
	dryRun := apiKey == ""
	if dryRun {
		log.Printf("CALLHOOK_API_KEY not set — running in DRY-RUN mode (no real calls placed)")
	}
	if intakeToken == "" || webhookSecret == "" {
		log.Printf("WARNING: CALLHOOK_INTAKE_TOKEN / CALLHOOK_WEBHOOK_SECRET not set — endpoints are UNAUTHENTICATED (fine locally, not on a public tunnel)")
	}

	// Polite calling hours: defers calls/redials outside 9:00–20:00 local.
	enforceWindows := getenv("CALLHOOK_ENFORCE_WINDOWS", "true") == "true"

	// Retry policy: redial after RetryDelay, at most session.MaxRetries times.
	retryDelay := getdur("CALLHOOK_RETRY_DELAY", 2*time.Hour)
	retryTick := getdur("CALLHOOK_RETRY_TICK", 15*time.Second)

	// Crash-safe persistence: replay the journal so in-flight sessions,
	// armed retries and schedules survive restarts.
	journal, err := store.Open(journalPath)
	if err != nil {
		log.Fatalf("open journal %s: %v", journalPath, err)
	}
	defer journal.Close()

	storeB := business.NewMockStore()
	sessions := session.NewStore()
	sessions.SetPersister(func(sess *session.Session) {
		if err := journal.Persist(sess); err != nil {
			log.Printf("journal persist %s: %v", sess.ID, err)
		}
	})

	restored, err := store.Replay(journalPath)
	if err != nil {
		log.Fatalf("replay journal: %v", err)
	}
	sessions.Restore(restored)
	if len(restored) > 0 {
		log.Printf("restored %d session(s) from %s", len(restored), journalPath)
	}

	client := callhookclient.New(apiKey, baseURL)
	client.DryRun = dryRun
	outcomes := outcome.New(storeB, sessions)
	outcomes.FetchCall = client.GetCall // enrich webhook payloads with full call (transcripts)
	outcomes.RetryDelay = retryDelay

	srv := &api.Server{
		Router:         events.NewRouter(),
		Store:          storeB,
		Sessions:       sessions,
		Calle:          client,
		Outcomes:       outcomes,
		PublicBaseURL:  publicURL,
		IntakeToken:    intakeToken,
		WebhookSecret:  webhookSecret,
		EnforceWindows: enforceWindows,
	}

	// Scheduler: redials unanswered customers, fires window deferrals and
	// scheduled starts when their slot comes due.
	sched := &retry.Scheduler{
		Sessions:       sessions,
		Place:          srv.PlaceCall,
		Tick:           retryTick,
		EnforceWindows: enforceWindows,
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go sched.Run(ctx)

	httpSrv := &http.Server{Addr: addr, Handler: srv.Routes()}
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		log.Printf("shutting down...")
		cancel()
		_ = httpSrv.Close()
	}()

	log.Printf("callhook %s listening on %s (dashboard: http://localhost%s/)", version, addr, addr)
	log.Printf("supported events: %s", "invoice.due, account.warning, promo.offer")
	log.Printf("intake: POST /api/events (+/batch)   webhook: POST /callhook/webhook   metrics: GET /api/metrics")
	log.Printf("retries: redial after %s, up to %d retries | windows: %v", retryDelay, session.MaxRetries, enforceWindows)
	if err := httpSrv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getdur(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
		log.Printf("invalid %s — using default %s", key, def)
	}
	return def
}
