import type { Pipeline } from '@/api/pipeline'

export type PipelineFeatureKey =
  | 'routeAutoBuild'
  | 'pipelineEdit'
  | 'pipelineDelete'
  | 'pipelineShortcutUpdate'
  | 'pipelineBatchDelete'
  | 'executionHistory'
  | 'pipelineExport'

export interface PipelineFeatureContext {
  isAdmin: boolean
}

export interface PipelineFeatureSupport {
  visible: boolean
  enabled: boolean
  reason?: string
}

type FeatureRule = {
  label: string
  requiresAdmin?: boolean
  isSupported: (pipeline: Pipeline) => boolean
  unsupportedReason?: string
}

function hasNonEmptyRoutes(pipeline: Pipeline): boolean {
  if (!pipeline?.nodes?.length) return false
  for (const node of pipeline.nodes) {
    if (node?.type !== 'router') continue
    const customConfig = node.config?.custom_config
    const routes = customConfig?.routes
    if (!routes || typeof routes !== 'object') continue
    for (const [key, value] of Object.entries(routes)) {
      if (String(key).trim() && String(value ?? '').trim()) {
        return true
      }
    }
  }
  return false
}

const featureRules: Record<PipelineFeatureKey, FeatureRule> = {
  routeAutoBuild: {
    label: '路由自动配置',
    requiresAdmin: true,
    isSupported: (pipeline) => hasNonEmptyRoutes(pipeline),
    unsupportedReason: '该流水线未配置可自动构建的路由分支'
  },
  pipelineEdit: {
    label: '编辑流水线',
    isSupported: (pipeline) => !!pipeline?.id,
    unsupportedReason: '流水线未保存，暂不可编辑'
  },
  pipelineDelete: {
    label: '删除流水线',
    isSupported: (pipeline) => !!pipeline?.id,
    unsupportedReason: '流水线未保存，暂不可删除'
  },
  pipelineShortcutUpdate: {
    label: '快捷码更新',
    isSupported: (pipeline) => !!pipeline?.id,
    unsupportedReason: '流水线未保存，暂不可更新快捷码'
  },
  pipelineBatchDelete: {
    label: '批量删除',
    isSupported: (pipeline) => !!pipeline?.id,
    unsupportedReason: '所选流水线中存在不可删除项'
  },
  executionHistory: {
    label: '执行历史',
    isSupported: (pipeline) => !!pipeline?.id,
    unsupportedReason: '流水线未保存，暂无执行历史'
  },
  pipelineExport: {
    label: '流水线导出',
    isSupported: (pipeline) => !!pipeline?.id,
    unsupportedReason: '流水线未保存，暂不可导出'
  }
}

export function getPipelineFeatureLabel(feature: PipelineFeatureKey): string {
  return featureRules[feature]?.label ?? feature
}

/**
 * 通用流水线特性可见性判定：
 * - visible: 是否应在 UI 显示入口
 * - enabled: 显示后是否允许点击
 */
export function resolvePipelineFeatureSupport(
  feature: PipelineFeatureKey,
  pipeline: Pipeline,
  ctx: PipelineFeatureContext
): PipelineFeatureSupport {
  const rule = featureRules[feature]
  if (!rule) {
    return { visible: false, enabled: false, reason: '未知特性' }
  }
  if (rule.requiresAdmin && !ctx.isAdmin) {
    return { visible: false, enabled: false, reason: '仅管理员可用' }
  }
  if (!rule.isSupported(pipeline)) {
    return { visible: true, enabled: false, reason: rule.unsupportedReason }
  }
  return { visible: true, enabled: true }
}

