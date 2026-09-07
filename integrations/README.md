# Platform integration guides

Each directory is one platform:

| Directory | Kind | Files |
|---|---|---|
| `stripe/` | webhook adapter guide | README (dashboard steps, env var, mapping) |
| `slack/`, `shopify/`, `hubspot/`, `pagerduty/`, `github/`, `calendly/`, `typeform/`, `woocommerce/`, `xero/`, `quickbooks/`, `paddle/`, `grafana/`, `datadog/`, `intercom/`, `chargebee/`, `pipedrive/`, `generic/`, `sns/` | webhook adapter guides | README each |
| `salesforce/` | recipe | `CallhookCallout.cls` (Apex + trigger example) |
| `airtable/` | recipe | `automation.js` (Automation "Run a script") |
| `google-forms/` | recipe | `onFormSubmit.gs` (Apps Script trigger) |
| `openai/` | agent tool | `callhook_openai.py` (function-calling) |
| `langchain/` | agent tool | `callchain_tool.py` (StructuredTool) |
| `agent-tools/` | agent tool | `callhook_agent_tool.py` (AutoGen / CrewAI) |

The Go adapters themselves live in `backend/internal/integrations/` —
one file per platform plus the shared, doc-verified `verify.go`.

Master table: [`INTEGRATIONS.md`](../INTEGRATIONS.md).
