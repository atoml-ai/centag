<template>
  <div class="session-browser" :class="mode">
    <template v-if="mode === 'compact'">
      <div class="session-head">
        <span>{{ t('sessionBrowser.compactTitle') }}</span>
        <el-button text type="primary" size="small" :loading="loading" @click="reload">{{ t('sessionBrowser.refresh') }}</el-button>
      </div>

      <el-empty
        v-if="!loading && sessions.length === 0"
        :description="t('sessionBrowser.noSessions')"
        :image-size="64"
      />
      <template v-else>
        <ul class="session-list compact-list">
          <li
            v-for="s in pagedSessions"
            :key="s.id"
            class="session-item"
            :class="{ active: s.id === selectedId }"
            @click="toggleSession(s.id)"
          >
            <div class="session-title">
              <el-icon class="expand-icon">
                <ArrowDown v-if="s.id === selectedId" />
                <ArrowRight v-else />
              </el-icon>
              {{ s.title || s.id }}
            </div>
            <div class="session-meta">
              <span>{{ s.category || t('sessionBrowser.categoryGeneral') }}</span>
              <span>{{ t('sessionBrowser.messagesCount', { count: s.message_count || 0 }) }}</span>
            </div>
          </li>
        </ul>

        <div class="pager">
          <el-pagination
            v-model:current-page="page"
            v-model:page-size="pageSize"
            :total="sessions.length"
            :page-sizes="[5, 10, 20]"
            layout="total, sizes, prev, pager, next, jumper"
            size="small"
            background
          />
        </div>

        <div v-if="selectedId" class="messages compact-messages">
          <div class="messages-head">
            <span>{{ t('sessionBrowser.messagesDetail') }}</span>
            <el-button text size="small" @click="collapseSession">{{ t('sessionBrowser.collapse') }}</el-button>
          </div>
          <div v-if="messagesLoading" class="msg-loading">{{ t('sessionBrowser.loading') }}</div>
          <template v-else>
            <div v-for="m in messages" :key="m.id" class="msg" :class="m.role">
              <span class="role">{{ roleLabel(m.role) }}</span>
              <pre>{{ displayContent(m) }}</pre>
            </div>
            <el-empty
              v-if="messages.length === 0"
              :description="emptyDetailHint"
              :image-size="48"
            />
          </template>
        </div>
      </template>
    </template>

    <template v-else>
      <div class="page-header">
        <div>
          <h2>{{ t('sessionBrowser.pageTitle') }}</h2>
          <p class="subtitle">{{ t('sessionBrowser.pageSubtitle') }}</p>
        </div>
        <div class="header-actions">
          <el-select
            v-model="category"
            clearable
            :placeholder="t('sessionBrowser.allCategories')"
            style="width: 160px"
            @change="reload"
          >
            <el-option v-for="c in categories" :key="c" :label="c" :value="c" />
          </el-select>
          <el-button type="primary" :loading="loading" @click="reload">{{ t('sessionBrowser.refresh') }}</el-button>
        </div>
      </div>

      <el-row :gutter="16" class="body-row">
        <el-col :xs="24" :md="10" :lg="9">
          <el-card shadow="never" class="list-card">
            <template #header>
              <span>{{ t('sessionBrowser.sessionList') }}</span>
              <span class="muted">{{ t('sessionBrowser.totalCount', { count: sessions.length }) }}</span>
            </template>
            <el-empty v-if="!loading && sessions.length === 0" :description="t('sessionBrowser.noSessionsFull')" />
            <div v-else class="session-list full-list">
              <button
                v-for="s in sessions"
                :key="s.id"
                type="button"
                class="session-item button-item"
                :class="{ active: selectedId === s.id }"
                @click="selectSession(s)"
              >
                <div class="session-title">{{ s.title || s.id }}</div>
                <div class="session-meta">
                  <el-tag size="small" type="info">{{ s.category || 'general' }}</el-tag>
                  <span>{{ t('sessionBrowser.messagesCount', { count: s.message_count }) }}</span>
                  <span>{{ formatTime(s.updated_at) }}</span>
                </div>
              </button>
            </div>
          </el-card>
        </el-col>

        <el-col :xs="24" :md="14" :lg="15">
          <el-card shadow="never" class="detail-card">
            <template #header>
              <span>{{ t('sessionBrowser.messagesTitle') }}</span>
              <span v-if="selected" class="muted">{{ selected.id }}</span>
            </template>
            <el-empty v-if="!selectedId" :description="t('sessionBrowser.selectSessionHint')" />
            <div v-else v-loading="messagesLoading" class="message-list">
              <div v-for="m in messages" :key="m.id" class="message" :class="m.role">
                <div class="message-role">{{ roleLabel(m.role) }}</div>
                <pre class="message-content">{{ displayContent(m) }}</pre>
                <div class="message-meta">
                  <span v-if="m.model">{{ m.model }}</span>
                  <span v-if="m.backend">{{ m.backend }}</span>
                  <span>{{ formatTime(m.created_at) }}</span>
                </div>
              </div>
              <el-empty
                v-if="!messagesLoading && messages.length === 0"
                :description="emptyDetailHint"
              />
            </div>
          </el-card>
        </el-col>
      </el-row>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ArrowDown, ArrowRight } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import * as convApi from '@/api/conversations'
import type { ConversationMessage, ConversationSession } from '@/api/conversations'

const { t } = useI18n()

const props = withDefaults(
  defineProps<{
    mode?: 'compact' | 'full'
  }>(),
  { mode: 'compact' }
)

const loading = ref(false)
const messagesLoading = ref(false)
const category = ref('')
const categories = ref<string[]>([])
const sessions = ref<ConversationSession[]>([])
const selectedId = ref('')
const selected = ref<ConversationSession | null>(null)
const messages = ref<ConversationMessage[]>([])
const page = ref(1)
const pageSize = ref(5)

const pagedSessions = computed(() => {
  const start = (page.value - 1) * pageSize.value
  return sessions.value.slice(start, start + pageSize.value)
})

function formatTime(v?: string) {
  if (!v) return ''
  const d = new Date(v)
  if (Number.isNaN(d.getTime())) return v
  return d.toLocaleString()
}

function roleLabel(role: string) {
  if (role === 'user') return t('sessionBrowser.roleUser')
  if (role === 'assistant') return t('sessionBrowser.roleAssistant')
  if (role === 'system') return t('sessionBrowser.roleSystem')
  return role
}

const emptyDetailHint = computed(() => {
  const s = sessions.value.find((x) => x.id === selectedId.value)
  if (s && (s.message_count || 0) > 0) {
    return t('sessionBrowser.loadMessagesFailed')
  }
  return t('sessionBrowser.noMessages')
})

function displayContent(m: ConversationMessage) {
  const raw = (m.content || '').trim()
  if (!raw) return t('sessionBrowser.emptyContent')
  if (m.role !== 'assistant' || !raw.includes('data:')) return raw
  const parts: string[] = []
  for (const line of raw.split('\n')) {
    const trimmed = line.trim()
    if (!trimmed.startsWith('data:')) continue
    const payload = trimmed.slice(5).trim()
    if (!payload || payload === '[DONE]') continue
    try {
      const o = JSON.parse(payload) as {
        choices?: Array<{
          delta?: { content?: string | null }
          message?: { content?: string | null }
        }>
      }
      for (const ch of o.choices || []) {
        const c = ch.delta?.content ?? ch.message?.content
        if (typeof c === 'string' && c) parts.push(c)
      }
    } catch {
    }
  }
  const text = parts.join('').trim()
  return text || raw
}

async function loadCategories() {
  if (props.mode !== 'full') return
  try {
    const res = await convApi.listCategories()
    categories.value = res.categories ?? []
  } catch {
  }
}

async function reload() {
  loading.value = true
  try {
    const res = await convApi.listSessions({
      category: props.mode === 'full' ? category.value || undefined : undefined,
      limit: props.mode === 'compact' ? 500 : 100,
      offset: 0
    })
    sessions.value = res?.sessions ?? []
    const maxPage = Math.max(1, Math.ceil(sessions.value.length / pageSize.value) || 1)
    if (page.value > maxPage) page.value = maxPage
    if (selectedId.value && !sessions.value.some((s) => s.id === selectedId.value)) {
      collapseSession()
    }
  } catch (e: any) {
    sessions.value = []
    if (props.mode === 'full') {
      ElMessage.error(t('loadSessionsFailed') + ' ' + (e?.message || e))
    }
  } finally {
    loading.value = false
  }
}

async function toggleSession(id: string) {
  if (selectedId.value === id) {
    collapseSession()
    return
  }
  selectedId.value = id
  messagesLoading.value = true
  messages.value = []
  try {
    const res = await convApi.listMessages(id, { limit: 100 })
    messages.value = res?.messages ?? []
  } catch (e: any) {
    messages.value = []
    ElMessage.error(t('loadMessagesFailed') + ' ' + (e?.message || e))
  } finally {
    messagesLoading.value = false
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
    ElMessage.error(t('loadMessagesFailed') + ' ' + (e?.message || e))
    messages.value = []
  } finally {
    messagesLoading.value = false
  }
}

function collapseSession() {
  selectedId.value = ''
  selected.value = null
  messages.value = []
}

async function reloadKeepSelection() {
  await reload()
  if (selectedId.value && props.mode === 'compact') {
    const id = selectedId.value
    selectedId.value = ''
    await toggleSession(id)
  }
}

onMounted(async () => {
  await loadCategories()
  await reload()
})

defineExpose({ reload: reloadKeepSelection })
</script>

<style scoped>
.session-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
  font-weight: 600;
}
.compact-list {
  list-style: none;
  margin: 0;
  padding: 0;
  max-height: 220px;
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
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 13px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.expand-icon {
  flex-shrink: 0;
  color: var(--el-text-color-secondary);
}
.session-meta {
  margin-top: 2px;
  margin-left: 18px;
  display: flex;
  gap: 10px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
}
.pager {
  margin-top: 8px;
  display: flex;
  justify-content: flex-end;
}
.compact-messages {
  margin-top: 10px;
  max-height: 240px;
  overflow: auto;
  display: flex;
  flex-direction: column;
  gap: 8px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 8px;
  padding: 8px;
}
.messages-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 12px;
  font-weight: 600;
  color: var(--el-text-color-secondary);
}
.msg-loading {
  font-size: 12px;
  color: var(--el-text-color-secondary);
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

.session-browser.full {
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
.full-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  max-height: 70vh;
  overflow: auto;
}
.button-item {
  text-align: left;
  border: 1px solid var(--el-border-color-lighter);
  background: var(--el-bg-color);
  border-radius: 8px;
  padding: 10px 12px;
  width: 100%;
}
.button-item:hover {
  border-color: var(--el-color-primary-light-5);
}
.button-item.active {
  border-color: var(--el-color-primary);
  background: var(--el-color-primary-light-9);
}
.button-item .session-title {
  font-weight: 600;
  margin-bottom: 6px;
  white-space: normal;
  word-break: break-all;
}
.button-item .session-meta {
  margin-left: 0;
  flex-wrap: wrap;
  gap: 8px;
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
