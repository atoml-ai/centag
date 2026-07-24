<template>
  <div class="request-trace">
    <div class="trace-header">
      <div class="trace-header-main">
        <h2>{{ t('requestTrace.title') }}</h2>
        <p class="trace-subtitle">{{ t('requestTrace.subtitle') }}</p>
      </div>
      <div class="header-actions">
        <button type="button" class="btn btn-secondary" @click="router.push('/logs')">{{ t('requestTrace.backToLogs') }}</button>
        <button
          v-if="trace"
          type="button"
          class="btn btn-success"
          @click="exportTrace"
        >
          {{ t('requestTrace.exportJson') }}
        </button>
      </div>
    </div>

    <p v-if="loading" class="trace-loading">{{ t('requestTrace.loadingTrace') }}</p>
    <p v-if="error" class="trace-error" role="alert">{{ error }}</p>

    <template v-if="trace">
      <div class="trace-id-bar">
        <code class="trace-id">{{ trace.request_id }}</code>
        <button type="button" class="btn btn-secondary btn-sm" @click="copyRequestId">{{ t('requestTrace.copyId') }}</button>
        <span class="trace-meta">{{ t('requestTrace.relatedLogs', { n: trace.raw_log_count }) }}</span>
      </div>

      <div class="summary-grid">
        <div class="summary-card" :class="{ success: trace.summary.success, error: !trace.summary.success }">
          <div class="summary-label">{{ t('requestTrace.status') }}</div>
          <div class="summary-value">
            <span v-if="trace.summary.status_code">{{ trace.summary.status_code }}</span>
            <span v-else>{{ trace.summary.success ? t('requestTrace.success') : t('requestTrace.failed') }}</span>
          </div>
        </div>
        <div class="summary-card">
          <div class="summary-label">{{ t('requestTrace.totalDuration') }}</div>
          <div class="summary-value">{{ trace.summary.duration_ms ? trace.summary.duration_ms + 'ms' : '—' }}</div>
        </div>
        <div class="summary-card">
          <div class="summary-label">{{ t('requestTrace.proxyMode') }}</div>
          <div class="summary-value">{{ trace.summary.proxy_mode || '—' }}</div>
        </div>
        <div class="summary-card">
          <div class="summary-label">{{ t('requestTrace.pipeline') }}</div>
          <div class="summary-value">{{ trace.summary.pipeline_id || '—' }}</div>
        </div>
        <div class="summary-card">
          <div class="summary-label">{{ t('requestTrace.backendModel') }}</div>
          <div class="summary-value">
            {{ [trace.summary.backend_id, trace.summary.model].filter(Boolean).join(' / ') || '—' }}
          </div>
        </div>
        <div class="summary-card">
          <div class="summary-label">{{ t('requestTrace.path') }}</div>
          <div class="summary-value mono">
            {{ trace.summary.method ? trace.summary.method + ' ' : '' }}{{ trace.summary.path || '—' }}
          </div>
        </div>
      </div>

      <div v-if="hasRouting" class="routing-panel">
        <h3>{{ t('requestTrace.routingDecision') }}</h3>
        <div class="routing-grid">
          <div v-if="trace.routing.detected_mode">
            <span class="routing-key">{{ t('requestTrace.detectedMode') }}</span>
            <span>{{ trace.routing.detected_mode }}</span>
            <span v-if="trace.routing.source" class="muted">（{{ trace.routing.source }}）</span>
          </div>
          <div v-if="trace.routing.resolved_mode">
            <span class="routing-key">{{ t('requestTrace.resolvedMode') }}</span>
            <span>{{ trace.routing.resolved_mode }}</span>
            <span v-if="trace.routing.resolved_source" class="muted">（{{ trace.routing.resolved_source }}）</span>
          </div>
        </div>
      </div>

      <div class="trace-body">
        <section class="timeline-panel">
          <h3>{{ t('requestTrace.timeline') }}</h3>
          <ol class="timeline">
            <li
              v-for="(event, index) in trace.timeline"
              :key="index"
              class="timeline-item"
              :class="'phase-' + event.phase"
            >
              <div class="timeline-dot" />
              <div class="timeline-content">
                <div class="timeline-head">
                  <span class="timeline-time">{{ formatTime(event.ts) }}</span>
                  <span class="timeline-label">{{ event.label }}</span>
                  <span v-if="event.duration_ms" class="timeline-duration">{{ event.duration_ms }}ms</span>
                  <span v-if="event.status_code" class="timeline-status">{{ event.status_code }}</span>
                </div>
                <p v-if="event.detail" class="timeline-detail">{{ event.detail }}</p>
                <p v-if="event.backend || event.model" class="timeline-meta">
                  {{ [event.backend, event.model].filter(Boolean).join(' · ') }}
                </p>
              </div>
            </li>
          </ol>
        </section>

        <aside v-if="hasPipelineGraph" class="pipeline-panel">
          <h3>{{ t('requestTrace.pipelineExecution') }}</h3>
          <div class="pipeline-id">{{ trace.pipeline_graph.pipeline_id || trace.summary.pipeline_id }}</div>
          <ul v-if="trace.pipeline_graph.executed_nodes?.length" class="node-list">
            <li v-for="node in trace.pipeline_graph.executed_nodes" :key="node" class="node-item executed">
              {{ node }}
            </li>
          </ul>
          <p v-if="trace.pipeline_graph.total_nodes" class="pipeline-meta">
            {{ t('requestTrace.totalNodes', { n: trace.pipeline_graph.total_nodes }) }}
            <span v-if="trace.pipeline_graph.total_tokens"> · {{ t('requestTrace.tokens', { n: trace.pipeline_graph.total_tokens }) }}</span>
          </p>
        </aside>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { tracesApi, type TraceResult } from '../api'
import { saveBlobAsFile } from '@/utils/downloadFile'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()

const trace = ref<TraceResult | null>(null)
const loading = ref(false)
const error = ref('')

const hasRouting = computed(() => {
  const r = trace.value?.routing
  return !!(r?.detected_mode || r?.resolved_mode)
})

const hasPipelineGraph = computed(() => {
  const g = trace.value?.pipeline_graph
  return !!(g?.pipeline_id || g?.executed_nodes?.length)
})

function syncFromRoute() {
  const id = String(route.params.requestId || '').trim()
  if (!id) {
    router.replace('/logs')
    return
  }
  loadTrace(id)
}

async function loadTrace(id: string) {
  loading.value = true
  error.value = ''
  trace.value = null
  try {
    trace.value = await tracesApi.getTrace(id)
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : t('requestTrace.loadTraceFailed')
    error.value = msg
  } finally {
    loading.value = false
  }
}

function formatTime(ts: string) {
  if (!ts) return '—'
  try {
    return new Date(ts).toLocaleTimeString('zh-CN', {
      hour12: false,
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      fractionalSecondDigits: 3
    } as Intl.DateTimeFormatOptions)
  } catch {
    return ts
  }
}

async function copyRequestId() {
  if (!trace.value?.request_id) return
  try {
    await navigator.clipboard.writeText(trace.value.request_id)
  } catch {
    /* ignore */
  }
}

function exportTrace() {
  if (!trace.value) return
  const blob = new Blob([JSON.stringify(trace.value, null, 2)], { type: 'application/json' })
  saveBlobAsFile(blob, `trace-${trace.value.request_id}.json`)
}

watch(() => route.params.requestId, syncFromRoute, { immediate: true })
</script>

<style scoped>
.request-trace {
  display: flex;
  flex-direction: column;
  min-height: 0;
  height: 100%;
}

.trace-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}

.trace-header h2 {
  margin: 0 0 4px;
  font-size: 1.35rem;
}

.trace-subtitle {
  margin: 0;
  font-size: 13px;
  color: #64748b;
}

.header-actions {
  display: flex;
  gap: 8px;
}

.trace-loading {
  margin: 0 0 16px;
  font-size: 14px;
  color: #64748b;
}

.trace-error {
  margin: 0 0 16px;
  padding: 12px 14px;
  border-radius: 8px;
  background: #fef2f2;
  border: 1px solid #fecaca;
  color: #b91c1c;
}

.trace-id-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}

.trace-id {
  font-size: 13px;
  background: #f1f5f9;
  padding: 6px 10px;
  border-radius: 6px;
  word-break: break-all;
}

.trace-meta {
  font-size: 12px;
  color: #64748b;
}

.summary-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
  gap: 12px;
  margin-bottom: 16px;
}

.summary-card {
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  padding: 12px;
}

.summary-card.success {
  background: #f0fdf4;
  border-color: #bbf7d0;
}

.summary-card.error {
  background: #fef2f2;
  border-color: #fecaca;
}

.summary-label {
  font-size: 12px;
  color: #64748b;
  margin-bottom: 4px;
}

.summary-value {
  font-size: 15px;
  font-weight: 600;
  word-break: break-word;
}

.summary-value.mono {
  font-family: ui-monospace, monospace;
  font-size: 13px;
  font-weight: 500;
}

.routing-panel {
  margin-bottom: 16px;
  padding: 14px 16px;
  border-radius: 8px;
  border: 1px solid #e2e8f0;
  background: #fff;
}

.routing-panel h3,
.timeline-panel h3,
.pipeline-panel h3 {
  margin: 0 0 12px;
  font-size: 15px;
}

.routing-grid {
  display: flex;
  flex-direction: column;
  gap: 8px;
  font-size: 14px;
}

.routing-key {
  display: inline-block;
  min-width: 72px;
  color: #64748b;
  margin-right: 8px;
}

.muted {
  color: #94a3b8;
  font-size: 13px;
}

.trace-body {
  display: grid;
  grid-template-columns: 1fr 260px;
  gap: 16px;
  min-height: 0;
  flex: 1;
}

@media (max-width: 900px) {
  .trace-body {
    grid-template-columns: 1fr;
  }
}

.timeline-panel,
.pipeline-panel {
  background: #fff;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  padding: 16px;
  min-height: 0;
  overflow: auto;
}

.timeline {
  list-style: none;
  margin: 0;
  padding: 0;
}

.timeline-item {
  display: flex;
  gap: 12px;
  padding-bottom: 16px;
  position: relative;
}

.timeline-item:not(:last-child)::before {
  content: '';
  position: absolute;
  left: 5px;
  top: 14px;
  bottom: 0;
  width: 2px;
  background: #e2e8f0;
}

.timeline-dot {
  width: 12px;
  height: 12px;
  border-radius: 50%;
  background: var(--shell-accent, #4070ff);
  flex-shrink: 0;
  margin-top: 4px;
  z-index: 1;
}

.phase-complete .timeline-dot { background: #16a34a; }
.phase-error .timeline-dot { background: #dc2626; }
.phase-routing .timeline-dot { background: #8b5cf6; }
.phase-pipeline .timeline-dot { background: #0ea5e9; }
.phase-backend .timeline-dot { background: #f59e0b; }

.timeline-head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;
}

.timeline-time {
  font-family: ui-monospace, monospace;
  font-size: 12px;
  color: #64748b;
}

.timeline-label {
  font-weight: 600;
  font-size: 14px;
}

.timeline-duration,
.timeline-status {
  font-size: 12px;
  padding: 2px 6px;
  border-radius: 4px;
  background: #f1f5f9;
}

.timeline-detail,
.timeline-meta {
  margin: 0;
  font-size: 13px;
  color: #475569;
  line-height: 1.45;
  word-break: break-word;
}

.pipeline-id {
  font-weight: 600;
  margin-bottom: 10px;
}

.node-list {
  list-style: none;
  margin: 0 0 10px;
  padding: 0;
}

.node-item {
  padding: 8px 10px;
  border-radius: 6px;
  background: #f0fdf4;
  border: 1px solid #bbf7d0;
  font-size: 13px;
  margin-bottom: 6px;
}

.pipeline-meta {
  margin: 0;
  font-size: 12px;
  color: #64748b;
}

.btn {
  padding: 8px 14px;
  border-radius: 8px;
  border: 1px solid transparent;
  font-size: 14px;
  cursor: pointer;
}

.btn-sm {
  padding: 4px 10px;
  font-size: 12px;
}

.btn-primary {
  background: var(--shell-accent, #4070ff);
  color: #fff;
}

.btn-secondary {
  background: #f8fafc;
  border-color: #e2e8f0;
  color: #334155;
}

.btn-success {
  background: #16a34a;
  color: #fff;
}

.btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}
</style>
