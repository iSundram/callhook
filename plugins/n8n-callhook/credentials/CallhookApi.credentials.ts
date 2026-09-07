import type { ICredentialType, INodeProperties } from 'n8n-workflow'

export class CallhookApi implements ICredentialType {
  name = 'callhookApi'
  displayName = 'Callhook API'
  documentationUrl = 'https://callhook.github.io'
  properties: INodeProperties[] = [
    {
      displayName: 'Base URL',
      name: 'baseUrl',
      type: 'string',
      default: 'http://localhost:8080',
      description: 'Where your callhook server listens',
    },
    {
      displayName: 'Bearer Token',
      name: 'token',
      type: 'string',
      typeOptions: { password: true },
      default: '',
      description: 'CALLHOOK_INTAKE_TOKEN if intake auth is enabled',
    },
  ]
}
