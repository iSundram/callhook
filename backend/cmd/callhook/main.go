// callhook — event-driven AI phone calls. A business system fires a webhook;
// callhook places an intelligent phone call via CALL-E and POSTs a structured
// outcome back.
package main

import (
	"encoding/json"
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/iSundram/callhook/internal/api"
	"github.com/iSundram/callhook/internal/business"
	"github.com/iSundram/callhook/internal/callhookclient"
	"github.com/iSundram/callhook/internal/campaign"
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
		log.Printf("docs: going live → https://callhook.github.io/pages/production.html#api-key")
	}
	if intakeToken == "" || webhookSecret == "" {
		log.Printf("WARNING: CALLHOOK_INTAKE_TOKEN / CALLHOOK_WEBHOOK_SECRET not set — endpoints are UNAUTHENTICATED (fine locally, not on a public tunnel)")
		log.Printf("docs: locking it down → https://callhook.github.io/pages/production.html#auth")
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

	// Campaign engine: goal-driven waves of calls.
	campaigns := campaign.NewStore()
	campaignJournalPath := getenv("CALLHOOK_CAMPAIGN_JOURNAL", "data/campaigns.jsonl")
	campaignJournal, err := store.Open(campaignJournalPath)
	if err != nil {
		log.Fatalf("open campaign journal %s: %v", campaignJournalPath, err)
	}
	defer campaignJournal.Close()
	campaigns.SetPersister(func(c *campaign.Campaign) {
		if err := campaignJournal.Persist(c); err != nil {
			log.Printf("campaign journal persist %s: %v", c.ID, err)
		}
	})
	restoredCampaigns, err := store.ReplayT[*campaign.Campaign](campaignJournalPath)
	if err != nil {
		log.Fatalf("replay campaign journal: %v", err)
	}
	campaigns.Restore(restoredCampaigns)
	if len(restoredCampaigns) > 0 {
		log.Printf("restored %d campaign(s) from %s", len(restoredCampaigns), campaignJournalPath)
	}

	outcomes := outcome.New(storeB, sessions)
	outcomes.FetchCall = client.GetCall // enrich webhook payloads with full call (transcripts)
	outcomes.RetryDelay = retryDelay

	maxConcurrent := getint("CALLHOOK_MAX_CONCURRENT", 3)

	broker := api.NewBroker()
	sessions.SetNotifier(func(sess *session.Session) {
		view := sessionViewOf(sess)
		broker.Publish("session", view)
	})
	campaigns.SetNotifier(func(c *campaign.Campaign) {
		broker.Publish("campaign", c)
	})

	srv := &api.Server{
		Router:         events.NewRouter(),
		Store:          storeB,
		Sessions:       sessions,
		Calle:          client,
		Outcomes:       outcomes,
		Campaigns:      campaigns,
		Stream:         broker,
		PublicBaseURL:  publicURL,
		IntakeToken:    intakeToken,
		WebhookSecret:  webhookSecret,
		EnforceWindows: enforceWindows,
	}

	// Campaign runner: waves through the shared pipeline, progress on every
	// terminal outcome, early-stop the moment the goal is met.
	campRunner := &campaign.Runner{
		Store:        campaigns,
		Fire:         srv.FireEvent,
		TagSession:   sessions.SetCampaign,
		MaxConcurrent: maxConcurrent,
		Stagger:      300 * time.Millisecond,
	}
	outcomes.CampaignHook = campRunner.OnTerminal

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
	go runCampaignPump(ctx, campRunner, retryTick)

	srv.BuildMCP() // MCP tools at POST /mcp

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
	log.Printf("docs: quickstart → https://callhook.github.io/pages/quickstart.html   troubleshooting → https://callhook.github.io/pages/troubleshooting.html")
	log.Printf("supported events: %s", "invoice.due, account.warning, promo.offer, delivery.window, appointment.reminder, payment.failed, subscription.expiring, feedback.request")
	log.Printf("intake: POST /api/events (+/batch)   campaigns: POST /api/campaigns   webhook: POST /callhook/webhook")
	log.Printf("integrations: 19 platform adapters under POST /integrations/{platform}/webhook   catalog: GET /api/integrations")
	log.Printf("retries: redial after %s, up to %d retries | windows: %v | max concurrent calls: %d", retryDelay, session.MaxRetries, enforceWindows, maxConcurrent)
	if err := httpSrv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

// runCampaignPump ticks the campaign engine: launches due waves.
func runCampaignPump(ctx context.Context, r *campaign.Runner, tick time.Duration) {
	if tick <= 0 {
		tick = 15 * time.Second
	}
	t := time.NewTicker(tick)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			r.Pump(time.Now().UTC())
		}
	}
}

func getint(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
		log.Printf("invalid %s — using default %d", key, def)
	}
	return def
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

// sessionViewOf converts a session to the JSON shape the stream sends.
func sessionViewOf(sess *session.Session) map[string]any {
	b, _ := json.Marshal(sess)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return m
}
