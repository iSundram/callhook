# callhook — backend

The callhook server: event-driven AI phone calls + goal-driven campaigns,
built on the [CALL-E](https://heycall-e.com) Developer API. Single Go binary,
zero external dependencies.

```
webhook in ──► router ──► prefetch ──► CALL-E call ──► customer
                  │
   outcome engine ◄── terminal webhook
                  ├── policy gate: certain → write · uncertain → escalate
                  ├── retry policy: no_answer → redial · refusal → stop
                  └── campaign progress: goal met → early-stop
```

## Development

```bash
cd backend
go run ./cmd/callhook          # dry-run server + dashboard on :8080
go test ./...                  # all suites
go vet ./...
make demo                      # dry-run with fast retries (5s)
```

Configuration is environment-only — see [`.env.example`](./.env.example).
No `CALLHOOK_API_KEY` = dry-run mode: the full pipeline runs (including
campaigns) with fabricated calls, zero balance.

## CLI

```bash
go run ./cmd/callhookctl fire invoice.due cus_1002
go run ./cmd/callhookctl sessions --watch
go run ./cmd/callhookctl metrics
```

## Layout

```
cmd/callhook/            server entrypoint, config, graceful shutdown
cmd/callhookctl/         CLI client
internal/api/            HTTP: intake (+batch), campaigns, webhook, dashboard, rate limiting
internal/campaign/       campaign engine: goals, waves, early-stop, budgets
internal/events/         event schema + router (event type → blueprint)
internal/business/       business Store interface + mock (swap for your CRM)
internal/callhookclient/ CALL-E Developer API client (docs/callhook.openapi.yaml)
internal/session/        session registry + audit log + retry/schedule triggers
internal/outcome/        outcome engine: policy-gated writes, escalation, callbacks
internal/retry/          scheduler: redials, calling-window deferrals, scheduled starts
internal/callwindow/     polite-hours gate (region → timezone, 9–20h weekdays)
internal/store/          crash-safe JSONL journal persistence
```

## Integration surface (for the frontend)

| Endpoint | What |
|---|---|
| `POST /api/events` (+`/batch`) | fire events |
| `POST /api/campaigns` · `GET /api/campaigns[/{id}]` · `POST /api/campaigns/{id}/stop` | campaigns |
| `GET /api/sessions` | live session feed (states, audit trail, transcripts) |
| `GET /api/metrics` | aggregate stats |
| `GET /api/health` | config snapshot |
| `GET /` | embedded dashboard (being replaced by the web app) |

The embedded dashboard in `internal/api/dashboard.go` is the interim UI; the
web app under `../web` is the real frontend.

## Full documentation

https://callhook.github.io — architecture, event types, campaigns, production
guide. Project-wide README: [repo root](../README.md).

## License

MIT
