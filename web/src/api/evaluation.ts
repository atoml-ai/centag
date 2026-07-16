import api from './index'

// ── Evaluation Plugin Types ────────────────────────────────────────────────────

export interface EvaluationPlugin {
  name: string
  type: string // 'evaluator' | 'aggregator'
  enabled: boolean
  label?: string
  description?: string
  icon?: string
  config?: Record<string, any>
}

export interface EvaluationStats {
  enabled: boolean
  total_evaluations: number
  allowed_count: number
  rejected_count: number
}

export interface PluginConfigSchema {
  type: string
  properties: Record<string, ConfigProperty>
  required?: string[]
}

export interface ConfigProperty {
  type: string
  description?: string
  minimum?: number
  maximum?: number
  multipleOf?: number
  default?: any
}

export interface EvaluationTestResult {
  passed: boolean
  score?: number
  labels?: string[]
  details?: Record<string, any>
}

export interface EvaluationTestRequest {
  question: string
  answer: string
  history_messages?: Array<{
    role: string
    content: string
  }>
}

// ── Evaluation Plugin API ─────────────────────────────────────────────────────

export function getEvaluationStats(): Promise<EvaluationStats> {
  return api.get('/api/v1/evaluation/stats')
}

export function getEvaluationPlugins(): Promise<{ plugins: EvaluationPlugin[] }> {
  return api.get('/api/v1/evaluation/plugins')
}

export function enableEvaluationPlugin(name: string): Promise<void> {
  return api.post(`/api/v1/evaluation/plugins/${name}/enable`)
}

export function disableEvaluationPlugin(name: string): Promise<void> {
  return api.post(`/api/v1/evaluation/plugins/${name}/disable`)
}

export function updatePluginOrder(names: string[]): Promise<void> {
  return api.put('/api/v1/evaluation/plugins/order', { names })
}

export function getPluginConfig(name: string): Promise<Record<string, any>> {
  return api.get(`/api/v1/evaluation/plugins/${name}/config`)
}

export function updatePluginConfig(name: string, config: Record<string, any>): Promise<void> {
  return api.put(`/api/v1/evaluation/plugins/${name}/config`, config)
}

export function getPluginSchema(name: string): Promise<PluginConfigSchema> {
  return api.get(`/api/v1/evaluation/plugins/${name}/schema`)
}

export function testEvaluation(request: EvaluationTestRequest): Promise<EvaluationTestResult> {
  return api.post('/api/v1/evaluation/test', request)
}

export function setExactMatchEnabled(enabled: boolean): Promise<{ enabled: boolean }> {
  return api.put('/api/v1/evaluation/config/exact-match', { enabled })
}
