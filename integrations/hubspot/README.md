# HubSpot → callhook

**Adapter:** `backend/internal/integrations/hubspot.go` (verified:
[request-validation guide](https://developers.hubspot.com/docs/apps/developer-platform/build-apps/authentication/request-validation)).

## Setup

1. Developer portal → create a **private app**
2. App settings → **Webhooks**: subscribe to
   `deal.propertyChange` (dealstage) and
   `contact.propertyChange` (hs_lead_status)
3. Target URL: `https://your-callhook.example.com/integrations/hubspot/webhook`
4. App credentials → **Client secret** → env var:

```bash
HUBSPOT_CLIENT_SECRET=...
# Behind a tunnel/proxy, so the signed URI can be reconstructed:
HUBSPOT_EXTERNAL_BASE_URL=https://your-callhook.example.com
```

## Signature verification

v3: `X-HubSpot-Signature-V3` = base64 HMAC-SHA256 of
`METHOD + URI + BODY + TIMESTAMP` (ms), 5-minute window. The legacy v1
header (plain SHA-256 hex of secret+body) is accepted as fallback.

## Mapping

| Subscription | callhook event |
|---|---|
| `deal.propertyChange` (dealstage) | `appointment.reminder` (follow-up) |
| `contact.propertyChange` (hs_lead_status) | `appointment.reminder` |

Customer = `hs_{objectId}` — resolve phone via your business store
(HubSpot payloads carry object ids, not numbers).
