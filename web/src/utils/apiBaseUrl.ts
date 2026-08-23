export interface ApiEndpoint {
  /** 协议稳定标识（选择器 value / 列表 key，同一 Base 可对应多个协议） */
  id: string
  label: string
  /**
   * 客户端 Base URL 路径后缀（通行做法：拷贝到 Agent 工具的是 Base，
   * SDK 自动拼接完整端点，如 /chat/completions、/v1/messages）。
   */
  path: string
  tagType: '' | 'success' | 'warning' | 'info' | 'danger'
  hint?: string
}

export const API_ENDPOINTS: ApiEndpoint[] = [
  {
    id: 'openai-chat',
    label: 'OpenAI Chat',
    path: '/v1',
    tagType: 'success',
    hint: 'OpenAI SDK、Cursor、Continue 等（自动拼接 /chat/completions）'
  },
  {
    id: 'openai-responses',
    label: 'OpenAI Responses',
    path: '/v1',
    tagType: 'success',
    hint: 'Codex 等 Responses 客户端（自动拼接 /responses）'
  },
  {
    id: 'anthropic',
    label: 'Anthropic',
    path: '/anthropic',
    tagType: 'warning',
    hint: 'DeepSeek / Kimi 等通行做法（自动拼接 /v1/messages）'
  },
  {
    id: 'gemini',
    label: 'Gemini',
    path: '',
    tagType: 'info',
    hint: 'Gemini SDK / CLI（自动拼接 /v1beta）'
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
