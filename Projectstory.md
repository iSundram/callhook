# The callhook project story

> How a one-line idea — *"a webhook goes in, a phone call comes out"* —
> became a single-binary product with 19 doc-verified integrations, a live
> demo, a documentation system that finds you when you're stuck — and a
> first real phone call that connected on attempt four.

---

## 1. Origin: calle → callhook

The project started as **calle** — event-driven AI phone calls on top of
the [CALL-E](https://heycall-e.com) Developer API. The first commit
(`de914a7`) was a single pipeline: webhook in, Call-E call, structured
outcome back.

The rename to **callhook** (`24d24d7`) was a positioning decision made
from the name itself: *call* + *hook*. It describes the entire
architecture in one word, and it sets up the sentence every developer
already understands — "if you've ever built a webhook, you already know
how to use this." Every subsequent design choice was checked against that
sentence: does this make callhook more like a webhook (familiar,
stateless, boring) and less like a telephony integration (bespoke,
fragile, clever)?

The etymology callouts from the `calle` era (a stale Spanish-language
explanation) were deliberately deleted (`7958b20`) rather than rewritten —
the name no longer needed a story.

---

## 2. Design language: Gothic noir

The brand was built before the product surfaces, and never changed after.

**Palette** (`b3b60a02`, `b060172`):

| Token | Value | Role |
|---|---|---|
| `--bg` | `#000000` | true black, not "dark gray" |
| `--brand-silver` | `#D1D0D0` | primary text |
| `--brand-mauve` | `#988686` | secondary |
| `--brand-umber` | `#5C4E4E` | tertiary / accents |

**Rules that held for the whole project:**

1. **No UI framework.** The war room is hand-rolled CSS on brand tokens —
   React + TypeScript + Vite, zero component libraries. The docs site and
   the app share the identical token set, so they read as one product.
2. **Monospace for anything machine-owned.** Customer IDs, phone numbers,
   env vars, session IDs, routes, outcomes — all `--font-mono`. Human
   copy is the only thing in proportional type.
3. **No emojis, anywhere.** Enforced across several commits (`8a752d5`,
   `97b4e58`, `3f99aec`) including the architecture diagrams. The tone is
   *instrument panel*, not *marketing site*.
4. **Hand-built SVG marks.** The logo (`icon-tight.svg`, `logo-full.svg`,
   `wordmark-tight.svg`) went through iterations including a lowercase
   `c` x-height experiment (`e9b21e8`) that was reverted (`dbfa128`)
   because the capital preserved the wordmark's baseline rhythm.
5. **Status colors carry meaning, not decoration.** Amber = dry-run /
   attention, green = live / success, red = error. A yellow dot in the
   header means fabricated calls — that's a *safety* signal, not a theme
   accent.

The docs site got its own redesign (`3eda39a`): three-column layout,
left sidebar, scroll-spy TOC, theme toggle, client-side search — still
zero dependencies, plain JS in `main.js`.

---

## 3. Backend architecture: the decisions that mattered

Single Go binary, **zero external dependencies** (`go.mod` has an empty
require block and stays that way). Five decisions shaped everything:

### 3.1 No LLM in the orchestration path

CALL-E's voice agent is the conversational brain. callhook's job is
deterministic: compose the task, prefetch the context, gate the actions.
A hallucinated amount or date spoken on a real call is a real-world
failure, so task composition is templated Go code with live business data
baked in — auditable, diffable, testable. The README states this as a
deliberate non-decision.

### 3.2 Read/write separation

Prefetched context is *given* to the voice agent (customer plan, invoice,
payment history). Business writes happen **only** post-call, from the
JSON-Schema-validated structured result — never from raw conversation
transcript. Every write is policy-gated: unambiguous outcomes write
(`mark_promise`), uncertain ones escalate to a human.

### 3.3 Crash-safe by journal, not by database

Sessions and campaigns persist to append-only JSONL journals
(`internal/store`). On boot, the journal replays: in-flight sessions,
armed retries, and scheduled calls all resume. No database dependency,
no migrations — a demo you can `Ctrl-C` mid-campaign and restart intact.

### 3.4 The polite-hours gate

`internal/callwindow` defers calls outside 9:00–20:00 recipient-local,
weekdays, per-region timezone, overridable per event. A voice product
that rings phones at 3am is a lawsuit, not a product. Deferrals are
first-class states (`call_deferred`), not retries.

### 3.5 Idempotency everywhere

Every event carries an ID that is also its idempotency key, persisted
across restarts. Every retry attempt gets its own attempt-scoped
idempotency key (`<session>-a<n>`) so a crash mid-retry can never
double-dial. "Never double-dial" is guarantee #1 in the README table.

### 3.6 Campaigns: goals over dial-lists

The campaign engine (`e424c11`) was the biggest architectural addition:
declare a **goal** (`count` N successes, or `reach_all`) and an audience;
the engine runs waves, requeues `no_answer` entries, hard-stops on
budget, and **stops the moment the goal is met**. The E2E-verified demo:
goal met in 12 of 15 budgeted calls — 18 people never called. That
result is quoted in the README because it's the product thesis: *never
waste a call*.

---

## 4. Event blueprints: 3 → 8

`internal/events/router.go` maps event types to blueprints (task composer
+ result schema). The core three (`invoice.due`, `account.warning`,
`promo.offer`) were joined by five more (`28efc2c`): `delivery.window`,
`appointment.reminder`, `payment.failed`, `subscription.expiring`,
`feedback.request`. Every blueprint follows the same contract, and the
intake API, session handling, retries, windows, persistence, and outcome
engine were already generic — adding a type is one file, no plumbing.

The presentation layer lagged (the docs and UI still said "three event
types") until commit `9387de6` pushed all 8 everywhere: a shared
`web/src/lib/events.ts` catalog, per-type campaign presets, outcome
tables in the docs, and the search index.

---

## 5. MCP: agents operate the voice channel

`2981781` added a Model Context Protocol server over Streamable HTTP at
`POST /mcp` — stdlib only, JSON-RPC 2.0. Six tools: `fire_event`,
`launch_campaign`, `get_campaign`, `list_sessions`, `list_event_types`,
`run_demo`. Any MCP client (Claude Code, Cursor) can operate callhook.
Alongside it, `f44d18c` added the SSE stream (`GET /api/stream`) so the
war room updates instantly rather than polling.

---

## 6. Live-fire: the first real call

Dry-run proves the pipeline; only live fire proves the integration. The
test session against the real CALL-E API — with a real phone number — is
the part of the story no mock can tell.

### 6.1 Method: read-only first, balance second

The API key was validated against `GET /v1/goals` (read-only, zero
balance) before any call was placed. Only then did testing touch the 20
free calls the account provides.

### 6.2 The carrier saga

The first real call failed **instantly** with a provider `404`. The
diagnosis came from CALL-E's own event stream (`GET /v1/calls/{id}/events`):
the task had run — bot created, robot resolved, dial attempted — but the
recipient was routed with `region: "US"` while the number was Indian
(`+91…`). CALL-E routes by region; the mismatch killed the call before
the carrier. Attempt two, correctly routed `IN`/`en-IN`, failed with a
provider `500`; attempt three with SIP `480` (temporarily unavailable) —
international lines into India are filtered aggressively, and CALL-E's
own docs mark international lines as "primarily intended for testing."

Attempt four **connected**. A real conversation, on a real phone:

```
AGENT: is this Priya Sharma?
AGENT: I'm calling for your service provider to thank you for being a
       loyal Pro plan customer since March twenty twenty-four, and to
       offer you 20% off your next invoice, valid until the end of this
       week; are you interested?
USER:  Hello. No.
AGENT: No problem at all—thanks for being a loyal Pro plan customer.
       Take care.
```

Every rule from the blueprint was obeyed — the loyalty framing, the
single offer, the never-push-twice exit. The structured result came back
schema-valid (`{"outcome": "declined"}` at 0.92 confidence) and the
terminal webhook arrived through the tunnel, was deduped, and drove the
outcome engine. Four calls of twenty spent on the truth.

### 6.3 What live fire caught that tests couldn't

1. **CALL-E redelivers terminal webhooks.** The same call was processed
   twice — a business write could have applied twice. Fix (`66846e1`):
   dedupe by the webhook event id, exactly as their docs recommend.
2. **Webhook payloads omit transcripts.** The full transcript exists on
   `GET /v1/calls/{id}` but not in the webhook body. Fix (same commit):
   the outcome engine re-fetches the call to enrich the session before
   running actions.

Both are invisible in dry-run and in mocks — they only exist in the
contract between two real systems.

---

## 7. The integrations saga: 19 platforms, doc-verified

This was the largest single effort, and the methodology matters more than
the count.

### 6.1 The rule

**No adapter ships without its signature scheme verified against the
vendor's own documentation.** Not from memory, not from a blog post —
from the vendor's docs (or, for WooCommerce, the plugin source). Each
adapter's header comment cites the URL it was verified against.

### 6.2 What the research corrected

Assumptions that would have produced silently-broken integrations:

| Platform | Assumption | Reality (verified) |
|---|---|---|
| Intercom | HMAC-SHA256 | **HMAC-SHA1**, 40 hex chars, `sha1=` prefix — a SHA256 implementation rejects 100% of real deliveries |
| Shopify | hex digest | **base64** digest |
| WooCommerce | hex digest | **base64** digest (confirmed from `class-wc-webhook.php`) |
| Typeform | hex digest | `sha256=` + **base64** |
| Datadog | HMAC signing | **no signing at all** — custom header or basic auth |
| Pipedrive | HMAC signing | **no signing** — HTTP Basic auth or URL token |
| Chargebee | HMAC signing | **no signing** — HTTP Basic auth |
| Zapier/Heroku | — | Zapier uses a Custom auth scheme, not the default |

These aren't nitpicks — each one is the difference between "works on the
first webhook" and "mysteriously 401s forever."

### 6.3 The adapter contract

Every adapter does exactly three things: **verify** the signature, **map**
the payload to an `Event`, **forward** through the shared Fire function.
Shared crypto primitives live in `verify.go` (hex/base64/SHA1 HMAC,
Stripe's `t=.v1=`, Slack's `v0:ts:body`, HubSpot v3's
`method+URI+body+timestamp`, Paddle's `ts;h1`, basic auth, bearer) —
stdlib `crypto/*` only.

### 6.4 The one that needed real work: AWS SNS

SNS doesn't HMAC — it signs with **RSA over a field-ordered string-to-sign**,
using an X.509 cert fetched from a URL inside the (untrusted) message. The
adapter implements the full documented procedure: validate the cert URL
(https, `*.amazonaws.com`, SNS path pattern) → fetch and cache the cert →
build the exact newline-delimited field string (different for
Notification vs SubscriptionConfirmation) → verify PKCS1v15 with SHA256
(SigV2) or SHA1 (SigV1). The tests generate a self-signed cert and sign
real messages, including a tamper test (sign, then mutate the message).

### 6.5 Testing philosophy

36 tests in `internal/integrations`. Each test **computes a valid
signature the way the platform does**, asserts the adapter accepts it,
and asserts 401 on forged, stale, or replayed input. GitHub's documented
test vector (`It's a Secret to Everybody` / `Hello, World!`) is asserted
verbatim. The tests caught two real bugs during development — HubSpot's
URI reconstruction and Pipedrive's numeric `webhook_id` — which is
exactly what they're for.

### 6.6 The 19

Stripe · Slack · Shopify · HubSpot · PagerDuty · GitHub · Calendly ·
Typeform · WooCommerce · Xero · QuickBooks · Paddle · Grafana · Datadog ·
Intercom · Chargebee · Pipedrive · AWS SNS · Generic HTTP

Plus outbound packages: an **n8n community node** (TypeScript +
credentials), a **Zapier CLI app**, agent tools for **OpenAI function
calling**, **LangChain**, **AutoGen/CrewAI**, and recipes for
**Salesforce Apex**, **Airtable**, and **Google Forms Apps Script**.

### 6.7 The catalog

`GET /api/integrations` serves the registry with live env-var status; the
war room's Integrations page renders it. An unconfigured platform answers
**401 loudly** on every delivery — a missing env var is caught the first
time a hook fires, not after a silent week.

---

## 8. Documentation as a product surface

Docs were never an afterthought. The insight that shaped the final
iteration: **documentation should find you at the moment you're stuck.**

### 7.1 Three layers

1. **Errors carry their own fix.** Every integration 401 and every
   pipeline error includes a `docs` field pointing at the exact page and
   anchor. `{"error": "STRIPE_WEBHOOK_SECRET not configured", "docs":
   ".../integrations.html#native-adapters"}`.
2. **The UI renders the pointer.** A `DocsHint`/`ErrorDocsHint` component
   wired into Connect (connection failures), Fire (error/deferred/
   scheduled states), Settings (dry-run, unauthenticated warnings), War
   Room (empty state), CampaignWizard, and Integrations — backed by one
   `lib/docs.ts` map so every page behaves consistently.
3. **A troubleshooting page built for real failures.** `troubleshooting.html`
   covers: war-room connect failures, webhook 401s *by cause* (hex-vs-base64
   per platform, HubSpot's tunnel URI gotcha, replay windows + clock sync,
   basic-auth-not-API-key), every `/api/events` error, "events fire but no
   calls happen" diagnosis, campaign problems, and install/startup issues.

### 7.2 The corpus

`INTEGRATIONS.md` (master table + env vars + honest notes) · 19 per-platform
guides under `integrations/` (dashboard steps, env vars, mappings) ·
8-page docs site (quickstart, install, API, events, campaigns,
integrations, production, architecture, troubleshooting) · root README ·
backend README · web README. Search index covers all of it.

---

## 9. The war room: UX iterations

The frontend went through several honest iterations, each fixing a real
observed problem:

| Commit | Change | Why |
|---|---|---|
| `7f7dfa1` | Full React war room | From a Go string-literal dashboard to a real app, embedded via `go:embed` |
| `5c30739` | Mobile drawer | The sidebar table is unusable on a phone |
| `9a82c45` | Dry-run badge → **colored dot** | The text badge crowded the header; the dot + click-popover keeps it compact while still being explicit |
| `eefb61b` | Connect **proves the token** | The live-demo failure: connect only checked `/api/health` (always open), so a stale empty token "connected" fine and every action 401'd invisibly |
| `b26a73a` | Progress bar + dim + skeletons | Loading feedback |
| `707ed83` | Google-style indeterminate bar + shimmer on slow refetches | The first bar grew-and-stalled; the first skeletons only appeared on cold load |
| `6dfc2ae` | Bar only on **user actions**; SSE-aware polling | The bar pulsed on every 2s background poll — the app looked permanently stuck. Reads now show local shimmer; only mutations dim the app. Polling backs off to a 20s heartbeat while SSE is connected |
| `8100dc8` | Minimum 600ms skeleton display | A 100ms ghost flash reads as a glitch. Skeletons now hold long enough to be *perceived* |

The through-line: **honest state over decoration.** Dry-run is a dot
because it's a safety signal. Loading is a skeleton where the data lives
and a bar only when *you* caused the work. Errors are loud, with the fix.

---

## 10. Deployment: Render

`26f9910` made the repo Render-ready: a three-stage `Dockerfile` (node
web build → Go embed+build → alpine runtime, non-root), a `render.yaml`
blueprint (health check `/api/health`, generated secrets, disk for the
journal), and a `PORT` convention fix in `main.go` (Render/Railway/Fly
inject `PORT`; callhook previously ignored it).

**The Dockerfile bug worth remembering** (`9d10a5d`): the first version
copied the web build with `RUN cp -r /web/dist/*` — but BuildKit prunes
unreferenced stages, so the `web` stage never ran, `dist/` didn't exist,
and the build failed. The fix is `COPY --from=web`, which creates the
stage dependency; the web stage also now asserts `dist/index.html` exists
so a silent vite failure can't slip through. CI gained a `docker build`
job (`1cd5ec8`) so this class of regression is caught pre-push.

**Live:** [callhook.onrender.com](https://callhook.onrender.com) — dry-run
mode (real pipeline, fabricated calls, zero balance). Free tier sleeps
after inactivity; the first request warms in ~50s.

---

## 11. Honest mistakes log

Kept because the fixes are the story:

1. **Intercom SHA256 assumption** — caught in docs research; would have
   rejected every real delivery.
2. **Dockerfile unreferenced stage** — `RUN cp` doesn't trigger a build;
   CI added to catch it.
3. **Connect didn't verify the token** — a stale empty token produced a
   broken-but-"connected" session showing `—` everywhere with no reason.
   Now connect proves the token, 401s clear the session back to Connect,
   and the war room shows the error explicitly.
4. **Global loading on every request** — background polls pulsed the
   progress bar; the app looked permanently loading. Now mutations only.
5. **Skeletons too fast to see** — added a minimum display time.
6. **Test flake from a shared clock** — a test overrode the package-level
   `now` without restoring it, failing later tests. Fixed with
   `t.Cleanup`.
7. **Duplicate JSX block** during the shimmer refactor — caught by
   re-reading the file after the edit, removed.
8. **CORS preflight returned 405** — Go's method-based routing rejected
   `OPTIONS` before the middleware could answer it. Caught by the new
   httptest suite; fixed by answering preflights at the router level.
9. **`Routes()` rebuilt the rate limiter on every call** — harmless in
   production (called once) but a footgun; routes are now cached on
   first build.
10. **Session store handed out live pointers** — `Get`/`FindByCallID`
    returned the shared `*Session`, so webhook-path reads could race
    scheduler writes. CI's `-race` caught it in a test; the fix makes
    both methods return copies (`62cedd0`) — a production hardening,
    not a test patch.

---

## 12. What was deliberately *not* built

- **No extra LLM** in orchestration (see §3.1).
- **No inbound calls / telephony** — that's CALL-E's job (~45 countries,
  IVR, voicemail, transfer handling). callhook is the layer above.
- **No database** — the JSONL journal is enough for crash-safety at this
  scale, and keeps the zero-dependency promise.
- **No fake integrations** — every adapter does real signature
  verification against the vendor's documented scheme, or it doesn't ship.

---

## 13. Timeline (commit archaeology)

| Era | Commits | What happened |
|---|---|---|
| Origin | `de914a7` → `2874e9d` | calle: pipeline, retries, auth, persistence, windows, Goals API, CLI |
| Brand | `b060172` → `7134197` | Gothic noir theme, hand-built SVG marks, emoji purge |
| Rename | `24d24d7` | calle → callhook |
| Frontend | `9fb1a3f`, `7f7dfa1`, `5c30739` | Monorepo restructure; the React war room; mobile drawer |
| Campaigns | `e424c11` | Goal-driven engine, waves, early-stop, budgets |
| Hardening | `d11e673`, `28efc2c`, `62cedd0` | Test coverage 4→10 files; 5 new blueprints (8 total); CI race caught → session store returns copies |
| Live-fire | `66846e1` | First real call: carrier saga, region fix, real transcript; webhook dedup + transcript enrichment |
| Agent surface | `2981781`, `f44d18c` | MCP server; SSE stream |
| Integrations | `17fe0a1` | 19 doc-verified adapters + packages + UI + docs |
| Docs system | `c045442` | Contextual references, troubleshooting page |
| Install & demo | `073dedf` | One-line installer (install.sh), first-run "Run the demo" button, docs refresh |
| Deploy | `26f9910`, `9d10a5d` | Render-ready Dockerfile + blueprints |
| Demo & polish | `d0956f7` → `8100dc8` | Live URL, mode dot, auth loudness, loading UX v1–v4 |

---

## 14. The thesis, restated

Most phone-agent projects are one workflow: confirm an appointment, fill a
shift, chase one invoice. **callhook is the layer underneath.** Point any
business system at one endpoint, fire any of 8 event types — or let one of
19 platforms' native webhooks do it — and get intelligent calls with
structured outcomes back, with retry policy, polite calling hours,
crash-safe persistence, goal-driven campaigns, an MCP surface for agents,
and a full audit trail included.

The integrations plan listed 50 platforms. 19 shipped, each verified
against its vendor's docs, each with tests that sign real fixtures. The
other 31 follow the same recipe — one adapter, one test, one catalog
entry — and the recipe is in [`INTEGRATIONS.md`](INTEGRATIONS.md).

*Fire a webhook. Your customer's phone rings.*
