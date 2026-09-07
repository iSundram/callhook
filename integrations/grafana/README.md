# Grafana → callhook

**Adapter:** `backend/internal/integrations/grafana.go` (verified:
[webhook notifier docs](https://grafana.com/docs/grafana/latest/alerting/configure-notifications/manage-contact-points/integrations/webhook-notifier/)).

## Setup

Grafana → Alerting → Contact points → **Add contact point**:
- Integration: **Webhook**
- URL: `https://your-callhook.example.com/integrations/grafana/webhook`
- Optional HMAC: set a shared secret (default header
  `X-Grafana-Alerting-Signature`) → env var:

```bash
GRAFANA_WEBHOOK_SECRET=...
```

Unset = unsigned payloads accepted (rely on network controls), and
resolved alerts never fire calls — only `firing` does.

## Signature verification

Hex HMAC-SHA256 over `timestamp:body` (when a timestamp header is
configured) or the body alone — exactly the Grafana scheme.

## Mapping

Firing alerts → `account.warning`; the "customer" is
`oncall_{team label}` — map teams in your business store.
