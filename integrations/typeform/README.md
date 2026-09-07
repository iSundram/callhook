# Typeform → callhook

**Adapter:** `backend/internal/integrations/typeform.go` (verified:
[secure your webhooks](https://www.typeform.com/developers/webhooks/secure-your-webhooks/)).

## Setup

```bash
curl -X POST "https://api.typeform.com/forms/{form_id}/webhooks" \
  -H "Authorization: Bearer {token}" \
  -d '{"url":"https://your-callhook.example.com/integrations/typeform/webhook","secret":"{secret}"}'
```

```bash
TYPEFORM_WEBHOOK_SECRET=...
```

Pro tip: add hidden fields `customer_id` and `phone` to the form and
append them to the share URL — the adapter uses them for routing.

## Signature verification

`Typeform-Signature` = `sha256=` + **base64** HMAC-SHA256 of the raw
body with the webhook secret.

## Mapping

`form_response` → `feedback.request` with all answers flattened into the
payload; phone-type answers and hidden fields become the call target.
