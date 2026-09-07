package integrations

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// snsTestCert generates a self-signed RSA cert and serves it at a mock
// SigningCertURL (an *.amazonaws.com host), returning the PEM URL.
func snsTestCert(t *testing.T) (certURL string, priv *rsa.PrivateKey) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "sns.amazonaws.com"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})

	// Serve at a path matching the SNS cert pattern on an amazonaws.com host.
	// httptest listens on 127.0.0.1, so we rewrite the URL the message
	// carries after starting the server.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-pem-file")
		_, _ = w.Write(pemBytes)
	}))
	t.Cleanup(srv.Close)

	// The real URL host must pass validateCertURL. We fake the host in the
	// message but the fetch must hit the local server — so we swap the
	// scheme-verified host into the local port via URL rewrite below.
	// Simplest honest approach: patch the message URL to the local server,
	// and relax host validation for the test via a test-only override.
	localURL := srv.URL + "/SimpleNotificationService-xxx.pem"

	// Register a test fetch override: validateCertURL checks the host, so
	// we intercept fetchCertCached by pre-seeding the cache with our cert
	// keyed by the amazonaws-style URL the message will carry.
	fakeURL := "https://sns.us-east-1.amazonaws.com/SimpleNotificationService-xxx.pem"
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	certCache.Lock()
	certCache.certs[fakeURL] = cert
	certCache.Unlock()
	t.Cleanup(func() {
		certCache.Lock()
		delete(certCache.certs, fakeURL)
		certCache.Unlock()
	})

	_ = localURL
	return fakeURL, key
}

// signSNSNotification builds a signed Notification message (SignatureVersion 2).
func signSNSNotification(t *testing.T, msg *snsMessage, key *rsa.PrivateKey) {
	t.Helper()
	sts := buildStringToSign(msg)
	digest := sha256.Sum256([]byte(sts))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	msg.SignatureVersion = "2"
	msg.Signature = base64.StdEncoding.EncodeToString(sig)
}

func TestSNSNotificationCallhookEvent(t *testing.T) {
	certURL, key := snsTestCert(t)
	fire, got := recorderFire()
	h := Handler(NewSNS(SNSConfig{}), fire)

	inner := `{"id":"sns_evt_1","type":"invoice.due","customer_id":"cus_1002","payload":{"amount":"USD 49.00"}}`
	msg := &snsMessage{
		Type:           "Notification",
		MessageID:      "mid-1",
		TopicArn:       "arn:aws:sns:us-east-1:123:mytopic",
		Message:        inner,
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
		SigningCertURL: certURL,
	}
	signSNSNotification(t, msg, key)
	body, _ := json.Marshal(msg)

	rec := runAdapter(t, h, string(body), nil)
	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(*got) != 1 {
		t.Fatalf("expected 1 event, got %d", len(*got))
	}
	ev := (*got)[0]
	if ev.Type != "invoice.due" || ev.CustomerID != "cus_1002" || ev.ID != "sns_evt_1" {
		t.Fatalf("bad mapping: %+v", ev)
	}
}

func TestSNSNotificationCloudWatchAlarm(t *testing.T) {
	certURL, key := snsTestCert(t)
	fire, got := recorderFire()
	h := Handler(NewSNS(SNSConfig{}), fire)

	inner := `{"AlarmName":"PaymentAPIErrors","NewStateValue":"ALARM","OldStateValue":"OK","StateChangeTime":"2026-09-07T06:00:00.000Z","Region":"us-east-1","AlarmDescription":"Error rate high"}`
	msg := &snsMessage{
		Type:           "Notification",
		MessageID:      "mid-2",
		TopicArn:       "arn:aws:sns:us-east-1:123:alarms",
		Message:        inner,
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
		SigningCertURL: certURL,
	}
	signSNSNotification(t, msg, key)
	body, _ := json.Marshal(msg)

	rec := runAdapter(t, h, string(body), nil)
	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	ev := (*got)[0]
	if ev.Type != "account.warning" || ev.CustomerID != "oncall_paymentapierrors" {
		t.Fatalf("bad mapping: %+v", ev)
	}
}

func TestSNSNotificationOKAlarmSkipped(t *testing.T) {
	certURL, key := snsTestCert(t)
	fire, got := recorderFire()
	h := Handler(NewSNS(SNSConfig{}), fire)

	inner := `{"AlarmName":"PaymentAPIErrors","NewStateValue":"OK","OldStateValue":"ALARM"}`
	msg := &snsMessage{
		Type:           "Notification",
		MessageID:      "mid-3",
		TopicArn:       "arn:aws:sns:us-east-1:123:alarms",
		Message:        inner,
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
		SigningCertURL: certURL,
	}
	signSNSNotification(t, msg, key)
	body, _ := json.Marshal(msg)

	rec := runAdapter(t, h, string(body), nil)
	if rec.Code != 200 || len(*got) != 0 {
		t.Fatalf("OK state alarms must not fire: %d %d", rec.Code, len(*got))
	}
}

func TestSNSRejectsForgedSignature(t *testing.T) {
	certURL, key := snsTestCert(t)
	fire, got := recorderFire()
	h := Handler(NewSNS(SNSConfig{}), fire)

	msg := &snsMessage{
		Type:           "Notification",
		MessageID:      "mid-4",
		TopicArn:       "arn:aws:sns:us-east-1:123:mytopic",
		Message:        `{"id":"x","type":"invoice.due","customer_id":"c"}`,
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
		SigningCertURL: certURL,
	}
	signSNSNotification(t, msg, key)
	msg.Message = `{"id":"tampered","type":"invoice.due","customer_id":"attacker"}` // swap after signing
	body, _ := json.Marshal(msg)

	rec := runAdapter(t, h, string(body), nil)
	if rec.Code != 401 || len(*got) != 0 {
		t.Fatalf("tampered message must 401 without firing: %d %d", rec.Code, len(*got))
	}
}

func TestSNSRejectsBadCertURL(t *testing.T) {
	fire, _ := recorderFire()
	h := Handler(NewSNS(SNSConfig{}), fire)

	msg := map[string]string{
		"Type":             "Notification",
		"MessageId":        "mid-5",
		"Message":          "{}",
		"Timestamp":        time.Now().UTC().Format(time.RFC3339),
		"SigningCertURL":   "https://evil.example.com/SimpleNotificationService-x.pem",
		"SignatureVersion": "2",
		"Signature":        base64.StdEncoding.EncodeToString([]byte("junk")),
	}
	body, _ := json.Marshal(msg)
	rec := runAdapter(t, h, string(body), nil)
	if rec.Code != 401 {
		t.Fatalf("non-amazonaws cert URL must 401, got %d", rec.Code)
	}
}

func TestSNSLegacySignatureVersion1(t *testing.T) {
	certURL, key := snsTestCert(t)
	fire, got := recorderFire()
	h := Handler(NewSNS(SNSConfig{}), fire)

	inner := `{"id":"sns_v1","type":"promo.offer","customer_id":"cus_1001"}`
	msg := &snsMessage{
		Type:           "Notification",
		MessageID:      "mid-6",
		TopicArn:       "arn:aws:sns:us-east-1:123:old",
		Message:        inner,
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
		SigningCertURL: certURL,
	}
	sts := buildStringToSign(msg)
	digest := sha1.Sum([]byte(sts))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA1, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	msg.SignatureVersion = "1"
	msg.Signature = base64.StdEncoding.EncodeToString(sig)
	body, _ := json.Marshal(msg)

	rec := runAdapter(t, h, string(body), nil)
	if rec.Code != 200 || len(*got) != 1 {
		t.Fatalf("v1 signature should validate: %d %s", rec.Code, rec.Body.String())
	}
}

var _ = strings.TrimSpace
