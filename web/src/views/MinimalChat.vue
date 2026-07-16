<template>
  <div class="minimal-chat">
    <!-- 顶部：流水线选择 -->
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

    <!-- 对话区 -->
    <div class="chat-container">
      <div class="chat-messages" ref="messagesContainer">
        <!-- 空态 -->
        <div v-if="messages.length === 0" class="empty-state">
          <el-icon :size="48" color="#c0c4cc"><ChatDotRound /></el-icon>
          <p>选择流水线后开始对话</p>
        </div>

        <!-- 消息列表 -->
        <div v-for="(msg, idx) in messages" :key="idx" :class="['message', msg.role]">
          <div class="message-avatar">
            <el-icon v-if="msg.role === 'user'" :size="18"><User /></el-icon>
            <el-icon v-else :size="18"><Monitor /></el-icon>
          </div>
          <div class="message-content">
            <div class="message-text" v-html="renderMarkdown(msg.content)"></div>
            <div v-if="msg.error" class="message-error">
              <el-icon><WarningFilled /></el-icon> {{ msg.error }}
            </div>
          </div>
        </div>

        <!-- 加载中 -->
        <div v-if="loading" class="message assistant">
          <div class="message-avatar"><el-icon :size="18"><Monitor /></el-icon></div>
          <div class="message-content">
            <div class="typing-indicator">
              <span></span><span></span><span></span>
            </div>
          </div>
        </div>
      </div>

      <!-- 输入区 -->
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
</template>

<script setup lang="ts">
import { ref, computed, nextTick, onMounted } from 'vue'
import { Promotion, User, Monitor, ChatDotRound, WarningFilled } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { getPipelines, type AgentPatternPipeline } from '@/api/pipeline'
interface ChatMsg {
  role: 'user' | 'assistant'
  content: string
  error?: string
}

const pipelines = ref<AgentPatternPipeline[]>([])
const pipelinesLoading = ref(false)
const selectedPipelineId = ref('')
const messages = ref<ChatMsg[]>([])
const inputText = ref('')
const loading = ref(false)
const messagesContainer = ref<HTMLElement>()

const selectedPipeline = computed(() =>
  pipelines.value.find(p => p.id === selectedPipelineId.value)
)

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
    if (pipelines.value.length > 0 && !selectedPipelineId.value) {
      selectedPipelineId.value = pipelines.value[0].id
    }
  } catch (e: any) {
    console.error('Failed to load pipelines:', e)
  } finally {
    pipelinesLoading.value = false
  }
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
    // 使用 pipeline ID 作为 model 字段，后端会路由到对应流水线
    const modelField = `pipeline.${selectedPipelineId.value}`

    const response = await fetch('/v1/chat/completions', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        model: modelField,
        messages: messages.value.slice(0, -2).map(m => ({
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

          // 后端错误
          if (data.error && !data.choices) {
            const errMsg = typeof data.error === 'string'
              ? data.error
              : data.error.message || JSON.stringify(data.error)
            assistantMsg.error = errMsg
            loading.value = false
            return
          }

          // 流式内容
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

onMounted(() => {
  loadPipelines()
})
</script>

<style scoped>
.minimal-chat {
  display: flex;
  flex-direction: column;
  height: calc(100vh - 120px);
  max-width: 900px;
  margin: 0 auto;
  padding: 16px;
}

.pipeline-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
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
  padding: 16px;
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: var(--el-text-color-secondary);
  gap: 12px;
}

.message {
  display: flex;
  gap: 10px;
  margin-bottom: 16px;
}

.message.user {
  flex-direction: row-reverse;
}

.message-avatar {
  width: 32px;
  height: 32px;
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
  padding: 10px 14px;
  border-radius: 12px;
  line-height: 1.6;
  font-size: 14px;
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
  margin-top: 6px;
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
  padding: 12px 16px;
  border-top: 1px solid var(--el-border-color-lighter);
  background: var(--el-bg-color);
}

.chat-input :deep(.el-textarea__inner) {
  box-shadow: none;
  resize: none;
  border-radius: 8px;
}
</style>
