# callhook — web

The callhook war room: the real frontend application for events, calls and
campaigns. Served by the Go backend as a **single binary** (embedded via
`go:embed`) — `./callhook` and you have the full app.

**Stack:** React 18 + TypeScript + Vite + React Router. Zero UI frameworks —
the Gothic noir design system is hand-rolled CSS with the brand tokens
(`#000000` / `#D1D0D0` / `#988686` / `#5C4E4E`), identical to the docs site.

## Views

| Route | What |
|---|---|
| `#/` | **War Room** — live stats, activity feed, outcome breakdown |
| `#/sessions` | All sessions: search, filter, status/outcome pills |
| `#/sessions/:id` | Session detail: transcript, outcome JSON, audit timeline, task prompt |
| `#/campaigns` | Campaign cards with progress + **create wizard** (goal → waves → budget) |
| `#/campaigns/:id` | Campaign war-room: progress, budget meter, audience grid, wave log, stop |
| `#/fire` | Fire single events with inline event-type docs |
| `#/integrations` | Platform-adapter catalog: routes, env status, signature schemes (live from `GET /api/integrations`) |
| `#/settings` | Connection + server config snapshot |

## Development

```bash
cd web
npm install
npm run dev        # vite on :5173, /api proxied to :8080
npm run build      # → dist/
```

Auth: the Connect screen stores server URL + `CALLHOOK_INTAKE_TOKEN` in
localStorage; every request sends it as a Bearer header. The backend requires
the token on all `/api/*` endpoints when configured (health is the open
connect probe).

## Production

Built by `make full` in `../backend`: the web app is embedded into the Go
binary — no node, no separate deploy, same single-file distribution.
