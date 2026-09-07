# callhook n8n node

Fire AI phone calls from any n8n workflow.

## Install (self-hosted n8n)

```bash
# from this repo:
cd plugins/n8n-callhook
npm install && npm run build

# into your n8n custom nodes dir:
mkdir -p ~/.n8n/custom
cp -r ~/.n8n/custom  # or symlink:
ln -s "$(pwd)" ~/.n8n/custom/n8n-nodes-callhook
# restart n8n
```

Docker: mount the package into `/home/node/.n8n/custom/`.

## Credentials

1. In n8n: **Credentials → New → Callhook API**
2. Base URL: where your callhook server runs (`http://localhost:8080`)
3. Bearer Token: your `CALLHOOK_INTAKE_TOKEN` (leave empty if unset)

## Node fields

| Field | Meaning |
|---|---|
| Event Type | one of the 8 callhook event types |
| Customer ID | resolves the customer + phone in the business store |
| Phone | optional E.164 override |
| Not Before | optional RFC3339 — schedules the call |
| Callback URL | where the structured outcome is POSTed |
| Payload | event-type-specific JSON context |

## Example workflow

Google Sheets row (overdue customer) → **Callhook** node
(`invoice.due`) → response `{"status":"call_placed",...}` → Slack.

## Why this exists

Any n8n trigger can become a phone call. The full pipeline — polite
calling hours, retries, structured outcomes, policy-gated writes —
happens inside callhook, not in your workflow.
