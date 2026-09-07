// Shared event-type catalog — single source of truth for the whole UI.
// Mirrors backend/internal/events (3 core + 5 blueprints).

export type EventTypeDef = {
  type: string
  label: string
  desc: string
  successOutcome: string
  outcomes: string[]
  demo: Record<string, any>
}

export const EVENT_TYPES: EventTypeDef[] = [
  {
    type: 'invoice.due',
    label: 'invoice.due — overdue invoice chase',
    desc: 'The overdue-invoice chase, done politely. Agent knows amount, due date, days overdue, last payment — handles "I already paid" with grace.',
    successOutcome: 'payment_promised',
    outcomes: ['payment_promised', 'claims_already_paid', 'disputed', 'callback_requested', 'refused', 'no_answer'],
    demo: {},
  },
  {
    type: 'account.warning',
    label: 'account.warning — security notice',
    desc: 'Urgent-but-calm security notice. Never asks for passwords, PINs or codes.',
    successOutcome: 'acknowledged',
    outcomes: ['acknowledged', 'activity_confirmed_legitimate', 'needs_human', 'no_answer'],
    demo: { reason: 'login from a new country', detail: 'A sign-in from Singapore was detected.' },
  },
  {
    type: 'promo.offer',
    label: 'promo.offer — loyalty offer',
    desc: 'Loyalty offer by voice. Under a minute if they\'re not interested; never pushes twice.',
    successOutcome: 'accepted',
    outcomes: ['accepted', 'declined', 'callback_requested', 'no_answer'],
    demo: { offer: '20% off your next invoice', expires: 'end of this week' },
  },
  {
    type: 'delivery.window',
    label: 'delivery.window — confirm delivery slot',
    desc: 'Confirm a delivery window or collect a preferred alternative. No order details in voicemails.',
    successOutcome: 'confirmed',
    outcomes: ['confirmed', 'reschedule_requested', 'callback_requested', 'no_answer'],
    demo: { window: 'tomorrow, 2:00–4:00 PM', order_ref: 'ORD-8472' },
  },
  {
    type: 'appointment.reminder',
    label: 'appointment.reminder — confirm attendance',
    desc: 'Warm reminder: confirm, reschedule, or cancel. Never invents available times itself.',
    successOutcome: 'confirmed',
    outcomes: ['confirmed', 'reschedule_requested', 'cancelled', 'no_answer'],
    demo: { what: 'annual check-up', when: 'tomorrow at 10:00 AM' },
  },
  {
    type: 'payment.failed',
    label: 'payment.failed — declined card notice',
    desc: 'Inform and restore service. Payment link emailed — card details are never discussed on the call.',
    successOutcome: 'will_update_payment',
    outcomes: ['will_update_payment', 'already_updated', 'callback_requested', 'no_answer'],
    demo: { amount: 'the monthly charge', reason: 'the card was declined' },
  },
  {
    type: 'subscription.expiring',
    label: 'subscription.expiring — renewal offer',
    desc: 'One clear renewal offer with a loyalty discount. Never pushes twice.',
    successOutcome: 'renewed',
    outcomes: ['renewed', 'declined', 'callback_requested', 'no_answer'],
    demo: { when: 'in 7 days', offer: 'a 10% loyalty discount on renewal' },
  },
  {
    type: 'feedback.request',
    label: 'feedback.request — 1-5 rating + comment',
    desc: 'Short feedback call: one rating, one open comment. Under two minutes.',
    successOutcome: 'provided',
    outcomes: ['provided', 'busy_callback_requested', 'no_answer'],
    demo: { topic: 'their recent experience with the service' },
  },
]

export function eventDef(type: string): EventTypeDef {
  return EVENT_TYPES.find(e => e.type === type) || EVENT_TYPES[0]
}
