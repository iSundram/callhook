// Package integrations adapts third-party platform webhooks to callhook
// events. Every adapter does exactly three things: verify the platform's
// signature, map the platform payload to an events.Event, and forward it
// into the pipeline via the Fire function.
//
// This file holds the shared signature-verification primitives — stdlib
// only, zero dependencies. Each scheme below was verified against the
// platform's official documentation (see integrations/README.md).
package integrations

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Fire forwards a verified, mapped event into the callhook pipeline.
// It is srv.ProcessEvent in production; a recorder in tests.
type Fire func(ev Event) error

// Event is the minimal shape an adapter produces; it is converted to
// events.Event by the wiring layer. Kept local so this package never
// imports the pipeline internals.
type Event struct {
	ID         string
	Type       string
	CustomerID string
	Phone      string
	Payload    string // JSON
}

// now is swappable for timestamp-tolerance tests.
var now = time.Now

// --- shared HMAC helpers ---

// hmacSHA256 returns the raw HMAC-SHA256 digest of message keyed by secret.
func hmacSHA256(secret string, message []byte) []byte {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(message)
	return mac.Sum(nil)
}

// hmacSHA1 returns the raw HMAC-SHA1 digest (Intercom's legacy scheme).
func hmacSHA1(secret string, message []byte) []byte {
	mac := hmac.New(sha1.New, []byte(secret))
	mac.Write(message)
	return mac.Sum(nil)
}

// hmacEqual is a constant-time comparison of two string digests.
func hmacEqual(a, b string) bool {
	return hmac.Equal([]byte(a), []byte(b))
}

// VerifyHexHMAC checks hexEncoded against hex(HMAC-SHA256(secret, message)).
// sigHeader may carry an optional prefix (e.g. "sha256=") which is stripped.
func VerifyHexHMAC(secret string, message []byte, sigHeader string) bool {
	sig := strings.TrimPrefix(strings.TrimSpace(sigHeader), "sha256=")
	expected := hex.EncodeToString(hmacSHA256(secret, message))
	return hmacEqual(expected, sig)
}

// VerifyBase64HMAC checks sigHeader against base64(HMAC-SHA256(secret, message)).
// An optional "sha256=" prefix (Typeform) is stripped.
func VerifyBase64HMAC(secret string, message []byte, sigHeader string) bool {
	sig := strings.TrimPrefix(strings.TrimSpace(sigHeader), "sha256=")
	expected := base64.StdEncoding.EncodeToString(hmacSHA256(secret, message))
	return hmacEqual(expected, sig)
}

// VerifySHA1HexHMAC checks sigHeader against hex(HMAC-SHA1(secret, message))
// with Intercom's "sha1=" prefix. (Intercom docs: "The X-Hub-Signature header
// value starts with the string sha1= followed by the signature", computed as
// "the hexadecimal (40-byte) representation of a SHA-1 signature".)
func VerifySHA1HexHMAC(secret string, message []byte, sigHeader string) bool {
	sig := strings.TrimPrefix(strings.TrimSpace(sigHeader), "sha1=")
	expected := hex.EncodeToString(hmacSHA1(secret, message))
	return hmacEqual(expected, sig)
}

// parseSignedHeader parses "t=123,v1=abc,..."-style headers (Stripe,
// Calendly). Returns the timestamp and all v1 signatures.
func parseSignedHeader(header string) (ts int64, v1s []string) {
	for _, kv := range strings.Split(header, ",") {
		p := strings.SplitN(kv, "=", 2)
		if len(p) != 2 {
			continue
		}
		key, val := strings.TrimSpace(p[0]), strings.TrimSpace(p[1])
		switch key {
		case "t":
			ts, _ = strconv.ParseInt(val, 10, 64)
		case "v1":
			v1s = append(v1s, val)
		}
	}
	return ts, v1s
}

// verifyTimestampSigned checks a "t=...,v1=..." header where the signed
// payload is timestamp + "." + body (Stripe and Calendly use exactly this
// construction). tolerance bounds the replay window; 0 disables the check.
//
// Stripe docs: signed_payload = "timestamp + '.' + JSON payload", HMAC-SHA256
// with the whsec_ endpoint secret, v1 scheme, 5-minute default tolerance.
// Calendly docs: "concatenating the timestamp (t), the character '.',
// and the request body's JSON payload", HMAC-SHA256 hex, ~3-minute tolerance.
func verifyTimestampSigned(secret string, body []byte, header string, tolerance time.Duration) bool {
	ts, v1s := parseSignedHeader(header)
	if len(v1s) == 0 {
		return false
	}
	if tolerance > 0 {
		age := now().Unix() - ts
		if age < 0 {
			age = -age
		}
		if age > int64(tolerance/time.Second) {
			return false
		}
	}
	signedPayload := append([]byte(fmt.Sprintf("%d.", ts)), body...)
	expected := hex.EncodeToString(hmacSHA256(secret, signedPayload))
	for _, v1 := range v1s {
		if hmacEqual(expected, v1) {
			return true
		}
	}
	return false
}

// VerifyStripe checks the Stripe-Signature header: t= and v1= entries,
// signed payload = "t.body", hex HMAC-SHA256, 5-minute tolerance.
func VerifyStripe(secret string, body []byte, header string) bool {
	return verifyTimestampSigned(secret, body, header, 5*time.Minute)
}

// VerifyCalendly checks the Calendly-Webhook-Signature header: t= and v1=
// entries, signed payload = "t.body", hex HMAC-SHA256, 5-minute tolerance.
func VerifyCalendly(secret string, body []byte, header string) bool {
	return verifyTimestampSigned(secret, body, header, 5*time.Minute)
}

// VerifySlack checks X-Slack-Signature against the basestring
// "v0:timestamp:body" (Slack docs), hex HMAC-SHA256 with the app signing
// secret, 5-minute replay window on X-Slack-Request-Timestamp.
func VerifySlack(secret string, body []byte, timestamp, sigHeader string) bool {
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false
	}
	age := now().Unix() - ts
	if age < 0 {
		age = -age
	}
	if age > 300 {
		return false
	}
	basestring := "v0:" + timestamp + ":" + string(body)
	expected := "v0=" + hex.EncodeToString(hmacSHA256(secret, []byte(basestring)))
	return hmacEqual(expected, sigHeader)
}

// VerifyGrafana checks X-Grafana-Alerting-Signature (header name is
// configurable; this is the default). Grafana signs "timestamp:body" when a
// timestamp header is configured, otherwise just the body, hex HMAC-SHA256.
func VerifyGrafana(secret string, body []byte, timestamp, sigHeader string) bool {
	var signed []byte
	if timestamp != "" {
		signed = []byte(timestamp + ":" + string(body))
	} else {
		signed = body
	}
	expected := hex.EncodeToString(hmacSHA256(secret, signed))
	return hmacEqual(expected, strings.TrimSpace(sigHeader))
}

// VerifyPaddle checks the Paddle-Signature header "ts=...;h1=...": the
// signed payload is "timestamp:body", hex HMAC-SHA256, 5-second tolerance
// per Paddle's SDK default. lenientTolerance widens it for clock skew.
func VerifyPaddle(secret string, body []byte, header string, lenientTolerance time.Duration) bool {
	var ts int64
	var h1s []string
	for _, part := range strings.Split(header, ";") {
		p := strings.SplitN(part, "=", 2)
		if len(p) != 2 {
			continue
		}
		switch strings.TrimSpace(p[0]) {
		case "ts":
			ts, _ = strconv.ParseInt(strings.TrimSpace(p[1]), 10, 64)
		case "h1":
			h1s = append(h1s, strings.TrimSpace(p[1]))
		}
	}
	if len(h1s) == 0 {
		return false
	}
	tol := 5 * time.Second
	if lenientTolerance > tol {
		tol = lenientTolerance
	}
	age := now().Unix() - ts
	if age < 0 {
		age = -age
	}
	if age > int64(tol/time.Second) {
		return false
	}
	signed := fmt.Sprintf("%d:%s", ts, body)
	expected := hex.EncodeToString(hmacSHA256(secret, []byte(signed)))
	for _, h1 := range h1s {
		if hmacEqual(expected, h1) {
			return true
		}
	}
	return false
}

// VerifyHubSpotV3 checks X-HubSpot-Signature-V3. The signed message is
// requestMethod + requestURI + requestBody + timestamp (milliseconds,
// from X-HubSpot-Request-Timestamp), HMAC-SHA256 keyed with the app client
// secret, base64-encoded, 5-minute tolerance. The URI must match the
// original request (scheme, host, path, query).
func VerifyHubSpotV3(secret, method, uri string, body []byte, timestampMs, sigHeader string) bool {
	ts, err := strconv.ParseInt(timestampMs, 10, 64)
	if err != nil {
		return false
	}
	age := now().UnixMilli() - ts
	if age < 0 {
		age = -age
	}
	if age > 5*60*1000 {
		return false
	}
	message := method + uri + string(body) + timestampMs
	expected := base64.StdEncoding.EncodeToString(hmacSHA256(secret, []byte(message)))
	return hmacEqual(expected, sigHeader)
}

// VerifyHubSpotV1 checks the legacy X-HubSpot-Signature: plain SHA-256 hex
// of clientSecret + body. No timestamp — replay protection is weaker, which
// is why v3 is preferred.
func VerifyHubSpotV1(secret string, body []byte, sigHeader string) bool {
	sum := sha256.Sum256(append([]byte(secret), body...))
	return hmacEqual(hex.EncodeToString(sum[:]), sigHeader)
}

// VerifyBearer checks an Authorization: Bearer header in constant time.
func VerifyBearer(got, expected string) bool {
	return hmacEqual(strings.TrimSpace(strings.TrimPrefix(got, "Bearer ")), expected)
}

// VerifyBasic checks HTTP Basic credentials (Chargebee, Pipedrive,
// Datadog, Zendesk) in constant time.
func VerifyBasic(gotUser, gotPass, wantUser, wantPass string) bool {
	return hmacEqual(gotUser, wantUser) && hmacEqual(gotPass, wantPass)
}
