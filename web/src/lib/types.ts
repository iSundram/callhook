// API types — mirrors backend JSON shapes.

export type Health = {
  ok: boolean
  dry_run: boolean
  windows_enforced: boolean
  auth_intake: boolean
  auth_webhook: boolean
}

export type Action = {
  at: string
  kind: string
  detail: string
}

export type Turn = {
  offset_seconds: number | null
  speaker: 'bot' | 'user' | 'unknown'
  text: string
}

export type Session = {
  id: string
  event_type: string
  customer_id: string
  customer: string
  phone: string
  call_id: string
  call_status: string
  outcome: Record<string, any> | null
  actions: Action[]
  task: string
  transcript: Turn[] | null
  retry_count: number
  next_retry_at: string | null
  next_retry_kind: string
  campaign_id?: string
  updated_at: string
}

export type Metrics = {
  sessions: number
  by_status: Record<string, number>
  by_outcome: Record<string, number>
  retries_armed: number
}

export type AudienceEntry = {
  customer_id: string
  phone?: string
  state: 'pending' | 'in_wave' | 'succeeded' | 'failed' | 'no_answer' | 'skipped'
  session_id?: string
  rounds: number
}

export type Campaign = {
  id: string
  name: string
  event_type: string
  goal: {
    type: 'count' | 'reach_all'
    target?: number
    success_outcomes: string[]
  }
  audience: AudienceEntry[]
  waves: { size: number; delay: string; max_waves: number }
  budget: { max_calls: number }
  status: 'running' | 'goal_met' | 'exhausted' | 'budget_exhausted' | 'stopped'
  progress: {
    successes: number
    failures: number
    no_answer: number
    calls_placed: number
    pending: number
  }
  wave_number: number
  wave_placed: number
  wave_done: number
  next_wave_at: string | null
  created_at: string
  completed_at?: string
  log: { at: string; event: string; note?: string }[]
}

export type FireEventResult = {
  status: string
  session_id?: string
  call_id?: string
  phone?: string
  error?: string
}
