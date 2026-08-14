<template>
  <div class="chat-area">
    <!-- 头部 -->
    <div class="chat-header">
      <div class="session-info">
        <el-icon class="header-icon" :size="18"><ChatDotRound /></el-icon>
        <span class="session-title">{{ session?.title || 'Agent 助手' }}</span>
        <el-tag v-if="session?.skill" size="small" type="primary" effect="plain">
          {{ skillLabel(session.skill) }}
        </el-tag>
      </div>
      <el-button v-if="isExecuting" size="small" type="danger" plain :icon="CircleClose" @click="$emit('cancel-execution')">
        取消
      </el-button>
    </div>

    <!-- 消息区 -->
    <div class="chat-messages" ref="messagesContainer">
      <div class="empty-hint" v-if="!messages.length">
        <el-icon class="empty-icon" :size="56"><ChatDotRound /></el-icon>
        <h3>向 Agent 提问</h3>
        <p>选择问题类型后可输入 @问题类型 快速切换，例如：@检查状态 帮我看看后端健康情况</p>
      </div>

      <div
        v-for="message in messages"
        :key="message.id"
        class="message"
        :class="message.role"
      >
        <div class="message-avatar" :class="message.role">
          <el-icon :size="16">
            <User v-if="message.role === 'user'" />
            <MagicStick v-else />
          </el-icon>
        </div>
        <div class="message-body">
          <div class="message-meta">
            <span class="message-role-label">{{ message.role === 'user' ? '你' : 'Agent' }}</span>
            <span class="message-time">{{ formatTime(message.created_at) }}</span>
          </div>
          <pre class="message-text">{{ message.content }}</pre>
          <el-tag v-if="message.tool_name" size="small" type="warning" class="tool-badge">
            工具调用: {{ message.tool_name }}
          </el-tag>
        </div>
      </div>

      <!-- 等待回复 -->
      <div v-if="isResponding" class="message assistant">
        <div class="message-avatar assistant">
          <el-icon :size="16"><MagicStick /></el-icon>
        </div>
        <div class="message-body">
          <div class="message-meta">
            <span class="message-role-label">Agent</span>
          </div>
          <div class="message-text thinking">
            <span class="thinking-dot"></span>
            <span class="thinking-dot"></span>
            <span class="thinking-dot"></span>
            正在思考...
          </div>
        </div>
      </div>
    </div>

    <!-- 输入区 -->
    <div class="chat-input">
      <div class="input-wrapper">
        <el-input
          v-model="inputText"
          type="textarea"
          :rows="2"
          :autosize="{ minRows: 2, maxRows: 6 }"
          placeholder="请输入你的问题，@问题类型 可快速切换，Enter 发送，Shift+Enter 换行"
          :disabled="isExecuting"
          resize="none"
          @keydown.enter.exact.prevent="handleSend"
        />
        <el-button
          type="primary"
          class="send-btn"
          :icon="Promotion"
          :loading="isExecuting"
          :disabled="!canSend"
          @click="handleSend"
        >
          发送
        </el-button>
      </div>

      <div class="input-toolbar">
        <div class="toolbar-group">
          <span class="toolbar-label">
            <el-icon :size="13"><MagicStick /></el-icon>
            问题类型
          </span>
          <el-select
            v-model="selectedSkill"
            size="small"
            placeholder="智能识别"
            clearable
            :disabled="isExecuting"
            class="toolbar-select"
            @change="onSkillChange"
          >
            <el-option label="智能识别" value="" />
            <el-option
              v-for="skill in skills"
              :key="skill.name"
              :label="skillLabel(skill.name)"
              :value="skill.name"
            />
          </el-select>
        </div>

        <div class="toolbar-group">
          <span class="toolbar-label">
            <el-icon :size="13"><Connection /></el-icon>
            后端
          </span>
          <el-select
            v-model="selectedBackend"
            size="small"
            placeholder="系统默认"
            clearable
            :disabled="isExecuting"
            class="toolbar-select"
            @change="onBackendChange"
          >
            <el-option label="系统默认" value="" />
            <el-option
              v-for="be in backends"
              :key="be.id"
              :label="be.name"
              :value="be.id"
            />
          </el-select>
        </div>

        <div class="toolbar-group">
          <span class="toolbar-label">
            <el-icon :size="13"><Cpu /></el-icon>
            模型
          </span>
          <el-select
            v-model="selectedModel"
            size="small"
            placeholder="系统默认"
            clearable
            :disabled="isExecuting"
            class="toolbar-select model-select"
          >
            <el-option label="系统默认" value="" />
            <el-option
              v-for="m in modelOptions"
              :key="m"
              :label="m"
              :value="m"
            />
          </el-select>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, nextTick, computed } from 'vue'
import {
  ChatDotRound,
  MagicStick,
  Promotion,
  CircleClose,
  User,
  Connection,
  Cpu
} from '@element-plus/icons-vue'
import { getBackends, getBackendModels } from '@/api/backend'

interface Message {
  id: string
  role: string
  content: string
  skill?: string
  tool_name?: string
  created_at: string
}

interface Session {
  id: string
  title: string
  skill: string
  status: string
}

interface Skill {
  name: string
  description: string
  category: string
}

interface BackendOption {
  id: string
  name: string
}

const props = defineProps<{
  session: Session | null
  messages: Message[]
  skills: Skill[]
  currentSkill: string | null
  isResponding?: boolean
}>()

const emit = defineEmits<{
  'send-message': [content: string, skill?: string, backend?: string, model?: string]
  'cancel-execution': []
}>()

const inputText = ref('')
const selectedSkill = ref(props.currentSkill || '')
const selectedBackend = ref('')
const selectedModel = ref('')
const backends = ref<BackendOption[]>([])
const modelOptions = ref<string[]>([])
const isExecuting = ref(props.isResponding || false)
const messagesContainer = ref<HTMLElement | null>(null)

// 发送时实际内容：去掉 @skill 前缀后；无内容但有选中 skill 时用默认指令
const canSend = computed(() => {
  return isExecuting.value
    ? false
    : !!effectiveContent.value || !!selectedSkill.value || !!parseSkillPrefix(inputText.value).skill
})

const effectiveContent = computed(() => {
  const parsed = parseSkillPrefix(inputText.value)
  return parsed.rest.trim()
})

// 解析输入框中的 @skill 前缀
const parseSkillPrefix = (text: string) => {
  const m = text.match(/^\s*@(\S+)\s*(.*)$/s)
  if (!m) return { skill: '', rest: text }
  return { skill: m[1], rest: m[2] }
}

const handleSend = () => {
  if (!canSend.value) return

  const parsed = parseSkillPrefix(inputText.value)
  const skill = parsed.skill || selectedSkill.value || undefined

  // 无正文时用 skill 默认指令
  let content = parsed.rest.trim()
  if (!content) {
    const skillName = skill || selectedSkill.value
    content = skillName ? defaultPromptForSkill(skillName) : ''
  }
  if (!content) return

  emit('send-message', content, skill, selectedBackend.value || undefined, selectedModel.value || undefined)
  inputText.value = ''
  selectedSkill.value = skill || ''
}

// skill 的默认触发指令
const defaultPromptForSkill = (skill: string) => {
  const prompts: Record<string, string> = {
    'status-check': '执行系统状态检查',
    'config-analysis': '分析当前配置',
    'error-diagnosis': '诊断最近的错误',
    'log-analysis': '分析日志',
    'strategy-recommend': '提供策略建议'
  }
  return prompts[skill] || `执行${skill}`
}

// 选择 skill 下拉时，若输入框无 @ 前缀则自动补上
const onSkillChange = (val: string) => {
  if (val) {
    const parsed = parseSkillPrefix(inputText.value)
    const tag = `@${val} `
    if (parsed.skill === val) {
      // 已存在同 skill 前缀，无需操作
    } else if (inputText.value.trim() === '') {
      inputText.value = tag
    } else if (!parsed.skill) {
      inputText.value = tag + inputText.value
    } else {
      // 替换已有 @xx
      inputText.value = tag + parsed.rest
    }
  }
}

// 选择后端后加载其模型列表
const onBackendChange = async (val: string) => {
  selectedModel.value = ''
  modelOptions.value = []
  if (!val) return
  try {
    const data = await getBackendModels(val)
    const list = Array.isArray(data) ? data : data?.models
    modelOptions.value = (Array.isArray(list) ? list : []).map((x: any) =>
      typeof x === 'string' ? x : x?.actual_model || x?.name || ''
    ).filter(Boolean)
  } catch (e) {
    console.error('Failed to load models:', e)
  }
}

const skillLabel = (name: string) => {
  const s = (props.skills || []).find((x) => x.name === name)
  return s ? `${s.name} - ${s.description}` : name
}

const formatTime = (t?: string) => {
  if (!t) return ''
  const d = new Date(t)
  return d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
}

// 加载后端列表
const loadBackends = async () => {
  try {
    const data = await getBackends()
    backends.value = (Array.isArray(data) ? data : []).map((b: any) => ({
      id: b.id,
      name: b.name
    })).filter((b) => b.id)
  } catch (e) {
    console.error('Failed to load backends:', e)
  }
}

watch(
  () => props.currentSkill,
  (v) => {
    if (v) selectedSkill.value = v
  }
)

watch(
  () => props.isResponding,
  (v) => {
    isExecuting.value = !!v
  },
  { immediate: true }
)

watch(
  () => props.messages,
  async () => {
    await nextTick()
    if (messagesContainer.value) {
      messagesContainer.value.scrollTop = messagesContainer.value.scrollHeight
    }
  },
  { deep: true }
)

loadBackends()
</script>

<style scoped>
.chat-area {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-width: 0;
  background: #fff;
}

.chat-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 14px 20px;
  border-bottom: 1px solid var(--el-border-color-light);
}

.session-info {
  display: flex;
  align-items: center;
  gap: 10px;
}

.header-icon {
  color: var(--el-color-primary);
}

.session-title {
  font-size: 16px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.chat-messages {
  flex: 1;
  overflow-y: auto;
  padding: 20px;
  background: #fafbfc;
}

.empty-hint {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: var(--el-text-color-secondary);
  text-align: center;
  gap: 8px;
}

.empty-icon {
  color: var(--el-color-primary-light-4);
  margin-bottom: 8px;
}

.empty-hint h3 {
  margin: 0;
  font-size: 18px;
  color: var(--el-text-color-primary);
}

.empty-hint p {
  margin: 0;
  font-size: 13px;
  max-width: 420px;
  line-height: 1.6;
}

.message {
  display: flex;
  gap: 12px;
  margin-bottom: 20px;
}

.message.user {
  flex-direction: row-reverse;
}

.message-avatar {
  width: 36px;
  height: 36px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.message-avatar.user {
  background: var(--el-color-primary);
  color: #fff;
}

.message-avatar.assistant {
  background: var(--el-color-success);
  color: #fff;
}

.message-body {
  max-width: 72%;
  min-width: 0;
}

.message.user .message-body {
  text-align: right;
}

.message-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 6px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.message.user .message-meta {
  flex-direction: row-reverse;
}

.message-role-label {
  font-weight: 600;
  color: var(--el-text-color-regular);
}

.message-text {
  margin: 0;
  padding: 12px 14px;
  border-radius: 10px;
  background: #fff;
  border: 1px solid var(--el-border-color-lighter);
  white-space: pre-wrap;
  word-break: break-word;
  font-family: inherit;
  font-size: 14px;
  line-height: 1.6;
  color: var(--el-text-color-primary);
  text-align: left;
}

.message.user .message-text {
  background: var(--el-color-primary);
  border-color: var(--el-color-primary);
  color: #fff;
}

.tool-badge {
  margin-top: 8px;
}

.chat-input {
  padding: 12px 20px 14px;
  border-top: 1px solid var(--el-border-color-light);
  background: #fff;
}

.input-wrapper {
  display: flex;
  align-items: flex-end;
  gap: 10px;
}

.input-wrapper :deep(.el-textarea__inner) {
  font-size: 14px;
  line-height: 1.6;
}

.send-btn {
  flex-shrink: 0;
  padding: 12px 26px;
}

.input-toolbar {
  display: flex;
  align-items: center;
  gap: 20px;
  margin-top: 10px;
  flex-wrap: wrap;
}

.toolbar-group {
  display: flex;
  align-items: center;
  gap: 6px;
}

.toolbar-label {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
  white-space: nowrap;
}

.toolbar-select {
  width: 180px;
}

.model-select {
  width: 200px;
}

.thinking {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--el-text-color-secondary);
  font-size: 14px;
  background: #fff;
  border: 1px solid var(--el-border-color-lighter);
}

.thinking-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--el-color-primary);
  animation: thinking-bounce 1.2s infinite ease-in-out;
}

.thinking-dot:nth-child(2) {
  animation-delay: 0.15s;
}

.thinking-dot:nth-child(3) {
  animation-delay: 0.3s;
}

@keyframes thinking-bounce {
  0%, 60%, 100% {
    transform: translateY(0);
    opacity: 0.4;
  }
  30% {
    transform: translateY(-4px);
    opacity: 1;
  }
}
</style>
