package events

import "github.com/iSundram/callhook/internal/business"

// Additional blueprints: delivery, appointments, payments, subscriptions,
// feedback. Each follows the same contract as the core three: compose a
// task with prefetched context and behavioral rules, define the structured
// outcome schema CALL-E extracts.

// ---- delivery.window ----

func deliveryWindow() Blueprint {
	type payload struct {
		Window     string `json:"window"`
		OrderRef   string `json:"order_ref"`
	}
	var p payload
	return Blueprint{
		Compose: func(c *business.Customer, e *Event, s business.Store) (string, error) {
			_ = e.DecodePayload(&p)
			if p.Window == "" {
				p.Window = "tomorrow, 2:00–4:00 PM"
			}
			if p.OrderRef == "" {
				p.OrderRef = "your recent order"
			}
			return "Call " + c.Name + " to confirm a delivery window on behalf of their delivery service. Be brief and friendly.\n\n" +
				"Context:\n- Customer: " + c.Name + "\n- Delivery: " + p.OrderRef + ", proposed window " + p.Window + "\n\n" +
				"Goal: confirm they can accept the window, or collect a preferred alternative.\n" +
				"- If they confirm, repeat the window back once and thank them.\n" +
				"- If they want a different time, ask for one alternative and confirm it back.\n" +
				"- If nobody answers, do not leave detailed order information in a voicemail; just leave a callback request.", nil
		},
		ResultSchema: map[string]any{
			"type": "object",
			"required": []string{"delivery_outcome", "summary"},
			"properties": map[string]any{
				"delivery_outcome": map[string]any{
					"type":        "string",
					"enum":        []string{"confirmed", "reschedule_requested", "callback_requested", "no_answer", "unknown"},
					"description": "confirmed if they accept the proposed window; reschedule_requested if they give an alternative; no_answer if unreachable.",
				},
				"preferred_window": map[string]any{
					"type":        "string",
					"description": "Alternative window the customer requested, if any. Empty string otherwise.",
				},
				"summary": map[string]any{"type": "string", "description": "One-sentence summary of the call."},
			},
			"additionalProperties": false,
		},
	}
}

// ---- appointment.reminder ----

func appointmentReminder() Blueprint {
	type payload struct {
		What  string `json:"what"`
		When  string `json:"when"`
	}
	var p payload
	return Blueprint{
		Compose: func(c *business.Customer, e *Event, s business.Store) (string, error) {
			_ = e.DecodePayload(&p)
			if p.What == "" {
				p.What = "your appointment"
			}
			if p.When == "" {
				p.When = "tomorrow at 10:00 AM"
			}
			return "Call " + c.Name + " with an appointment reminder on behalf of their service provider. Warm, brief, no pressure.\n\n" +
				"Context:\n- Customer: " + c.Name + "\n- Appointment: " + p.What + ", " + p.When + "\n\n" +
				"Goal: confirm they will attend.\n" +
				"- If they confirm, thank them and end the call.\n" +
				"- If they need to reschedule, offer to have the office call them back with available times — do not invent times yourself.\n" +
				"- If they cancel, accept it graciously.", nil
		},
		ResultSchema: map[string]any{
			"type": "object",
			"required": []string{"appointment_outcome", "summary"},
			"properties": map[string]any{
				"appointment_outcome": map[string]any{
					"type":        "string",
					"enum":        []string{"confirmed", "reschedule_requested", "cancelled", "no_answer", "unknown"},
					"description": "confirmed if they will attend; reschedule_requested if they want a new time; cancelled if they cancel; no_answer if unreachable.",
				},
				"summary": map[string]any{"type": "string", "description": "One-sentence summary of the call."},
			},
			"additionalProperties": false,
		},
	}
}

// ---- payment.failed ----

func paymentFailed() Blueprint {
	type payload struct {
		Amount string `json:"amount"`
		Reason string `json:"reason"`
	}
	var p payload
	return Blueprint{
		Compose: func(c *business.Customer, e *Event, s business.Store) (string, error) {
			_ = e.DecodePayload(&p)
			if p.Amount == "" {
				p.Amount = "the monthly charge"
			}
			if p.Reason == "" {
				p.Reason = "the card was declined"
			}
			return "Call " + c.Name + " about a failed payment, on behalf of their service provider. Matter-of-fact and helpful — never accusatory.\n\n" +
				"Context:\n- Customer: " + c.Name + " (plan: " + c.Plan + ")\n- Payment: " + p.Amount + " failed — " + p.Reason + "\n\n" +
				"Goal: inform them and restore service.\n" +
				"- Tell them the payment didn't go through and that an updated payment link was emailed to " + c.Email + ".\n" +
				"- Never ask for card numbers, CVV, or any payment details on this call.\n" +
				"- If they say they already updated the card, acknowledge it and say the account will be checked automatically.\n" +
				"- If they are frustrated, apologize once and offer a callback from billing.", nil
		},
		ResultSchema: map[string]any{
			"type": "object",
			"required": []string{"payment_outcome", "summary"},
			"properties": map[string]any{
				"payment_outcome": map[string]any{
					"type":        "string",
					"enum":        []string{"will_update_payment", "already_updated", "callback_requested", "no_answer", "unknown"},
					"description": "will_update_payment if they will use the emailed link; already_updated if they say it's fixed; no_answer if unreachable.",
				},
				"summary": map[string]any{"type": "string", "description": "One-sentence summary of the call."},
			},
			"additionalProperties": false,
		},
	}
}

// ---- subscription.expiring ----

func subscriptionExpiring() Blueprint {
	type payload struct {
		When  string `json:"when"`
		Offer string `json:"offer"`
	}
	var p payload
	return Blueprint{
		Compose: func(c *business.Customer, e *Event, s business.Store) (string, error) {
			_ = e.DecodePayload(&p)
			if p.When == "" {
				p.When = "in 7 days"
			}
			if p.Offer == "" {
				p.Offer = "a 10% loyalty discount on renewal"
			}
			return "Call " + c.Name + " about their subscription expiring, on behalf of their service provider. Friendly, one clear offer, no pressure.\n\n" +
				"Context:\n- Customer: " + c.Name + ", on the " + c.Plan + " plan since " + c.JoinedAt.Format("Jan 2006") + "\n- Subscription expires " + p.When + "\n- Renewal offer: " + p.Offer + "\n\n" +
				"Goal: find out if they want to renew.\n" +
				"- If they accept, confirm the renewal and that details were emailed to " + c.Email + ".\n" +
				"- If they hesitate, mention the offer once. If they still decline, thank them warmly — never push twice.\n" +
				"- If they ask about cancellation, offer a callback from the team.", nil
		},
		ResultSchema: map[string]any{
			"type": "object",
			"required": []string{"renewal_outcome", "summary"},
			"properties": map[string]any{
				"renewal_outcome": map[string]any{
					"type":        "string",
					"enum":        []string{"renewed", "declined", "callback_requested", "no_answer", "unknown"},
					"description": "renewed if they accept the renewal; declined if they refuse; no_answer if unreachable.",
				},
				"summary": map[string]any{"type": "string", "description": "One-sentence summary of the call."},
			},
			"additionalProperties": false,
		},
	}
}

// ---- feedback.request ----

func feedbackRequest() Blueprint {
	type payload struct {
		Topic string `json:"topic"`
	}
	var p payload
	return Blueprint{
		Compose: func(c *business.Customer, e *Event, s business.Store) (string, error) {
			_ = e.DecodePayload(&p)
			if p.Topic == "" {
				p.Topic = "their recent experience with the service"
			}
			return "Call " + c.Name + " for a short feedback request on behalf of their service provider. Respect their time — under two minutes.\n\n" +
				"Context:\n- Customer: " + c.Name + " (plan: " + c.Plan + ")\n- Topic: " + p.Topic + "\n\n" +
				"Goal: collect one rating and one open comment.\n" +
				"- Ask how satisfied they are on a scale of 1 to 5.\n" +
				"- Ask one open question about what could be better.\n" +
				"- If they are unhappy, apologize sincerely and offer a callback from the team.\n" +
				"- If they are busy, offer to call back later — do not push.", nil
		},
		ResultSchema: map[string]any{
			"type": "object",
			"required": []string{"feedback_outcome", "summary"},
			"properties": map[string]any{
				"feedback_outcome": map[string]any{
					"type":        "string",
					"enum":        []string{"provided", "busy_callback_requested", "no_answer", "unknown"},
					"description": "provided if they gave feedback; busy_callback_requested if they asked to call back; no_answer if unreachable.",
				},
				"rating": map[string]any{
					"type":        "integer",
					"description": "Satisfaction rating 1-5, if given. Omit if not given.",
				},
				"comment": map[string]any{
					"type":        "string",
					"description": "One-sentence summary of their open feedback, if given.",
				},
				"summary": map[string]any{"type": "string", "description": "One-sentence summary of the call."},
			},
			"additionalProperties": false,
		},
	}
}
