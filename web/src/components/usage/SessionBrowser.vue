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
              <el-button
                class="compact-delete"
                text
                type="danger"
                size="small"
                :aria-label="t('sessionBrowser.delete')"
                @click.stop="deleteOneSession(s)"
              >
                <el-icon><Delete /></el-icon>
              </el-button>
            </div>
            <div class="session-meta">
              <span>{{ s.category || t('sessionBrowser.categoryGeneral') }}</span>
              <span>{{ t('sessionBrowser.messagesCount', { count: s.message_count || 0 }) }}</span>
            </div>
            <div v-if="sessionUsage[s.id]" class="session-usage">
              <span class="usage-item">
                <span class="usage-label">{{ t('sessionBrowser.inputTokens') }}</span>
                {{ formatTokens(sessionUsage[s.id].input_tokens) }}
              </span>
              <span class="usage-item">
                <span class="usage-label">{{ t('sessionBrowser.outputTokens') }}</span>
                {{ formatTokens(sessionUsage[s.id].output_tokens) }}
              </span>
              <span class="usage-item">
                <span class="usage-label">{{ t('sessionBrowser.inputPrice') }}</span>
                ${{ formatPrice(sessionUsage[s.id].cost_input_price) }}
              </span>
              <span class="usage-item">
                <span class="usage-label">{{ t('sessionBrowser.outputPrice') }}</span>
                ${{ formatPrice(sessionUsage[s.id].cost_output_price) }}
              </span>
              <span class="usage-item usage-cost">
                <span class="usage-label">{{ t('sessionBrowser.totalCost') }}</span>
                ${{ formatCost(sessionUsage[s.id].total_cost) }}
              </span>
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
            style="width: 150px"
            @change="onFilterChange"
          >
            <el-option v-for="c in categories" :key="c" :label="c" :value="c" />
          </el-select>
          <el-date-picker
            v-model="dateRange"
            type="datetimerange"
            range-separator="~"
            :start-placeholder="t('sessionBrowser.since')"
            :end-placeholder="t('sessionBrowser.until')"
            :shortcuts="dateShortcuts"
            style="width: 280px"
            @change="onFilterChange"
          />
          <el-button type="danger" plain :disabled="!hasFilter" @click="deleteFilteredSessions">
            {{ t('sessionBrowser.deleteFiltered') }}
          </el-button>
          <el-button type="primary" :loading="loading" @click="reload">{{ t('sessionBrowser.refresh') }}</el-button>
        </div>
      </div>

      <el-row :gutter="16" class="body-row">
        <el-col :xs="24" :md="10" :lg="9">
          <el-card shadow="never" class="list-card">
            <template #header>
              <div class="list-head">
                <div class="list-title">
                  <span>{{ t('sessionBrowser.sessionList') }}</span>
                  <span class="muted">{{ t('sessionBrowser.totalCount', { count: sessions.length }) }}</span>
                </div>
                <div class="list-actions">
                  <el-checkbox
                    v-model="allSessionsChecked"
                    :indeterminate="someSessionsChecked"
                    size="small"
                  >
                    {{ t('sessionBrowser.selectAll') }}
                  </el-checkbox>
                  <el-button
                    type="danger"
                    plain
                    size="small"
                    :disabled="checkedSessionIds.length === 0"
                    @click="deleteSelectedSessions"
                  >
                    {{ t('sessionBrowser.deleteSelected') }}
                  </el-button>
                </div>
              </div>
            </template>
            <el-empty v-if="!loading && sessions.length === 0" :description="t('sessionBrowser.noSessionsFull')" />
            <div v-else class="session-list full-list">
              <div
                v-for="s in sessions"
                :key="s.id"
                class="session-row"
                :class="{ active: selectedId === s.id }"
              >
                <el-checkbox v-model="checkedSessionIds" :value="s.id" size="small" class="row-checkbox" @click.stop />
                <button type="button" class="session-item button-item" @click="selectSession(s)">
                  <div class="session-title">{{ s.title || s.id }}</div>
                  <div class="session-meta">
                    <el-tag size="small" type="info">{{ s.category || 'general' }}</el-tag>
                    <span>{{ t('sessionBrowser.messagesCount', { count: s.message_count }) }}</span>
                    <span>{{ formatTime(s.updated_at) }}</span>
                  </div>
                </button>
                <el-button
                  class="row-delete"
                  text
                  type="danger"
                  size="small"
                  :aria-label="t('sessionBrowser.delete')"
                  @click.stop="deleteOneSession(s)"
                >
                  <el-icon><Delete /></el-icon>
                </el-button>
              </div>
            </div>
          </el-card>
        </el-col>

        <el-col :xs="24" :md="14" :lg="15">
          <el-card shadow="never" class="detail-card">
            <template #header>
              <div class="detail-header">
                <div>
                  <span>{{ t('sessionBrowser.messagesTitle') }}</span>
                  <span v-if="selected" class="muted">{{ selected.id }}</span>
                </div>
                <div class="detail-actions">
                  <el-checkbox
                    v-if="selectedId"
                    v-model="allMessagesChecked"
                    :indeterminate="someMessagesChecked"
                    size="small"
                  >
                    {{ t('sessionBrowser.selectAllMessages') }}
                  </el-checkbox>
                  <el-button
                    v-if="selectedId"
                    text
                    type="danger"
                    size="small"
                    :disabled="checkedMessageIds.length === 0"
                    @click="deleteSelectedMessages"
                  >
                    {{ t('sessionBrowser.deleteSelected') }}
                  </el-button>
                  <el-select
                    v-if="selectedId"
                    v-model="messageRoleFilter"
                    :placeholder="t('sessionBrowser.allRoles')"
                    size="small"
                    clearable
                    style="width: 110px"
                  >
                    <el-option :label="t('sessionBrowser.roleUser')" value="user" />
                    <el-option :label="t('sessionBrowser.roleAssistant')" value="assistant" />
                  </el-select>
                  <el-button
                    v-if="selectedId"
                    text
                    type="danger"
                    size="small"
                    :disabled="!messageRoleFilter"
                    @click="deleteMessagesByRole"
                  >
                    {{ t('sessionBrowser.deleteByRole') }}
                  </el-button>
                  <el-button
                    v-if="selectedId"
                    text
                    type="primary"
                    size="small"
                    @click="openRelatedCache"
                  >
                    {{ t('sessionBrowser.viewRelatedCache') }}
                  </el-button>
                </div>
              </div>
            </template>
            <el-empty v-if="!selectedId" :description="t('sessionBrowser.selectSessionHint')" />
            <div v-else v-loading="messagesLoading" class="message-list">
              <div v-for="m in messages" :key="m.id" class="message" :class="m.role">
                <div class="message-head">
                  <div class="message-role">{{ roleLabel(m.role) }}</div>
                  <el-checkbox v-model="checkedMessageIds" :value="m.id" size="small" @click.stop />
                </div>
                <pre class="message-content">{{ displayContent(m) }}</pre>
                <div class="message-meta">
                  <span v-if="m.model">{{ m.model }}</span>
                  <span v-if="m.backend">{{ m.backend }}</span>
                  <span>{{ formatTime(m.created_at) }}</span>
                  <el-button
                    class="msg-delete"
                    text
                    type="danger"
                    size="small"
                    :aria-label="t('sessionBrowser.delete')"
                    @click="deleteOneMessage(m)"
                  >
                    <el-icon><Delete /></el-icon>
                  </el-button>
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
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { ArrowDown, ArrowRight, Delete } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import * as convApi from '@/api/conversations'
import { getSessionsUsageBreakdown } from '@/api/token-usage'
import { formatTokens } from '@/utils/format'
import type { ConversationMessage, ConversationSession } from '@/api/conversations'

const { t } = useI18n()
const router = useRouter()

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
const dateRange = ref<[Date, Date] | null>(null)
const checkedSessionIds = ref<string[]>([])
const checkedMessageIds = ref<string[]>([])
const messageRoleFilter = ref('')

// 每个会话的计量计价汇总（键为 session_id，来自批量接口）
interface SessionUsageSummary {
  session_id: string
  request_count: number
  input_tokens: number
  output_tokens: number
  total_tokens: number
  cost_input_price: number
  cost_output_price: number
  input_cost: number
  output_cost: number
  total_cost: number
}
const sessionUsage = ref<Record<string, SessionUsageSummary>>({})

const dateShortcuts = [
  {
    text: () => t('sessionBrowser.lastDay'),
    value: () => {
      const end = new Date()
      const start = new Date()
      start.setTime(start.getTime() - 3600 * 1000 * 24)
      return [start, end]
    }
  },
  {
    text: () => t('sessionBrowser.last7Days'),
    value: () => {
      const end = new Date()
      const start = new Date()
      start.setTime(start.getTime() - 3600 * 1000 * 24 * 7)
      return [start, end]
    }
  },
  {
    text: () => t('sessionBrowser.last30Days'),
    value: () => {
      const end = new Date()
      const start = new Date()
      start.setTime(start.getTime() - 3600 * 1000 * 24 * 30)
      return [start, end]
    }
  }
]

const allSessionsChecked = computed({
  get: () => sessions.value.length > 0 && sessions.value.every((s) => checkedSessionIds.value.includes(s.id)),
  set: (v: boolean) => {
    checkedSessionIds.value = v ? sessions.value.map((s) => s.id) : []
  }
})
const someSessionsChecked = computed(
  () => checkedSessionIds.value.length > 0 && checkedSessionIds.value.length < sessions.value.length
)

const allMessagesChecked = computed({
  get: () => messages.value.length > 0 && messages.value.every((m) => checkedMessageIds.value.includes(m.id)),
  set: (v: boolean) => {
    checkedMessageIds.value = v ? messages.value.map((m) => m.id) : []
  }
})
const someMessagesChecked = computed(
  () => checkedMessageIds.value.length > 0 && checkedMessageIds.value.length < messages.value.length
)

const hasFilter = computed(
  () => category.value !== '' || (dateRange.value != null && dateRange.value[0] != null && dateRange.value[1] != null)
)

const pagedSessions = computed(() => {
  const start = (page.value - 1) * pageSize.value
  return sessions.value.slice(start, start + pageSize.value)
})

async function loadSessionUsage() {
  if (props.mode !== 'compact' || pagedSessions.value.length === 0) return
  try {
    const data: any = await getSessionsUsageBreakdown(pagedSessions.value.map((s) => s.id))
    sessionUsage.value = data?.sessions ?? {}
  } catch {
    sessionUsage.value = {}
  }
}

function formatPrice(n: number | undefined | null): string {
  return Number(n || 0).toFixed(6)
}

function formatCost(n: number | undefined | null): string {
  const v = Number(n || 0)
  if (v === 0) return '0.00'
  if (v < 0.01) return v.toFixed(6)
  if (v < 1000) return v.toFixed(4)
  return v.toFixed(2)
}

watch(
  () => [page.value, pageSize.value],
  () => {
    void loadSessionUsage()
  }
)

function filterParams() {
  const params: { category?: string; since?: string; until?: string } = {}
  if (props.mode === 'full' && category.value) params.category = category.value
  if (dateRange.value && dateRange.value[0] && dateRange.value[1]) {
    params.since = new Date(dateRange.value[0]).toISOString()
    params.until = new Date(dateRange.value[1]).toISOString()
  }
  return params
}

function onFilterChange() {
  checkedSessionIds.value = []
  reload()
}

function formatTime(v?: string) {
  if (!v) return ''
  const d = new Date(v)
  if (Number.isNaN(d.getTime())) return v
  return d.toLocaleString()
}

function openRelatedCache() {
  if (!selectedId.value) return
  router.push({ path: '/cache', query: { tab: 'data', session_id: selectedId.value } })
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
      ...filterParams(),
      limit: props.mode === 'compact' ? 500 : 100,
      offset: 0
    })
    sessions.value = res?.sessions ?? []
    const kept = checkedSessionIds.value.filter((id) => sessions.value.some((s) => s.id === id))
    checkedSessionIds.value = kept
    const maxPage = Math.max(1, Math.ceil(sessions.value.length / pageSize.value) || 1)
    if (page.value > maxPage) page.value = maxPage
    if (selectedId.value && !sessions.value.some((s) => s.id === selectedId.value)) {
      collapseSession()
    }
    void loadSessionUsage()
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
  checkedMessageIds.value = []
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
  checkedMessageIds.value = []
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
  checkedMessageIds.value = []
}

async function deleteOneSession(s: ConversationSession) {
  try {
    await ElMessageBox.confirm(
      t('sessionBrowser.deleteOneConfirm', { title: s.title || s.id }),
      t('sessionBrowser.deleteConfirmTitle'),
      { type: 'warning' }
    )
    await convApi.deleteSession(s.id)
    ElMessage.success(t('sessionBrowser.deleteSuccess'))
    if (selectedId.value === s.id) collapseSession()
    checkedSessionIds.value = checkedSessionIds.value.filter((id) => id !== s.id)
    reload()
  } catch (e: any) {
    if (e !== 'cancel') {
      ElMessage.error(t('sessionBrowser.deleteFailed') + ' ' + (e?.message || e))
    }
  }
}

async function deleteSelectedSessions() {
  if (checkedSessionIds.value.length === 0) return
  try {
    await ElMessageBox.confirm(
      t('sessionBrowser.deleteSelectedConfirm', { count: checkedSessionIds.value.length }),
      t('sessionBrowser.deleteConfirmTitle'),
      { type: 'warning' }
    )
    const res = await convApi.deleteSessions({ ids: checkedSessionIds.value })
    ElMessage.success(t('sessionBrowser.deleteCount', { count: res.deleted ?? 0 }))
    if (selectedId.value && checkedSessionIds.value.includes(selectedId.value)) collapseSession()
    checkedSessionIds.value = []
    reload()
  } catch (e: any) {
    if (e !== 'cancel') {
      ElMessage.error(t('sessionBrowser.deleteFailed') + ' ' + (e?.message || e))
    }
  }
}

async function deleteFilteredSessions() {
  const params = filterParams()
  const hasCategory = !!params.category
  const hasDate = !!params.since && !!params.until
  if (!hasCategory && !hasDate) {
    ElMessage.info(t('sessionBrowser.noFilterHint'))
    return
  }
  try {
    await ElMessageBox.confirm(
      t('sessionBrowser.deleteFilteredConfirm'),
      t('sessionBrowser.deleteConfirmTitle'),
      { type: 'warning' }
    )
    const res = await convApi.deleteSessions(params)
    ElMessage.success(t('sessionBrowser.deleteCount', { count: res.deleted ?? 0 }))
    checkedSessionIds.value = []
    collapseSession()
    reload()
  } catch (e: any) {
    if (e !== 'cancel') {
      ElMessage.error(t('sessionBrowser.deleteFailed') + ' ' + (e?.message || e))
    }
  }
}

async function deleteOneMessage(m: ConversationMessage) {
  if (!selectedId.value) return
  try {
    await ElMessageBox.confirm(
      t('sessionBrowser.deleteOneMessageConfirm'),
      t('sessionBrowser.deleteConfirmTitle'),
      { type: 'warning' }
    )
    await convApi.deleteMessages(selectedId.value, { ids: [m.id] })
    ElMessage.success(t('sessionBrowser.deleteSuccess'))
    messages.value = messages.value.filter((x) => x.id !== m.id)
    checkedMessageIds.value = checkedMessageIds.value.filter((id) => id !== m.id)
  } catch (e: any) {
    if (e !== 'cancel') {
      ElMessage.error(t('sessionBrowser.deleteFailed') + ' ' + (e?.message || e))
    }
  }
}

async function deleteSelectedMessages() {
  if (!selectedId.value || checkedMessageIds.value.length === 0) return
  try {
    await ElMessageBox.confirm(
      t('sessionBrowser.deleteSelectedMessagesConfirm', { count: checkedMessageIds.value.length }),
      t('sessionBrowser.deleteConfirmTitle'),
      { type: 'warning' }
    )
    const res = await convApi.deleteMessages(selectedId.value, { ids: checkedMessageIds.value })
    ElMessage.success(t('sessionBrowser.deleteCount', { count: res.deleted ?? 0 }))
    messages.value = messages.value.filter((x) => !checkedMessageIds.value.includes(x.id))
    checkedMessageIds.value = []
  } catch (e: any) {
    if (e !== 'cancel') {
      ElMessage.error(t('sessionBrowser.deleteFailed') + ' ' + (e?.message || e))
    }
  }
}

async function deleteMessagesByRole() {
  if (!selectedId.value || !messageRoleFilter.value) return
  try {
    await ElMessageBox.confirm(
      t('sessionBrowser.deleteByRoleConfirm', { role: roleLabel(messageRoleFilter.value) }),
      t('sessionBrowser.deleteConfirmTitle'),
      { type: 'warning' }
    )
    const res = await convApi.deleteMessages(selectedId.value, { role: messageRoleFilter.value })
    ElMessage.success(t('sessionBrowser.deleteCount', { count: res.deleted ?? 0 }))
    messages.value = messages.value.filter((x) => x.role !== messageRoleFilter.value)
    checkedMessageIds.value = []
  } catch (e: any) {
    if (e !== 'cancel') {
      ElMessage.error(t('sessionBrowser.deleteFailed') + ' ' + (e?.message || e))
    }
  }
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
.session-usage {
  margin-top: 4px;
  margin-left: 18px;
  display: flex;
  flex-wrap: wrap;
  gap: 4px 12px;
  font-size: 11px;
  color: var(--el-text-color-secondary);
  line-height: 1.6;
}
.usage-item {
  white-space: nowrap;
}
.usage-label {
  margin-right: 2px;
  color: var(--el-text-color-placeholder);
}
.usage-cost {
  font-weight: 600;
  color: var(--el-text-color-primary);
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
.detail-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  width: 100%;
  gap: 8px;
}
.list-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  width: 100%;
  gap: 8px;
}
.list-title {
  display: flex;
  align-items: baseline;
  gap: 8px;
  min-width: 0;
}
.list-actions,
.detail-actions {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
}
.list-actions :deep(.el-checkbox),
.detail-actions :deep(.el-checkbox) {
  margin-right: 0;
  white-space: nowrap;
}
.list-actions :deep(.el-button),
.detail-actions :deep(.el-button) {
  margin-left: 0;
  margin-right: 0;
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
.compact-delete {
  flex-shrink: 0;
  margin-left: auto;
}
.full-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  max-height: 70vh;
  overflow: auto;
}
.session-row {
  display: flex;
  align-items: center;
  gap: 8px;
}
.row-checkbox {
  flex-shrink: 0;
}
.row-delete {
  flex-shrink: 0;
  margin-left: auto;
}
.msg-delete {
  margin-left: auto;
}
.message-meta {
  margin-top: 6px;
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  font-size: 0.75rem;
  color: var(--el-text-color-secondary);
  align-items: center;
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
