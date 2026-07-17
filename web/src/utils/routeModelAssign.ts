import type { AgentPatternPipeline, PipelineNodeConfig } from '@/api/pipeline'

export const SYSTEM_DEFAULT_BACKEND = '{{system.default_backend}}'
export const SYSTEM_DEFAULT_MODEL = '{{system.default_model}}'

export interface RouteModelTarget {
  node_id: string
  label: string
  hint?: string
  default_follow_system?: boolean
}

export interface RouteModelRow {
  nodeId: string
  label: string
  hint: string
  followSystem: boolean
  backend: string
  model: string
}

/** Fallback when seeded pipeline has no metadata.route_model_targets yet */
export const DEFAULT_ROUTE_MODEL_TARGETS: RouteModelTarget[] = [
  { node_id: 'code-generator', label: '代码生成', hint: '建议绑定强代码能力模型' },
  { node_id: 'translate-gen', label: '翻译', hint: '建议绑定通用/多语言模型' },
  { node_id: 'summary-gen', label: '摘要', hint: '可用更便宜或长上下文模型' },
  {
    node_id: 'chat-generator',
    label: '通用对话',
    hint: '日常对话；默认可跟随系统默认',
    default_follow_system: true
  }
]

export function isRouterModePipeline(pipeline: AgentPatternPipeline | null | undefined): boolean {
  if (!pipeline) return false
  if (pipeline.id === 'router-mode') return true
  const mode = pipeline.metadata?.aligned_proxy_mode
  return mode === 'router-mode' || mode === 'router'
}

export function getRouteModelTargets(pipeline: AgentPatternPipeline | null | undefined): RouteModelTarget[] {
  if (!pipeline) return []
  const fromMeta = pipeline.metadata?.route_model_targets
  if (Array.isArray(fromMeta) && fromMeta.length > 0) {
    return fromMeta
      .filter((t: any) => t && typeof t.node_id === 'string')
      .map((t: any) => ({
        node_id: String(t.node_id),
        label: String(t.label || t.node_id),
        hint: t.hint ? String(t.hint) : '',
        default_follow_system: !!t.default_follow_system
      }))
  }
  if (isRouterModePipeline(pipeline)) {
    return DEFAULT_ROUTE_MODEL_TARGETS
  }
  return []
}

export function isFollowSystemBinding(backend?: string, model?: string): boolean {
  const b = (backend || '').trim()
  const m = (model || '').trim()
  const backendFollow = !b || b === SYSTEM_DEFAULT_BACKEND
  const modelFollow = !m || m === SYSTEM_DEFAULT_MODEL
  return backendFollow && modelFollow
}

export function buildRouteModelRows(pipeline: AgentPatternPipeline): RouteModelRow[] {
  const targets = getRouteModelTargets(pipeline)
  const byId = new Map((pipeline.nodes || []).map((n) => [n.id, n]))
  return targets.map((t) => {
    const node = byId.get(t.node_id)
    const backend = node?.backend || node?.config?.backend || ''
    const model = node?.model || node?.config?.model || ''
    const follow = isFollowSystemBinding(backend, model)
    return {
      nodeId: t.node_id,
      label: t.label,
      hint: t.hint || '',
      followSystem: follow,
      backend: follow ? '' : backend,
      model: follow ? '' : model
    }
  })
}

function ensureNodeConfig(node: PipelineNodeConfig): PipelineNodeConfig {
  if (!node.config) {
    node.config = { backend: '', model: '' }
  }
  if (!node.config.template_vars) {
    node.config.template_vars = {}
  }
  return node
}

/** Apply assignment rows onto a deep-cloned pipeline; returns the mutated clone. */
export function applyRouteModelAssignments(
  pipeline: AgentPatternPipeline,
  rows: RouteModelRow[]
): AgentPatternPipeline {
  const next: AgentPatternPipeline = JSON.parse(JSON.stringify(pipeline))
  const byId = new Map((next.nodes || []).map((n) => [n.id, n]))

  for (const row of rows) {
    const node = byId.get(row.nodeId)
    if (!node) {
      throw new Error(`流水线中找不到节点 ${row.nodeId}`)
    }
    ensureNodeConfig(node)

    if (row.followSystem) {
      node.backend = SYSTEM_DEFAULT_BACKEND
      node.model = SYSTEM_DEFAULT_MODEL
      node.config.backend = SYSTEM_DEFAULT_BACKEND
      node.config.model = SYSTEM_DEFAULT_MODEL
      node.config.template_vars = {
        ...(node.config.template_vars || {}),
        backend: 'system.default_backend',
        model: 'system.default_model'
      }
    } else {
      const backend = (row.backend || '').trim()
      const model = (row.model || '').trim()
      if (!backend) throw new Error(`「${row.label}」请选择后端，或勾选跟随系统默认`)
      if (!model) throw new Error(`「${row.label}」请选择模型，或勾选跟随系统默认`)
      node.backend = backend
      node.model = model
      node.config.backend = backend
      node.config.model = model
      // Keep template_vars keys but point to literals via empty removal of system paths
      const tv = { ...(node.config.template_vars || {}) }
      delete tv.backend
      delete tv.model
      node.config.template_vars = tv
    }
  }

  // Ensure metadata targets exist for next open
  if (!next.metadata) next.metadata = {}
  if (!Array.isArray(next.metadata.route_model_targets) || !next.metadata.route_model_targets.length) {
    next.metadata.route_model_targets = DEFAULT_ROUTE_MODEL_TARGETS
  }
  if (!next.metadata.aligned_proxy_mode) {
    next.metadata.aligned_proxy_mode = 'router-mode'
  }

  return next
}

export function canAssignRouteModels(pipeline: AgentPatternPipeline | null | undefined): boolean {
  return getRouteModelTargets(pipeline).length > 0
}
