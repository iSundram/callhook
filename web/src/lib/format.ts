// Formatting helpers.

export function timeAgo(iso: string): string {
  const s = Math.floor((Date.now() - new Date(iso).getTime()) / 1000)
  if (s < 5) return 'just now'
  if (s < 60) return `${s}s ago`
  if (s < 3600) return `${Math.floor(s / 60)}m ago`
  if (s < 86400) return `${Math.floor(s / 3600)}h ago`
  return `${Math.floor(s / 86400)}d ago`
}

export function clock(iso: string): string {
  return new Date(iso).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
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
