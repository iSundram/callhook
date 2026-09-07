# QuickBooks Online → callhook

**Adapter:** `backend/internal/integrations/quickbooks.go` (verified:
[QBO webhooks](https://developer.intuit.com/app/developer/qbo/docs/develop/webhooks)).

## Setup

1. Intuit Developer portal → your app → **Webhooks**
2. Endpoint URL: `https://your-callhook.example.com/integrations/quickbooks/webhook`
3. Copy the **verifier token** → env var:

```bash
QUICKBOOKS_VERIFIER_TOKEN=...
```

## Signature verification

`X-Intuit-Signature` = hex HMAC-SHA256 of the raw body with the verifier
token; comma-separated signature lists accepted (any match validates).

## Mapping

Invoice Update events → `invoice.due` (customer =
`qbo_{realmId}_inv_{entityId}`; resolve contacts via your store — QBO
notifications are intentionally thin, per their docs).
