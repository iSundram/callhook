# PagerDuty → callhook

**What it does:** incident triggers at 3am → the on-call engineer's phone
rings with an agent that states incident ID, severity, and URL, and asks
for acknowledgement.

**Adapter:** `backend/internal/integrations/pagerduty.go` (verified:
[webhook v3 docs](https://developer.pagerduty.com/docs/webhooks/webhook-v3-overview)).

## Setup

1. PagerDuty → **Automation → Webhooks → New Webhook**
2. URL: `https://your-callhook.example.com/integrations/pagerduty/webhook`
3. Configure the **signing secret** in the webhook settings:

```bash
PAGERDUTY_WEBHOOK_SECRET=...
```

## Signature verification

`X-PagerDuty-Signature` = HMAC-SHA256 of the raw body with the signing
secret; constant-time compare. (Also present: `X-PagerDuty-Event`,
`X-PagerDuty-Event-Id`.)

## Mapping

`incident.triggered` / `incident.escalated` / `incident.reopened` →
`account.warning`. Customer = `pd_{assignee email}` (first user
assignee) — map engineers in your business store.
