<template>
  <transition name="log-panel-slide">
    <div v-if="visible" class="log-panel">
      <div class="log-panel-header">
        <div class="log-panel-title">
          <el-icon :size="14"><Monitor /></el-icon>
          <span>{{ t('liveLogSidebar.title') }}</span>
          <span v-if="liveTail" class="live-dot">●</span>
        </div>
        <div class="log-panel-controls">
          <div class="log-filter-group">
            <label class="log-filter-label">{{ t('liveLogSidebar.categoryLabel') }}</label>
            <select v-model="categoryFilter" class="log-filter-select">
              <option value="">{{ t('liveLogSidebar.categoryAll') }}</option>
              <option value="llm">{{ t('liveLogSidebar.categoryLlm') }}</option>
              <option value="pipeline">{{ t('liveLogSidebar.categoryPipeline') }}</option>
              <option value="system">{{ t('liveLogSidebar.categorySystem') }}</option>
            </select>
          </div>
          <div class="log-filter-group">
            <label class="log-filter-label">{{ t('liveLogSidebar.levelLabel') }}</label>
            <select v-model="levelFilter" class="log-filter-select">
              <option value="">{{ t('liveLogSidebar.levelAll') }}</option>
              <option value="error">Error</option>
              <option value="warn">Warn</option>
              <option value="info">Info</option>
              <option value="debug">Debug</option>
            </select>
          </div>
          <label class="log-autoscroll">
            <input type="checkbox" v-model="autoScroll" /> {{ t('liveLogSidebar.autoScroll') }}
          </label>
        </div>
        <div class="log-panel-actions">
          <button
            type="button"
            class="log-btn"
            :class="{ active: liveTail }"
            @click="toggleLiveTail"
            :title="liveTail ? t('liveLogSidebar.pauseLive') : t('liveLogSidebar.startLive')"
          >
            {{ liveTail ? t('liveLogSidebar.pause') : t('liveLogSidebar.follow') }}
          </button>
          <button
            type="button"
            class="log-btn"
            @click="clearConsole"
            :title="t('liveLogSidebar.clearTitle')"
          >
            {{ t('liveLogSidebar.clear') }}
          </button>
          <button
            type="button"
            class="log-btn log-btn-close"
            @click="$emit('close')"
            :title="t('liveLogSidebar.close')"
          >
            ✕
          </button>
        </div>
      </div>
      <div class="log-panel-meta">
        <span class="log-file-path" :title="logFilePath">{{ logFilePath || '...' }}</span>
        <span class="log-stats">{{ t('liveLogSidebar.linesCount', { count: lineCount, bytes: formatBytes(totalBytes) }) }}</span>
      </div>
      <div ref="consoleEl" class="log-console" @scroll="onScroll">
        <pre class="log-content"><code>{{ displayContent }}</code></pre>
        <div v-if="!liveTail && pausedHint" class="paused-hint">
          {{ t('liveLogSidebar.paused') }}<a href="#" @click.prevent="resumeLiveTail">{{ t('liveLogSidebar.resume') }}</a>
        </div>
      </div>
    </div>
  </transition>
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, nextTick, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Monitor } from '@element-plus/icons-vue'
import { useAuthStore } from '@/stores/auth'

const { t } = useI18n()

const props = defineProps<{ visible: boolean }>()
const emit = defineEmits<{ (e: 'close'): void }>()

const consoleEl = ref<HTMLElement | null>(null)
const displayContent = ref('')
const logFilePath = ref('')
const liveTail = ref(true)
const autoScroll = ref(true)
const levelFilter = ref('')
const categoryFilter = ref('')
const lineCount = ref(0)
const totalBytes = ref(0)
const pausedHint = ref(false)
const rawBuffer = ref('')

const authStore = useAuthStore()

let pollTimer: ReturnType<typeof setInterval> | null = null
let currentOffset = 0
const MAX_BUFFER_LINES = 5000

const filteredContent = computed(() => {
  const buffer = rawBuffer.value
  if (!levelFilter.value && !categoryFilter.value) return buffer

  const level = levelFilter.value.toLowerCase()
  const category = categoryFilter.value.toLowerCase()
  const lines = buffer.split('\n')

  const filtered = lines.filter((line) => {
    const lower = line.toLowerCase()

    if (level && !(lower.includes(`"level":"${level}"`) || lower.includes(`level=${level}`))) {
      return false
    }

    if (category) {
      if (category === 'llm') {
        const llmMarkers = [
          'chat completions', 'request started', '[request]', '[config] proxy mode',
          '/v1/chat', '/v1/messages', '/v1/completions', '/v1/embeddings',
          'transparent proxy', 'cache hit mode', 'cache mode', 'proxy auth rejected',
          'requested_model', 'selected_backend',
          '"model":', '"backend":'
        ]
        const isLLM = llmMarkers.some(marker => lower.includes(marker))
        if (!isLLM) return false
      } else if (category === 'pipeline') {
        const pipelineMarkers = [
          'pipeline', 'node', 'execution', 'resolved pipeline'
        ]
        const isPipeline = pipelineMarkers.some(marker => lower.includes(marker))
        if (!isPipeline) return false
      } else if (category === 'system') {
        const llmMarkers = [
          'chat completions', 'request started', '[request]', '[config] proxy mode',
          '/v1/chat', '/v1/messages', '/v1/completions', '/v1/embeddings',
          'transparent proxy', 'cache hit mode', 'cache mode', 'proxy auth rejected',
          'requested_model', 'selected_backend',
          '"model":', '"backend":'
        ]
        const pipelineMarkers = [
          'pipeline', 'node', 'execution', 'resolved pipeline'
        ]
        const isLLM = llmMarkers.some(marker => lower.includes(marker))
        const isPipeline = pipelineMarkers.some(marker => lower.includes(marker))
        if (isLLM || isPipeline) return false
      }
    }

    return true
  })

  return filtered.join('\n')
})

watch(filteredContent, () => {
  displayContent.value = filteredContent.value
  nextTick(() => {
    if (autoScroll.value && liveTail.value) {
      scrollToBottom()
    }
  })
})

function scrollToBottom() {
  if (consoleEl.value) {
    consoleEl.value.scrollTop = consoleEl.value.scrollHeight
  }
}

function onScroll() {
  if (!consoleEl.value) return
  const el = consoleEl.value
  const atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 40
  if (!atBottom && liveTail.value) {
    liveTail.value = false
    pausedHint.value = true
  }
}

function resumeLiveTail() {
  liveTail.value = true
  pausedHint.value = false
  autoScroll.value = true
  nextTick(scrollToBottom)
}

function toggleLiveTail() {
  liveTail.value = !liveTail.value
  if (liveTail.value) {
    pausedHint.value = false
    nextTick(scrollToBottom)
  }
}

function appendContent(chunk: string) {
  const nextBuffer = rawBuffer.value + chunk
  const lines = nextBuffer.split('\n')
  if (lines.length > MAX_BUFFER_LINES) {
    rawBuffer.value = lines.slice(lines.length - MAX_BUFFER_LINES).join('\n')
  } else {
    rawBuffer.value = nextBuffer
  }
  totalBytes.value += chunk.length
  lineCount.value = rawBuffer.value.split('\n').length
}

async function pollLogs() {
  try {
    const params = new URLSearchParams()
    params.set('offset', String(currentOffset))
    params.set('tail', currentOffset === 0 ? 'true' : 'false')
    const resp = await fetch(`/api/v1/logs/tail?${params.toString()}`, {
      headers: { 'Authorization': `Bearer ${getAccessToken()}` }
    })
    if (resp.ok) {
      const payload = await resp.json()
      const chunk = payload?.data ?? payload
      const content = chunk?.content ?? ''
      if (content) {
        appendContent(content)
      }
      currentOffset = chunk?.offset ?? currentOffset
      if (chunk?.log_path && !logFilePath.value) {
        logFilePath.value = chunk.log_path
      }
    } else if (resp.status === 401) {
      console.warn('[LiveLogSidebar] 401 Unauthorized, stop polling')
      stopPolling()
    }
  } catch (e) {
    console.warn('[LiveLogSidebar] HTTP tail failed:', e)
  }
}

function getAccessToken(): string {
  return authStore.accessToken || ''
}

async function loadFilePath() {
  try {
    const resp = await fetch(`/api/v1/logs/tail?offset=0&tail=true&limit=1`, {
      headers: { 'Authorization': `Bearer ${getAccessToken()}` }
    })
    if (resp.ok) {
      const payload = await resp.json()
      const chunk = payload?.data ?? payload
      if (chunk?.log_path) {
        logFilePath.value = chunk.log_path
      }
    }
  } catch {
  }
}

async function clearConsole() {
  rawBuffer.value = ''
  displayContent.value = ''
  currentOffset = 0
  lineCount.value = 0
  totalBytes.value = 0
}

function startPolling() {
  if (pollTimer) return
  pollTimer = setInterval(pollLogs, 800)
}

function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

watch(
  () => props.visible,
  (v) => {
    if (v) {
      loadFilePath()
      startPolling()
    } else {
      stopPolling()
    }
  }
)

watch(liveTail, (v) => {
  if (v) startPolling()
})

onMounted(() => {
  if (props.visible) {
    loadFilePath()
    startPolling()
  }
})

onBeforeUnmount(() => {
  stopPolling()
})

function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
  return `${(n / 1024 / 1024).toFixed(2)} MB`
}
</script>

<style scoped>
.log-panel {
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 300px;
  min-height: 200px;
  max-height: 50vh;
  background: var(--desktop-sidebar-bg, #1e1e2e);
  border-top: 1px solid var(--desktop-sidebar-border, #313244);
  color: #cdd6f4;
  font-family: 'Menlo', 'Consolas', 'Courier New', monospace;
  font-size: 12px;
  overflow: hidden;
}

.log-panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  background: #181825;
  border-bottom: 1px solid #313244;
  flex-shrink: 0;
  gap: 12px;
}

.log-panel-title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-weight: 600;
  color: #cdd6f4;
  flex-shrink: 0;
}

.log-panel-controls {
  display: flex;
  align-items: center;
  gap: 12px;
  flex: 1;
  justify-content: center;
}

.log-filter-group {
  display: flex;
  align-items: center;
  gap: 4px;
}

.log-filter-label {
  color: #a6adc8;
  font-size: 11px;
}

.log-filter-select {
  background: #313244;
  color: #cdd6f4;
  border: 1px solid #45475a;
  border-radius: 3px;
  padding: 2px 6px;
  font-size: 11px;
}

.log-panel-actions {
  display: flex;
  gap: 4px;
  flex-shrink: 0;
}

.live-dot {
  color: #a6e3a1;
  animation: blink 1.2s infinite;
}

@keyframes blink {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.3; }
}

.log-panel-actions {
  display: flex;
  gap: 4px;
}

.log-btn {
  background: transparent;
  border: 1px solid #45475a;
  color: #cdd6f4;
  padding: 2px 8px;
  border-radius: 4px;
  cursor: pointer;
  font-size: 11px;
  transition: all 0.15s;
}

.log-btn:hover {
  background: #313244;
  border-color: #585b70;
}

.log-btn.active {
  background: #a6e3a1;
  color: #1e1e2e;
  border-color: #a6e3a1;
}

.log-btn-close:hover {
  background: #f38ba8;
  color: #1e1e2e;
  border-color: #f38ba8;
}

.log-panel-meta {
  display: flex;
  justify-content: space-between;
  padding: 4px 12px;
  background: #181825;
  border-bottom: 1px solid #313244;
  font-size: 10px;
  color: #6c7086;
  flex-shrink: 0;
}

.log-file-path {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 280px;
}

.log-console {
  flex: 1;
  overflow-y: auto;
  overflow-x: auto;
  padding: 8px 12px;
  background: #1e1e2e;
  position: relative;
  scroll-behavior: auto;
}

.log-console::-webkit-scrollbar {
  width: 8px;
  height: 8px;
}

.log-console::-webkit-scrollbar-track {
  background: #181825;
}

.log-console::-webkit-scrollbar-thumb {
  background: #45475a;
  border-radius: 4px;
}

.log-console::-webkit-scrollbar-thumb:hover {
  background: #585b70;
}

.log-content {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-all;
  line-height: 1.5;
  color: #cdd6f4;
}

.log-content code {
  font-family: inherit;
}

.paused-hint {
  position: sticky;
  bottom: 8px;
  left: 50%;
  transform: translateX(-50%);
  background: #f9e2af;
  color: #1e1e2e;
  padding: 4px 12px;
  border-radius: 12px;
  font-size: 11px;
  display: inline-block;
  width: fit-content;
  margin: 0 auto;
}

.paused-hint a {
  color: #1e1e2e;
  font-weight: 600;
  text-decoration: underline;
}

.log-autoscroll {
  display: flex;
  align-items: center;
  gap: 4px;
  cursor: pointer;
  color: #a6adc8;
  font-size: 11px;
}

.log-panel-slide-enter-active,
.log-panel-slide-leave-active {
  transition: transform 0.25s ease, opacity 0.25s ease;
}

.log-panel-slide-enter-from,
.log-panel-slide-leave-to {
  transform: translateY(100%);
  opacity: 0;
}
</style>
