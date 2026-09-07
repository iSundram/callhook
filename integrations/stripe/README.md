# Stripe → callhook

**What it does:** `invoice.payment_failed` fires an `invoice.due` call —
payment fails at 2am, the customer's phone rings with a polite agent that
knows the amount, due date, and how to handle "I already paid".

**Adapter:** `backend/internal/integrations/stripe.go` (verified:
[docs.stripe.com/webhooks](https://docs.stripe.com/webhooks)).

## Setup

1. Stripe Dashboard → **Developers → Webhooks → Add endpoint**:
   `https://your-callhook.example.com/integrations/stripe/webhook`
2. Select events: `invoice.payment_failed`,
   `invoice.payment_action_required`, `customer.subscription.deleted`
3. Copy the signing secret (`whsec_...`) → env var:

```bash
STRIPE_WEBHOOK_SECRET=whsec_...
```

4. Restart callhook.

## Signature verification

The adapter validates `Stripe-Signature` exactly per Stripe's docs:
`t=` + `v1=` entries, signed payload `timestamp + "." + raw body`,
HMAC-SHA256 hex, 5-minute replay tolerance, constant-time compare.
Unverified requests get **401** (Stripe will treat it as a permanent
failure — correct, since a forged payload will never verify).

## Test locally

```bash
# stripe CLI forwards real signed events:
stripe listen --forward-to localhost:8080/integrations/stripe/webhook
stripe trigger invoice.payment_failed

# or craft one yourself:
go run ./cmd/callhookctl test-webhook stripe
```

## Event mapping

| Stripe event | callhook event | Notes |
|---|---|---|
| `invoice.payment_failed` | `invoice.due` | dunning call, attempt count in payload |
| `invoice.payment_action_required` | `invoice.due` | 3DS needed |
| `customer.subscription.deleted` | `account.warning` | churn notice |

Phone resolution: Stripe sends `customer` (id) — the callhook business
store resolves the number, or set a `phone` in the customer object's
metadata (`customer_phone` on the invoice is used when present).
