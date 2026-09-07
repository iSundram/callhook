# Slack → callhook

**What it does:** `/call invoice.due cus_1002` from any channel — the
phone rings, the outcome is posted back to the channel.

**Adapter:** `backend/internal/integrations/slack.go` (verified:
[docs.slack.dev](https://docs.slack.dev/authentication/verifying-requests-from-slack)).

## Setup

1. api.slack.com/apps → **Create App → From scratch**
2. **Slash Commands → Create New Command**:
   - Command: `/call`
   - Request URL: `https://your-callhook.example.com/integrations/slack/slash`
   - Usage hint: `[event_type] [customer_id] [+phone]`
3. **Basic Information → App Credentials → Signing Secret** → env var:

```bash
SLACK_SIGNING_SECRET=...
```

## Signature verification

`X-Slack-Signature` = `v0=` + hex HMAC-SHA256 of `v0:TIMESTAMP:RAW_BODY`
with the signing secret; timestamps older than 5 minutes are rejected.
Forged requests → 401.

## Usage

```
/call invoice.due cus_1002
/call account.warning cus_1003 +15551234567
```

Malformed commands get a friendly usage reply in-channel (ephemeral).
