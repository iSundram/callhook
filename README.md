# calle

**Event-driven AI phone calls.** Your system fires a webhook; calle places an
intelligent phone call via [CALL-E](https://heycall-e.com) and POSTs a
structured outcome back. No scripts, no robocalls — a voice agent that knows
the customer's context before it dials.

```
business app ──POST /api/events──► calle ──CALL-E API──► 📞 customer
     ▲                                │
     └── outcome webhook ◄────────────┘
         {outcome, promise_date, actions, transcript, confidence}
```

## How it works

1. **Event in** — any system POSTs an event (`invoice.due`, `account.warning`,
   `promo.offer`, ...). Idempotent: the same event never calls twice.
2. **Prefetch** — the full customer record (plan, invoice, payment history) is
   loaded *before* dialing and baked into the call task, so the voice agent
   never wastes seconds asking for account details.
3. **Intelligent call** — CALL-E's voice agent runs the conversation with that
   context: handles "I already paid" gracefully, collects promise dates,
   offers callbacks.
4. **Structured outcome** — a JSON-Schema-validated result is extracted from
   the call evidence (`payment_promised`, `claims_already_paid`, `no_answer`, ...).
5. **Policy-gated actions** — unambiguous outcomes write to the business store
   (`mark_promise`); uncertain ones escalate to a human instead of acting.
6. **Loop closed** — the outcome, actions taken, transcript, and confidence
   are POSTed back to the originating system.

## Quickstart

```bash
# Dry-run mode (no API key needed — fabricates results, burns no balance):
go run ./cmd/calle

# Real calls:
export CALLE_API_KEY=iams_live_...
export CALLE_PUBLIC_URL=https://your-tunnel.example.com   # so CALL-E can reach /calle/webhook
go run ./cmd/calle
```

Open the live dashboard at `http://localhost:8080/` — fire demo events with
one click and watch the pipeline: event → prefetch → call → outcome → actions.

## Fire an event

```bash
curl -X POST localhost:8080/api/events -H 'Content-Type: application/json' -d '{
  "id": "evt_001",
  "type": "invoice.due",
  "customer_id": "cus_1002",
  "callback_url": "https://your-app.example.com/hooks/calle",
  "payload": {}
}'
```

→ returns `{status: "call_placed", call_id: ...}` immediately; the outcome
lands on your callback URL when the call finishes.

### Supported events

| type | example use | structured outcomes |
|---|---|---|
| `invoice.due` | overdue invoice chase | `payment_promised` (+date), `claims_already_paid`, `disputed`, `callback_requested`, `refused`, `no_answer` |
| `account.warning` | security notice | `acknowledged`, `activity_confirmed_legitimate`, `needs_human`, `no_answer` |
| `promo.offer` | loyalty offer | `accepted`, `declined`, `callback_requested`, `no_answer` |

Adding a new event type = one blueprint in `internal/events/router.go`
(task composer + result schema). Everything else is generic.

## Architecture

```
cmd/calle/            entrypoint, config
internal/api/         HTTP: POST /api/events, POST /calle/webhook, dashboard
internal/events/      event schema + router (event type → call blueprint)
internal/business/    business Store interface + mock (swap for your CRM/billing)
internal/calleclient/ CALL-E Developer API client (docs/calle.openapi.yaml)
internal/session/     session registry + audit log
internal/outcome/     outcome engine: policy-gated writes, escalation, callback
web/                  (dashboard is embedded in internal/api/dashboard.go)
```

- **Read/write separation:** prefetched context is *given* to the voice agent;
  business writes only happen post-call, from the structured result — never
  from raw conversation. Every action is audit-logged.
- **The mock store is the product surface:** implementing `business.Store`
  against a real CRM/billing API turns calle into a production integration.

## Hackathon

Built for the [CALL-E: Your Code Is Calling](https://call-e.devpost.com/)
hackathon. CALL-E is genuinely called at runtime via the Developer API
(`POST /v1/calls` with `result_schema`, terminal results via webhook).

## License

MIT
