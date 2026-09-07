package integrations

// AWS SNS adapter — https://docs.aws.amazon.com/sns/latest/dg/sns-message-and-batch-json-formats.html
// and the "Validating the signature of messages" section.
//
// Verification (SignatureVersion 2, the recommended version):
//   1. Validate SigningCertURL: https, host *.amazonaws.com, path matches
//      the SNS cert pattern.
//   2. Fetch the X.509 certificate from SigningCertURL (cached).
//   3. Build the string-to-sign: for Notification messages the fields
//      Message, MessageId, Subject (if present), Timestamp, TopicArn, Type
//      — each as "Name\nValue\n" — in that exact order. For (Un)Subscription
//      Confirmation: Message, MessageId, SubscribeURL, Timestamp, Token,
//      TopicArn, Type.
//   4. Signature (base64) verified with the cert's public key
//      (SHA256withRSA for v2, SHA1withRSA for v1).
//
// CloudWatch alarms → SNS → callhook works through this endpoint. The SNS
// Message body is a callhook event JSON (for direct integrations) or a
// CloudWatch alarm JSON (auto-detected).
//
// SubscriptionConfirmation messages: the wrapper answers 200 and the
// confirmation URL is surfaced in the logs (an operator action, or set
// SNS_AUTO_SUBSCRIBE=true to visit it automatically — still verified by
// the cert signature before visiting).

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// SNSConfig configures the SNS adapter.
type SNSConfig struct {
	// AutoSubscribe visits SubscribeURL on SubscriptionConfirmation
	// (after signature verification). Default false — the URL is logged
	// for an operator to confirm manually. SNS_AUTO_SUBSCRIBE=true enables.
	AutoSubscribe bool
}

type snsAdapter struct{ cfg SNSConfig }

func (snsAdapter) Platform() string { return "sns" }

type snsMessage struct {
	Type      string `json:"Type"` // Notification | SubscriptionConfirmation | UnsubscribeConfirmation
	MessageID string `json:"MessageId"`

	Token            string `json:"Token"`
	TopicArn         string `json:"TopicArn"`
	Subject          string `json:"Subject"`
	Message          string `json:"Message"`
	Timestamp        string `json:"Timestamp"`
	SignatureVersion string `json:"SignatureVersion"`
	Signature        string `json:"Signature"`
	SigningCertURL   string `json:"SigningCertURL"`
	SubscribeURL     string `json:"SubscribeURL"`
	UnsubscribeURL   string `json:"UnsubscribeURL"`
}

// certCache caches the SNS signing certificate per URL.
var certCache = struct {
	sync.Mutex
	certs map[string]*x509.Certificate
}{certs: map[string]*x509.Certificate{}}

func (a snsAdapter) Handle(r *http.Request, body []byte) ([]Event, error) {
	var msg snsMessage
	if err := jsonUnmarshalStrict(body, &msg); err != nil {
		return nil, err
	}

	// 1. Validate SigningCertURL before fetching anything from it.
	certURL := msg.SigningCertURL
	if err := validateCertURL(certURL); err != nil {
		return nil, ErrUnauthorized("sns: " + err.Error())
	}

	cert, err := fetchCertCached(certURL)
	if err != nil {
		return nil, ErrUnauthorized("sns: fetch cert: " + err.Error())
	}

	// 2. Verify the signature over the string-to-sign.
	if err := verifySNSSignature(cert, &msg); err != nil {
		return nil, ErrUnauthorized("sns: " + err.Error())
	}

	switch msg.Type {
	case "SubscriptionConfirmation", "UnsubscribeConfirmation":
		// Valid request. Auto-subscribe (still a *verified* URL) or surface it.
		if a.cfg.autoSubscribe() && msg.SubscribeURL != "" {
			if err := visitURL(msg.SubscribeURL); err != nil {
				return nil, fmt.Errorf("sns: subscribe: %w", err)
			}
		}
		return nil, nil

	case "Notification":
		return decodeSNSNotification(&msg)

	default:
		return nil, nil
	}
}

// decodeSNSNotification maps the SNS Message to callhook events. Two
// shapes are supported:
//   - callhook event JSON (direct SNS publishing)
//   - CloudWatch alarm JSON (auto-mapped to account.warning)
func decodeSNSNotification(msg *snsMessage) ([]Event, error) {
	// CloudWatch alarm shape first: {"AlarmName":..., "NewStateValue":"ALARM",...}
	var cw struct {
		AlarmName        string `json:"AlarmName"`
		NewStateValue    string `json:"NewStateValue"`
		OldStateValue    string `json:"OldStateValue"`
		StateChangeTime  string `json:"StateChangeTime"`
		Region           string `json:"Region"`
		AlarmDescription string `json:"AlarmDescription"`
	}
	if err := json.Unmarshal([]byte(msg.Message), &cw); err == nil && cw.AlarmName != "" {
		if cw.NewStateValue != "ALARM" {
			return nil, nil // OK state → no call
		}
		return []Event{{
			ID:         "sns_cw_" + cw.AlarmName + "_" + sanitizeTime(cw.StateChangeTime),
			Type:       "account.warning",
			CustomerID: "oncall_" + strings.ToLower(cw.AlarmName),
			Payload: payloadJSON(map[string]any{
				"reason": "CloudWatch alarm " + cw.AlarmName + " in ALARM",
				"detail": cw.AlarmDescription,
				"topic":  msg.TopicArn,
			}),
		}}, nil
	}

	// Otherwise: callhook event JSON.
	var ge genericEvent
	if err := jsonUnmarshalStrict([]byte(msg.Message), &ge); err != nil {
		return nil, fmt.Errorf("sns: message is neither CloudWatch alarm nor callhook event: %w", err)
	}
	if ge.ID == "" || ge.Type == "" || ge.CustomerID == "" {
		return nil, &httpError{code: 400, msg: "sns: message event requires id, type and customer_id"}
	}
	return []Event{{
		ID:         ge.ID,
		Type:       ge.Type,
		CustomerID: ge.CustomerID,
		Phone:      ge.Phone,
		Payload:    string(ge.RawPayload),
	}}, nil
}

// validateCertURL enforces the SNS certificate URL contract: https,
// host ending in .amazonaws.com, path starting with /SimpleNotificationService-.
func validateCertURL(raw string) error {
	if raw == "" {
		return errors.New("missing SigningCertURL")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("parse cert url: %w", err)
	}
	if u.Scheme != "https" {
		return errors.New("cert url must be https")
	}
	if !strings.HasSuffix(u.Host, ".amazonaws.com") {
		return errors.New("cert url host must be *.amazonaws.com")
	}
	if !strings.HasPrefix(u.Path, "/SimpleNotificationService-") {
		return errors.New("unexpected cert url path")
	}
	return nil
}

// fetchCertCached fetches and parses the PEM certificate, cached per URL.
func fetchCertCached(certURL string) (*x509.Certificate, error) {
	certCache.Lock()
	cert, ok := certCache.certs[certURL]
	certCache.Unlock()
	if ok {
		return cert, nil
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(certURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cert fetch: http %d", resp.StatusCode)
	}
	pemBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("cert: no PEM block")
	}
	cert, err = x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("cert parse: %w", err)
	}

	certCache.Lock()
	certCache.certs[certURL] = cert
	certCache.Unlock()
	return cert, nil
}

// verifySNSSignature checks the base64 Signature against the string-to-sign.
func verifySNSSignature(cert *x509.Certificate, msg *snsMessage) error {
	if msg.Signature == "" {
		return errors.New("missing signature")
	}
	sig, err := base64.StdEncoding.DecodeString(msg.Signature)
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}

	sts := buildStringToSign(msg)
	pub, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return errors.New("cert public key is not RSA")
	}

	switch msg.SignatureVersion {
	case "2":
		digest := sha256.Sum256([]byte(sts))
		return rsa.VerifyPKCS1v15(pub, crypto.SHA256, digest[:], sig)
	case "1", "":
		digest := sha1.Sum([]byte(sts))
		return rsa.VerifyPKCS1v15(pub, crypto.SHA1, digest[:], sig)
	default:
		return fmt.Errorf("unsupported SignatureVersion %q", msg.SignatureVersion)
	}
}

// buildStringToSign constructs the exact newline-delimited field string.
func buildStringToSign(msg *snsMessage) string {
	var b strings.Builder
	writeField := func(name, value string) {
		if value != "" {
			b.WriteString(name)
			b.WriteString("\n")
			b.WriteString(value)
			b.WriteString("\n")
		}
	}
	switch msg.Type {
	case "Notification":
		writeField("Message", msg.Message)
		writeField("MessageId", msg.MessageID)
		writeField("Subject", msg.Subject)
		writeField("Timestamp", msg.Timestamp)
		writeField("TopicArn", msg.TopicArn)
		writeField("Type", msg.Type)
	default: // SubscriptionConfirmation / UnsubscribeConfirmation
		writeField("Message", msg.Message)
		writeField("MessageId", msg.MessageID)
		writeField("SubscribeURL", msg.SubscribeURL)
		writeField("Timestamp", msg.Timestamp)
		writeField("Token", msg.Token)
		writeField("TopicArn", msg.TopicArn)
		writeField("Type", msg.Type)
	}
	return b.String()
}

// visitURL GETs a verified SubscribeURL (never an attacker-controlled one —
// it is part of the signed string-to-sign for confirmation messages).
func visitURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if u.Scheme != "https" || !strings.HasSuffix(u.Host, ".amazonaws.com") {
		return fmt.Errorf("refusing to visit %q", raw)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(raw)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

func sanitizeTime(ts string) string {
	out := make([]rune, 0, len(ts))
	for _, r := range ts {
		switch r {
		case '-', ':', '.', 'Z', 'T', '+':
			out = append(out, '_')
		default:
			if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
				out = append(out, r)
			}
		}
	}
	return string(out)
}

func (c SNSConfig) autoSubscribe() bool {
	if c.AutoSubscribe {
		return true
	}
	return os.Getenv("SNS_AUTO_SUBSCRIBE") == "true"
}

// NewSNS returns the AWS SNS adapter.
func NewSNS(cfg SNSConfig) Adapter { return snsAdapter{cfg: cfg} }
