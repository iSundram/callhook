# callhook Zapier app

Fire AI phone calls from any Zap (3M+ businesses, no code).

## Publish

```bash
npm install -g zapier-cli
cd plugins/zapier-callhook
npm install
zapier register "callhook"        # create the app in your Zapier account
zapier push                        # deploy version 1.0.0
zapier promote 1.0.0              # make it public (or invite-only)
```

## Authentication

Set up "Custom" auth per Zap:
- **Base URL** — where your callhook server runs
- **Bearer Token** — `CALLHOOK_INTAKE_TOKEN` (empty if unset)

The auth test hits `GET /api/health`.

## The action

**"Fire Phone Call Event"** — fields:
- Event Type (dropdown of the 8 callhook types)
- Customer ID (required)
- Phone (E.164, optional)
- Not Before (RFC3339 schedule)
- Callback URL (outcome delivery)
- Payload (JSON string)

Returns `{status: "call_placed"|"scheduled"|"deferred", session_id, call_id, phone}`.

## Example Zap

**Trigger:** Google Sheets — new row in "Overdue Customers"
**Action:** callhook — Fire Phone Call Event (`invoice.due`)
**Result:** the customer's phone rings; the structured outcome POSTs back to your callback URL.

## Private vs public

`zapier push` + `zapier promote` makes it public (submit for review to
appear in the directory). For a private integration, use `zapier invite`.
