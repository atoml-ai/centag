import api from './index'

export interface ConversationSession {
  id: string
  user_id: number
  tenant_id?: string
  title?: string
  category?: string
  pipeline_id?: string
  proxy_mode?: string
  message_count: number
  created_at: string
  updated_at: string
}

export interface ConversationMessage {
  id: string
  session_id: string
  role: string
  content: string
  request_id?: string
  model?: string
  backend?: string
  pipeline_id?: string
  input_tokens?: number
  output_tokens?: number
  latency_ms?: number
  status_code?: number
  created_at: string
}

export function listSessions(params?: {
  category?: string
  limit?: number
  offset?: number
  since?: string
  until?: string
  user_id?: number
}) {
  return api({
    url: '/api/v1/conversations/sessions',
    method: 'get',
    params
  }) as Promise<{ sessions: ConversationSession[]; count: number }>
}

export function getSession(id: string) {
  return api({
    url: `/api/v1/conversations/sessions/${encodeURIComponent(id)}`,
    method: 'get'
  }) as Promise<ConversationSession>
}

export function listMessages(id: string, params?: { limit?: number; offset?: number }) {
  return api({
    url: `/api/v1/conversations/sessions/${encodeURIComponent(id)}/messages`,
    method: 'get',
    params
  }) as Promise<{ messages: ConversationMessage[]; count: number }>
}

export function listCategories() {
  return api({
    url: '/api/v1/conversations/categories',
    method: 'get'
  }) as Promise<{ categories: string[] }>
}
