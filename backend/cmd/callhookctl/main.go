// callhookctl — command line client for a running callhook server.
//
//	callhookctl fire invoice.due cus_1002 [--phone +1...] [--not-before 2026-09-07T10:00:00Z]
//	callhookctl batch events.json
//	callhookctl sessions [--watch]
//	callhookctl metrics
package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

func main() {
	base := envOr("CALLHOOK_URL", "http://localhost:8080")
	token := os.Getenv("CALLHOOK_INTAKE_TOKEN")

	if len(os.Args) < 2 {
		usage()
	}
	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "fire":
		fire(base, token, args)
	case "batch":
		batch(base, token, args)
	case "sessions":
		sessions(base, args)
	case "metrics":
		metrics(base)
	case "test-webhook":
		testWebhook(base, args)
	default:
		usage()
	}
}

func fire(base, token string, args []string) {
	fs := flag.NewFlagSet("fire", flag.ExitOnError)
	phone := fs.String("phone", "", "E.164 phone override")
	notBefore := fs.String("not-before", "", "RFC3339 earliest call time")
	payload := fs.String("payload", "{}", "event payload JSON")
	_ = fs.Parse(args)
	if fs.NArg() < 2 {
		fmt.Fprintln(os.Stderr, "usage: callhookctl fire <type> <customer_id> [flags]")
		os.Exit(1)
	}
	ev := map[string]any{
		"id":          fmt.Sprintf("evt_cli_%d", time.Now().UnixMilli()),
		"type":        fs.Arg(0),
		"customer_id": fs.Arg(1),
		"payload":     json.RawMessage(*payload),
	}
	if *phone != "" {
		ev["phone"] = *phone
	}
	if *notBefore != "" {
		ev["not_before"] = *notBefore
	}
	body, _ := json.Marshal(ev)
	out := post(base+"/api/events", token, body)
	printJSON(out)
}

func batch(base, token string, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: callhookctl batch events.json")
		os.Exit(1)
	}
	raw, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var events []map[string]any
	if err := json.Unmarshal(raw, &events); err != nil {
		// also accept {"events": [...]}
		var wrapper struct{ Events []map[string]any }
		if err2 := json.Unmarshal(raw, &wrapper); err2 != nil {
			fmt.Fprintln(os.Stderr, "invalid batch file:", err)
			os.Exit(1)
		}
		events = wrapper.Events
	}
	body, _ := json.Marshal(map[string]any{"events": events})
	out := post(base+"/api/events/batch", token, body)
	printJSON(out)
}

func sessions(base string, args []string) {
	watch := len(args) > 0 && args[0] == "--watch"
	for {
		out := get(base + "/api/sessions")
		var views []map[string]any
		_ = json.Unmarshal(out, &views)
		fmt.Print("\033[H\033[2J") // clear screen
		for _, v := range views {
			name, _ := v["customer"].(string)
			typ, _ := v["event_type"].(string)
			status, _ := v["call_status"].(string)
			if status == "" {
				status = "intake"
			}
			kind, _ := v["next_retry_kind"].(string)
			fmt.Printf("%-16s %-18s %-10s %-12s %s\n", name, typ, status, kind, fmt.Sprint(v["id"]))
		}
		if !watch {
			return
		}
		time.Sleep(2 * time.Second)
	}
}

func metrics(base string) {
	printJSON(get(base + "/api/metrics"))
}

// testWebhook signs a fixture payload with the platform's real signature
// scheme and POSTs it to the running server's adapter endpoint — the
// quickest way to prove an integration works end-to-end.
//
//	callhookctl test-webhook stripe   # uses STRIPE_WEBHOOK_SECRET if set
func testWebhook(base string, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: callhookctl test-webhook <stripe|generic>")
		os.Exit(1)
	}
	platform := args[0]

	switch platform {
	case "stripe":
		secret := os.Getenv("STRIPE_WEBHOOK_SECRET")
		if secret == "" {
			fmt.Fprintln(os.Stderr, "set STRIPE_WEBHOOK_SECRET first")
			os.Exit(1)
		}
		body := []byte(`{"id":"evt_cli_test","type":"invoice.payment_failed","data":{"object":{"customer":"cus_1002","amount_due":4900,"currency":"usd","attempt_count":1}}}`)
		ts := time.Now().Unix()
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte(fmt.Sprintf("%d.", ts)))
		mac.Write(body)
		sig := fmt.Sprintf("t=%d,v1=%s", ts, hex.EncodeToString(mac.Sum(nil)))

		req, _ := http.NewRequest(http.MethodPost, base+"/integrations/stripe/webhook", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Stripe-Signature", sig)
		fmt.Println(string(doReq(req)))

	case "generic":
		body := []byte(`{"id":"evt_cli_test","type":"invoice.due","customer_id":"cus_1002"}`)
		req, _ := http.NewRequest(http.MethodPost, base+"/integrations/generic/webhook", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if tok := os.Getenv("GENERIC_BEARER_TOKEN"); tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}
		fmt.Println(string(doReq(req)))

	default:
		fmt.Fprintf(os.Stderr, "no fixture for platform %q yet — try: stripe, generic\n", platform)
		os.Exit(1)
	}
}

func post(url, token string, body []byte) []byte {
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return doReq(req)
}

func get(url string) []byte {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	return doReq(req)
}

func doReq(req *http.Request) []byte {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		fmt.Fprintf(os.Stderr, "HTTP %s: %s\n", resp.Status, out)
		os.Exit(1)
	}
	return out
}

func printJSON(b []byte) {
	var pretty any
	if err := json.Unmarshal(b, &pretty); err == nil {
		out, _ := json.MarshalIndent(pretty, "", "  ")
		fmt.Println(string(out))
		return
	}
	fmt.Println(string(b))
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func usage() {
	fmt.Fprintln(os.Stderr, `callhookctl — command line client for callhook

usage:
  callhookctl fire <type> <customer_id> [--phone +E164] [--not-before RFC3339] [--payload JSON]
  callhookctl batch <events.json>
  callhookctl sessions [--watch]
  callhookctl metrics
  callhookctl test-webhook <stripe|generic>

env:
  CALLHOOK_URL          server base URL (default http://localhost:8080)
  CALLHOOK_INTAKE_TOKEN bearer token when the server has auth enabled`)
	os.Exit(1)
}
