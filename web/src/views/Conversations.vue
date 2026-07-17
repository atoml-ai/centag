<template>
  <div class="conversations">
    <div class="page-header">
      <div>
        <h2>会话记录</h2>
        <p class="subtitle">浏览已记录的对话 Session 与消息（按发行版落库：文件 / SQLite / PG）</p>
      </div>
      <div class="header-actions">
        <el-select
          v-model="category"
          clearable
          placeholder="全部分类"
          style="width: 160px"
          @change="reload"
        >
          <el-option v-for="c in categories" :key="c" :label="c" :value="c" />
        </el-select>
        <el-button type="primary" :loading="loading" @click="reload">刷新</el-button>
      </div>
    </div>

    <el-row :gutter="16" class="body-row">
      <el-col :xs="24" :md="10" :lg="9">
        <el-card shadow="never" class="list-card">
          <template #header>
            <span>会话列表</span>
            <span class="muted">共 {{ sessions.length }} 条</span>
          </template>
          <el-empty v-if="!loading && sessions.length === 0" description="暂无会话" />
          <div v-else class="session-list">
            <button
              v-for="s in sessions"
              :key="s.id"
              type="button"
              class="session-item"
              :class="{ active: selectedId === s.id }"
              @click="selectSession(s)"
            >
              <div class="session-title">{{ s.title || s.id }}</div>
              <div class="session-meta">
                <el-tag size="small" type="info">{{ s.category || 'general' }}</el-tag>
                <span>{{ s.message_count }} 条消息</span>
                <span>{{ formatTime(s.updated_at) }}</span>
              </div>
            </button>
          </div>
        </el-card>
      </el-col>

      <el-col :xs="24" :md="14" :lg="15">
        <el-card shadow="never" class="detail-card">
          <template #header>
            <span>消息</span>
            <span v-if="selected" class="muted">{{ selected.id }}</span>
          </template>
          <el-empty v-if="!selectedId" description="选择左侧会话查看消息" />
          <div v-else v-loading="messagesLoading" class="message-list">
            <div
              v-for="m in messages"
              :key="m.id"
              class="message"
              :class="m.role"
            >
              <div class="message-role">{{ roleLabel(m.role) }}</div>
              <pre class="message-content">{{ m.content }}</pre>
              <div class="message-meta">
                <span v-if="m.model">{{ m.model }}</span>
                <span v-if="m.backend">{{ m.backend }}</span>
                <span>{{ formatTime(m.created_at) }}</span>
              </div>
            </div>
            <el-empty v-if="!messagesLoading && messages.length === 0" description="该会话暂无消息" />
          </div>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import * as convApi from '@/api/conversations'
import type { ConversationMessage, ConversationSession } from '@/api/conversations'

const loading = ref(false)
const messagesLoading = ref(false)
const category = ref('')
const categories = ref<string[]>([])
const sessions = ref<ConversationSession[]>([])
const selectedId = ref('')
const selected = ref<ConversationSession | null>(null)
const messages = ref<ConversationMessage[]>([])

function formatTime(v?: string) {
  if (!v) return ''
  const d = new Date(v)
  if (Number.isNaN(d.getTime())) return v
  return d.toLocaleString()
}

function roleLabel(role: string) {
  if (role === 'user') return '用户'
  if (role === 'assistant') return '助手'
  if (role === 'system') return '系统'
  return role
}

async function loadCategories() {
  try {
    const res = await convApi.listCategories()
    categories.value = res.categories ?? []
  } catch (e: any) {
    // categories are optional filters; ignore soft failures
    console.warn('load categories failed', e)
  }
}

async function reload() {
  loading.value = true
  try {
    const res = await convApi.listSessions({
      category: category.value || undefined,
      limit: 100
    })
    sessions.value = res.sessions ?? []
    if (selectedId.value && !sessions.value.some((s) => s.id === selectedId.value)) {
      selectedId.value = ''
      selected.value = null
      messages.value = []
    }
  } catch (e: any) {
    ElMessage.error('加载会话失败：' + (e?.message || e))
  } finally {
    loading.value = false
  }
}

async function selectSession(s: ConversationSession) {
  selectedId.value = s.id
  selected.value = s
  messagesLoading.value = true
  try {
    const res = await convApi.listMessages(s.id, { limit: 200 })
    messages.value = res.messages ?? []
  } catch (e: any) {
    ElMessage.error('加载消息失败：' + (e?.message || e))
    messages.value = []
  } finally {
    messagesLoading.value = false
  }
}

onMounted(async () => {
  await loadCategories()
  await reload()
})
</script>

<style scoped>
.conversations {
  padding: 16px 20px 32px;
}
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
  margin-bottom: 16px;
}
.page-header h2 {
  margin: 0 0 4px;
  font-size: 1.35rem;
}
.subtitle {
  margin: 0;
  color: var(--el-text-color-secondary);
  font-size: 0.9rem;
}
.header-actions {
  display: flex;
  gap: 8px;
  align-items: center;
}
.body-row {
  min-height: 420px;
}
.list-card :deep(.el-card__header),
.detail-card :deep(.el-card__header) {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.muted {
  color: var(--el-text-color-secondary);
  font-size: 0.85rem;
}
.session-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  max-height: 70vh;
  overflow: auto;
}
.session-item {
  text-align: left;
  border: 1px solid var(--el-border-color-lighter);
  background: var(--el-bg-color);
  border-radius: 8px;
  padding: 10px 12px;
  cursor: pointer;
}
.session-item:hover {
  border-color: var(--el-color-primary-light-5);
}
.session-item.active {
  border-color: var(--el-color-primary);
  background: var(--el-color-primary-light-9);
}
.session-title {
  font-weight: 600;
  margin-bottom: 6px;
  word-break: break-all;
}
.session-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  font-size: 0.8rem;
  color: var(--el-text-color-secondary);
}
.message-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
  max-height: 70vh;
  overflow: auto;
}
.message {
  border-radius: 8px;
  padding: 10px 12px;
  background: var(--el-fill-color-light);
}
.message.user {
  background: var(--el-color-primary-light-9);
}
.message.assistant {
  background: var(--el-fill-color);
}
.message-role {
  font-size: 0.75rem;
  font-weight: 600;
  margin-bottom: 4px;
  color: var(--el-text-color-secondary);
}
.message-content {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-word;
  font-family: inherit;
  font-size: 0.92rem;
  line-height: 1.5;
}
.message-meta {
  margin-top: 6px;
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  font-size: 0.75rem;
  color: var(--el-text-color-secondary);
}
</style>
