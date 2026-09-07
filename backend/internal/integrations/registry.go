package integrations

// Registry wires platform adapters into the HTTP mux. The wiring layer
// (cmd/callhook + internal/api) provides a Fire function that runs events
// through the pipeline; the registry maps each platform to its route.
//
// Every route is "POST /integrations/{platform}/webhook" (Slack uses
// /integrations/slack/slash for the slash command). A platform whose
// secret is not configured answers 401 on every request — loud, not
// silent, so a missing env var is caught the first time a hook fires.

import (
	"encoding/json"
	"net/http"
)

// FirePipeline converts an adapter Event into a pipeline call. The api
// package implements this by converting to events.Event and invoking
// Server.ProcessEvent.
type FirePipeline func(ev Event) (status string, err error)

// Registry holds the platform → adapter wiring.
type Registry struct {
	entries []entry
}

type entry struct {
	platform string
	route    string
	handler  http.HandlerFunc
}

// NewRegistry builds the full integration route table with the given Fire.
func NewRegistry(fire Fire) *Registry {
	reg := &Registry{}

	add := func(route string, a Adapter) {
		reg.entries = append(reg.entries, entry{
			platform: a.Platform(),
			route:    route,
			handler:  Handler(a, fire),
		})
	}

	// Tier 1
	add("POST /integrations/stripe/webhook", NewStripe(StripeConfig{}))
	add("POST /integrations/slack/slash", NewSlack(SlackConfig{}))
	add("POST /integrations/shopify/webhook", NewShopify(ShopifyConfig{}))
	add("POST /integrations/hubspot/webhook", NewHubSpot(HubSpotConfig{}))
	add("POST /integrations/pagerduty/webhook", NewPagerDuty(PagerDutyConfig{}))
	add("POST /integrations/github/webhook", NewGitHub(GitHubConfig{}))
	add("POST /integrations/generic/webhook", NewGeneric(GenericConfig{}))
	add("POST /integrations/sns/webhook", NewSNS(SNSConfig{}))

	// Tier 2
	add("POST /integrations/calendly/webhook", NewCalendly(CalendlyConfig{}))
	add("POST /integrations/typeform/webhook", NewTypeform(TypeformConfig{}))
	add("POST /integrations/woocommerce/webhook", NewWooCommerce(WooCommerceConfig{}))
	add("POST /integrations/xero/webhook", NewXero(XeroConfig{}))
	add("POST /integrations/quickbooks/webhook", NewQuickBooks(QuickBooksConfig{}))
	add("POST /integrations/paddle/webhook", NewPaddle(PaddleConfig{}))
	add("POST /integrations/grafana/webhook", NewGrafana(GrafanaConfig{}))
	add("POST /integrations/datadog/webhook", NewDatadog(DatadogConfig{}))
	add("POST /integrations/intercom/webhook", NewIntercom(IntercomConfig{}))
	add("POST /integrations/chargebee/webhook", NewChargebee(ChargebeeConfig{}))
	add("POST /integrations/pipedrive/webhook", NewPipedrive(PipedriveConfig{}))

	return reg
}

// Register mounts every integration route on the mux.
func (reg *Registry) Register(mux *http.ServeMux) {
	for _, e := range reg.entries {
		mux.HandleFunc(e.route, e.handler)
	}
}

// CatalogEntry describes one integration for the UI/docs surface.
type CatalogEntry struct {
	Platform string   `json:"platform"`
	Route    string   `json:"route"`
	EnvVars  []string `json:"env_vars"`
	Verified string   `json:"verified"` // short description of the signature scheme
	DocURL   string   `json:"doc_url"`
}

// Catalog returns the static catalog of integrations (for GET /api/integrations).
func (reg *Registry) Catalog() []CatalogEntry {
	return catalog
}

// catalog is the single source of truth mirrored by INTEGRATIONS.md and
// the docs-site page. envVarConfigured is computed per request in the API
// layer, not here.
var catalog = []CatalogEntry{
	{"stripe", "/integrations/stripe/webhook", []string{"STRIPE_WEBHOOK_SECRET"}, "Stripe-Signature: t=.body HMAC-SHA256 hex, 5-min replay window", "https://docs.stripe.com/webhooks"},
	{"slack", "/integrations/slack/slash", []string{"SLACK_SIGNING_SECRET"}, "X-Slack-Signature: v0:ts:body HMAC-SHA256 hex, 5-min window", "https://docs.slack.dev/authentication/verifying-requests-from-slack"},
	{"shopify", "/integrations/shopify/webhook", []string{"SHOPIFY_API_SECRET"}, "X-Shopify-Hmac-Sha256: base64 HMAC-SHA256 of raw body", "https://shopify.dev/docs/apps/webhooks"},
	{"hubspot", "/integrations/hubspot/webhook", []string{"HUBSPOT_CLIENT_SECRET"}, "X-HubSpot-Signature-V3: HMAC-SHA256 base64 of method+URI+body+ts", "https://developers.hubspot.com/docs/apps/developer-platform/build-apps/authentication/request-validation"},
	{"pagerduty", "/integrations/pagerduty/webhook", []string{"PAGERDUTY_WEBHOOK_SECRET"}, "X-PagerDuty-Signature: HMAC-SHA256 of raw body", "https://developer.pagerduty.com/docs/webhooks/webhook-v3-overview"},
	{"github", "/integrations/github/webhook", []string{"GITHUB_WEBHOOK_SECRET"}, "X-Hub-Signature-256: sha256= hex HMAC-SHA256 of raw body", "https://docs.github.com/en/webhooks/using-webhooks/validating-webhook-deliveries"},
	{"calendly", "/integrations/calendly/webhook", []string{"CALENDLY_WEBHOOK_SECRET"}, "Calendly-Webhook-Signature: t=.body HMAC-SHA256 hex", "https://developer.calendly.com/api-docs/overview/webhooks/webhook-signatures"},
	{"typeform", "/integrations/typeform/webhook", []string{"TYPEFORM_WEBHOOK_SECRET"}, "Typeform-Signature: sha256= base64 HMAC-SHA256 of raw body", "https://www.typeform.com/developers/webhooks/secure-your-webhooks/"},
	{"woocommerce", "/integrations/woocommerce/webhook", []string{"WOOCOMMERCE_WEBHOOK_SECRET"}, "X-WC-Webhook-Signature: base64 HMAC-SHA256 of encoded body", "https://woocommerce.com/document/webhooks/"},
	{"xero", "/integrations/xero/webhook", []string{"XERO_WEBHOOK_KEY"}, "X-Xero-Signature: base64 HMAC-SHA256 of raw body (200/401 intent)", "https://developer.xero.com/documentation/guides/webhooks/overview/"},
	{"quickbooks", "/integrations/quickbooks/webhook", []string{"QUICKBOOKS_VERIFIER_TOKEN"}, "X-Intuit-Signature: hex HMAC-SHA256 of raw body", "https://developer.intuit.com/app/developer/qbo/docs/develop/webhooks"},
	{"paddle", "/integrations/paddle/webhook", []string{"PADDLE_WEBHOOK_SECRET"}, "Paddle-Signature: ts;h1 HMAC-SHA256 of ts:body, 5s window", "https://developer.paddle.com/webhooks/about/signature-verification"},
	{"grafana", "/integrations/grafana/webhook", []string{"GRAFANA_WEBHOOK_SECRET"}, "X-Grafana-Alerting-Signature: hex HMAC-SHA256 of ts:body (optional)", "https://grafana.com/docs/grafana/latest/alerting/configure-notifications/manage-contact-points/integrations/webhook-notifier/"},
	{"datadog", "/integrations/datadog/webhook", []string{"DATADOG_SHARED_SECRET"}, "shared-secret header or basic auth (Datadog does not sign)", "https://docs.datadoghq.com/integrations/webhooks/"},
	{"intercom", "/integrations/intercom/webhook", []string{"INTERCOM_CLIENT_SECRET"}, "X-Hub-Signature: sha1= hex HMAC-SHA1 of raw body", "https://developers.intercom.com/docs/references/1.4/webhooks/webhook-models"},
	{"chargebee", "/integrations/chargebee/webhook", []string{"CHARGEBEE_WEBHOOK_USER", "CHARGEBEE_WEBHOOK_PASSWORD"}, "HTTP Basic auth (username/password in webhook settings)", "https://apidocs.chargebee.com/docs/api/events"},
	{"pipedrive", "/integrations/pipedrive/webhook", []string{"PIPEDRIVE_WEBHOOK_USER", "PIPEDRIVE_WEBHOOK_PASSWORD"}, "HTTP Basic auth or URL token (no HMAC)", "https://pipedrive.readme.io/docs/guide-for-webhooks"},
	{"generic", "/integrations/generic/webhook", []string{"GENERIC_WEBHOOK_SECRET"}, "Bearer token or X-Callhook-Secret (callhook event JSON passthrough)", "https://callhook.github.io/pages/integrations.html"},
	{"sns", "/integrations/sns/webhook", []string{"SNS_AUTO_SUBSCRIBE"}, "RSA cert-fetched verification (SigV2 SHA256withRSA), CloudWatch alarms or event JSON", "https://docs.aws.amazon.com/sns/latest/dg/sns-message-and-batch-json-formats.html"},
}

// MarshalCatalog renders the catalog as JSON (convenience for handlers).
func MarshalCatalog() string {
	b, _ := json.MarshalIndent(catalog, "", "  ")
	return string(b)
}
