# Datadog → callhook

**Adapter:** `backend/internal/integrations/datadog.go` (verified:
[webhooks integration](https://docs.datadoghq.com/integrations/webhooks/)).

**Note:** Datadog does **not** sign webhook payloads. Their documented
options are custom headers, basic auth in the URL, or OAuth. We verify a
shared-secret header (or basic auth) — stated plainly, not hidden.

## Setup

1. Datadog → Integrations → **Webhooks** → New:
   - Name: `callhook`
   - URL: `https://your-callhook.example.com/integrations/datadog/webhook`
   - Headers (JSON): `{"X-Callhook-Secret": "<secret>"}`
2. Create a custom payload (recommended):

```json
{
  "title": "$ALERT_TITLE",
  "alert_transition": "$ALERT_TRANSITION",
  "alert_priority": "$ALERT_PRIORITY",
  "event_msg": "$EVENT_MSG",
  "tags": "$TAGS",
  "date_posix": "$DATE_POSIX",
  "customer_id": "oncall_payments"
}
```

3. Env var:

```bash
DATADOG_SHARED_SECRET=...
# or basic auth (user:pass in the URL):
DATADOG_WEBHOOK_USER=... DATADOG_WEBHOOK_PASSWORD=...
```

## Mapping

`Triggered`/`Re-Triggered` transitions → `account.warning`
(customer = `customer_id` field or `oncall_{team: tag}`). `Recovered` →
skipped — nobody wants a "it's fine now" call.
