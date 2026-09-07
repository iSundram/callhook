# GitHub → callhook

**What it does:** production deploy fails → the engineer who shipped it
gets a call explaining the incident. Main-branch CI failures too.

**Adapter:** `backend/internal/integrations/github.go` (verified:
[docs.github.com](https://docs.github.com/en/webhooks/using-webhooks/validating-webhook-deliveries)
— their documented test vector passes in our test suite).

## Setup

Repo → Settings → Webhooks → Add webhook:
- Payload URL: `https://your-callhook.example.com/integrations/github/webhook`
- Content type: `application/json`
- Secret: a high-entropy string → env var:

```bash
GITHUB_WEBHOOK_SECRET=...
```

- Events: **Deployment statuses** and/or **Workflow runs**

## Signature verification

`X-Hub-Signature-256` = `sha256=` + hex HMAC-SHA256 of the raw body.
Constant-time compare; 401 on mismatch.

## Mapping

| GitHub event | callhook event |
|---|---|
| `deployment_status` (failure/error, production) | `account.warning` |
| `workflow_run` (failure, main/master) | `account.warning` |

Customer = `gh_{sender.login}` — map logins to directory records in your
business store.
