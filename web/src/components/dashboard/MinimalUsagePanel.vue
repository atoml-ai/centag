<template>
  <div class="minimal-usage">
    <div class="usage-stats">
      <div class="stat">
        <div class="stat-value">{{ formatNumber(stats.total_tokens) }}</div>
        <div class="stat-label">Token 消耗</div>
      </div>
      <div class="stat">
        <div class="stat-value">{{ formatNumber(stats.request_count) }}</div>
        <div class="stat-label">请求次数</div>
      </div>
      <div class="stat">
        <div class="stat-value">{{ formatNumber(stats.total_prompt_tokens) }}</div>
        <div class="stat-label">输入</div>
      </div>
      <div class="stat">
        <div class="stat-value">{{ formatNumber(stats.total_completion_tokens) }}</div>
        <div class="stat-label">输出</div>
      </div>
    </div>
    <p class="hint">本次进程内计量（重启后清零）</p>

    <div class="session-block">
      <div class="session-head">
        <span>最近对话</span>
        <el-button text type="primary" size="small" :loading="loading" @click="reload">刷新</el-button>
      </div>
      <el-empty v-if="!loading && sessions.length === 0" description="暂无会话，先发起一次 AI 对话" :image-size="64" />
      <ul v-else class="session-list">
        <li
          v-for="s in sessions"
          :key="s.id"
          class="session-item"
          :class="{ active: s.id === selectedId }"
          @click="selectSession(s.id)"
        >
          <div class="session-title">{{ s.title || s.id }}</div>
          <div class="session-meta">
            <span>{{ s.category || 'general' }}</span>
            <span>{{ s.message_count || 0 }} 条</span>
          </div>
        </li>
      </ul>
      <div v-if="selectedId" class="messages">
        <div v-for="m in messages" :key="m.id" class="msg" :class="m.role">
          <span class="role">{{ m.role }}</span>
          <pre>{{ m.content }}</pre>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'
import { getUserUsage } from '@/api/token-usage'
import * as convApi from '@/api/conversations'
import type { ConversationMessage, ConversationSession } from '@/api/conversations'

const loading = ref(false)
const stats = reactive({
  total_tokens: 0,
  total_prompt_tokens: 0,
  total_completion_tokens: 0,
  request_count: 0
})
const sessions = ref<ConversationSession[]>([])
const selectedId = ref('')
const messages = ref<ConversationMessage[]>([])

function formatNumber(n: number | undefined | null): string {
  if (!n) return '0'
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M'
  if (n >= 1_000) return (n / 1_000).toFixed(1) + 'K'
  return String(n)
}

async function loadUsage() {
  try {
    const res: any = await getUserUsage()
    const data = res?.stats ?? res?.data?.stats ?? res
    stats.total_tokens = Number(data?.total_tokens || 0)
    stats.total_prompt_tokens = Number(data?.total_prompt_tokens || 0)
    stats.total_completion_tokens = Number(data?.total_completion_tokens || 0)
    stats.request_count = Number(data?.request_count || 0)
  } catch {
    /* ignore */
  }
}

async function loadSessions() {
  try {
    const res = await convApi.listSessions({ limit: 20 })
    sessions.value = res?.sessions ?? []
  } catch {
    sessions.value = []
  }
}

async function selectSession(id: string) {
  selectedId.value = id
  try {
    const res = await convApi.listMessages(id, { limit: 50 })
    messages.value = res?.messages ?? []
  } catch {
    messages.value = []
  }
}

async function reload() {
  loading.value = true
  try {
    await Promise.all([loadUsage(), loadSessions()])
    if (selectedId.value) {
      await selectSession(selectedId.value)
    }
  } finally {
    loading.value = false
  }
}

// Parent collapse loads on expand via expose.reload(); avoid eager fetch when folded.
defineExpose({ reload })
</script>

<style scoped>
.minimal-usage {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.usage-stats {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 8px;
}
.stat {
  background: var(--el-fill-color-light);
  border-radius: 8px;
  padding: 10px 8px;
  text-align: center;
}
.stat-value {
  font-size: 18px;
  font-weight: 600;
  line-height: 1.2;
}
.stat-label {
  margin-top: 4px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
}
.hint {
  margin: 0;
  font-size: 12px;
  color: var(--el-text-color-secondary);
}
.session-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
  font-weight: 600;
}
.session-list {
  list-style: none;
  margin: 0;
  padding: 0;
  max-height: 180px;
  overflow: auto;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 8px;
}
.session-item {
  padding: 8px 10px;
  cursor: pointer;
  border-bottom: 1px solid var(--el-border-color-extra-light);
}
.session-item:last-child {
  border-bottom: none;
}
.session-item:hover,
.session-item.active {
  background: var(--el-fill-color-light);
}
.session-title {
  font-size: 13px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.session-meta {
  margin-top: 2px;
  display: flex;
  gap: 10px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
}
.messages {
  margin-top: 10px;
  max-height: 220px;
  overflow: auto;
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.msg {
  border-radius: 8px;
  padding: 8px 10px;
  background: var(--el-fill-color-lighter);
}
.msg.assistant {
  background: var(--el-color-primary-light-9);
}
.msg .role {
  font-size: 11px;
  color: var(--el-text-color-secondary);
  text-transform: uppercase;
}
.msg pre {
  margin: 4px 0 0;
  white-space: pre-wrap;
  word-break: break-word;
  font-family: inherit;
  font-size: 13px;
}
@media (max-width: 900px) {
  .usage-stats {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
