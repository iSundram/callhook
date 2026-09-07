# Pipedrive → callhook

**Adapter:** `backend/internal/integrations/pipedrive.go` (verified:
[guide for webhooks](https://pipedrive.readme.io/docs/guide-for-webhooks)).

**Note:** Pipedrive webhooks do **not** HMAC-sign. Documented options:
basic auth on the webhook (http_auth_user/http_auth_password) or a
secret token in the URL.

## Setup

Pipedrive → Settings → Webhooks (or API):

```bash
curl -X POST "https://api.pipedrive.com/v1/webhooks" \
  -H "Authorization: Bearer {api_token}" \
  -d '{"subscription_url":"https://user:pass@your-callhook.example.com/integrations/pipedrive/webhook","event_action":"updated","event_object":"deal"}'
```

```bash
PIPEDRIVE_WEBHOOK_USER=...
PIPEDRIVE_WEBHOOK_PASSWORD=...
# or, if you embed a token in the URL path instead:
PIPEDRIVE_URL_TOKEN=...
```

## Mapping

| Pipedrive event | callhook event |
|---|---|
| `updated.deal` (status → lost) | `promo.offer` (win-back) |
| `updated.deal` (rotting_since set) | `appointment.reminder` (nudge) |
