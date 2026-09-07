/**
 * Airtable Automation → callhook recipe.
 *
 * Setup:
 *   1. Create an Airtable Automation: trigger "When record matches
 *      condition" (e.g. Status = "Needs Call").
 *   2. Action: "Run a script" — paste this file.
 *   3. Add input variables: customer_id, phone, event_type, amount, due.
 *      (Map them from the record's fields.)
 *
 * Airtable scripts run in a sandbox with fetch() available (15s limit);
 * callhook answers 2xx immediately, so the script finishes fast.
 *
 * Docs: https://support.airtable.com/docs/automations-overview
 */

// Input variables configured on the "Run a script" action:
const config = input.config()

const BASE_URL = 'https://your-callhook.example.com' // your server
const TOKEN = '' // CALLHOOK_INTAKE_TOKEN if set — better: an Airtable secret/env var

const body = {
  id: `airtable_${config.customer_id}_${Date.now()}`,
  type: config.event_type || 'invoice.due',
  customer_id: config.customer_id,
  ...(config.phone && { phone: config.phone }),
  payload: {
    ...(config.amount && { amount: config.amount }),
    ...(config.due && { due_date: config.due }),
  },
}

const response = await fetch(`${BASE_URL}/api/events`, {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
    ...(TOKEN && { Authorization: `Bearer ${TOKEN}` }),
  },
  body: JSON.stringify(body),
})

const result = await response.json()
output.set('status', result.status || 'error')
output.set('session_id', result.session_id || '')
console.log('callhook:', JSON.stringify(result))
