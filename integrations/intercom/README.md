# Intercom → callhook

**Adapter:** `backend/internal/integrations/intercom.go` (verified from
Intercom's webhook-models reference).

## Setup

1. Intercom Developer Hub → your app → **Webhooks**
2. Subscribe: `user.tag.created`, `conversation.admin.closed`
3. Endpoint URL: `https://your-callhook.example.com/integrations/intercom/webhook`
4. **Basic Info → Client Secret** → env var:

```bash
INTERCOM_CLIENT_SECRET=...
```

## Signature verification

`X-Hub-Signature` = `sha1=` + hex **HMAC-SHA1** of the raw body with the
client secret — Intercom's exact scheme (40 hex chars, per their
reference; a SHA256 assumption would silently reject everything).

## Mapping

| Topic | callhook event |
|---|---|
| `user.tag.created` (tag: at-risk / churn-risk) | `account.warning` |
| `conversation.admin.closed` | `feedback.request` (satisfaction call) |

`user.deleted` is never called.
