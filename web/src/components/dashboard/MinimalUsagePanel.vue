<template>
  <div class="minimal-usage">
    <!-- 筛选：原成本看板能力 -->
    <div class="filter-bar">
      <el-date-picker
        v-model="dateRange"
        type="daterange"
        size="small"
        range-separator="至"
        start-placeholder="开始"
        end-placeholder="结束"
        value-format="YYYY-MM-DD"
        style="width: 260px"
        @change="reloadMetrics"
      />
      <el-select v-model="groupBy" size="small" style="width: 120px" @change="reloadMetrics">
        <el-option label="按模型" value="model" />
        <el-option label="按后端" value="backend" />
        <el-option label="按日期" value="date" />
      </el-select>
      <el-button size="small" :loading="metricsLoading" @click="reloadMetrics">刷新计量</el-button>
      <el-button size="small" type="primary" plain @click="billingVisible = true">计费规则</el-button>
      <el-radio-group v-model="displayCurrency" size="small" @change="onDisplayCurrencyChange">
        <el-radio-button value="USD">美元</el-radio-button>
        <el-radio-button value="CNY">人民币</el-radio-button>
      </el-radio-group>
    </div>

    <div class="usage-stats">
      <div class="stat">
        <div class="stat-value">{{ currencySymbol }}{{ formatCost(summary.total_cost_usd) }}</div>
        <div class="stat-label">总成本 ({{ displayCurrency }})</div>
      </div>
      <div class="stat">
        <div class="stat-value">{{ formatNumber(summary.total_tokens || stats.total_tokens) }}</div>
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

    <div v-if="summary.groups?.length" class="groups-block">
      <div class="block-title">成本分布</div>
      <el-table :data="summary.groups" size="small" stripe max-height="180">
        <el-table-column prop="key" :label="groupByLabel" min-width="120" />
        <el-table-column label="成本" width="110">
          <template #default="{ row }">{{ currencySymbol }}{{ formatCost(row.cost_usd) }}</template>
        </el-table-column>
        <el-table-column prop="tokens" label="Token" width="100">
          <template #default="{ row }">{{ formatNumber(row.tokens) }}</template>
        </el-table-column>
        <el-table-column prop="request_count" label="请求" width="80" />
      </el-table>
    </div>

    <p class="hint">{{ hint }}</p>

    <!-- 最近对话 -->
    <div class="session-block">
      <div class="session-head">
        <span>最近对话</span>
        <el-button text type="primary" size="small" :loading="sessionsLoading" @click="reloadSessions">刷新</el-button>
      </div>

      <el-empty
        v-if="!sessionsLoading && allSessions.length === 0"
        description="暂无会话，先发起一次 AI 对话"
        :image-size="64"
      />
      <template v-else>
        <ul class="session-list">
          <li
            v-for="s in pagedSessions"
            :key="s.id"
            class="session-item"
            :class="{ active: s.id === selectedId }"
            @click="toggleSession(s.id)"
          >
            <div class="session-title">
              <el-icon class="expand-icon"><ArrowDown v-if="s.id === selectedId" /><ArrowRight v-else /></el-icon>
              {{ s.title || s.id }}
            </div>
            <div class="session-meta">
              <span>{{ s.category || 'general' }}</span>
              <span>{{ s.message_count || 0 }} 条</span>
            </div>
          </li>
        </ul>

        <div class="pager">
          <el-pagination
            v-model:current-page="page"
            v-model:page-size="pageSize"
            :total="allSessions.length"
            :page-sizes="[5, 10, 20]"
            layout="total, sizes, prev, pager, next, jumper"
            small
            background
          />
        </div>

        <div v-if="selectedId" class="messages">
          <div class="messages-head">
            <span>消息详情</span>
            <el-button text size="small" @click="collapseSession">收起</el-button>
          </div>
          <div v-if="messagesLoading" class="msg-loading">加载中…</div>
          <template v-else>
            <div v-for="m in messages" :key="m.id" class="msg" :class="m.role">
              <span class="role">{{ m.role }}</span>
              <pre>{{ m.content }}</pre>
            </div>
            <el-empty v-if="messages.length === 0" description="无消息" :image-size="48" />
          </template>
        </div>
      </template>
    </div>

    <BillingRulesDialog v-model="billingVisible" @saved="reloadMetrics" />
  </div>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { ArrowDown, ArrowRight } from '@element-plus/icons-vue'
import { getUserUsage } from '@/api/token-usage'
import * as costApi from '@/api/cost'
import * as convApi from '@/api/conversations'
import type { ConversationMessage, ConversationSession } from '@/api/conversations'
import BillingRulesDialog from '@/components/dashboard/BillingRulesDialog.vue'
import {
  currencySymbol as symbolOf,
  formatDisplayCost,
  getDisplayCurrency,
  setDisplayCurrency,
  type DisplayCurrency
} from '@/utils/billing-currency'

withDefaults(
  defineProps<{
    /** 用量提示文案 */
    hint?: string
  }>(),
  {
    hint: '计量与成本估算（按当前服务存储策略保留）。'
  }
)

const metricsLoading = ref(false)
const sessionsLoading = ref(false)
const messagesLoading = ref(false)
const billingVisible = ref(false)

const dateRange = ref<[string, string] | null>(null)
const groupBy = ref<'model' | 'backend' | 'date'>('model')

const stats = reactive({
  total_tokens: 0,
  total_prompt_tokens: 0,
  total_completion_tokens: 0,
  request_count: 0
})

const summary = ref<costApi.CostSummary>({
  total_cost_usd: 0,
  total_tokens: 0,
  cache_saved_usd: 0,
  currency: 'USD',
  usd_to_cny: 7.2,
  groups: [],
  from: '',
  to: '',
  group_by: 'model'
})

const displayCurrency = ref<DisplayCurrency>(getDisplayCurrency())
const allSessions = ref<ConversationSession[]>([])
const page = ref(1)
const pageSize = ref(5)
const selectedId = ref('')
const messages = ref<ConversationMessage[]>([])

const usdToCny = computed(() => summary.value.usd_to_cny || 7.2)
const currencySymbol = computed(() => symbolOf(displayCurrency.value))
const groupByLabel = computed(() => {
  if (groupBy.value === 'backend') return '后端'
  if (groupBy.value === 'date') return '日期'
  return '模型'
})

const pagedSessions = computed(() => {
  const start = (page.value - 1) * pageSize.value
  return allSessions.value.slice(start, start + pageSize.value)
})

function formatNumber(n: number | undefined | null): string {
  if (!n) return '0'
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M'
  if (n >= 1_000) return (n / 1_000).toFixed(1) + 'K'
  return String(n)
}

function formatCost(n: number | undefined | null): string {
  return formatDisplayCost(n, displayCurrency.value, usdToCny.value)
}

function onDisplayCurrencyChange(v: DisplayCurrency | string | number | boolean | undefined) {
  const c = v === 'CNY' ? 'CNY' : 'USD'
  displayCurrency.value = c
  setDisplayCurrency(c)
}

async function loadUsage() {
  try {
    const res: any = await getUserUsage(
      dateRange.value
        ? { from: dateRange.value[0], to: dateRange.value[1] }
        : undefined
    )
    const data = res?.stats ?? res?.data?.stats ?? res
    stats.total_tokens = Number(data?.total_tokens || 0)
    stats.total_prompt_tokens = Number(data?.total_prompt_tokens || 0)
    stats.total_completion_tokens = Number(data?.total_completion_tokens || 0)
    stats.request_count = Number(data?.request_count || 0)
  } catch {
    /* ignore */
  }
}

async function loadCostSummary() {
  try {
    const params: costApi.CostSummaryParams = { group_by: groupBy.value }
    if (dateRange.value) {
      params.from = dateRange.value[0]
      params.to = dateRange.value[1]
    }
    summary.value = await costApi.getCostSummary(params)
  } catch {
    summary.value = {
      ...summary.value,
      total_cost_usd: 0,
      groups: []
    }
  }
}

async function reloadMetrics() {
  metricsLoading.value = true
  try {
    await Promise.all([loadUsage(), loadCostSummary()])
  } finally {
    metricsLoading.value = false
  }
}

async function reloadSessions() {
  sessionsLoading.value = true
  try {
    const res = await convApi.listSessions({ limit: 500, offset: 0 })
    allSessions.value = res?.sessions ?? []
    const maxPage = Math.max(1, Math.ceil(allSessions.value.length / pageSize.value) || 1)
    if (page.value > maxPage) page.value = maxPage
    if (selectedId.value && !allSessions.value.some((s) => s.id === selectedId.value)) {
      collapseSession()
    }
  } catch {
    allSessions.value = []
  } finally {
    sessionsLoading.value = false
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
  } catch {
    messages.value = []
  } finally {
    messagesLoading.value = false
  }
}

function collapseSession() {
  selectedId.value = ''
  messages.value = []
}

async function reloadKeepSelection() {
  await Promise.all([reloadMetrics(), reloadSessions()])
  if (selectedId.value) {
    const id = selectedId.value
    selectedId.value = ''
    await toggleSession(id)
  }
}

defineExpose({ reload: reloadKeepSelection })
</script>

<style scoped>
.minimal-usage {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.filter-bar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}
.usage-stats {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 8px;
}
.stat {
  background: var(--el-fill-color-light);
  border-radius: 8px;
  padding: 10px 8px;
  text-align: center;
}
.stat-value {
  font-size: 16px;
  font-weight: 600;
  line-height: 1.2;
}
.stat-label {
  margin-top: 4px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
}
.groups-block {
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 8px;
  padding: 8px;
}
.block-title {
  font-size: 13px;
  font-weight: 600;
  margin-bottom: 6px;
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
.messages {
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
@media (max-width: 900px) {
  .usage-stats {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
