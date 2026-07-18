<template>
  <el-drawer
    v-model="visible"
    title="AI 对话测试"
    direction="rtl"
    size="50%"
    :close-on-click-modal="true"
    destroy-on-close
    class="minimal-chat-drawer"
    @opened="onOpened"
  >
    <div class="minimal-chat-body">
      <div class="pipeline-bar">
        <span class="pipeline-bar-label">选择流水线</span>
        <el-select
          v-model="selectedPipelineId"
          placeholder="选择流水线"
          size="default"
          style="width: 260px"
          :loading="pipelinesLoading"
        >
          <el-option
            v-for="p in pipelines"
            :key="p.id"
            :label="p.name || p.id"
            :value="p.id"
          >
            <span>{{ p.name || p.id }}</span>
            <span v-if="p.description" class="pipeline-option-desc">{{ p.description }}</span>
          </el-option>
        </el-select>
        <span v-if="selectedPipeline" class="pipeline-hint">{{ selectedPipeline.description }}</span>
      </div>

      <div class="chat-container">
        <div class="chat-messages" ref="messagesContainer">
          <div v-if="messages.length === 0" class="empty-state">
            <el-icon :size="40" color="#c0c4cc"><ChatDotRound /></el-icon>
            <p>选择流水线后发送消息测试后端</p>
          </div>

          <div v-for="(msg, idx) in messages" :key="idx" :class="['message', msg.role]">
            <div class="message-avatar">
              <el-icon v-if="msg.role === 'user'" :size="16"><User /></el-icon>
              <el-icon v-else :size="16"><Monitor /></el-icon>
            </div>
            <div class="message-content">
              <div class="message-text" v-html="renderMarkdown(msg.content)"></div>
              <div v-if="msg.error" class="message-error">
                <el-icon><WarningFilled /></el-icon> {{ msg.error }}
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
            placeholder="输入消息..."
            :disabled="loading"
            @keydown.enter.exact.prevent="sendMessage"
            ref="inputRef"
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
import { Promotion, User, Monitor, ChatDotRound, WarningFilled } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { getPipelines, getPipelineDefaults, type AgentPatternPipeline } from '@/api/pipeline'
import { useAuthStore } from '@/stores/auth'
import api from '@/api'

interface ChatMsg {
  role: 'user' | 'assistant'
  content: string
  error?: string
}

const visible = defineModel<boolean>({ default: false })
const props = withDefaults(defineProps<{
  /** 打开对话框时预选的流水线 ID */
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
/** Cached plaintext API key for test chat when JWT is unavailable. */
const cachedTestAPIKey = ref('')

const selectedPipeline = computed(() =>
  pipelines.value.find(p => p.id === selectedPipelineId.value)
)

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
  return content
    .replace(/\n/g, '<br>')
    .replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>')
    .replace(/\*(.*?)\*/g, '<em>$1</em>')
    .replace(/`(.*?)`/g, '<code>$1</code>')
    .replace(/```([\s\S]*?)```/g, '<pre class="code-block">$1</pre>')
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

/** Prefer admin JWT (accepted by proxy); else first configured API key. */
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
    /* ignore — let request fail with 401 */
  }
  return {}
}

async function sendMessage() {
  const text = inputText.value.trim()
  if (!text || !selectedPipelineId.value || loading.value) return

  messages.value.push({ role: 'user', content: text })
  inputText.value = ''
  loading.value = true
  await scrollToBottom()

  const assistantMsg: ChatMsg = { role: 'assistant', content: '' }
  messages.value.push(assistantMsg)

  try {
    const modelField = `pipeline.${selectedPipelineId.value}`
    const authHeaders = await resolveProxyAuthHeader()

    const response = await fetch('/v1/chat/completions', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
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

    if (!response.ok) {
      const errBody = await response.text()
      throw new Error(`HTTP ${response.status}: ${errBody}`)
    }

    const reader = response.body?.getReader()
    const decoder = new TextDecoder()
    if (!reader) throw new Error('无法读取响应流')

    let buffer = ''
    while (true) {
      const { done, value } = await reader.read()
      if (done) break

      buffer += decoder.decode(value, { stream: true })
      const lines = buffer.split('\n')
      buffer = lines.pop() || ''

      for (const line of lines) {
        const trimmed = line.trim()
        if (!trimmed || trimmed === 'data: [DONE]') continue
        if (!trimmed.startsWith('data: ')) continue

        try {
          const data = JSON.parse(trimmed.slice(6))

          if (data.error && !data.choices) {
            const errMsg = typeof data.error === 'string'
              ? data.error
              : data.error.message || JSON.stringify(data.error)
            assistantMsg.error = errMsg
            loading.value = false
            return
          }

          const delta = data.choices?.[0]?.delta?.content
          if (delta) {
            assistantMsg.content += delta
            await scrollToBottom()
          }
        } catch {
          // 忽略解析失败的行
        }
      }
    }
  } catch (e: any) {
    assistantMsg.error = e.message || '请求失败'
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
    if (props.initialPipelineId) {
      selectedPipelineId.value = props.initialPipelineId
    }
    loadPipelines()
  } else {
    selectedPipelineId.value = ''
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
}

.pipeline-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 14px;
  background: var(--el-bg-color);
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 8px;
  margin-bottom: 12px;
  flex-shrink: 0;
}

.pipeline-bar-label {
  font-weight: 500;
  color: var(--el-text-color-primary);
  white-space: nowrap;
}

.pipeline-hint {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pipeline-option-desc {
  display: block;
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-top: 2px;
}

.chat-container {
  flex: 1;
  display: flex;
  flex-direction: column;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 8px;
  overflow: hidden;
  background: var(--el-bg-color);
}

.chat-messages {
  flex: 1;
  overflow-y: auto;
  padding: 14px;
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: var(--el-text-color-secondary);
  gap: 10px;
}

.message {
  display: flex;
  gap: 8px;
  margin-bottom: 12px;
}

.message.user {
  flex-direction: row-reverse;
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
  max-width: 75%;
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
  align-items: center;
  gap: 4px;
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
</style>

<style>
/* Drawer body fills remaining viewport height so chat can stretch */
.minimal-chat-drawer.el-drawer.rtl {
  --el-drawer-padding-primary: 16px;
}
.minimal-chat-drawer .el-drawer__body {
  display: flex;
  flex-direction: column;
  height: calc(100% - 55px);
  padding-top: 0;
  overflow: hidden;
}
</style>
