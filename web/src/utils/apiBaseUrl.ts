export interface ApiEndpoint {
  label: string
  path: string
  tagType: '' | 'success' | 'warning' | 'info' | 'danger'
  hint?: string
}

export const API_ENDPOINTS: ApiEndpoint[] = [
  {
    label: 'OpenAI Chat',
    path: '/v1/chat/completions',
    tagType: 'success',
    hint: 'OpenAI SDK、Cursor、Continue 等'
  },
  {
    label: 'OpenAI Models',
    path: '/v1/models',
    tagType: 'success',
    hint: '模型列表'
  },
  {
    label: 'Anthropic',
    path: '/v1/messages',
    tagType: 'warning',
    hint: 'Claude / Anthropic SDK'
  },
  {
    label: 'Completions',
    path: '/v1/completions',
    tagType: '',
    hint: '旧版 Completions API'
  }
]

export interface StatusLike {
  external_url?: string
  host?: string
  port?: number
}

/** Resolve HTTP base URL for client apps. */
export function resolveApiBaseUrl(status: StatusLike | null | undefined): string {
  const ext = status?.external_url?.trim()
  if (ext) {
    return /^https?:\/\//.test(ext) ? ext.replace(/\/$/, '') : `http://${ext}`
  }

  const host = status?.host || '127.0.0.1'
  const port = status?.port ?? 20060
  const hostname = host === '0.0.0.0' || host === '::' ? '127.0.0.1' : host

  const origin = window.location.origin
  if (/^https?:\/\//.test(origin)) {
    return origin.replace(/\/$/, '')
  }

  return `http://${hostname}:${port}`
}

export function buildEndpointUrl(baseUrl: string, path: string): string {
  return `${baseUrl.replace(/\/$/, '')}${path}`
}
