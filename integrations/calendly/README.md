# Calendly → callhook

**Adapter:** `backend/internal/integrations/calendly.go` (verified:
[webhook signatures](https://developer.calendly.com/api-docs/overview/webhooks/webhook-signatures)).

## Setup

```bash
curl -X POST https://api.calendly.com/webhook_subscriptions \
  -H "Authorization: Bearer {token}" -H "Content-Type: application/json" \
  -d '{"url":"https://your-callhook.example.com/integrations/calendly/webhook","events":["invitee.created","invitee.canceled","invitee.no_show"],"organization":"https://api.calendly.com/organizations/{org}","signing_key":"{secret}"}'
```

```bash
CALENDLY_WEBHOOK_SECRET=...
```

## Signature verification

`Calendly-Webhook-Signature` (no X- prefix): `t=UNIX,v1=HEX`; signed
payload = `timestamp + "." + raw body`, HMAC-SHA256 hex, stale
timestamps rejected.

## Mapping

| Calendly event | callhook event |
|---|---|
| `invitee.no_show` | `appointment.reminder` (offer new time) |
| `invitee.canceled` | `appointment.reminder` |
| `invitee.created` | `appointment.reminder` (confirmation) |

Customer = `calendly_{invitee id}`; phone picked from
`text_reminder_number` when present.
