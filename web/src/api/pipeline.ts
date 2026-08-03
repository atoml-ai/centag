import api from '@/api'

export interface PluginDescriptor {
  name: string
  implementation: string
  kind: string
  version: string
  description: string
  config_schema: any
  input_schema: any
  output_schema: any
  permissions: string[]
  supports_stream: boolean
  concurrent: boolean
  api_version: string
  min_centag_version: string
}

export interface NodeConfig {
  backend: string
  model: string
  prompt_template?: string
  system_prompt?: string
  temperature?: number
  max_tokens?: number
  custom_config?: Record<string, any>
  template_vars?: Record<string, string>
  max_input_bytes?: number
  secrets_ref?: string[]
  permissions?: string[]
}

export interface PipelineNodeConfig {
  id: string
  type: string
  kind?: string
  implementation?: string
  name: string
  backend?: string
  model?: string
  config: NodeConfig
  inputs?: Record<string, string>
  outputs?: Record<string, any>
  config_schema_ref?: string
  secrets_ref?: Record<string, string>
  permissions?: string[]
  timeout?: number
  retry?: {
    max_attempts: number
    backoff_strategy: string
    initial_delay: number
    max_delay: number
  }
  condition?: string
  next_nodes?: string[]
  depends_on?: string[]
  route_config?: {
    router_node_id?: string
    route_value?: string
    is_default?: boolean
  }
}

export type Pipeline = AgentPatternPipeline

export interface StorageHookConfig {
  enabled: boolean
  namespace: string
  auto_save: boolean
  save_interval: number
  retention_days: number
}

export interface HookConfig {
  type: string
  on: string[]
  storage_name?: string
  storage_type?: string
  config?: Record<string, any>
}

export interface AgentPatternPipeline {
  schema_version?: string
  id: string
  name: string
  description: string
  version: string
  shortcut_code?: string
  nodes: PipelineNodeConfig[]
  global_config: {
    timeout: number
    max_retries: number
    bypass_on_error: boolean
    stream_mode: boolean
    parallel_limit: number
    log_level?: string
    system_prompt?: string
    fallback_groups?: Array<{
      primary_node_id: string
      fallback_nodes: string[]
      max_attempts: number
    }>
    storage?: StorageHookConfig
    hooks?: HookConfig[]
  }
  metadata?: Record<string, any>
}

export interface PluginRegistration {
  implementation: string
  kind: string
  version: string
  descriptor_json: string
  source: string
  enabled: boolean
  signature_status: string
  created_at: string
  updated_at: string
}

// 获取可用的流水线节点插件
export function getNodePlugins() {
  return api.get('/api/v1/pipelines/node-plugins')
}

/** 解析 node-plugins 接口响应（拦截器已解包为 { schema_version, plugins } 或数组） */
export function parseNodePluginsResponse(resp: unknown): PluginDescriptor[] {
  if (!resp) return []
  if (Array.isArray(resp)) return resp as PluginDescriptor[]
  if (typeof resp === 'object' && resp !== null && 'plugins' in resp) {
    const plugins = (resp as { plugins?: unknown }).plugins
    return Array.isArray(plugins) ? (plugins as PluginDescriptor[]) : []
  }
  return []
}

// 测试节点插件
export function testNodePlugin(implementation: string, data: any) {
  return api.post(`/api/v1/pipelines/node-plugins/${implementation}/test`, data)
}

/** 解析 pipelines 列表接口响应（拦截器已解包为数组或 { data: [] }） */
export function parsePipelinesResponse(resp: unknown): AgentPatternPipeline[] {
  if (!resp) return []
  if (Array.isArray(resp)) return resp as AgentPatternPipeline[]
  if (typeof resp === 'object' && resp !== null && 'data' in resp) {
    const data = (resp as { data?: unknown }).data
    return Array.isArray(data) ? (data as AgentPatternPipeline[]) : []
  }
  return []
}

// 获取流水线列表
export function getPipelines() {
  return api.get('/api/v1/pipelines')
}

// 获取单个流水线
export function getPipeline(id: string) {
  return api.get(`/api/v1/pipelines/${id}`)
}

// 创建流水线
// overwrite=true（默认）允许覆盖已有流水线；overwrite=false 时重复 ID 返回 409 冲突
export function createPipeline(data: AgentPatternPipeline, overwrite: boolean = true) {
  return api.post('/api/v1/pipelines', data, { params: { overwrite: String(overwrite) } })
}

// 更新流水线
export function updatePipeline(id: string, data: AgentPatternPipeline) {
  return api.put(`/api/v1/pipelines/${id}`, data)
}

// 删除流水线
export function deletePipeline(id: string) {
  return api.delete(`/api/v1/pipelines/${id}`)
}

// 复制流水线（从系统/其他流水线克隆到当前用户空间）
export function clonePipeline(id: string, data?: { id?: string; name?: string }) {
  return api.post(`/api/v1/pipelines/${id}/clone`, data || {})
}

// 执行流水线（超时 120s，与后端 global_config.timeout 对齐）
export function executePipeline(id: string, data: any) {
  return api.post(`/api/v1/pipelines/${id}/execute`, data, { timeout: 120000 })
}

// 直接执行流水线定义（超时 120s，与后端 global_config.timeout 对齐）
export function executePipelineDirect(data: any) {
  return api.post('/api/v1/pipelines/execute-direct', data, { timeout: 120000 })
}

// 验证流水线
export function validatePipeline(id: string, data?: any) {
  return api.post(`/api/v1/pipelines/${id}/validate`, data)
}

// 导出流水线
export function exportPipeline(id: string) {
  return api.get(`/api/v1/pipelines/${id}/export`)
}

// 获取流水线模板
export function getPipelineTemplates() {
  return api.get('/api/v1/pipelines/templates')
}

// 获取插件注册表列表
export function getPluginRegistrations() {
  return api.get('/api/v1/pipelines/plugin-registry')
}

// 获取单个插件注册信息
export function getPluginRegistration(implementation: string) {
  return api.get(`/api/v1/pipelines/plugin-registry/${implementation}`)
}

// 更新插件注册信息
export function updatePluginRegistration(implementation: string, data: any) {
  return api.put(`/api/v1/pipelines/plugin-registry/${implementation}`, data)
}

// 删除插件注册信息
export function deletePluginRegistration(implementation: string) {
  return api.delete(`/api/v1/pipelines/plugin-registry/${implementation}`)
}

// 更新节点配置
export function updateNodeConfig(pipelineId: string, nodeId: string, config: any) {
  return api.put(`/api/v1/pipelines/${pipelineId}/nodes/${nodeId}/config`, config)
}

// 获取执行历史
export function getExecutionHistory(pipelineId: string, limit: number = 50) {
  return api.get(`/api/v1/pipelines/${pipelineId}/executions`, { params: { limit } })
}

// 执行记录类型定义
export interface ExecutionRecord {
  id: string
  pipeline_id: string
  status: 'success' | 'failed'
  duration_ms: number
  total_tokens?: number
  input_content: string
  output_content?: string
  error_message?: string
  node_audit_log?: string
  created_at: string
}

export interface AutoBuildRouteRequest {
  strategy?: 'balance' | 'cost' | 'quality' | 'latency' | 'fast'
  dry_run?: boolean
  apply?: boolean
  probe_backends?: boolean
  canary?: boolean
  max_updates?: number
  categories?: string[]
  preview_updates?: AutoBuildRouteUpdate[]
}

export interface AutoBuildRouteUpdate {
  category: string
  target_node: string
  old_backend?: string
  old_model?: string
  new_backend: string
  new_model?: string
  reason?: string
  sample?: string
  strategy_factors?: Record<string, any>
}

export interface AutoBuildRouteResponse {
  pipeline_id: string
  strategy: string
  dry_run: boolean
  applied: boolean
  canary?: boolean
  max_updates?: number
  updates: AutoBuildRouteUpdate[]
  warnings: string[]
  pipeline: AgentPatternPipeline
}

export interface AutoBuildRollbackResponse {
  pipeline_id: string
  rolled_back?: boolean
  rollback_from?: string
  strategy?: string
  update_count?: number
  pipeline: AgentPatternPipeline
}

// 路由自动构建（仅路由型流水线适用）
export function autoBuildPipeline(id: string, data: AutoBuildRouteRequest) {
  return api.post(`/api/v1/pipelines/${id}/auto-build`, data) as Promise<AutoBuildRouteResponse>
}

// 自动构建回滚
export function rollbackAutoBuildPipeline(id: string, dryRun: boolean = false) {
  return api.post(`/api/v1/pipelines/${id}/auto-build/rollback`, { dry_run: dryRun }) as Promise<AutoBuildRollbackResponse>
}

// 节点审计摘要类型定义
export interface NodeAuditSummary {
  node_id: string
  implementation?: string
  kind?: string
  success: boolean
  error_code?: string
  duration_ms: number
}

// ==================== 流水线默认模式相关 API ====================

// 默认流水线配置
export interface PipelineDefaults {
  default_pipeline_id: string
  allow_user_override: boolean
}

// 获取默认流水线配置
export function getPipelineDefaults() {
  return api.get('/api/v1/pipeline/defaults')
}

// 更新默认流水线配置
export function updatePipelineDefaults(data: { default_pipeline_id: string }) {
  return api.put('/api/v1/pipeline/defaults', data)
}
