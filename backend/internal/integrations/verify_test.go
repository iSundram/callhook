package integrations

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// --- verification primitive tests ---

func TestVerifyHexHMAC(t *testing.T) {
	body := []byte(`{"hello":"world"}`)
	mac := hmac.New(sha256.New, []byte("secret"))
	mac.Write(body)
	sig := hex.EncodeToString(mac.Sum(nil))

	if !VerifyHexHMAC("secret", body, sig) {
		t.Fatal("valid signature rejected")
	}
	if !VerifyHexHMAC("secret", body, "sha256="+sig) {
		t.Fatal("sha256= prefix should be stripped")
	}
	if VerifyHexHMAC("secret", body, strings.Repeat("a", len(sig))) {
		t.Fatal("forged signature accepted")
	}
	if VerifyHexHMAC("wrong", body, sig) {
		t.Fatal("wrong secret accepted")
	}
}

func TestVerifyBase64HMAC(t *testing.T) {
	body := []byte(`{"a":1}`)
	mac := hmac.New(sha256.New, []byte("s"))
	mac.Write(body)
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	if !VerifyBase64HMAC("s", body, sig) {
		t.Fatal("valid base64 signature rejected")
	}
	if !VerifyBase64HMAC("s", body, "sha256="+sig) {
		t.Fatal("sha256= prefix should be stripped (Typeform)")
	}
	if VerifyBase64HMAC("s", body, "AAAA"+sig[4:]) {
		t.Fatal("forged base64 signature accepted")
	}
}

// GitHub's documented test vector: secret "It's a Secret to Everybody",
// payload "Hello, World!", signature
// 757107ea0eb2509fc211221cce984b8a37570b6d7586c22c46f4379c8b043e17.
func TestGitHubDocsTestVector(t *testing.T) {
	if !VerifyHexHMAC("It's a Secret to Everybody", []byte("Hello, World!"),
		"sha256=757107ea0eb2509fc211221cce984b8a37570b6d7586c22c46f4379c8b043e17") {
		t.Fatal("GitHub documented test vector failed")
	}
}

func TestVerifySHA1HexHMAC(t *testing.T) {
	body := []byte(`{"topic":"user.created"}`)
	mac := hmac.New(sha1.New, []byte("cs"))
	mac.Write(body)
	sig := hex.EncodeToString(mac.Sum(nil))

	if !VerifySHA1HexHMAC("cs", body, "sha1="+sig) {
		t.Fatal("valid intercom signature rejected")
	}
	if VerifySHA1HexHMAC("cs", body, "sha1="+strings.Repeat("0", 40)) {
		t.Fatal("forged intercom signature accepted")
	}
}

func TestVerifyStripeReplayWindow(t *testing.T) {
	body := []byte(`{}`)
	origNow := now
	now = func() time.Time { return time.Unix(1_000_000, 0) }
	t.Cleanup(func() { now = origNow })
	// Freeze within the test body but use wall time for tolerance math by
	// pinning everything relative to the frozen clock.

	// Fresh timestamp passes.
	fresh := signTimestampBody("sec", 1_000_000-60, body)
	if !VerifyStripe("sec", body, fresh) {
		t.Fatal("fresh stripe signature rejected")
	}
	// Stale timestamp (beyond 5 min) rejected even with valid HMAC.
	stale := signTimestampBody("sec", 1_000_000-3600, body)
	if VerifyStripe("sec", body, stale) {
		t.Fatal("stale stripe signature accepted (replay)")
	}
	// Bad secret rejected.
	if VerifyStripe("nope", body, fresh) {
		t.Fatal("wrong stripe secret accepted")
	}
}

func signTimestampBody(secret string, ts int64, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(fmt.Sprintf("%d.", ts)))
	mac.Write(body)
	return fmt.Sprintf("t=%d,v1=%s", ts, hex.EncodeToString(mac.Sum(nil)))
}

func TestVerifySlack(t *testing.T) {
	body := []byte(`command=/call&text=invoice.due+cus_1002`)
	ts := fmt.Sprintf("%d", time.Now().Unix())
	mac := hmac.New(sha256.New, []byte("signing-secret"))
	mac.Write([]byte("v0:" + ts + ":" + string(body)))
	sig := "v0=" + hex.EncodeToString(mac.Sum(nil))

	if !VerifySlack("signing-secret", body, ts, sig) {
		t.Fatal("valid slack signature rejected")
	}
	if VerifySlack("signing-secret", body, ts, "v0="+strings.Repeat("a", 64)) {
		t.Fatal("forged slack signature accepted")
	}
	// Replay: 10 minutes old.
	oldTS := fmt.Sprintf("%d", time.Now().Add(-10*time.Minute).Unix())
	mac2 := hmac.New(sha256.New, []byte("signing-secret"))
	mac2.Write([]byte("v0:" + oldTS + ":" + string(body)))
	if VerifySlack("signing-secret", body, oldTS, "v0="+hex.EncodeToString(mac2.Sum(nil))) {
		t.Fatal("replayed slack signature accepted")
	}
}

func TestVerifyHubSpotV3(t *testing.T) {
	method, uri := "POST", "https://example.com/integrations/hubspot/webhook"
	body := []byte(`{"subscriptionType":"deal.propertyChange"}`)
	ts := fmt.Sprintf("%d", time.Now().UnixMilli())

	message := method + uri + string(body) + ts
	mac := hmac.New(sha256.New, []byte("cs"))
	mac.Write([]byte(message))
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	if !VerifyHubSpotV3("cs", method, uri, body, ts, sig) {
		t.Fatal("valid hubspot v3 signature rejected")
	}
	if VerifyHubSpotV3("cs", method, uri, body, ts, "AAAA") {
		t.Fatal("forged hubspot signature accepted")
	}
	// Stale timestamp (6 minutes).
	oldTS := fmt.Sprintf("%d", time.Now().Add(-6*time.Minute).UnixMilli())
	if VerifyHubSpotV3("cs", method, uri, body, oldTS, sig) {
		t.Fatal("stale hubspot signature accepted")
	}
}

func TestVerifyPaddle(t *testing.T) {
	body := []byte(`{"event_type":"transaction.payment_failed"}`)
	ts := time.Now().Unix()
	mac := hmac.New(sha256.New, []byte("sk"))
	mac.Write([]byte(fmt.Sprintf("%d:%s", ts, body)))
	sig := hex.EncodeToString(mac.Sum(nil))
	header := fmt.Sprintf("ts=%d;h1=%s", ts, sig)

	if !VerifyPaddle("sk", body, header, 60*time.Second) {
		t.Fatal("valid paddle signature rejected")
	}
	if VerifyPaddle("sk", body, "ts=1;h1="+strings.Repeat("a", 64), 60*time.Second) {
		t.Fatal("forged paddle signature accepted")
	}
	// Signing with a 61s-old timestamp must fail within the 60s window.
	oldTS := time.Now().Add(-61 * time.Second).Unix()
	mac2 := hmac.New(sha256.New, []byte("sk"))
	mac2.Write([]byte(fmt.Sprintf("%d:%s", oldTS, body)))
	if VerifyPaddle("sk", body, fmt.Sprintf("ts=%d;h1=%s", oldTS, hex.EncodeToString(mac2.Sum(nil))), 60*time.Second) {
		t.Fatal("stale paddle signature accepted")
	}
}

func TestVerifyGrafana(t *testing.T) {
	body := []byte(`{"status":"firing"}`)
	// With timestamp.
	mac := hmac.New(sha256.New, []byte("g"))
	mac.Write([]byte("123:"))
	mac.Write(body)
	if !VerifyGrafana("g", body, "123", hex.EncodeToString(mac.Sum(nil))) {
		t.Fatal("valid grafana signature (timestamped) rejected")
	}
	// Body-only.
	mac2 := hmac.New(sha256.New, []byte("g"))
	mac2.Write(body)
	if !VerifyGrafana("g", body, "", hex.EncodeToString(mac2.Sum(nil))) {
		t.Fatal("valid grafana signature (body only) rejected")
	}
}

// --- handler wrapper tests ---

// recorderFire captures fired events.
func recorderFire() (Fire, *[]Event) {
	var got []Event
	return func(ev Event) error {
		got = append(got, ev)
		return nil
	}, &got
}

func TestHandlerRejectsBadSignature(t *testing.T) {
	fire, got := recorderFire()
	h := Handler(NewStripe(StripeConfig{Secret: "whsec_x"}), fire)

	body := `{"id":"evt_1","type":"invoice.payment_failed","data":{"object":{"customer":"cus_1","amount_due":4900,"currency":"usd"}}}`
	req := httptest.NewRequest("POST", "/integrations/stripe/webhook", strings.NewReader(body))
	req.Header.Set("Stripe-Signature", "t=1,v1="+strings.Repeat("a", 64))
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != 401 {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	if len(*got) != 0 {
		t.Fatal("event fired despite bad signature")
	}
}

func TestHandlerMissingSecret(t *testing.T) {
	fire, _ := recorderFire()
	h := Handler(NewStripe(StripeConfig{Secret: ""}), fire)

	req := httptest.NewRequest("POST", "/x", strings.NewReader(`{}`))
	req.Header.Set("Stripe-Signature", "t=1,v1=x")
	rec := httptest.NewRecorder()
	h(rec, req)
	if rec.Code != 401 {
		t.Fatalf("expected 401 for unconfigured secret, got %d", rec.Code)
	}
}
