// Contextual docs references — every user state maps to the exact doc.
// The docsBase must match the deployed docs site (or CALLHOOK_DOCS_URL
// on the server for self-hosted docs; the API surfaces it in error bodies).

export const DOCS_BASE = 'https://callhook.github.io'

export const doc = (path: string) => `${DOCS_BASE}${path}`

// The full doc map: one entry per "stuck moment" in the product.
export const DOCS = {
  quickstart: doc('/pages/quickstart.html'),
  install: doc('/pages/install.html'),
  api: doc('/pages/api.html'),
  apiEvents: doc('/pages/api.html#post-events'),
  events: doc('/pages/events.html'),
  campaigns: doc('/pages/campaigns.html'),
  campaignsCreate: doc('/pages/campaigns.html#create'),
  integrations: doc('/pages/integrations.html'),
  nativeAdapters: doc('/pages/integrations.html#native-adapters'),
  production: doc('/pages/production.html'),
  apiKey: doc('/pages/production.html#api-key'),
  auth: doc('/pages/production.html#auth'),
  architecture: doc('/pages/architecture.html'),
  storeAdapters: doc('/pages/architecture.html#adapters'),
  troubleshooting: doc('/pages/troubleshooting.html'),
  tsConnect: doc('/pages/troubleshooting.html#connect'),
  tsWebhook401: doc('/pages/troubleshooting.html#webhook-401'),
  tsEvents: doc('/pages/troubleshooting.html#events'),
  tsNoCalls: doc('/pages/troubleshooting.html#no-calls'),
  tsCampaigns: doc('/pages/troubleshooting.html#campaigns'),
  tsInstall: doc('/pages/troubleshooting.html#install'),
} as const

// Map a connect/fire error message to the doc that fixes it.
export function docForError(message: string): string | null {
  const m = message.toLowerCase()
  if (m.includes('bearer') || m.includes('token')) return DOCS.tsConnect
  if (m.includes('could not reach') || m.includes('fetch') || m.includes('network')) return DOCS.tsConnect
  if (m.includes('unsupported event type')) return DOCS.events
  if (m.includes('not found')) return DOCS.storeAdapters
  if (m.includes('e.164') || m.includes('phone')) return DOCS.apiEvents
  if (m.includes('not_before') || m.includes('rfc3339')) return DOCS.apiEvents
  if (m.includes('signature') || m.includes('401')) return DOCS.tsWebhook401
  if (m.includes('audience') || m.includes('goal') || m.includes('campaign')) return DOCS.tsCampaigns
  if (m.includes('blueprint') || m.includes('prefetch')) return DOCS.tsEvents
  return DOCS.troubleshooting
}

// Server error bodies may carry their own docs pointer — prefer it.
export function docFromBody(body: any): string | null {
  if (body?.docs) {
    const d = String(body.docs)
    return d.startsWith('http') ? d : DOCS_BASE + d
  }
  return null
}
