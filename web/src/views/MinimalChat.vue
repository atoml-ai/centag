<template>
  <el-drawer
    v-model="visible"
    :title="t('minimalChat.title')"
    direction="rtl"
    size="1280px"
    :close-on-click-modal="true"
    destroy-on-close
    class="minimal-chat-drawer"
    @opened="onOpened"
  >
    <div class="minimal-chat-body">
      <!-- 流水线选择：单独一行，加长下拉 -->
      <div class="pipeline-panel">
        <div class="pipeline-row pipeline-row-select">
          <label class="pipeline-label">{{ t('minimalChat.selectPipeline') }}</label>
          <el-select
            v-model="selectedPipelineId"
            :placeholder="t('minimalChat.selectPipeline')"
            size="default"
            class="pipeline-select"
            popper-class="pipeline-select-dropdown"
            filterable
            :loading="pipelinesLoading"
          >
            <el-option
              v-for="p in pipelines"
              :key="p.id"
              :label="formatPipelineOptionLabel(p)"
              :value="p.id"
            >
              <div class="pipeline-option">
                <span class="pipeline-option-name">{{ p.name || p.id }}</span>
                <span class="pipeline-option-id">{{ p.id }}</span>
              </div>
            </el-option>
          </el-select>
          <div class="pipeline-mode-toggle">
            <el-radio-group v-model="chatMode" :disabled="loading" size="small">
              <el-radio-button value="traditional">{{ t('minimalChat.modeTraditional') }}</el-radio-button>
              <el-radio-button value="agent">{{ t('minimalChat.modeAgent') }}</el-radio-button>
            </el-radio-group>
            <span v-if="chatMode === 'agent' && agentSessionId" class="agent-hint">
              {{ agentSessionId.slice(0, 8) }}…
            </span>
          </div>
        </div>
        <div v-if="selectedPipeline" class="pipeline-row pipeline-row-meta">
          <el-tag size="small" type="info" effect="plain">{{ selectedPipeline.id }}</el-tag>
          <el-tag v-if="selectedPipeline.shortcut_code" size="small" effect="plain">
            {{ selectedPipeline.shortcut_code }}
          </el-tag>
          <el-tag size="small" type="primary" effect="plain">
            {{ t('minimalChat.nodeCount', { count: selectedPipeline.nodes?.length || 0 }) }}
          </el-tag>
          <span v-if="selectedPipeline.description" class="pipeline-desc">
            {{ selectedPipeline.description }}
          </span>
        </div>
      </div>

      <div class="chat-container">
        <div class="chat-messages" ref="messagesContainer">
          <div v-if="messages.length === 0" class="empty-state">
            <el-icon :size="40" color="#c0c4cc"><ChatDotRound /></el-icon>
            <p>{{ t('minimalChat.selectPipelineHint') }}</p>
            <p class="empty-sub">{{ t('minimalChat.testHint') }}</p>
          </div>

          <div v-for="(msg, idx) in messages" :key="idx" :class="['message', msg.role]">
            <div class="message-avatar">
              <el-icon v-if="msg.role === 'user'" :size="16"><User /></el-icon>
              <el-icon v-else :size="16"><Monitor /></el-icon>
            </div>
            <div class="message-stack">
              <div class="message-content">
                <div class="message-text" v-html="renderMarkdown(msg.content || (msg.error ? '' : '…'))"></div>
                <div v-if="msg.error" class="message-error">
                  <el-icon><WarningFilled /></el-icon> {{ msg.error }}
                </div>
              </div>

              <!-- 助手消息：可视化流程 + 详情 -->
              <div v-if="msg.role === 'assistant' && msg.trace" class="trace-card">
                <div class="flow-strip" :title="t('minimalChat.flowTitle')">
                  <template v-for="(step, si) in msg.trace.steps" :key="si">
                    <div
                      class="flow-step"
                      :class="[
                        `status-${step.status}`,
                        { 'is-fallback': step.kind === 'fallback' }
                      ]"
                    >
                      <div class="flow-dot">
                        <el-icon v-if="step.status === 'ok'" :size="12"><CircleCheckFilled /></el-icon>
                        <el-icon v-else-if="step.status === 'fail'" :size="12"><CircleCloseFilled /></el-icon>
                        <el-icon v-else-if="step.status === 'skip'" :size="12"><RemoveFilled /></el-icon>
                        <span v-else class="flow-dot-inner" />
                      </div>
                      <div class="flow-label">{{ step.label }}</div>
                      <div class="flow-status-text" :class="`text-${step.status}`">
                        {{ stepStatusText(step) }}
                      </div>
                      <div v-if="step.sub" class="flow-sub">{{ step.sub }}</div>
                    </div>
                    <div
                      v-if="si < msg.trace.steps.length - 1"
                      class="flow-arrow"
                      :class="{ fail: step.status === 'fail', skip: step.status === 'skip' }"
                    >→</div>
                  </template>
                </div>

                <div class="flow-legend">
                  <span><i class="lg ok" />{{ t('minimalChat.legendOk') }}</span>
                  <span><i class="lg skip" />{{ t('minimalChat.legendSkip') }}</span>
                  <span><i class="lg fail" />{{ t('minimalChat.legendFail') }}</span>
                </div>

                <div class="trace-badges">
                  <el-tag v-if="msg.trace.pipelineId" size="small" type="info" effect="plain">
                    {{ msg.trace.pipelineId }}
                  </el-tag>
                  <el-tag
                    v-if="msg.trace.fallbackUsed"
                    size="small"
                    type="warning"
                    effect="dark"
                  >
                    {{ t('minimalChat.fallbackUsed') }}
                  </el-tag>
                  <el-tag v-else size="small" type="success" effect="plain">
                    {{ t('minimalChat.fallbackNotUsed') }}
                  </el-tag>
                  <el-tag v-if="msg.trace.durationMs" size="small" effect="plain">
                    {{ msg.trace.durationMs }}ms
                  </el-tag>
                  <el-tag v-if="msg.trace.backendId" size="small" effect="plain">
                    {{ msg.trace.backendId }}
                  </el-tag>
                  <el-tag v-if="msg.trace.model" size="small" effect="plain">
                    {{ msg.trace.model }}
                  </el-tag>
                </div>

                <p v-if="msg.trace.fallbackNotice" class="fallback-notice">
                  {{ msg.trace.fallbackNotice }}
                </p>

                <el-collapse class="trace-collapse">
                  <el-collapse-item :title="t('minimalChat.detailTitle')" name="detail">
                    <div class="detail-grid">
                      <div v-for="row in detailRows(msg.trace)" :key="row.key" class="detail-row">
                        <span class="detail-key">{{ row.key }}</span>
                        <code class="detail-val">{{ row.value }}</code>
                      </div>
                    </div>
                    <div v-if="msg.trace.nodeResults?.length" class="node-table-wrap">
                      <div class="node-table-title">{{ t('minimalChat.nodeResults') }}</div>
                      <table class="node-table">
                        <thead>
                          <tr>
                            <th>{{ t('minimalChat.nodeId') }}</th>
                            <th>{{ t('minimalChat.nodeType') }}</th>
                            <th>{{ t('minimalChat.status') }}</th>
                            <th>{{ t('minimalChat.model') }}</th>
                            <th>{{ t('minimalChat.duration') }}</th>
                          </tr>
                        </thead>
                        <tbody>
                          <tr v-for="(n, ni) in msg.trace.nodeResults" :key="ni">
                            <td>{{ n.node_id || n.id || '—' }}</td>
                            <td>{{ n.node_type || n.type || '—' }}</td>
                            <td>
                              <span :class="n.success === false || n.status === 'failed' ? 'bad' : 'ok'">
                                {{ formatNodeStatus(n) }}
                              </span>
                            </td>
                            <td>{{ n.model || '—' }}</td>
                            <td>{{ n.duration_ms != null ? `${n.duration_ms}ms` : '—' }}</td>
                          </tr>
                        </tbody>
                      </table>
                    </div>
                    <pre v-if="msg.trace.rawMeta" class="raw-meta">{{ pretty(msg.trace.rawMeta) }}</pre>
                  </el-collapse-item>
                </el-collapse>
              </div>

              <!-- Agent 模式：工具执行步骤 -->
              <div v-if="msg.role === 'assistant' && msg.agentSteps?.length" class="trace-card agent-trace">
                <div class="agent-trace-title">{{ t('minimalChat.agentThinking') }}</div>
                <div class="agent-steps">
                  <div v-for="(step, si) in msg.agentSteps" :key="si" class="agent-step">
                    <el-icon v-if="step.type === 'tool_call'" :size="14" color="#e6a23c"><Loading /></el-icon>
                    <el-icon v-else-if="step.type === 'tool_result'" :size="14" color="#67c23a"><CircleCheckFilled /></el-icon>
                    <el-icon v-else-if="step.type === 'error'" :size="14" color="#f56c6c"><CircleCloseFilled /></el-icon>
                    <el-icon v-else :size="14" color="#909399"><InfoFilled /></el-icon>
                    <span class="agent-step-label">{{ step.label }}</span>
                    <span v-if="step.detail" class="agent-step-detail">{{ step.detail }}</span>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <div v-if="loading" class="message assistant">
            <div class="message-avatar"><el-icon :size="16"><Monitor /></el-icon></div>
            <div class="message-content">
              <div class="typing-indicator">
                <span></span><span></span><span></span>
              </div>
            </div>
          </div>
        </div>

        <div class="chat-input">
          <el-input
            v-model="inputText"
            type="textarea"
            :autosize="{ minRows: 1, maxRows: 4 }"
            :placeholder="t('minimalChat.inputMessage')"
            :disabled="loading"
            @keydown="handleKeydown"
          />
          <el-button
            type="primary"
            :icon="Promotion"
            :loading="loading"
            :disabled="!inputText.trim() || !selectedPipelineId"
            @click="sendMessage"
            circle
            size="large"
          />
        </div>
      </div>
    </div>
  </el-drawer>
</template>

<script setup lang="ts">
import { ref, computed, nextTick, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  Promotion, User, Monitor, ChatDotRound, WarningFilled,
  CircleCheckFilled, CircleCloseFilled, RemoveFilled, Loading, InfoFilled,
} from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { getPipelines, getPipelineDefaults, type AgentPatternPipeline } from '@/api/pipeline'
import { agentApi, type Session as AgentSession } from '@/api/agent'
import { useAuthStore } from '@/stores/auth'
import { sanitizeHtml } from '@/utils/sanitize'
import api from '@/api'

const { t } = useI18n()

interface FlowStep {
  label: string
  sub?: string
  status: 'ok' | 'fail' | 'skip' | 'pending' | 'info'
  kind?: 'client' | 'pipeline' | 'routing' | 'node' | 'fallback' | 'backend' | 'result'
}

interface TraceInfo {
  pipelineId?: string
  proxyMode?: string
  backendId?: string
  model?: string
  executorModel?: string
  durationMs?: number
  success?: boolean
  fallbackUsed?: boolean
  fallbackFrom?: string
  fallbackTo?: string
  fallbackNotice?: string
  fallbackReason?: string
  targetBaseUrl?: string
  requestId?: string
  sessionId?: string
  nodeResults?: any[]
  steps: FlowStep[]
  headers?: Record<string, string>
  rawMeta?: Record<string, unknown>
}

interface AgentStep {
  type: 'tool_call' | 'tool_result' | 'info' | 'error'
  label: string
  detail?: string
}

interface ChatMsg {
  role: 'user' | 'assistant'
  content: string
  error?: string
  trace?: TraceInfo
  agentSteps?: AgentStep[]
}

const visible = defineModel<boolean>({ default: false })
const props = withDefaults(defineProps<{
  initialPipelineId?: string
}>(), {
  initialPipelineId: ''
})
const authStore = useAuthStore()

const pipelines = ref<AgentPatternPipeline[]>([])
const pipelinesLoading = ref(false)
const selectedPipelineId = ref('')
const messages = ref<ChatMsg[]>([])
const inputText = ref('')
const loading = ref(false)
const messagesContainer = ref<HTMLElement>()
const cachedTestAPIKey = ref('')

// Agent mode state
const chatMode = ref<'traditional' | 'agent'>('traditional')
const agentSessionId = ref('')
const agentSessionLoading = ref(false)

const selectedPipeline = computed(() =>
  pipelines.value.find(p => p.id === selectedPipelineId.value)
)

function formatPipelineOptionLabel(p: AgentPatternPipeline) {
  const name = p.name || p.id
  return p.shortcut_code ? `${name} (${p.shortcut_code})` : name
}

function pretty(v: unknown) {
  try {
    return JSON.stringify(v, null, 2)
  } catch {
    return String(v)
  }
}

function formatNodeStatus(n: any) {
  if (n.success === false || n.status === 'failed' || n.status === 'error') return t('minimalChat.statusFail')
  if (n.status === 'skipped' || n.skipped) return t('minimalChat.statusSkip')
  return t('minimalChat.statusOk')
}

function stepStatusText(step: FlowStep) {
  if (step.kind === 'fallback') {
    if (step.status === 'ok') return t('minimalChat.stepRanFallback')
    if (step.status === 'skip') return t('minimalChat.stepSkippedFallback')
    if (step.status === 'fail') return t('minimalChat.stepFailedFallback')
  }
  if (step.status === 'ok') return t('minimalChat.stepRan')
  if (step.status === 'fail') return t('minimalChat.stepFailedShort')
  if (step.status === 'skip') return t('minimalChat.stepSkipped')
  return t('minimalChat.stepUnknown')
}

function detailRows(trace: TraceInfo) {
  const rows: Array<{ key: string; value: string }> = []
  const push = (key: string, value?: string | number | boolean | null) => {
    if (value === undefined || value === null || value === '') return
    rows.push({ key, value: String(value) })
  }
  push(t('minimalChat.pipeline'), trace.pipelineId)
  push(t('minimalChat.proxyMode'), trace.proxyMode)
  if (trace.rawMeta) {
    push(t('minimalChat.routingStrategy'), trace.rawMeta.routing_strategy as string)
    push(t('minimalChat.selectedRoute'), trace.rawMeta.selected_route as string)
    push(t('minimalChat.routeCategory'), trace.rawMeta.route_category as string)
  }
  push(t('minimalChat.backend'), trace.backendId)
  push(t('minimalChat.model'), trace.model || trace.executorModel)
  push(t('minimalChat.duration'), trace.durationMs != null ? `${trace.durationMs}ms` : undefined)
  push(t('minimalChat.fallback'), trace.fallbackUsed ? t('minimalChat.yes') : t('minimalChat.no'))
  push(t('minimalChat.fallbackFrom'), trace.fallbackFrom)
  push(t('minimalChat.fallbackTo'), trace.fallbackTo)
  push('降级原因', trace.fallbackReason)
  push(t('minimalChat.targetUrl'), trace.targetBaseUrl)
  push('Request ID', trace.requestId)
  push('Session ID', trace.sessionId)
  if (trace.headers) {
    for (const [k, v] of Object.entries(trace.headers)) {
      if (!rows.some(r => r.key === k)) push(k, v)
    }
  }
  return rows
}

function collectResponseHeaders(res: Response): Record<string, string> {
  const want = [
    'x-pipeline-id', 'x-pipeline-executed', 'x-pipeline-success', 'x-pipeline-duration-ms',
    'x-proxy-mode', 'x-backend-id', 'x-model', 'x-executor-model', 'x-executor-backend',
    'x-fallback-used', 'x-fallback-from-model', 'x-fallback-to-model', 'x-fallback-notice',
    'x-fallback-reason', 'x-target-baseurl', 'x-session-id', 'x-request-id', 'x-cache-hit', 'x-response-trace',
    'x-pipeline-bypass', 'x-pipeline-bypass-node', 'x-pipeline-bypass-reason',
  ]
  const out: Record<string, string> = {}
  for (const k of want) {
    const v = res.headers.get(k)
    if (v) out[k] = v
  }
  return out
}

function truthy(v: unknown): boolean {
  if (v === true || v === 1) return true
  if (typeof v === 'string') {
    const s = v.trim().toLowerCase()
    return s === 'true' || s === '1' || s === 'yes'
  }
  return false
}

function latestNodeResult(nodeResults: any[], nodeId: string) {
  let found: any
  for (const r of nodeResults) {
    if (r?.node_id === nodeId || r?.id === nodeId) found = r
  }
  return found
}

/** 综合 meta / 响应头 / last_node / 错误文本判断是否发生降级 */
function detectFallbackUsed(meta: Record<string, any>, headers: Record<string, string>, errorText?: string): boolean {
  if (truthy(meta.fallback_used) || truthy(headers['x-fallback-used']) || truthy(meta.billing_fallback_used)) {
    return true
  }
  const lastNode = String(meta.last_node || '').toLowerCase()
  if (lastNode.includes('fallback')) return true
  const fromNode = String(meta.fallback_from_node || '').trim()
  if (fromNode) return true
  // 失败响应没有 _centag_meta：但降级聚合错误文本一定包含 "fallback"（如
  // "all fallback attempts failed ... last fallback: ..."），据此识别已走降级。
  if (errorText && /fallback/i.test(errorText)) return true
  return false
}

function buildFlowSteps(
  pipeline: AgentPatternPipeline | undefined,
  meta: Record<string, any>,
  headers: Record<string, string>,
  ok: boolean,
  errorText?: string
): FlowStep[] {
  const steps: FlowStep[] = [
    { label: t('minimalChat.stepClient'), status: 'ok', kind: 'client' },
  ]

  const pipelineId = String(meta.pipeline_id || headers['x-pipeline-id'] || pipeline?.id || '')
  steps.push({
    label: t('minimalChat.stepPipeline'),
    sub: pipelineId || undefined,
    status: 'ok',
    kind: 'pipeline',
  })

  // 路由/分类步骤（router 节点输出的分类决策信息）
  const routingStrategy = String(meta.routing_strategy || '')
  const selectedRoute = String(meta.selected_route || '')
  const routeCategory = String(meta.route_category || meta.category || '')
  if (routingStrategy || selectedRoute) {
    const subParts = [routingStrategy].filter(Boolean)
    if (routeCategory && routeCategory !== selectedRoute) {
      subParts.push(`→ ${routeCategory}`)
    }
    if (selectedRoute) {
      subParts.push(`→ ${selectedRoute}`)
    }
    steps.push({
      label: t('minimalChat.stepRouting'),
      sub: subParts.join(' · ') || undefined,
      status: 'ok',
      kind: 'routing',
    })
  }

  const nodeResults: any[] = Array.isArray(meta.node_results) ? meta.node_results : []
  const nodes = pipeline?.nodes || []
  const fallbackUsed = detectFallbackUsed(meta, headers, errorText)

  if (nodes.length > 0) {
    for (const node of nodes) {
      const nr = latestNodeResult(nodeResults, node.id)
      const isFallbackNode = !!(
        node.config?.custom_config?.is_fallback ||
        String(node.id || '').includes('fallback')
      )
      let status: FlowStep['status'] = 'info'
      if (fallbackUsed && !isFallbackNode) {
        // 主节点：发生了降级即未产出最终结果
        status = 'fail'
      } else if (isFallbackNode) {
        if (fallbackUsed) {
          // 降级节点：如实反映真实结果——若降级模型也失败（整体请求失败），不能标绿打勾
          if (nr && (nr.success === false || nr.status === 'failed')) {
            status = 'fail'
          } else if (nr && nr.success) {
            status = 'ok'
          } else {
            status = ok ? 'ok' : 'fail'
          }
        } else {
          status = 'skip' // 未走降级 → 备用节点未执行
        }
      } else if (nr) {
        if (nr.success === false || nr.status === 'failed') status = 'fail'
        else if (nr.status === 'skipped' || nr.skipped) status = 'skip'
        else status = 'ok'
      } else {
        status = ok ? 'ok' : 'fail'
      }
      const subParts = [node.type || node.kind].filter(Boolean) as string[]
      if (isFallbackNode) {
        subParts.unshift(t('minimalChat.fallbackNodeTag'))
      }
      steps.push({
        label: node.name || node.id,
        sub: subParts.join(' · ') || undefined,
        status,
        kind: isFallbackNode ? 'fallback' : 'node',
      })
    }
  } else if (nodeResults.length > 0) {
    for (const nr of nodeResults) {
      const fail = nr.success === false || nr.status === 'failed'
      const isFb = String(nr.node_id || nr.id || '').includes('fallback')
      steps.push({
        label: nr.node_id || nr.id || 'node',
        sub: [isFb ? t('minimalChat.fallbackNodeTag') : '', nr.model || nr.node_type].filter(Boolean).join(' · '),
        status: fail ? 'fail' : (nr.status === 'skipped' ? 'skip' : 'ok'),
        kind: isFb ? 'fallback' : 'node',
      })
    }
  }

  const backend = String(meta.executor_backend || meta.backend_id || headers['x-backend-id'] || '')
  const model = String(
    meta.executor_model || meta.fallback_to_model || meta.model || headers['x-executor-model'] || headers['x-model'] || ''
  )
  steps.push({
    label: t('minimalChat.stepBackend'),
    sub: [backend, model].filter(Boolean).join(' · ') || undefined,
    status: ok ? 'ok' : 'fail',
    kind: 'backend',
  })

  steps.push({
    label: ok ? t('minimalChat.stepDone') : t('minimalChat.stepFailed'),
    status: ok ? 'ok' : 'fail',
    kind: 'result',
  })

  return steps
}

function buildTraceFromMeta(
  meta: Record<string, any>,
  headers: Record<string, string>,
  pipeline: AgentPatternPipeline | undefined,
  ok: boolean,
  observedModel?: string,
  errorText?: string
): TraceInfo {
  const mergedMeta = { ...meta }
  // 流式 chunk 里的 model（如 deepseek-v4-flash）可作为降级后模型的补充信号
  if (observedModel && !mergedMeta.executor_model && !mergedMeta.fallback_to_model) {
    mergedMeta.observed_model = observedModel
  }
  let fallbackUsed = detectFallbackUsed(mergedMeta, headers, errorText)
  // 兜底：实际响应模型与主节点配置模型不同，且命中备用节点模型时，视为已降级
  if (!fallbackUsed && observedModel && pipeline) {
    const primary = pipeline.nodes?.find(n => !String(n.id).includes('fallback'))
    const fb = pipeline.nodes?.find(n => String(n.id).includes('fallback') || n.config?.custom_config?.is_fallback)
    const primaryModel = String(primary?.model || primary?.config?.model || '')
    const fbModel = String(fb?.model || fb?.config?.model || '')
    if (
      fbModel &&
      observedModel.toLowerCase() === fbModel.toLowerCase() &&
      primaryModel &&
      observedModel.toLowerCase() !== primaryModel.toLowerCase() &&
      !primaryModel.includes('{{')
    ) {
      fallbackUsed = true
      mergedMeta.fallback_used = true
      mergedMeta.fallback_to_model = observedModel
    } else if (
      fb &&
      observedModel &&
      primaryModel.includes('{{') &&
      fbModel &&
      !fbModel.includes('{{') &&
      observedModel.toLowerCase() === fbModel.toLowerCase()
    ) {
      fallbackUsed = true
      mergedMeta.fallback_used = true
      mergedMeta.fallback_to_model = observedModel
    }
  }
  if (fallbackUsed) {
    mergedMeta.fallback_used = true
  }
  const durationMs = Number(mergedMeta.duration_ms || headers['x-pipeline-duration-ms'] || 0) || undefined
  return {
    pipelineId: String(mergedMeta.pipeline_id || headers['x-pipeline-id'] || pipeline?.id || ''),
    proxyMode: String(mergedMeta.mode || headers['x-proxy-mode'] || ''),
    backendId: String(mergedMeta.executor_backend || mergedMeta.backend_id || headers['x-backend-id'] || ''),
    model: String(
      mergedMeta.executor_model ||
      mergedMeta.fallback_to_model ||
      observedModel ||
      headers['x-executor-model'] ||
      headers['x-model'] ||
      ''
    ),
    executorModel: String(mergedMeta.executor_model || ''),
    durationMs,
    success: mergedMeta.success !== undefined ? !!mergedMeta.success : ok,
    fallbackUsed,
    fallbackFrom: String(mergedMeta.fallback_from_model || headers['x-fallback-from-model'] || ''),
    fallbackTo: String(mergedMeta.fallback_to_model || headers['x-fallback-to-model'] || observedModel || ''),
    fallbackNotice: String(mergedMeta.fallback_notice || headers['x-fallback-notice'] || ''),
    fallbackReason: String(mergedMeta.fallback_reason || headers['x-fallback-reason'] || ''),
    targetBaseUrl: String(mergedMeta.target_base_url || headers['x-target-baseurl'] || ''),
    requestId: headers['x-request-id'] || undefined,
    sessionId: headers['x-session-id'] || undefined,
    nodeResults: Array.isArray(mergedMeta.node_results) ? mergedMeta.node_results : [],
    steps: buildFlowSteps(pipeline, mergedMeta, headers, ok, errorText),
    headers,
    rawMeta: Object.keys(mergedMeta).length ? mergedMeta : undefined,
  }
}

async function applyPreferredPipeline() {
  let preferred = (props.initialPipelineId || '').trim()
  if (!preferred) {
    try {
      const res: any = await getPipelineDefaults()
      const data = res?.data ?? res
      preferred = String(data?.default_pipeline_id || '').trim()
    } catch {
      /* ignore */
    }
  }
  if (preferred && pipelines.value.some(p => p.id === preferred)) {
    selectedPipelineId.value = preferred
    return
  }
  if (pipelines.value.length > 0 && !selectedPipelineId.value) {
    selectedPipelineId.value = pipelines.value[0].id
  }
}

function renderMarkdown(content: string): string {
  if (!content) return ''
  return sanitizeHtml(content
    .replace(/\n/g, '<br>')
    .replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>')
    .replace(/\*(.*?)\*/g, '<em>$1</em>')
    .replace(/`(.*?)`/g, '<code>$1</code>')
    .replace(/```([\s\S]*?)```/g, '<pre class="code-block">$1</pre>'))
}

async function scrollToBottom() {
  await nextTick()
  if (messagesContainer.value) {
    messagesContainer.value.scrollTop = messagesContainer.value.scrollHeight
  }
}

async function loadPipelines() {
  pipelinesLoading.value = true
  try {
    const resp = await getPipelines()
    const data = resp?.data ?? resp
    pipelines.value = Array.isArray(data) ? data : []
    await applyPreferredPipeline()
  } catch (e: any) {
    console.error('Failed to load pipelines:', e)
  } finally {
    pipelinesLoading.value = false
  }
}

async function resolveProxyAuthHeader(): Promise<Record<string, string>> {
  if (authStore.accessToken) {
    return { Authorization: `Bearer ${authStore.accessToken}` }
  }
  if (cachedTestAPIKey.value) {
    return { Authorization: `Bearer ${cachedTestAPIKey.value}` }
  }
  try {
    const status: any = await api.get('/api/v1/settings/api-keys/status')
    const required = !!(status?.auth_required ?? status?.data?.auth_required)
    if (!required) return {}
    const res: any = await api.get('/api/v1/settings/api-keys')
    const list = Array.isArray(res?.data) ? res.data : Array.isArray(res) ? res : []
    const full = list.find((k: any) => k?.api_key)?.api_key
    if (full) {
      cachedTestAPIKey.value = full
      return { Authorization: `Bearer ${full}` }
    }
  } catch {
    /* ignore */
  }
  return {}
}

function handleKeydown(event: KeyboardEvent) {
  if (event.key === 'Enter' && event.ctrlKey) {
    event.preventDefault()
    sendMessage()
  }
}

async function sendMessage() {
  if (chatMode.value === 'agent') {
    return sendAgentMessage()
  }
  return sendTraditionalMessage()
}

async function sendAgentMessage() {
  const text = inputText.value.trim()
  if (!text || !selectedPipelineId.value || loading.value) return

  messages.value.push({ role: 'user', content: text })
  inputText.value = ''
  loading.value = true
  agentSessionLoading.value = true
  await scrollToBottom()

  const assistantMsg: ChatMsg = { role: 'assistant', content: '', agentSteps: [] }
  messages.value.push(assistantMsg)

  try {
    // Create session if needed
    if (!agentSessionId.value) {
      const session = await agentApi.createSession({
        skill: '',
        backend_id: '',
        model: ''
      })
      agentSessionId.value = session.id
      assistantMsg.agentSteps!.push({
        type: 'info',
        label: `${t('minimalChat.agentSession')}: ${session.id.slice(0, 8)}…`
      })
    }

    // Send message via builtin-agent API
    const response = await agentApi.sendMessage(agentSessionId.value, {
      content: text
    })

    assistantMsg.content = response.content || ''

    // Parse agent steps from the response content if it contains tool execution info
    // The backend returns the final answer; we show it directly
    assistantMsg.agentSteps!.push({
      type: 'info',
      label: t('minimalChat.agentThinking'),
      detail: assistantMsg.content ? assistantMsg.content.slice(0, 100) + (assistantMsg.content.length > 100 ? '...' : '') : ''
    })
  } catch (e: any) {
    assistantMsg.error = e.message || t('minimalChat.agentError')
    assistantMsg.agentSteps!.push({
      type: 'error',
      label: t('minimalChat.agentError'),
      detail: assistantMsg.error
    })
    ElMessage.error(assistantMsg.error)
  } finally {
    loading.value = false
    agentSessionLoading.value = false
    await scrollToBottom()
  }
}

async function sendTraditionalMessage() {
  const text = inputText.value.trim()
  if (!text || !selectedPipelineId.value || loading.value) return

  messages.value.push({ role: 'user', content: text })
  inputText.value = ''
  loading.value = true
  await scrollToBottom()

  const assistantMsg: ChatMsg = { role: 'assistant', content: '' }
  messages.value.push(assistantMsg)

  const pipeline = selectedPipeline.value
  let headers: Record<string, string> = {}
  let meta: Record<string, any> = {}
  let observedModel = ''

  try {
    const modelField = `pipeline.${selectedPipelineId.value}`
    const authHeaders = await resolveProxyAuthHeader()

    const response = await fetch('/v1/chat/completions', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-Pipeline-ID': selectedPipelineId.value,
        'X-Centag-Include-Meta': 'true',
        'User-Agent': 'centag/webui-pipeline-test',
        ...authHeaders
      },
      body: JSON.stringify({
        model: modelField,
        messages: messages.value.slice(0, -1).map(m => ({
          role: m.role,
          content: m.content
        })),
        stream: true
      })
    })

    headers = collectResponseHeaders(response)

    if (!response.ok) {
      const errBody = await response.text()
      throw new Error(`HTTP ${response.status}: ${errBody}`)
    }

    const reader = response.body?.getReader()
    const decoder = new TextDecoder()
    if (!reader) throw new Error(t('minimalChat.streamReadFailed'))

    let buffer = ''
    let eventName = ''
    while (true) {
      const { done, value } = await reader.read()
      if (done) break

      buffer += decoder.decode(value, { stream: true })
      const lines = buffer.split('\n')
      buffer = lines.pop() || ''

      for (const line of lines) {
        const trimmed = line.trimEnd()
        if (!trimmed) {
          eventName = ''
          continue
        }
        if (trimmed.startsWith('event:')) {
          eventName = trimmed.slice(6).trim()
          continue
        }
        if (!trimmed.startsWith('data:')) continue
        const payload = trimmed.slice(5).trim()
        if (!payload || payload === '[DONE]') continue

        try {
          const data = JSON.parse(payload)

          if (eventName === 'centag.meta' || data?._centag_meta) {
            meta = { ...meta, ...data }
            continue
          }

          if (typeof data.model === 'string' && data.model.trim()) {
            observedModel = data.model.trim()
          }

          if (data.error && !data.choices) {
            const errMsg = typeof data.error === 'string'
              ? data.error
              : data.error.message || JSON.stringify(data.error)
            assistantMsg.error = errMsg
            assistantMsg.trace = buildTraceFromMeta(meta, headers, pipeline, false, observedModel, errMsg)
            loading.value = false
            return
          }

          const delta = data.choices?.[0]?.delta?.content
          const reasoning = data.choices?.[0]?.delta?.reasoning_content
          if (delta) {
            assistantMsg.content += delta
            await scrollToBottom()
          } else if (reasoning && !assistantMsg.content) {
            // 部分免费模型只吐 reasoning；先展示以免空白
            assistantMsg.content += reasoning
            await scrollToBottom()
          }
        } catch {
          /* ignore parse errors */
        }
      }
    }

    // 流结束后再读一次 header（部分环境可补全；多数浏览器已冻结，故以 meta 为主）
    headers = { ...headers, ...collectResponseHeaders(response) }
    const ok = !assistantMsg.error && (!!assistantMsg.content || truthy(meta.success))
    assistantMsg.trace = buildTraceFromMeta(meta, headers, pipeline, ok || !assistantMsg.error, observedModel, assistantMsg.error)
  } catch (e: any) {
    assistantMsg.error = e.message || t('minimalChat.requestFailed')
    assistantMsg.trace = buildTraceFromMeta(meta, headers, pipeline, false, observedModel, assistantMsg.error)
    ElMessage.error(assistantMsg.error)
  } finally {
    loading.value = false
    await scrollToBottom()
  }
}

function onOpened() {
  loadPipelines()
}

watch(visible, (val) => {
  if (val) {
    messages.value = []
    inputText.value = ''
    chatMode.value = 'traditional'
    agentSessionId.value = ''
    if (props.initialPipelineId) {
      selectedPipelineId.value = props.initialPipelineId
    }
    loadPipelines()
  } else {
    selectedPipelineId.value = ''
    agentSessionId.value = ''
  }
})

watch(() => props.initialPipelineId, (id) => {
  if (visible.value && id) {
    selectedPipelineId.value = id
    applyPreferredPipeline()
  }
})
</script>

<style scoped>
.minimal-chat-body {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  gap: 12px;
}

.pipeline-panel {
  flex-shrink: 0;
  padding: 12px 14px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 12px;
  background:
    linear-gradient(135deg, color-mix(in srgb, var(--el-color-primary) 6%, transparent), transparent 55%),
    var(--el-bg-color);
}

.pipeline-row {
  display: flex;
  align-items: center;
  gap: 10px;
}

.pipeline-row-select {
  width: 100%;
  gap: 12px;
}

.pipeline-mode-toggle {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.pipeline-row-meta {
  margin-top: 10px;
  flex-wrap: wrap;
  row-gap: 6px;
}

.pipeline-label {
  font-weight: 600;
  font-size: 13px;
  color: var(--el-text-color-primary);
  white-space: nowrap;
  flex-shrink: 0;
}

.pipeline-select {
  flex: 1;
  min-width: 0;
  width: 100%;
}

.pipeline-select :deep(.el-select__wrapper) {
  width: 100%;
}

.pipeline-desc {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  line-height: 1.4;
  flex: 1;
  min-width: 160px;
}

.pipeline-option {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 2px 0;
}

.pipeline-option-name {
  font-weight: 500;
}

.pipeline-option-id {
  font-size: 11px;
  color: var(--el-text-color-secondary);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.chat-container {
  flex: 1;
  display: flex;
  flex-direction: column;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 12px;
  overflow: hidden;
  background: var(--el-bg-color);
  min-height: 0;
}

.chat-messages {
  flex: 1;
  overflow-y: auto;
  padding: 14px;
  background:
    radial-gradient(ellipse at top right, color-mix(in srgb, var(--el-color-primary) 5%, transparent), transparent 45%),
    var(--el-bg-color);
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: var(--el-text-color-secondary);
  gap: 8px;
  text-align: center;
  padding: 24px;
}

.empty-sub {
  font-size: 12px;
  max-width: 320px;
  opacity: 0.85;
}

.message {
  display: flex;
  gap: 8px;
  margin-bottom: 14px;
}

.message.user {
  flex-direction: row-reverse;
}

.message-stack {
  max-width: 92%;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.message.user .message-stack {
  align-items: flex-end;
}

.message-avatar {
  width: 28px;
  height: 28px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.message.user .message-avatar {
  background: var(--el-color-primary);
  color: white;
}

.message.assistant .message-avatar {
  background: var(--el-fill-color-light);
  color: var(--el-text-color-regular);
}

.message-content {
  padding: 8px 12px;
  border-radius: 10px;
  line-height: 1.5;
  font-size: 13px;
}

.message.user .message-content {
  background: var(--el-color-primary);
  color: white;
  border-top-right-radius: 4px;
}

.message.assistant .message-content {
  background: var(--el-fill-color-light);
  color: var(--el-text-color-primary);
  border-top-left-radius: 4px;
}

.message-error {
  margin-top: 4px;
  color: var(--el-color-danger);
  font-size: 12px;
  display: flex;
  align-items: flex-start;
  gap: 4px;
  word-break: break-word;
}

.trace-card {
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 10px;
  padding: 10px 12px;
  background: color-mix(in srgb, var(--el-bg-color) 88%, var(--el-fill-color));
}

.flow-strip {
  display: flex;
  align-items: flex-start;
  gap: 4px;
  overflow-x: auto;
  padding-bottom: 4px;
}

.flow-step {
  min-width: 72px;
  max-width: 110px;
  text-align: center;
  flex-shrink: 0;
}

.flow-dot {
  width: 22px;
  height: 22px;
  margin: 0 auto 4px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--el-fill-color);
  color: var(--el-text-color-secondary);
  border: 1.5px solid var(--el-border-color);
}

.flow-step.status-ok .flow-dot {
  background: color-mix(in srgb, var(--el-color-success) 18%, white);
  color: var(--el-color-success);
  border-color: var(--el-color-success);
}

.flow-step.status-fail .flow-dot {
  background: color-mix(in srgb, var(--el-color-danger) 16%, white);
  color: var(--el-color-danger);
  border-color: var(--el-color-danger);
}

.flow-step.status-skip .flow-dot {
  opacity: 0.55;
}

.flow-step.is-fallback .flow-dot {
  border-style: dashed;
  color: var(--el-color-warning);
  border-color: var(--el-color-warning);
}

.flow-dot-inner {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
}

.flow-label {
  font-size: 11px;
  font-weight: 600;
  line-height: 1.25;
  color: var(--el-text-color-primary);
  word-break: break-word;
}

.flow-status-text {
  margin-top: 3px;
  font-size: 10px;
  font-weight: 700;
  line-height: 1.2;
}

.flow-status-text.text-ok { color: var(--el-color-success); }
.flow-status-text.text-fail { color: var(--el-color-danger); }
.flow-status-text.text-skip { color: var(--el-color-warning-dark-2); }
.flow-status-text.text-info,
.flow-status-text.text-pending { color: var(--el-text-color-secondary); }

.flow-sub {
  font-size: 10px;
  color: var(--el-text-color-secondary);
  margin-top: 2px;
  word-break: break-all;
}

.flow-arrow {
  color: var(--el-text-color-placeholder);
  font-size: 12px;
  padding-top: 4px;
  flex-shrink: 0;
}

.flow-arrow.fail {
  color: var(--el-color-danger);
}

.flow-arrow.skip {
  color: var(--el-color-warning);
  opacity: 0.7;
}

.flow-legend {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin-top: 8px;
  font-size: 11px;
  color: var(--el-text-color-secondary);
}

.flow-legend span {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.flow-legend .lg {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  display: inline-block;
}

.flow-legend .lg.ok { background: var(--el-color-success); }
.flow-legend .lg.skip { background: var(--el-color-warning); }
.flow-legend .lg.fail { background: var(--el-color-danger); }

.flow-step.status-skip {
  opacity: 0.75;
}

.trace-badges {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 10px;
}

.fallback-notice {
  margin: 8px 0 0;
  font-size: 12px;
  color: var(--el-color-warning-dark-2);
  line-height: 1.4;
  white-space: pre-line;
}

.trace-collapse {
  margin-top: 8px;
  border: none;
}

.trace-collapse :deep(.el-collapse-item__header) {
  height: 32px;
  line-height: 32px;
  font-size: 12px;
  background: transparent;
  border: none;
  color: var(--el-color-primary);
}

.trace-collapse :deep(.el-collapse-item__wrap) {
  border: none;
  background: transparent;
}

.trace-collapse :deep(.el-collapse-item__content) {
  padding-bottom: 4px;
}

.detail-grid {
  display: grid;
  gap: 6px;
}

.detail-row {
  display: grid;
  grid-template-columns: 110px 1fr;
  gap: 8px;
  font-size: 12px;
  align-items: start;
}

.detail-key {
  color: var(--el-text-color-secondary);
}

.detail-val {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  word-break: break-all;
  white-space: pre-wrap;
  font-size: 11px;
}

.node-table-wrap {
  margin-top: 10px;
}

.node-table-title {
  font-size: 12px;
  font-weight: 600;
  margin-bottom: 6px;
}

.node-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 11px;
}

.node-table th,
.node-table td {
  border: 1px solid var(--el-border-color-lighter);
  padding: 4px 6px;
  text-align: left;
}

.node-table th {
  background: var(--el-fill-color-light);
}

.node-table .ok { color: var(--el-color-success); }
.node-table .bad { color: var(--el-color-danger); }

.raw-meta {
  margin-top: 8px;
  max-height: 180px;
  overflow: auto;
  font-size: 10px;
  line-height: 1.35;
  background: var(--el-fill-color-lighter);
  border-radius: 6px;
  padding: 8px;
}

.typing-indicator {
  display: flex;
  gap: 4px;
  padding: 4px 0;
}

.typing-indicator span {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--el-text-color-secondary);
  animation: typing 1.4s infinite;
}

.typing-indicator span:nth-child(2) { animation-delay: 0.2s; }
.typing-indicator span:nth-child(3) { animation-delay: 0.4s; }

@keyframes typing {
  0%, 60%, 100% { opacity: 0.3; transform: scale(0.8); }
  30% { opacity: 1; transform: scale(1); }
}

.chat-input {
  display: flex;
  align-items: flex-end;
  gap: 8px;
  padding: 10px 14px;
  border-top: 1px solid var(--el-border-color-lighter);
  background: var(--el-bg-color);
}

.chat-input :deep(.el-textarea__inner) {
  box-shadow: none;
  resize: none;
  border-radius: 8px;
}

.pipeline-row-mode {
  gap: 12px;
  align-items: center;
}
.agent-hint {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}
.agent-trace {
  border-left: 3px solid var(--el-color-primary);
}
.agent-trace-title {
  font-weight: 600;
  margin-bottom: 8px;
  color: var(--el-color-primary);
}
.agent-steps {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.agent-step {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
}
.agent-step-label {
  font-weight: 500;
}
.agent-step-detail {
  color: var(--el-text-color-secondary);
  font-size: 12px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 400px;
}
</style>

<style>
.minimal-chat-drawer.el-drawer.rtl {
  --el-drawer-padding-primary: 16px;
  width: min(1280px, 96vw) !important;
}
.minimal-chat-drawer .el-drawer__body {
  display: flex;
  flex-direction: column;
  height: calc(100% - 55px);
  padding-top: 0;
  overflow: hidden;
}
.minimal-chat-drawer .el-drawer__header {
  margin-bottom: 12px;
}

/* 下拉弹出层加宽，完整显示流水线名称 */
.pipeline-select-dropdown.el-select__popper,
.pipeline-select-dropdown {
  min-width: 480px !important;
}
.pipeline-select-dropdown .el-select-dropdown__item {
  height: auto;
  min-height: 34px;
  line-height: 1.35;
  padding-top: 6px;
  padding-bottom: 6px;
  white-space: normal;
}
</style>
