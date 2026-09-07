# Generic HTTP → callhook

**The universal adapter.** Any platform, script, or cron job that can
POST JSON can fire calls.

## Setup

```bash
curl -X POST https://your-callhook.example.com/integrations/generic/webhook \
  -H "Authorization: Bearer $GENERIC_BEARER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "id": "myplatform_123",
    "type": "invoice.due",
    "customer_id": "cus_1002",
    "phone": "+15551234567",
    "payload": {"amount": "USD 49.00"}
  }'
```

```bash
GENERIC_BEARER_TOKEN=...
# or:
GENERIC_WEBHOOK_SECRET=...   # sent as X-Callhook-Secret header
```

No translation happens — you send the callhook event shape directly.
AWS SNS users: point the topic at `/integrations/sns/webhook` instead
(cert-verified); CloudWatch alarms map automatically.
