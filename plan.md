# callhook — Upgrade Plan to 100/100

> **Context:** Written after cloning and reading every serious submission in
> `CALLE-AI/awesome-phone-call-agents`. Every gap below is grounded in actual
> code comparison, not guesswork. The four submissions we must beat:
> `ringer` (10k TS lines, full React UI), `linecanary` (6k TS, 16 test files,
> GitHub Action), `mobilize` (5.7k Python, 26 test files, MCP server, benchmark
> harness), `kept` (5.3k Python, 12 modules, bank-feed reconciliation).
> Our current total: **3,617 Go lines, 3 test files (282 lines), no demo video,
> dashboard in a Go string literal.**

---

## Score card (current vs target)

| Dimension | Now | Target | Gap |
|---|---|---|---|
| Technical depth & correctness | 9/10 | 10/10 | More tests, staticcheck in CI |
| CALL-E API coverage | 7/10 | 10/10 | MCP server, streaming webhook, Goals coverage |
| Feature breadth | 6/10 | 10/10 | 3 event types → 8+; real DB adapter |
| UI / Demo | 3/10 | 9/10 | React dashboard, live SSE stream |
| Documentation | 6/10 | 10/10 | OpenAPI rendered, docs site complete, video |
| Tests | 6/10 | 10/10 | Integration suite, campaign tests, fuzz |
| Presentation / wow factor | 4/10 | 10/10 | Demo video, screenshots, hosted playground |

---

## Phase 1 — Foundational fixes (1–2 days)

These are small gaps that cost disproportionate points. Do these before anything flashy.

### 1.1 — Expand test coverage to match `mobilize`/`linecanary`

**Target:** 20+ test files, every package covered, race detector green.

- `internal/events/router_test.go` — test all three blueprints: compose output
  contains expected strings, result schema has required fields, `VariablesFor`
  returns empty map when nil.
- `internal/api/server_test.go` — HTTP handler tests with `httptest`:
  - `POST /api/events` happy path → 202, idempotency replay → 200 duplicate,
    bad event type → 400, missing phone → 400, batch endpoint.
  - Rate limiter: 61 requests from same IP → 61st returns 429.
  - Auth middleware: missing bearer → 401, wrong secret → 401.
- `internal/campaign/runner_test.go` — the campaign engine is complex and
  currently has **zero tests**. Cover: wave launch, early-stop on goal met,
  budget exhaustion, all-fires-failed retry.
- `internal/campaign/campaign_test.go` — `Met()`, `skipPendingLocked()`,
  `countLocked()`.
- `internal/outcome/outcome_test.go` — already 124 lines; extend with:
  - Webhook dedup (same event ID sent twice → Apply executes once).
  - `retryable()` matrix for all failure codes.
  - `postBack` called with correct payload shape.
- `internal/retry/` — add a test file; test scheduler fires at the right time,
  doesn't double-fire after crash-replay.
- `internal/callwindow/callwindow_test.go` — already good; add DST edge case
  and `NextOpen` boundary.
- `internal/store/journal_test.go` — already good; add torn-line crash
  tolerance test (write half a JSON line, replay should skip it gracefully).
- **Integration test** (`cmd/callhook/integration_test.go`): spin up the full
  server with a mock CALL-E server (`httptest`), fire an event, assert the
  session appears, POST a fake webhook result, assert the callback URL receives
  the outcome. No real CALL-E credentials needed.

```bash
# Goal: go test -race -count=1 -cover ./... reports >80% coverage
```

### 1.2 — Add coverage gate and `staticcheck` to CI

Current CI runs `go test -race` — good. Add two steps in `.github/workflows/ci.yml`:

```yaml
- name: Coverage gate
  run: |
    go test -race -coverprofile=coverage.out ./...
    go tool cover -func=coverage.out | grep total

- name: staticcheck
  uses: dominikh/staticcheck-action@v1
  with:
    version: latest
```

Signals to judges: **this project takes quality seriously** — distinguishes us
from the TypeScript apps with zero linting or tests.

### 1.3 — Finish the docs-site

`docs-site/` exists but pages aren't all filled in. Judges click the docs badge.
Make sure every page is real:

- `index.html` — landing with architecture SVG (upgrade the ASCII diagram).
- `quickstart.html` — copy README quickstart verbatim, formatted.
- `events.html` — event type table with result schema shown per type.
- `openapi.html` — embed Swagger UI iframe pointing at `callhook.openapi.yaml`.
- `safety.html` — the operational guarantees table.

One afternoon of HTML work makes the badge link credible.

---

## Phase 2 — Feature expansion (2–3 days)

This is where we close the gap on `ringer` and `mobilize` in breadth.

### 2.1 — MCP server (matches `mobilize`, beats everyone else)

`mobilize` ships an MCP server and it's called out prominently. Add
`cmd/callhook-mcp/` — an MCP server over stdio/HTTP using `mcp-go` or a
minimal hand-rolled JSON-RPC transport (~200 lines Go, no extra deps).

Expose these tools:

| Tool | Description |
|---|---|
| `fire_event` | Fire a callhook event and place an AI phone call |
| `list_sessions` | List all call sessions and their outcomes |
| `get_session` | Get full session detail including transcript and actions |
| `create_campaign` | Launch a wave-based outbound call campaign |
| `get_campaign` | Get campaign progress and wave status |
| `stop_campaign` | Stop a running campaign early |

**The killer demo:** any MCP-compatible agent (Claude, Cursor, etc.) can use
callhook as a tool. You show an **AI agent placing phone calls through
another AI agent**. None of the TypeScript apps have this.

### 2.2 — SSE live stream endpoint (matches `dispatch-pulse`)

`dispatch-pulse` uses Server-Sent Events for real-time updates. Add:

```
GET /api/stream  →  text/event-stream
```

Push an event every time a session changes state: `call_placed`, `terminal`,
`retry_scheduled`, `action`. The dashboard replaces its polling loop with a
single `EventSource`. ~80 lines of Go (channel broadcast, `http.Flusher`).

Benefits:
- Dashboard feels instant instead of polling every 2 seconds.
- Judges watching the demo see state changes live — visually stunning.

### 2.3 — Expand event types from 3 → 8+

Current: `invoice.due`, `account.warning`, `promo.offer`.
`ringer` has 8 task templates. Add these blueprints to `internal/events/router.go`:

| New event type | Outcome enum |
|---|---|
| `appointment.confirm` | `confirmed`, `reschedule_requested`, `cancelled`, `no_answer` |
| `lead.followup` | `interested`, `not_interested`, `callback_requested`, `no_answer` |
| `service.dispatch` | `vendor_available`, `vendor_unavailable`, `callback_needed`, `no_answer` |
| `survey.collect` | `completed`, `refused`, `partial`, `no_answer` |
| `shift.fill` | `accepted`, `declined`, `no_answer` |

Each is a ~40-line function. Going from 3 → 8 makes the "generic
infrastructure" pitch credible — adding an event type is genuinely one function.

### 2.4 — Real `business.Store` adapter (SQLite, pure Go)

The mock store is the biggest "not production-ready" signal. Add a SQLite
adapter using `modernc.org/sqlite` (pure Go, zero cgo):

```
internal/business/sqlite/
  store.go
  migrations/001_init.sql
```

Schema: `customers`, `invoices`, `escalations`, `contacts`, `promises`.
Seed with the same mock data. Config: `CALLHOOK_DB=data/callhook.db`.

When `CALLHOOK_DB` is set → use SQLite. When unset → fall back to mock.
Judges can inspect `data/callhook.db` after a run to see **real writes**.

This directly counters the weakness: "The mock store is the product surface."

### 2.5 — Webhook at-least-once delivery queue

Currently `postBack()` fires once and logs failure. `mobilize` has a durable
webhook ledger. Add:

```
internal/webhookq/queue.go
```

In-memory queue with exponential backoff: 3 retries at 10s, 60s, 300s.
Persisted in the JSONL journal so retries survive restarts. ~100 lines.
Grounds the claim "your callback URL always gets the outcome."

---

## Phase 3 — UI overhaul (2 days, highest visual impact)

The dashboard is currently a Go string literal. `ringer` is 10k lines of React.
We don't need to match that, but we need to be credible.

### 3.1 — Embedded React dashboard

Replace `internal/api/dashboard.go` with a proper frontend:

```
web/src/
  main.tsx
  App.tsx
  components/
    SessionList.tsx     — sessions table with status pills
    SessionDetail.tsx   — expandable: transcript turns, actions, outcome JSON
    MetricsBar.tsx      — total sessions, outcomes breakdown, retries armed
    CampaignPanel.tsx   — campaign cards with progress bar, wave log
    EventFirer.tsx      — fire demo events with a form (not just buttons)
  hooks/
    useStream.ts        — EventSource consumer (SSE endpoint)
    useSessions.ts
    useCampaigns.ts
```

Build: `vite build --outDir ../../internal/api/dist` and embed with
`//go:embed dist/*`. The server binary stays completely self-contained.

**Key UI features that win demo points:**
- **Live transcript viewer** — SSE stream feeds transcript turns one-by-one in
  real time as the call happens. Visually unlike anything else in the field.
- **Campaign progress bar** — animated fill as outcomes arrive.
- **One-click event firing** — dropdown of all 8 event types, phone/customer
  fields, a "Fire" button. Beats "run this curl command."
- **Dark theme** — port the existing colors (`#0b0e14`, `#7dd3fc`) to Tailwind.

### 3.2 — Screenshot / GIF in README

After the UI is done:

1. Run `make demo`, open browser, fire an invoice event.
2. Record a 30-second GIF (`peek`, `kap`, or `ffmpeg`).
3. Embed at the top of `README.md`:

```markdown
![callhook dashboard — live call pipeline](assets/demo.gif)
```

`ringer` has no GIF. `linecanary` has no GIF. This makes our README the most
visually engaging one in the entire repo.

---

## Phase 4 — Demo video (1 day, highest ROI for hackathon)

**This is the single highest-leverage thing you can do.** `GridGuard` has a
demo link. Nobody else in the whole list does. A 3-minute video = top 3.

### Script

```
00:00 — Problem statement (15s)
  "Most AI agent projects implement one phone workflow. callhook is the
   layer underneath: any business system fires a webhook, callhook places
   an intelligent call with full customer context, and POSTs back a
   structured outcome. The voice channel as an API."

00:15 — Architecture (20s)
  Show the diagram: business app → POST /api/events → callhook → CALL-E
  → phone → outcome webhook back.

00:35 — Quickstart (30s)
  Terminal: `make demo` → server starts in dry-run mode, dashboard opens.

01:05 — Fire an invoice event (45s)
  Click "Fire invoice.due" in the dashboard. Session appears.
  Watch: prefetch → call placed → [simulated] transcript turns →
  outcome: payment_promised → action: mark_promise written.
  Callback URL receives the JSON payload (show in second terminal with nc).

01:50 — Campaign launch (40s)
  POST /api/campaigns with audience of 3 customers.
  Campaign panel: wave 1 fires, outcomes arrive, progress bar fills,
  campaign completes (goal met after 2 successes).

02:30 — MCP demo (20s)
  Open Claude / Cursor, call the `fire_event` MCP tool directly.
  Session appears in the dashboard — an AI agent just placed a phone
  call through another AI agent.

02:50 — Code architecture (10s)
  "Adding a new event type is one 40-line function."
  Show router.go briefly.

03:00 — End.
```

Host on YouTube (unlisted). Add the link to README header and Devpost.

---

## Phase 5 — Submission polish (1 day)

### 5.1 — Devpost submission text structure

Judges read in order. Hit these sections:

1. **Inspiration** — "Every team building phone agents re-implements the same
   plumbing. We built the platform so they don't have to."
2. **What it does** — architecture diagram.
3. **How we built it** — Go, stdlib only, crash-safe JSONL journal, wave-based
   campaign runner, polite calling hours, MCP server.
4. **Challenges** — idempotency across crashes, CALL-E webhook redelivery
   dedup, wave timing without an external scheduler, dry-run without any API.
5. **Accomplishments** — zero external runtime deps. Any event type in one
   function. MCP means any AI agent can use it. Full test suite, race-free.
6. **What we learned** — (honest, 2 sentences).
7. **What's next** — Postgres adapter for scale, n8n plugin, hosted playground.

### 5.2 — README final checklist

- [ ] Demo GIF embedded near the top
- [ ] Demo video link (YouTube unlisted)
- [ ] Badges: CI ✅, coverage %, Go version, MIT
- [ ] Architecture diagram upgraded: ASCII → SVG
- [ ] "Try it in 60 seconds" block (one command, no API key)
- [ ] "Add your own event type" code example (the 5-line pattern)
- [ ] MCP server quickstart section
- [ ] Hackathon link at the bottom

### 5.3 — OpenAPI completeness

`docs/callhook.openapi.yaml` is already 64k — solid. Verify:
- Campaign endpoints (`POST /api/campaigns`, `GET /api/campaigns/{id}`) are
  documented (added after the spec was originally written).
- `/api/stream` SSE endpoint is documented with the event schema.
- Request/response examples are populated, not just schemas.

### 5.4 — `docker-compose.yml`

You already have `make docker`. Add:

```yaml
version: "3.9"
services:
  callhook:
    build: .
    ports: ["8080:8080"]
    environment:
      CALLHOOK_API_KEY: ${CALLHOOK_API_KEY:-}
      CALLHOOK_ENFORCE_WINDOWS: "true"
    volumes:
      - ./data:/app/data
```

One command to run (`docker compose up`). Judges without Go can still try it.

---

## Priority order if time is short

| Time available | Do this |
|---|---|
| 1 day | Phase 4 (demo video) + Phase 3.2 (GIF in README) |
| 2 days | Above + Phase 1.1 (tests) + Phase 2.1 (MCP server) |
| 3 days | Above + Phase 2.3 (event types) + Phase 3.1 (React dashboard) |
| 5 days | All phases — full 100/100 |

---

## Final comparison after upgrades

| Project | Their strength | Our position after upgrades |
|---|---|---|
| `ringer` | React UI, 8 task templates | ✅ We match on UI; beat on infra depth, crash-safety, zero deps |
| `linecanary` | 16 test files, GitHub Action | ✅ We match on tests and CI; beat on feature breadth |
| `mobilize` | 26 test files, MCP, benchmarks | ✅ We match on MCP; our campaign engine is more general |
| `kept` | 12 modules, bank reconciliation | ✅ We beat on breadth; both are infrastructure-grade Python/Go |

**The one thing none of them can claim after these upgrades:**
Generic event-driven AI call infrastructure + full test suite + MCP server +
React live dashboard + demo video + self-contained Go binary with zero runtime
dependencies. That combination is unique in the entire hackathon field.
