package integrations

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// post adapter-helper: builds a request, runs the handler, returns recorder.
func runAdapter(t *testing.T, h http.HandlerFunc, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", "/hook", strings.NewReader(body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

// ---- Stripe ----

func TestStripeAdapterMapsPaymentFailed(t *testing.T) {
	fire, got := recorderFire()
	h := Handler(NewStripe(StripeConfig{Secret: "whsec_test"}), fire)

	body := `{"id":"evt_100","type":"invoice.payment_failed","data":{"object":{"customer":"cus_1002","customer_phone":"+14155550002","amount_due":4900,"currency":"usd","attempt_count":2}}}`
	sig := signTimestampBody("whsec_test", time.Now().Unix(), []byte(body))

	rec := runAdapter(t, h, body, map[string]string{"Stripe-Signature": sig})
	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(*got) != 1 {
		t.Fatalf("expected 1 event, got %d", len(*got))
	}
	ev := (*got)[0]
	if ev.Type != "invoice.due" || ev.CustomerID != "cus_1002" || ev.ID != "stripe_evt_100" {
		t.Fatalf("bad mapping: %+v", ev)
	}
	if ev.Phone != "+14155550002" {
		t.Fatalf("phone not passed through: %q", ev.Phone)
	}
}

func TestStripeAdapterSkipsUnsubscribed(t *testing.T) {
	fire, got := recorderFire()
	h := Handler(NewStripe(StripeConfig{Secret: "whsec_test"}), fire)

	body := `{"id":"evt_101","type":"invoice.paid","data":{"object":{"customer":"cus_1"}}}`
	sig := signTimestampBody("whsec_test", time.Now().Unix(), []byte(body))
	rec := runAdapter(t, h, body, map[string]string{"Stripe-Signature": sig})
	if rec.Code != 200 || len(*got) != 0 {
		t.Fatalf("unsubscribed event should 200 with no fire: %d %d", rec.Code, len(*got))
	}
}

// ---- Slack ----

func TestSlackAdapterSlashCommand(t *testing.T) {
	fire, got := recorderFire()
	h := Handler(NewSlack(SlackConfig{SigningSecret: "ss"}), fire)

	body := "command=%2Fcall&text=invoice.due+cus_1002&user_id=U123&channel_id=C456"
	ts := fmt.Sprintf("%d", time.Now().Unix())
	mac := hmac.New(sha256.New, []byte("ss"))
	mac.Write([]byte("v0:" + ts + ":" + body))
	sig := "v0=" + hex.EncodeToString(mac.Sum(nil))

	rec := runAdapter(t, h, body, map[string]string{
		"X-Slack-Request-Timestamp": ts,
		"X-Slack-Signature":         sig,
		"Content-Type":              "application/x-www-form-urlencoded",
	})
	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if len(*got) != 1 {
		t.Fatalf("expected 1 event, got %d", len(*got))
	}
	ev := (*got)[0]
	if ev.Type != "invoice.due" || ev.CustomerID != "cus_1002" {
		t.Fatalf("bad mapping: %+v", ev)
	}
}

func TestSlackAdapterUsageMessage(t *testing.T) {
	fire, got := recorderFire()
	h := Handler(NewSlack(SlackConfig{SigningSecret: "ss"}), fire)

	body := "command=%2Fcall&text=help&user_id=U123"
	ts := fmt.Sprintf("%d", time.Now().Unix())
	mac := hmac.New(sha256.New, []byte("ss"))
	mac.Write([]byte("v0:" + ts + ":" + body))
	sig := "v0=" + hex.EncodeToString(mac.Sum(nil))

	rec := runAdapter(t, h, body, map[string]string{
		"X-Slack-Request-Timestamp": ts,
		"X-Slack-Signature":         sig,
	})
	if rec.Code != 200 {
		t.Fatalf("usage response should be 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Usage") {
		t.Fatalf("expected usage message, got %s", rec.Body.String())
	}
	if len(*got) != 0 {
		t.Fatal("no event should fire for malformed command")
	}
}

// ---- Shopify ----

func TestShopifyAdapterPaymentFailure(t *testing.T) {
	fire, got := recorderFire()
	h := Handler(NewShopify(ShopifyConfig{Secret: "shp"}), fire)

	body := `{"id":4507894699,"phone":"+15551230000","total_price":"41.94","currency":"USD","customer":{"id":207119551,"phone":"+15551230000"}}`
	mac := hmac.New(sha256.New, []byte("shp"))
	mac.Write([]byte(body))
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	rec := runAdapter(t, h, body, map[string]string{
		"X-Shopify-Hmac-Sha256": sig,
		"X-Shopify-Topic":       "orders/payment_failure",
	})
	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	ev := (*got)[0]
	if ev.Type != "invoice.due" || ev.CustomerID != "shopify_cus_207119551" {
		t.Fatalf("bad mapping: %+v", ev)
	}
}

// ---- GitHub ----

func TestGitHubAdapterDeploymentFailure(t *testing.T) {
	fire, got := recorderFire()
	h := Handler(NewGitHub(GitHubConfig{Secret: "ghs"}), fire)

	body := `{"action":"completed","deployment":{"id":123,"environment":"production"},"deployment_status":{"state":"failure"},"repository":{"full_name":"acme/app"},"sender":{"login":"octocat"}}`
	mac := hmac.New(sha256.New, []byte("ghs"))
	mac.Write([]byte(body))
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	rec := runAdapter(t, h, body, map[string]string{
		"X-Hub-Signature-256": sig,
		"X-GitHub-Event":      "deployment_status",
	})
	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	ev := (*got)[0]
	if ev.Type != "account.warning" || ev.CustomerID != "gh_octocat" {
		t.Fatalf("bad mapping: %+v", ev)
	}
}

func TestGitHubAdapterSkipsNonProduction(t *testing.T) {
	fire, got := recorderFire()
	h := Handler(NewGitHub(GitHubConfig{Secret: "ghs"}), fire)

	body := `{"deployment":{"id":124,"environment":"staging"},"deployment_status":{"state":"failure"},"sender":{"login":"octocat"}}`
	mac := hmac.New(sha256.New, []byte("ghs"))
	mac.Write([]byte(body))
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	rec := runAdapter(t, h, body, map[string]string{
		"X-Hub-Signature-256": sig,
		"X-GitHub-Event":      "deployment_status",
	})
	if rec.Code != 200 || len(*got) != 0 {
		t.Fatal("staging deployment failures should be skipped")
	}
}

// ---- HubSpot ----

func TestHubSpotAdapterV3(t *testing.T) {
	fire, got := recorderFire()
	h := Handler(NewHubSpot(HubSpotConfig{ClientSecret: "cs"}), fire)

	// The adapter reconstructs the URI as scheme://host + RequestURI
	// (http for httptest, which has no TLS).
	uri := "http://example.com/hook"
	body := `{"subscriptionType":"deal.propertyChange","objectId":"987654","propertyName":"dealstage","propertyValue":"negotiation"}`
	ts := fmt.Sprintf("%d", time.Now().UnixMilli())
	message := "POST" + uri + body + ts
	mac := hmac.New(sha256.New, []byte("cs"))
	mac.Write([]byte(message))
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest("POST", uri, strings.NewReader(body))
	req.Host = "example.com"
	req.Header.Set("X-HubSpot-Signature-V3", sig)
	req.Header.Set("X-HubSpot-Request-Timestamp", ts)
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	ev := (*got)[0]
	if ev.CustomerID != "hs_987654" || ev.Type != "appointment.reminder" {
		t.Fatalf("bad mapping: %+v", ev)
	}
}

// ---- PagerDuty ----

func TestPagerDutyAdapterIncidentTriggered(t *testing.T) {
	fire, got := recorderFire()
	h := Handler(NewPagerDuty(PagerDutyConfig{Secret: "pds"}), fire)

	body := `{"id":"pd_evt_1","event_type":"incident.triggered","resource":{"type":"incident","id":"Q2J9X9","title":"Database is down","urgency":"high","html_url":"https://acme.pagerduty.com/incidents/Q2J9X9","assignees":[{"assignee":{"id":"P1K9ZZ","summary":"Jane Doe (jane@corp.com)","email":"jane@corp.com","type":"user"}}]}}`
	mac := hmac.New(sha256.New, []byte("pds"))
	mac.Write([]byte(body))
	sig := hex.EncodeToString(mac.Sum(nil))

	rec := runAdapter(t, h, body, map[string]string{"X-PagerDuty-Signature": sig})
	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	ev := (*got)[0]
	if ev.Type != "account.warning" || ev.CustomerID != "pd_jane@corp.com" {
		t.Fatalf("bad mapping: %+v", ev)
	}
}

// ---- Calendly ----

func TestCalendlyAdapterNoShow(t *testing.T) {
	fire, got := recorderFire()
	h := Handler(NewCalendly(CalendlyConfig{Secret: "cals"}), fire)

	body := `{"event":"invitee.no_show","created_at":"2026-09-07T10:00:00Z","payload":{"uri":"https://api.calendly.com/invitees/AAAA-1111","email":"j@example.com","name":"Jo","text_reminder_number":"+15551234321","scheduled_event":{"start_time":"2026-09-08T15:00:00Z"},"event_type":{"name":"30 Minute Meeting"}}}`
	sig := signTimestampBody("cals", time.Now().Unix(), []byte(body))

	rec := runAdapter(t, h, body, map[string]string{"Calendly-Webhook-Signature": sig})
	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	ev := (*got)[0]
	if ev.Type != "appointment.reminder" || ev.CustomerID != "calendly_AAAA-1111" {
		t.Fatalf("bad mapping: %+v", ev)
	}
	if ev.Phone != "+15551234321" {
		t.Fatalf("phone expected from text_reminder_number: %q", ev.Phone)
	}
}

// ---- Typeform ----

func TestTypeformAdapterFormResponse(t *testing.T) {
	fire, got := recorderFire()
	h := Handler(NewTypeform(TypeformConfig{Secret: "tfs"}), fire)

	body := `{"event_id":"01HVEM...","event_type":"form_response","form_response":{"form_id":"abc123","token":"xyz","submitted_at":"2026-09-07","hidden":{"customer_id":"cus_1001","phone":"+919900000001"},"answers":[{"field":{"id":"nC4ka2","type":"phone","ref":"phone"},"type":"phone","phone":"+919900000001"},{"field":{"id":"k6a2","type":"text","ref":"comments"},"type":"text","text":"Great service"}]}}`
	mac := hmac.New(sha256.New, []byte("tfs"))
	mac.Write([]byte(body))
	sig := "sha256=" + base64.StdEncoding.EncodeToString(mac.Sum(nil))

	rec := runAdapter(t, h, body, map[string]string{"Typeform-Signature": sig})
	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	ev := (*got)[0]
	if ev.Type != "feedback.request" || ev.CustomerID != "cus_1001" {
		t.Fatalf("bad mapping: %+v", ev)
	}
	if ev.Phone != "+919900000001" {
		t.Fatalf("hidden phone not picked up: %q", ev.Phone)
	}
}

// ---- WooCommerce ----

func TestWooCommerceAdapterFailedOrder(t *testing.T) {
	fire, got := recorderFire()
	h := Handler(NewWooCommerce(WooCommerceConfig{Secret: "wcs"}), fire)

	body := `{"id":120,"status":"failed","total":"31.95","customer_id":55,"billing":{"phone":"+15559990111","email":"s@example.com"}}`
	mac := hmac.New(sha256.New, []byte("wcs"))
	mac.Write([]byte(body))
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	rec := runAdapter(t, h, body, map[string]string{"X-WC-Webhook-Signature": sig})
	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	ev := (*got)[0]
	if ev.Type != "payment.failed" || ev.CustomerID != "woo_cus_55" {
		t.Fatalf("bad mapping: %+v", ev)
	}
	if ev.Phone != "+15559990111" {
		t.Fatalf("billing phone not passed: %q", ev.Phone)
	}
}

// ---- Xero ----

func TestXeroAdapterOverdueInvoice(t *testing.T) {
	fire, got := recorderFire()
	h := Handler(NewXero(XeroConfig{Key: "xk"}), fire)

	body := `{"events":[{"eventId":"guid-1","eventDateUtc":"2026-09-07T00:00:00","eventType":"UPDATE","eventCategory":"INVOICE","tenantId":"t1","resourceId":"r1","payload":{"Invoice":{"InvoiceID":"inv-9","InvoiceNumber":"INV-0042","Type":"ACCREC","Total":250.0,"CurrencyCode":"AUD","Contact":{"ContactID":"c-77","Name":"Wool","EmailAddress":"w@example.com"}}}}]}`
	mac := hmac.New(sha256.New, []byte("xk"))
	mac.Write([]byte(body))
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	rec := runAdapter(t, h, body, map[string]string{"X-Xero-Signature": sig})
	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	ev := (*got)[0]
	if ev.Type != "invoice.due" || ev.CustomerID != "xero_c-77" {
		t.Fatalf("bad mapping: %+v", ev)
	}
}

// ---- QuickBooks ----

func TestQuickBooksAdapterInvoiceUpdate(t *testing.T) {
	fire, got := recorderFire()
	h := Handler(NewQuickBooks(QuickBooksConfig{Token: "qbt"}), fire)

	body := `{"eventNotifications":[{"realmId":"123146096291789","dataChangeEvent":{"entities":[{"name":"Invoice","id":"173","operation":"Update","lastUpdated":"2026-09-07"}]}}]}`
	mac := hmac.New(sha256.New, []byte("qbt"))
	mac.Write([]byte(body))
	sig := hex.EncodeToString(mac.Sum(nil))

	rec := runAdapter(t, h, body, map[string]string{"X-Intuit-Signature": sig})
	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	ev := (*got)[0]
	if ev.Type != "invoice.due" || ev.CustomerID != "qbo_123146096291789_inv_173" {
		t.Fatalf("bad mapping: %+v", ev)
	}
}

// ---- Paddle ----

func TestPaddleAdapterPaymentFailed(t *testing.T) {
	fire, got := recorderFire()
	h := Handler(NewPaddle(PaddleConfig{Secret: "pdl"}), fire)

	body := `{"event_id":"evt_01h","event_type":"transaction.payment_failed","data":{"customer_id":"ctm_01j","transaction":{"id":"txn_01h","billing_details":{"totals":[{"total":"2500","currency_code":"USD"}]}}}}`
	ts := time.Now().Unix()
	mac := hmac.New(sha256.New, []byte("pdl"))
	mac.Write([]byte(fmt.Sprintf("%d:%s", ts, body)))
	header := fmt.Sprintf("ts=%d;h1=%s", ts, hex.EncodeToString(mac.Sum(nil)))

	rec := runAdapter(t, h, body, map[string]string{"Paddle-Signature": header})
	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	ev := (*got)[0]
	if ev.Type != "payment.failed" || ev.CustomerID != "paddle_ctm_01j" {
		t.Fatalf("bad mapping: %+v", ev)
	}
}

// ---- Grafana ----

func TestGrafanaAdapterFiringAlert(t *testing.T) {
	fire, got := recorderFire()
	h := Handler(NewGrafana(GrafanaConfig{Secret: "gr"}), fire)

	body := `{"receiver":"oncall","status":"firing","alerts":[{"status":"firing","labels":{"alertname":"HighErrorRate","team":"payments"},"annotations":{"summary":"Error rate > 5%"},"startsAt":"2026-09-07T06:00:00Z","generatorURL":"https://grafana.example.com","fingerprint":"fp1"}],"commonLabels":{"team":"payments"}}`
	mac := hmac.New(sha256.New, []byte("gr"))
	mac.Write([]byte(body))
	sig := hex.EncodeToString(mac.Sum(nil))

	rec := runAdapter(t, h, body, map[string]string{"X-Grafana-Alerting-Signature": sig})
	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	ev := (*got)[0]
	if ev.Type != "account.warning" || ev.CustomerID != "oncall_payments" {
		t.Fatalf("bad mapping: %+v", ev)
	}
}

// ---- Intercom ----

func TestIntercomAdapterAtRiskTag(t *testing.T) {
	fire, got := recorderFire()
	h := Handler(NewIntercom(IntercomConfig{ClientSecret: "ics"}), fire)

	body := `{"type":"notification_event","id":"notif_1","topic":"user.tag.created","app_id":"a86d","data":{"item":{"type":"tag","id":"t1","user":{"id":"usr_531","email":"u@example.com","phone":"+15558889999"},"tag":{"id":"t1","name":"at-risk"}}}}`
	mac := hmac.New(sha1.New, []byte("ics"))
	mac.Write([]byte(body))
	sig := "sha1=" + hex.EncodeToString(mac.Sum(nil))

	rec := runAdapter(t, h, body, map[string]string{"X-Hub-Signature": sig})
	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	ev := (*got)[0]
	if ev.Type != "account.warning" || ev.CustomerID != "intercom_usr_531" {
		t.Fatalf("bad mapping: %+v", ev)
	}
}

// ---- Chargebee (basic auth) ----

func TestChargebeeAdapterPaymentFailed(t *testing.T) {
	fire, got := recorderFire()
	h := Handler(NewChargebee(ChargebeeConfig{User: "cbuser", Pass: "cbpass"}), fire)

	body := `{"id":"ev_1","event_type":"payment_failed","occurred_at":1690000000,"content":{"customer":{"id":"11u","email":"c@example.com","phone":"+15557776666"},"invoice":{"id":"INV-1","total":4900,"currency_code":"USD"}}}`
	auth := base64.StdEncoding.EncodeToString([]byte("cbuser:cbpass"))

	rec := runAdapter(t, h, body, map[string]string{"Authorization": "Basic " + auth})
	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	ev := (*got)[0]
	if ev.Type != "payment.failed" || ev.CustomerID != "chargebee_11u" {
		t.Fatalf("bad mapping: %+v", ev)
	}
	if ev.Phone != "+15557776666" {
		t.Fatalf("customer phone not passed: %q", ev.Phone)
	}
}

func TestChargebeeAdapterRejectsBadBasicAuth(t *testing.T) {
	fire, got := recorderFire()
	h := Handler(NewChargebee(ChargebeeConfig{User: "cbuser", Pass: "cbpass"}), fire)

	body := `{"id":"ev_1","event_type":"payment_failed","content":{"customer":{"id":"11u"}}}`
	auth := base64.StdEncoding.EncodeToString([]byte("cbuser:wrong"))

	rec := runAdapter(t, h, body, map[string]string{"Authorization": "Basic " + auth})
	if rec.Code != 401 || len(*got) != 0 {
		t.Fatalf("bad basic auth must 401 without firing: %d", rec.Code)
	}
}

// ---- Pipedrive ----

func TestPipedriveAdapterLostDeal(t *testing.T) {
	fire, got := recorderFire()
	h := Handler(NewPipedrive(PipedriveConfig{User: "pduser", Pass: "pdpass"}), fire)

	body := `{"meta":{"action":"updated","object":"deal","webhook_id":1},"data":{"id":42,"title":"Big deal","status":"lost","person_id":7,"rotting_since":""},"previous":{"status":"open"}}`
	auth := base64.StdEncoding.EncodeToString([]byte("pduser:pdpass"))

	rec := runAdapter(t, h, body, map[string]string{"Authorization": "Basic " + auth})
	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	ev := (*got)[0]
	if ev.Type != "promo.offer" || ev.CustomerID != "pipedrive_deal_42" {
		t.Fatalf("bad mapping: %+v", ev)
	}
}

// ---- Datadog ----

func TestDatadogAdapterTriggeredMonitor(t *testing.T) {
	fire, got := recorderFire()
	h := Handler(NewDatadog(DatadogConfig{SharedSecret: "dds", SharedHeader: "X-Callhook-Secret"}), fire)

	body := `{"title":"CPU is high","alert_transition":"Triggered","alert_priority":"P1","event_msg":"CPU > 90%","tags":"env:prod,team:payments","date_posix":1690000000}`
	rec := runAdapter(t, h, body, map[string]string{"X-Callhook-Secret": "dds"})
	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	ev := (*got)[0]
	if ev.Type != "account.warning" || ev.CustomerID != "oncall_payments" {
		t.Fatalf("bad mapping: %+v", ev)
	}
}

func TestDatadogAdapterSkipsRecovered(t *testing.T) {
	fire, got := recorderFire()
	h := Handler(NewDatadog(DatadogConfig{SharedSecret: "dds"}), fire)

	body := `{"title":"CPU is high","alert_transition":"Recovered","tags":"team:payments","date_posix":1690000000}`
	rec := runAdapter(t, h, body, map[string]string{"X-Callhook-Secret": "dds"})
	if rec.Code != 200 || len(*got) != 0 {
		t.Fatal("recovered monitors must not fire")
	}
}

// ---- Generic ----

func TestGenericAdapterBearer(t *testing.T) {
	fire, got := recorderFire()
	h := Handler(NewGeneric(GenericConfig{Token: "tok123"}), fire)

	body := `{"id":"evt_9","type":"invoice.due","customer_id":"cus_1002","phone":"+15551234567","payload":{"amount":"USD 49.00"}}`
	rec := runAdapter(t, h, body, map[string]string{"Authorization": "Bearer tok123"})
	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	ev := (*got)[0]
	if ev.Type != "invoice.due" || ev.CustomerID != "cus_1002" || ev.Phone != "+15551234567" {
		t.Fatalf("bad mapping: %+v", ev)
	}
	if !strings.Contains(ev.Payload, "49.00") {
		t.Fatalf("payload not passed: %q", ev.Payload)
	}
}

func TestGenericAdapterRejectsMissingFields(t *testing.T) {
	fire, _ := recorderFire()
	h := Handler(NewGeneric(GenericConfig{Secret: "s"}), fire)

	rec := runAdapter(t, h, `{"id":"x"}`, map[string]string{"X-Callhook-Secret": "s"})
	if rec.Code != 400 {
		t.Fatalf("incomplete event must 400, got %d", rec.Code)
	}
}
