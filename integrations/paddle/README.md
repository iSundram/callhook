# Paddle (Billing) → callhook

**Adapter:** `backend/internal/integrations/paddle.go` (verified:
[signature verification](https://developer.paddle.com/webhooks/about/signature-verification)).

## Setup

Paddle dashboard → **Developer tools → Notifications** → add destination:
- URL: `https://your-callhook.example.com/integrations/paddle/webhook`
- Copy the endpoint secret key:

```bash
PADDLE_WEBHOOK_SECRET=pdl_ntfset_...
```

## Signature verification

`Paddle-Signature: ts=UNIX;h1=HEX` — the signed payload is
`timestamp:rawBody`, HMAC-SHA256 hex. Paddle's SDK default tolerance is
5 seconds; ours is 60s (clock-skew-safe) — tighten via `PADDLE_TOLERANCE`
if you want strict.

## Mapping

| Paddle event | callhook event |
|---|---|
| `transaction.payment_failed` | `payment.failed` |
| `customer.subscription.canceled` | `account.warning` (win-back) |
