// Formatting helpers.

export function timeAgo(iso: string | null | undefined): string {
  if (!iso) return '—'
  const t = new Date(iso).getTime()
  if (isNaN(t)) return '—'
  const s = Math.floor((Date.now() - t) / 1000)
  if (s < 5) return 'just now'
  if (s < 60) return `${s}s ago`
  if (s < 3600) return `${Math.floor(s / 60)}m ago`
  if (s < 86400) return `${Math.floor(s / 3600)}h ago`
  return `${Math.floor(s / 86400)}d ago`
}

export function clock(iso: string | null | undefined): string {
  if (!iso) return '—'
  const d = new Date(iso)
  if (isNaN(d.getTime())) return '—'
  return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

export const OUTCOME_TONE: Record<string, string> = {
  payment_promised: 'ok',
  accepted: 'ok',
  acknowledged: 'ok',
  activity_confirmed_legitimate: 'ok',
  no_answer: 'muted',
  declined: 'warn',
  refused: 'warn',
  disputed: 'err',
  claims_already_paid: 'warn',
  callback_requested: 'muted',
  needs_human: 'warn',
}

// maskPhone hides the middle digits of an E.164 number in UI/audit output:
// +919229373153 → +9192 ••• 3153. Privacy per the awesome-repo review.
export function maskPhone(phone: string | undefined | null): string {
  if (!phone) return '—'
  if (phone.length <= 7) return phone
  return phone.slice(0, 5) + ' ••• ' + phone.slice(-4)
}
