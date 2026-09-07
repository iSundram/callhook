import type {
  IExecuteFunctions,
  INodeType,
  INodeTypeDescription,
  INodeExecutionData,
} from 'n8n-workflow'

// The 8 callhook event types — mirrors backend/internal/events/router.go.
const EVENT_TYPES = [
  { name: 'Invoice Due', value: 'invoice.due' },
  { name: 'Account Warning', value: 'account.warning' },
  { name: 'Promo Offer', value: 'promo.offer' },
  { name: 'Delivery Window', value: 'delivery.window' },
  { name: 'Appointment Reminder', value: 'appointment.reminder' },
  { name: 'Payment Failed', value: 'payment.failed' },
  { name: 'Subscription Expiring', value: 'subscription.expiring' },
  { name: 'Feedback Request', value: 'feedback.request' },
] as const

export class Callhook implements INodeType {
  description: INodeTypeDescription = {
    displayName: 'Callhook',
    name: 'callhook',
    icon: 'file:callhook.svg',
    group: ['output'],
    version: 1,
    subtitle: '={{$parameter["eventType"]}}',
    description: 'Fire an AI phone call via callhook',
    defaults: { name: 'Callhook' },
    inputs: ['main'],
    outputs: ['main'],
    credentials: [{ name: 'callhookApi', required: true }],
    properties: [
      {
        displayName: 'Event Type',
        name: 'eventType',
        type: 'options',
        options: EVENT_TYPES.map((t) => ({ name: t.name, value: t.value })),
        default: 'invoice.due',
        required: true,
      },
      {
        displayName: 'Customer ID',
        name: 'customerId',
        type: 'string',
        default: '',
        required: true,
        description: 'Resolves the customer (and phone) in the callhook business store',
      },
      {
        displayName: 'Phone (E.164)',
        name: 'phone',
        type: 'string',
        default: '',
        description: 'Optional override; defaults to the customer record',
      },
      {
        displayName: 'Not Before',
        name: 'notBefore',
        type: 'string',
        default: '',
        description: 'Optional RFC3339 — schedule the call instead of firing now',
      },
      {
        displayName: 'Callback URL',
        name: 'callbackUrl',
        type: 'string',
        default: '',
        description: 'Where the structured outcome is POSTed',
      },
      {
        displayName: 'Payload (JSON)',
        name: 'payload',
        type: 'json',
        default: '{}',
        description: 'Event-type-specific context for the voice agent',
      },
    ],
  }

  async execute(this: IExecuteFunctions): Promise<INodeExecutionData[][]> {
    const credentials = await this.getCredentials('callhookApi')
    const items = this.getInputData()

    const results: INodeExecutionData[] = []
    for (let i = 0; i < items.length; i++) {
      const eventType = this.getNodeParameter('eventType', i) as string
      const customerId = this.getNodeParameter('customerId', i) as string
      const phone = this.getNodeParameter('phone', i) as string
      const notBefore = this.getNodeParameter('notBefore', i) as string
      const callbackUrl = this.getNodeParameter('callbackUrl', i) as string
      const payload = this.getNodeParameter('payload', i, '{}') as object

      const body: Record<string, unknown> = {
        id: `n8n_${Date.now()}_${i}`,
        type: eventType,
        customer_id: customerId,
        payload,
      }
      if (phone) body.phone = phone
      if (notBefore) body.not_before = notBefore
      if (callbackUrl) body.callback_url = callbackUrl

      const response = await this.helpers.httpRequest({
        method: 'POST',
        url: `${(credentials.baseUrl as string).replace(/\/$/, '')}/api/events`,
        headers: {
          'Content-Type': 'application/json',
          ...((credentials.token as string) && {
            Authorization: `Bearer ${credentials.token}`,
          }),
        },
        body,
        json: true,
      })
      results.push({ json: response as object })
    }
    return [results]
  }
}
