# Chargebee → callhook

**Adapter:** `backend/internal/integrations/chargebee.go` (verified:
[events & securing your webhook URL](https://apidocs.chargebee.com/docs/api/events)).

**Note:** Chargebee does **not** HMAC-sign webhooks. Their documented
mechanism is HTTP Basic auth configured in the webhook settings.

## Setup

1. Chargebee → **Settings → Configure Chargebee → Webhooks**
2. Add webhook URL: `https://your-callhook.example.com/integrations/chargebee/webhook`
3. Check **"My webhook URL is protected by basic authentication"** and
   set username + password → env vars:

```bash
CHARGEBEE_WEBHOOK_USER=...
CHARGEBEE_WEBHOOK_PASSWORD=...
```

4. Select events: `payment_failed`, `subscription_cancelled`

## Verification

HTTP Basic auth, constant-time compare, 401 on mismatch. (Their IP
allowlist is an additional option if you want belt-and-braces.)

## Mapping

| Chargebee event | callhook event |
|---|---|
| `payment_failed` / `invoice_payment_failed` | `payment.failed` |
| `subscription_cancelled` | `account.warning` (win-back) |
