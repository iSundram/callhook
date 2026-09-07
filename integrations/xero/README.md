# Xero → callhook

**Adapter:** `backend/internal/integrations/xero.go` (verified:
[Xero webhooks overview](https://developer.xero.com/documentation/guides/webhooks/overview)).

## Setup

1. Xero developer portal → your app → **Webhooks**
2. Set the endpoint URL and copy the **webhook key**:

```bash
XERO_WEBHOOK_KEY=...
```

3. Xero sends a handshake (intent-to-receive): our 200/401 semantics
   answer it correctly automatically.

## Signature verification

`X-Xero-Signature` = base64 HMAC-SHA256 of the raw body with the
webhook key. Valid → 200, invalid → 401 (Xero requires exactly this).

## Mapping

`INVOICE` / `UPDATE` / `ACCREC` events → `invoice.due`
(customer = `xero_{ContactID}`).
