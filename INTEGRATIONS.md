# Integrations

Connect any platform to callhook. Point its webhook at one endpoint;
callhook verifies the signature, maps the payload, and places the call.

```
platform webhook ──► /integrations/{platform}/webhook ──► verify ──► map ──► pipeline
                          (per-platform adapter)          (doc-verified)      (call + outcome)
```

Every adapter lives in `backend/internal/integrations/`. Signature schemes
were **verified against each platform's official docs** (and, for
WooCommerce, the plugin source) — see the *Verification* column. Stdlib
only, zero dependencies.

## Built — 19 webhook adapters

| Platform | Route | Maps to | Verification (doc-verified) |
|---|---|---|---|
| [Stripe](integrations/stripe/README.md) | `POST /integrations/stripe/webhook` | `invoice.due`, `account.warning` | `Stripe-Signature`: t=.body, HMAC-SHA256 hex, 5-min replay window |
| Slack | `POST /integrations/slack/slash` | all 8 types (`/call` command) | `X-Slack-Signature`: v0:ts:body, HMAC-SHA256 hex, 5-min window |
| Shopify | `POST /integrations/shopify/webhook` | `invoice.due`, `account.warning`, `promo.offer` | `X-Shopify-Hmac-Sha256`: base64 HMAC-SHA256 of raw body |
| HubSpot | `POST /integrations/hubspot/webhook` | `appointment.reminder` | `X-HubSpot-Signature-V3`: HMAC-SHA256 base64 of method+URI+body+ts, 5-min |
| PagerDuty | `POST /integrations/pagerduty/webhook` | `account.warning` | `X-PagerDuty-Signature`: HMAC-SHA256 of raw body |
| GitHub | `POST /integrations/github/webhook` | `account.warning` (prod deploys, main-branch CI) | `X-Hub-Signature-256`: sha256= hex HMAC-SHA256 (GitHub test vector passes) |
| Calendly | `POST /integrations/calendly/webhook` | `appointment.reminder` | `Calendly-Webhook-Signature`: t=.body, HMAC-SHA256 hex |
| Typeform | `POST /integrations/typeform/webhook` | `feedback.request` | `Typeform-Signature`: sha256= base64 HMAC-SHA256 |
| WooCommerce | `POST /integrations/woocommerce/webhook` | `payment.failed`, `promo.offer` | `X-WC-Webhook-Signature`: base64 HMAC-SHA256 (matches plugin source) |
| Xero | `POST /integrations/xero/webhook` | `invoice.due` | `X-Xero-Signature`: base64 HMAC-SHA256, 200/401 intent semantics |
| QuickBooks | `POST /integrations/quickbooks/webhook` | `invoice.due` | `X-Intuit-Signature`: hex HMAC-SHA256, multi-sig list |
| Paddle | `POST /integrations/paddle/webhook` | `payment.failed`, `account.warning` | `Paddle-Signature`: ts;h1 HMAC-SHA256 of ts:body, tight window |
| Grafana | `POST /integrations/grafana/webhook` | `account.warning` (firing alerts) | `X-Grafana-Alerting-Signature`: hex HMAC-SHA256 of ts:body |
| Datadog | `POST /integrations/datadog/webhook` | `account.warning` (Triggered) | shared-secret header / basic auth (Datadog doesn't sign) |
| Intercom | `POST /integrations/intercom/webhook` | `account.warning`, `feedback.request` | `X-Hub-Signature`: sha1= hex HMAC-SHA1 (per Intercom reference) |
| Chargebee | `POST /integrations/chargebee/webhook` | `payment.failed`, `account.warning` | HTTP Basic auth (webhook settings user/pass) |
| Pipedrive | `POST /integrations/pipedrive/webhook` | `promo.offer`, `appointment.reminder` | HTTP Basic auth or URL token (no HMAC, per docs) |
| Generic HTTP | `POST /integrations/generic/webhook` | all 8 types (passthrough) | Bearer token or `X-Callhook-Secret` |
| AWS SNS | `POST /integrations/sns/webhook` | all 8 types + CloudWatch alarms | RSA cert-fetched verification (SigV1/v2), string-to-sign per AWS docs |

## Built — outbound packages & recipes

| Package | Where | What |
|---|---|---|
| n8n community node | `plugins/n8n-callhook/` | TypeScript node + credentials; all 8 event types |
| Zapier app | `plugins/zapier-callhook/` | CLI app with "Fire Phone Call Event" action |
| OpenAI tools | `integrations/openai/callhook_openai.py` | `fire_phone_call` function-calling tool |
| LangChain tool | `integrations/langchain/callchain_tool.py` | `StructuredTool` with pydantic schema |
| AutoGen / CrewAI | `integrations/agent-tools/callhook_agent_tool.py` | plain callable both frameworks accept |
| Salesforce | `integrations/salesforce/CallhookCallout.cls` | Apex callout + trigger example |
| Airtable | `integrations/airtable/automation.js` | Automation "Run a script" recipe |
| Google Forms | `integrations/google-forms/onFormSubmit.gs` | Apps Script on-form-submit trigger |
| MCP server | built into callhook, `POST /mcp` | Claude/Cursor/any MCP client operates the voice channel |

## Environment variables

| Platform | Env vars |
|---|---|
| Stripe | `STRIPE_WEBHOOK_SECRET` |
| Slack | `SLACK_SIGNING_SECRET` |
| Shopify | `SHOPIFY_API_SECRET` |
| HubSpot | `HUBSPOT_CLIENT_SECRET`, `HUBSPOT_EXTERNAL_BASE_URL` (behind tunnels) |
| PagerDuty | `PAGERDUTY_WEBHOOK_SECRET` |
| GitHub | `GITHUB_WEBHOOK_SECRET` |
| Calendly | `CALENDLY_WEBHOOK_SECRET` |
| Typeform | `TYPEFORM_WEBHOOK_SECRET` |
| WooCommerce | `WOOCOMMERCE_WEBHOOK_SECRET` |
| Xero | `XERO_WEBHOOK_KEY` |
| QuickBooks | `QUICKBOOKS_VERIFIER_TOKEN` |
| Paddle | `PADDLE_WEBHOOK_SECRET` |
| Grafana | `GRAFANA_WEBHOOK_SECRET` (optional; unsigned accepted when unset) |
| Datadog | `DATADOG_SHARED_SECRET` (+ optional `DATADOG_SHARED_HEADER`) or `DATADOG_WEBHOOK_USER`/`PASSWORD` |
| Intercom | `INTERCOM_CLIENT_SECRET` |
| Chargebee | `CHARGEBEE_WEBHOOK_USER`, `CHARGEBEE_WEBHOOK_PASSWORD` |
| Pipedrive | `PIPEDRIVE_WEBHOOK_USER`/`PIPEDRIVE_WEBHOOK_PASSWORD` or `PIPEDRIVE_URL_TOKEN` |
| Generic | `GENERIC_BEARER_TOKEN` or `GENERIC_WEBHOOK_SECRET` |
| SNS | `SNS_AUTO_SUBSCRIBE=true` to auto-confirm subscriptions |

A platform whose secret is unset answers **401** on every delivery — loud
by design; the war-room Integrations page shows which are configured.

## Testing

- `go test ./internal/integrations/` — 36 tests: every adapter signs
  fixtures the way the platform does and asserts accept + 401-on-forgery.
- `callhookctl test-webhook stripe` / `test-webhook generic` — signs a
  real fixture and posts it to your running server (same code path a live
  delivery takes).
- GitHub's documented test vector (secret `It's a Secret to Everybody`,
  body `Hello, World!`) is asserted verbatim in the test suite.

## Adding your own

Any platform that sends a webhook can integrate. Copy
`backend/internal/integrations/generic.go`, rename, implement:

1. `verify()` — the platform's signature scheme (see `verify.go` for
   ready-made HMAC helpers)
2. `map()` — platform payload → `Event{ID, Type, CustomerID, Phone, Payload}`
3. Register the route in `registry.go` (one line)
4. Add an entry to the `catalog` and a test that signs a fixture

One adapter, one test file, one catalog entry. PRs welcome.

## Honest notes

- **Datadog, Pipedrive, Chargebee, Zendesk do not sign their webhooks.**
  We use their documented auth (custom secret header, basic auth). This
  is stated in each adapter's doc comment — not hidden.
- **Intercom signs with HMAC-SHA1** (40 hex chars), not SHA256 — matching
  their reference exactly.
- **SNS** verification fetches the X.509 cert from a validated
  `*.amazonaws.com` URL and verifies RSA over the exact field-ordered
  string-to-sign — the full AWS-documented procedure, cached per URL.
- **HubSpot v3** signs `method + URI + body + timestamp` — behind a
  tunnel, set `HUBSPOT_EXTERNAL_BASE_URL` so the reconstructed URI
  matches what HubSpot signed.
