# WooCommerce → callhook

**Adapter:** `backend/internal/integrations/woocommerce.go` (verified from
the plugin source: `class-wc-webhook.php`, `generate_signature()`).

## Setup

WooCommerce Admin → Settings → Advanced → Webhooks → Add webhook:
- Name: callhook; Status: Active
- Topic: `Order updated`
- Delivery URL: `https://your-callhook.example.com/integrations/woocommerce/webhook`
- Secret: the shared secret → env var:

```bash
WOOCOMMERCE_WEBHOOK_SECRET=...
```

(blank secret defaults to the API user's consumer secret — set it
explicitly to avoid surprises when users rotate keys.)

## Signature verification

`X-WC-Webhook-Signature` = base64 HMAC-SHA256 of the encoded body
with the webhook secret — exactly the plugin's `generate_signature()`.

## Mapping

| Order status | callhook event |
|---|---|
| `failed` | `payment.failed` |
| `cancelled` | `promo.offer` (win-back) |

Customer = `woo_cus_{customer_id}` (guest orders: `woo_anon_{order id}`);
phone from billing.
