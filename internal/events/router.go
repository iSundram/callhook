package events

import (
	"fmt"
	"strings"

	"github.com/iSundram/callhook/internal/business"
)

// Blueprint describes how one event type becomes a phone call: the task
// prompt template (fed with prefetched customer data), the structured result
// schema CALL-E must extract, and the outcome mapping rules.
type Blueprint struct {
	// Compose builds the natural-language task for CALL-E's voice agent,
	// with all live business data already baked in.
	Compose func(c *business.Customer, e *Event, s business.Store) (string, error)
	// ResultSchema is the JSON Schema CALL-E extracts after the call.
	ResultSchema map[string]any
	// GoalID optionally pins this event type to a published CALL-E Goal
	// (versioned workflow). When set, the goal path is used instead of the
	// free-text task. See README "Goals".
	GoalID string
	// VariablesFor maps an event + customer to goal input variables.
	Variables func(c *business.Customer, e *Event) map[string]any
}

// VariablesFor returns the goal input variables, or an empty map.
func (b Blueprint) VariablesFor(c *business.Customer, e *Event) map[string]any {
	if b.Variables == nil {
		return map[string]any{}
	}
	return b.Variables(c, e)
}

// Router maps event types to blueprints. New event types are added here —
// the intake API, session handling, and outcome engine are generic.
type Router struct {
	blueprints map[string]Blueprint
}

func NewRouter() *Router {
	return &Router{blueprints: map[string]Blueprint{
		"invoice.due":    invoiceDue(),
		"account.warning": accountWarning(),
		"promo.offer":    promoOffer(),
	}}
}

func (r *Router) Blueprint(eventType string) (Blueprint, bool) {
	b, ok := r.blueprints[eventType]
	return b, ok
}

func (r *Router) Supported() []string {
	return []string{"invoice.due", "account.warning", "promo.offer"}
}

// ---- invoice.due ----

func invoiceDue() Blueprint {
	return Blueprint{
		Compose: func(c *business.Customer, e *Event, s business.Store) (string, error) {
			inv, err := s.GetOpenInvoice(c.ID)
			if err != nil {
				return "", fmt.Errorf("prefetch invoice: %w", err)
			}
			amount := fmt.Sprintf("%s %.2f", inv.Currency, float64(inv.AmountCents)/100)
			lastPay := "no payment on record"
			if inv.LastPayment != nil {
				lastPay = inv.LastPayment.Format("Jan 2")
			}
			return fmt.Sprintf(`Call %s about their overdue invoice. Be polite and professional; you are calling on behalf of their service provider.

Context you already know (do not ask the customer for any of this):
- Customer: %s (plan: %s)
- Invoice %s for %s was due %s (%d days overdue)
- Last payment received: %s

Goal: find out when they will pay, or resolve the situation.
- If they say they already paid, acknowledge it and tell them you will flag the account for a billing review — do not argue.
- If they commit to a date, confirm the date back to them.
- If they dispute the charge or ask for a person, offer a callback from the billing team.
- If they ask how to pay, tell them the payment link was re-sent to their email (%s).`,
				c.Name, c.Name, c.Plan, inv.ID, amount, inv.DueDate.Format("Jan 2"), inv.DaysOverdue, lastPay, c.Email), nil
		},
		ResultSchema: map[string]any{
			"type": "object",
			"required": []string{"outcome", "summary"},
			"properties": map[string]any{
				"outcome": map[string]any{
					"type":        "string",
					"enum":        []string{"payment_promised", "claims_already_paid", "disputed", "callback_requested", "no_answer", "refused", "unknown"},
					"description": "payment_promised if a payment date was agreed; claims_already_paid if they say they already paid; disputed if they contest the charge; callback_requested if they want a human; no_answer if the call did not reach them; refused if they decline to pay without a date.",
				},
				"promise_date": map[string]any{
					"type":        "string",
					"description": "The payment date the customer committed to, in YYYY-MM-DD format. Empty string if no date was given.",
				},
				"summary": map[string]any{
					"type":        "string",
					"description": "One-sentence summary of how the call went.",
				},
			},
			"additionalProperties": false,
		},
	}
}

// ---- account.warning ----

func accountWarning() Blueprint {
	type payload struct {
		Reason  string `json:"reason"`
		Detail  string `json:"detail"`
	}
	var p payload
	return Blueprint{
		Compose: func(c *business.Customer, e *Event, s business.Store) (string, error) {
			_ = e.DecodePayload(&p)
			if p.Reason == "" {
				p.Reason = "unusual activity detected on the account"
			}
			return fmt.Sprintf(`Call %s with an urgent but calm security notice on behalf of their service provider. Do not request any password, PIN, or code during the call.

Context:
- Customer: %s (plan: %s)
- Warning: %s. %s

Goal: notify them, confirm they are aware, and offer the next step.
- If they confirm the activity was theirs, reassure them no action is needed.
- If they do not recognize the activity, advise them their session has been flagged for review and a secure reset link was emailed to %s. Offer a callback from the security team if they are worried.
- Never ask them to read out credentials or one-time codes.`,
				c.Name, c.Name, c.Plan, p.Reason, p.Detail, c.Email), nil
		},
		ResultSchema: map[string]any{
			"type": "object",
			"required": []string{"outcome", "summary"},
			"properties": map[string]any{
				"outcome": map[string]any{
					"type":        "string",
					"enum":        []string{"acknowledged", "activity_confirmed_legitimate", "needs_human", "no_answer", "unknown"},
					"description": "acknowledged if they received the notice; activity_confirmed_legitimate if the activity was theirs; needs_human if they are worried and want the security team to call back; no_answer if unreachable.",
				},
				"summary": map[string]any{
					"type":        "string",
					"description": "One-sentence summary of how the call went.",
				},
			},
			"additionalProperties": false,
		},
	}
}

// ---- promo.offer ----

func promoOffer() Blueprint {
	type payload struct {
		Offer   string `json:"offer"`
		Expires string `json:"expires"`
	}
	var p payload
	return Blueprint{
		Compose: func(c *business.Customer, e *Event, s business.Store) (string, error) {
			_ = e.DecodePayload(&p)
			if p.Offer == "" {
				p.Offer = "20% off the next invoice"
			}
			if p.Expires == "" {
				p.Expires = "end of this week"
			}
			return fmt.Sprintf(`Call %s with a short, friendly offer on behalf of their service provider. Keep it brief — under one minute if they are not interested.

Context:
- Customer: %s, on the %s plan since %s — a loyal customer, thank them.
- Offer: %s, valid until %s.

Goal: present the offer once, answer one or two questions, and record their interest.
- If they are interested, confirm the offer was applied and they will get an email confirmation at %s.
- If they decline, thank them warmly and end the call. Never push a second time.`,
				c.Name, c.Name, c.Plan, c.JoinedAt.Format("Jan 2006"), p.Offer, p.Expires, c.Email), nil
		},
		ResultSchema: map[string]any{
			"type": "object",
			"required": []string{"outcome", "summary"},
			"properties": map[string]any{
				"outcome": map[string]any{
					"type":        "string",
					"enum":        []string{"accepted", "declined", "callback_requested", "no_answer", "unknown"},
					"description": "accepted if they took the offer; declined if they said no; callback_requested if they want to decide later with a human; no_answer if unreachable.",
				},
				"summary": map[string]any{
					"type":        "string",
					"description": "One-sentence summary of how the call went.",
				},
			},
			"additionalProperties": false,
		},
	}
}

// DescribeEventTypes renders supported events for docs/dashboards.
func (r *Router) DescribeEventTypes() string {
	var b strings.Builder
	for _, t := range r.Supported() {
		fmt.Fprintf(&b, "- %s\n", t)
	}
	return b.String()
}
