/**
 * Google Forms → callhook recipe (Apps Script).
 *
 * Setup:
 *   1. Open your form → ⋮ menu → Script editor.
 *   2. Paste this file, fill in BASE_URL and TOKEN.
 *   3. Set up a trigger: onFormSubmit (Apps Script triggers → From form →
 *      "On form submit").
 *   4. Advanced: make sure only respondents you want to call can submit,
 *      and gate on the phone/customer fields.
 *
 * URL Fetch is synchronous in Apps Script; callhook replies 2xx fast.
 */

const BASE_URL = 'http://localhost:8080' // your callhook server
const TOKEN = '' // CALLHOOK_INTAKE_TOKEN if set (Script Properties recommended)

function onFormSubmit(e) {
  // e.response.getItemResponses() → [ { getItem().getTitle(), getResponse() } ]
  const byTitle = {}
  for (const ir of e.response.getItemResponses()) {
    byTitle[ir.getItem().getTitle().toLowerCase()] = ir.getResponse()
  }

  // Adjust these to your form's question titles:
  const customerId = byTitle['customer id'] || byTitle['email']
  const phone = byTitle['phone']
  const eventType = byTitle['event type'] || 'feedback.request'

  if (!customerId) {
    console.log('no customer id in response — skipping')
    return
  }

  const payload = {
    url: BASE_URL + '/api/events',
    method: 'post',
    contentType: 'application/json',
    headers: TOKEN ? { Authorization: 'Bearer ' + TOKEN } : {},
    payload: JSON.stringify({
      id: 'gform_' + e.response.getId(),
      type: eventType,
      customer_id: customerId,
      ...(phone && { phone: normalizePhone(phone) }),
      payload: {
        topic: 'their recent form submission',
      },
    }),
    muteHttpExceptions: true,
  }

  const res = UrlFetchApp.fetch(payload)
  console.log('callhook responded ' + res.getResponseCode() + ': ' + res.getContentText())
}

// "+1 (555) 123-4567" → "+15551234567"
function normalizePhone(raw) {
  const digits = String(raw).replace(/[^\d+]/g, '')
  return digits.startsWith('+') ? digits : '+' + digits
}
