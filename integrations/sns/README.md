# AWS SNS → callhook

**Adapter:** `backend/internal/integrations/sns.go` (verified:
[SNS message formats & signature verification](https://docs.aws.amazon.com/sns/latest/dg/sns-message-and-batch-json-formats.html)).

**The full AWS procedure, implemented:** SigningCertURL validated
(https, `*.amazonaws.com`, SNS path pattern) → X.509 cert fetched and
cached → field-ordered string-to-sign (Message, MessageId, Subject?,
Timestamp, TopicArn, Type — confirmation messages use their own field
set) → RSA verify (SHA256withRSA for SignatureVersion 2, SHA1 for 1).

## Setup

1. SNS topic → **Create subscription**:
   - Protocol: HTTPS
   - Endpoint: `https://your-callhook.example.com/integrations/sns/webhook`
2. A SubscriptionConfirmation arrives. Confirm it:

```bash
SNS_AUTO_SUBSCRIBE=true   # auto-visits the (signature-verified) SubscribeURL
```

(or leave unset and click the URL manually — it's validated either way).

## Two message shapes

1. **CloudWatch alarms** (topic subscribed to an alarm): auto-detected —
   `NewStateValue: ALARM` → `account.warning` for `oncall_{alarm name}`;
   OK state → skipped.
2. **Direct publish** — the Message body is callhook event JSON
   (any of the 8 types) → fired as-is.

```bash
aws sns publish --topic-arn ... --message '{"id":"evt_1","type":"invoice.due","customer_id":"cus_1002"}'
```
