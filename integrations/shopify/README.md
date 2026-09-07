# Shopify → callhook

**Adapter:** `backend/internal/integrations/shopify.go` (verified:
[shopify.dev webhooks](https://shopify.dev/docs/apps/webhooks)).

## Setup

Shopify Admin (or API):

```bash
curl -X POST "https://{shop}.myshopify.com/admin/api/2026-01/webhooks.json" \
  -H "X-Shopify-Access-Token: {token}" -H "Content-Type: application/json" \
  -d '{"webhook":{"topic":"orders/payment_failure","address":"https://your-callhook.example.com/integrations/shopify/webhook","format":"json"}}'
```

Also subscribe: `refunds/create`, `orders/cancelled` (map to win-back
calls). Set the app's API secret key:

```bash
SHOPIFY_API_SECRET=shpat_...
```

## Signature verification

`X-Shopify-Hmac-Sha256` = **base64** HMAC-SHA256 of the raw body
(hex/base64 mixups are the #1 Shopify integration bug — ours matches
the docs). Topic comes from `X-Shopify-Topic`.

## Mapping

| Topic | callhook event |
|---|---|
| `orders/payment_failure` | `invoice.due` |
| `refunds/create` | `account.warning` |
| `orders/cancelled` | `promo.offer` (win-back) |
