// Zapier app definition — callhook: fire AI phone calls from any Zap.
// Push with: npm install && zapier register "callhook" && zapier push

const EVENT_TYPES = [
  'invoice.due', 'account.warning', 'promo.offer', 'delivery.window',
  'appointment.reminder', 'payment.failed', 'subscription.expiring', 'feedback.request',
]

module.exports = {
  version: require('./package.json').version,

  authentication: {
    fields: [
      { key: 'base_url', label: 'Base URL', required: true, default: 'http://localhost:8080' },
      { key: 'token', label: 'Bearer Token (CALLHOOK_INTAKE_TOKEN)', required: false },
    ],
    test: {
      url: '{{bundle.authData.base_url}}/api/health',
    },
  },

  resources: {},

  triggers: {},

  creates: {
    fire_event: {
      key: 'fire_event',
      noun: 'Call Event',
      display: {
        label: 'Fire Phone Call Event',
        description: 'Place an AI phone call via callhook for any event type.',
      },
      operation: {
        inputFields: [
          {
            key: 'event_type',
            label: 'Event Type',
            type: 'string',
            required: true,
            choices: EVENT_TYPES,
            helpText: 'What kind of call the voice agent will make.',
          },
          {
            key: 'customer_id',
            label: 'Customer ID',
            type: 'string',
            required: true,
            helpText: 'Resolves the customer record (and phone) in callhook.',
          },
          {
            key: 'phone',
            label: 'Phone (E.164)',
            type: 'string',
            required: false,
            helpText: 'Optional; defaults to the customer record number.',
          },
          {
            key: 'not_before',
            label: 'Not Before (RFC3339)',
            type: 'string',
            required: false,
          },
          {
            key: 'callback_url',
            label: 'Outcome Callback URL',
            type: 'string',
            required: false,
          },
          {
            key: 'payload',
            label: 'Payload (JSON string)',
            type: 'string',
            required: false,
            helpText: 'Event-type-specific context, e.g. {"amount":"USD 49.00"}',
          },
        ],
        perform: {
          url: '{{bundle.authData.base_url}}/api/events',
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            '{{bundle.authData.token ? "Authorization" : "X-Empty"}}':
              'Bearer {{bundle.authData.token}}',
          },
          body: {
            id: 'zapier_{{zapier.execution.createdAt}}_{{bundle.inputData.customer_id}}',
            type: '{{bundle.inputData.event_type}}',
            customer_id: '{{bundle.inputData.customer_id}}',
            phone: '{{bundle.inputData.phone}}',
            not_before: '{{bundle.inputData.not_before}}',
            callback_url: '{{bundle.inputData.callback_url}}',
            payload: '{{bundle.inputData.payload}}',
          },
          removeMissingValuesFrom: { body: true },
        },
        sample: {
          status: 'call_placed',
          session_id: 'sess_123',
          call_id: 'call_456',
          phone: '+15551234567',
        },
      },
    },
  },

  searchOrCreate: {},
}
