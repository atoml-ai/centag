<template>
  <div class="pricing-sync-panel">
    <el-alert type="info" :closable="false" class="help-alert" show-icon>
      <template #title>{{ t('pricingSync.help') }}</template>
    </el-alert>

    <el-card shadow="never" class="list-card">
      <template #header>
        <div class="card-header">
          <div class="card-title">
            <span>{{ t('billingRules.syncTabHint') }}</span>
          </div>
          <div class="header-actions">
            <el-button size="small" @click="load">{{ t('pricingSync.refresh') }}</el-button>
            <el-button size="small" type="primary" :loading="syncing" @click="runTrigger">
              {{ t('pricingSync.fetchAndSync') }}
            </el-button>
          </div>
        </div>
      </template>

      <el-table
        v-loading="loading"
        :data="batches"
        stripe
        :empty-text="t('pricingSync.emptyText')"
      >
        <el-table-column prop="id" :label="t('pricingSync.table.id')" width="70" />
        <el-table-column prop="source" :label="t('pricingSync.table.source')" min-width="140" show-overflow-tooltip />
        <el-table-column :label="t('pricingSync.table.status')" width="120">
          <template #default="{ row }">
            <el-tag :type="statusType(row.status)" size="small" effect="light">
              {{ t(`pricingSync.status.${row.status}`) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="t('pricingSync.table.createdAt')" width="180">
          <template #default="{ row }">
            {{ formatTime(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column prop="error" :label="t('pricingSync.table.error')" min-width="180" show-overflow-tooltip />
        <el-table-column :label="t('pricingSync.table.actions')" width="160" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openBatch(row)">{{ t('pricingSync.table.review') }}</el-button>
            <el-button link type="danger" @click="remove(row)">{{ t('pricingSync.table.delete') }}</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Review dialog: per-item confirm + apply/reject -->
    <el-dialog
      v-model="reviewVisible"
      :title="t('pricingSync.review.title')"
      width="1000px"
      class="review-dialog"
      destroy-on-close
      @open="loadBatch"
    >
      <template v-if="current">
        <div class="review-header">
          <div class="review-meta">
            <el-tag :type="statusType(current.status)" size="small" effect="light">
              {{ t(`pricingSync.status.${current.status}`) }}
            </el-tag>
            <span class="batch-source">{{ current.source }}</span>
            <span class="batch-time">{{ formatTime(current.created_at) }}</span>
          </div>
          <div class="currency-switch">
            <span class="currency-label">{{ t('pricingSync.currency') }}</span>
            <el-switch
              v-model="cny"
              inline-prompt
              :active-text="t('pricingSync.cny')"
              :inactive-text="t('pricingSync.usd')"
            />
          </div>
        </div>

        <el-alert type="info" :closable="false" class="price-hint" show-icon>
          <template #title>{{ t('pricingSync.review.priceHint') }}</template>
        </el-alert>

        <div class="summary-chips">
          <el-tag size="small" type="success" effect="plain">
            {{ t('pricingSync.review.resolvedCount', { count: resolvedCount }) }}
          </el-tag>
          <el-tag size="small" type="warning" effect="plain">
            {{ t('pricingSync.review.unresolvedCount', { count: unresolvedCount }) }}
          </el-tag>
          <el-tag size="small" type="primary" effect="plain">
            {{ t('pricingSync.review.selectedCount', { count: selectedRows.length }) }}
          </el-tag>
        </div>

        <el-alert v-if="unresolvedCount > 0" type="warning" :closable="false" class="unresolved-tip">
          <template #title>{{ t('pricingSync.review.unresolvedTip') }}</template>
        </el-alert>

        <el-table
          v-loading="detailLoading"
          :data="current.items"
          size="small"
          stripe
          row-key="id"
          :empty-text="t('pricingSync.review.emptyText')"
          @selection-change="onSelectionChange"
        >
          <el-table-column type="selection" width="50" :selectable="selectable" reserve-selection />
          <el-table-column prop="backend_id" :label="t('pricingSync.review.backend')" width="130" show-overflow-tooltip />
          <el-table-column prop="model" :label="t('pricingSync.review.model')" min-width="170" show-overflow-tooltip />
          <el-table-column :label="priceHeader('input')" width="150" align="right">
            <template #header>{{ priceHeader('input') }}</template>
            <template #default="{ row }">
              <span class="price-cell">{{ fmtPrice(row.input_price_per_m) }}</span>
            </template>
          </el-table-column>
          <el-table-column :label="priceHeader('output')" width="150" align="right">
            <template #header>{{ priceHeader('output') }}</template>
            <template #default="{ row }">
              <span class="price-cell">{{ fmtPrice(row.output_price_per_m) }}</span>
            </template>
          </el-table-column>
          <el-table-column :label="t('pricingSync.review.resolution')" width="110">
            <template #default="{ row }">
              <el-tooltip v-if="row.resolution === 'unresolved'" :content="row.source" placement="top">
                <el-tag type="warning" size="small">{{ t('pricingSync.review.unresolved') }}</el-tag>
              </el-tooltip>
              <el-tag v-else type="success" size="small">{{ t('pricingSync.review.resolved') }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="source" :label="t('pricingSync.review.source')" width="130" show-overflow-tooltip />
          <el-table-column :label="t('pricingSync.review.status')" width="100">
            <template #default="{ row }">
              <el-tag v-if="row.status === 'selected'" type="success" size="small">
                {{ t('pricingSync.review.selected') }}
              </el-tag>
              <el-tag v-else type="info" size="small">{{ t('pricingSync.review.unselected') }}</el-tag>
            </template>
          </el-table-column>
        </el-table>

        <el-alert type="info" :closable="false" class="usage-tip">
          <template #title>{{ t('pricingSync.review.usageTip') }}</template>
        </el-alert>
      </template>

      <template #footer>
        <div class="dialog-footer">
          <el-button @click="reviewVisible = false">{{ t('pricingSync.review.cancel') }}</el-button>
          <div class="footer-actions">
            <el-button
              type="warning"
              plain
              :disabled="current?.status !== 'pending'"
              @click="rejectCurrent"
            >
              {{ t('pricingSync.review.reject') }}
            </el-button>
            <el-button
              type="primary"
              :loading="applying"
              :disabled="current?.status !== 'pending'"
              @click="applyCurrent"
            >
              {{ t('pricingSync.review.apply') }}
            </el-button>
          </div>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import * as billingApi from '@/api/billing'
import type { PricingSyncBatch } from '@/api/billing'

const { t } = useI18n()

const emit = defineEmits<{ applied: [] }>()

// Display-only conversion: prices are stored in USD; CNY source prices were
// divided by usd_to_cny (default 7.2) at write time, so showing CNY means
// multiplying back. This never touches the stored rules.
const USD_TO_CNY = 7.2

const loading = ref(false)
const syncing = ref(false)
const detailLoading = ref(false)
const applying = ref(false)
const batches = ref<PricingSyncBatch[]>([])
const reviewVisible = ref(false)
const current = ref<PricingSyncBatch | null>(null)
const selectedRows = ref<PricingSyncBatch['items'][number][]>([])
const cny = ref(false)

const resolvedCount = computed(() => (current.value?.items ?? []).filter((i) => i.resolution !== 'unresolved').length)
const unresolvedCount = computed(() => (current.value?.items ?? []).filter((i) => i.resolution === 'unresolved').length)

async function load() {
  loading.value = true
  try {
    const data = await billingApi.listPricingSyncBatches()
    batches.value = Array.isArray(data) ? data : []
  } catch (e: unknown) {
    ElMessage.error(e instanceof Error ? e.message : t('pricingSync.message.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function runTrigger() {
  syncing.value = true
  try {
    const batch = await billingApi.triggerPricingSync({})
    if (batch?.id) {
      current.value = batch
      reviewVisible.value = true
      ElMessage.success(t('pricingSync.message.syncSuccess'))
    } else if ((batch as unknown as { items?: number })?.items === 0) {
      ElMessage.info(t('pricingSync.message.noProposed'))
    }
    await load()
  } catch (e: unknown) {
    ElMessage.error(e instanceof Error ? e.message : t('pricingSync.message.syncFailed'))
  } finally {
    syncing.value = false
  }
}

async function openBatch(row: PricingSyncBatch) {
  current.value = row
  reviewVisible.value = true
}

async function loadBatch() {
  if (!current.value?.id) return
  detailLoading.value = true
  try {
    const data = await billingApi.getPricingSyncBatch(current.value.id)
    current.value = data
  } catch (e: unknown) {
    ElMessage.error(e instanceof Error ? e.message : t('pricingSync.message.loadFailed'))
  } finally {
    detailLoading.value = false
  }
}

function selectable(row: PricingSyncBatch['items'][number]) {
  return current.value?.status === 'pending' && row.resolution !== 'unresolved'
}

function onSelectionChange(rows: PricingSyncBatch['items'][number][]) {
  selectedRows.value = rows ?? []
}

function selectedItemIds(): number[] {
  return selectedRows.value.map((r) => r.id ?? 0).filter((id) => id > 0)
}

async function saveSelection(ids: number[]) {
  if (!current.value?.id) return
  const updated = await billingApi.selectPricingSyncItems(current.value.id, ids)
  if (Array.isArray(updated)) {
    const byId = new Map(updated.map((it) => [it.id, it]))
    current.value.items = (current.value.items ?? []).map((it) => byId.get(it.id) ?? it)
  }
}

async function applyCurrent() {
  if (!current.value?.id) return
  applying.value = true
  try {
    const ids = selectedItemIds()
    await saveSelection(ids)
    const res = await billingApi.applyPricingSyncBatch(current.value.id)
    const msg =
      res.skipped && res.skipped > 0
        ? t('pricingSync.message.applySkipped', { count: res.applied ?? 0, skipped: res.skipped })
        : t('pricingSync.message.applySuccess', { count: res.applied ?? 0 })
    ElMessage.success(msg)
    reviewVisible.value = false
    await load()
    emit('applied')
  } catch (e: unknown) {
    ElMessage.error(e instanceof Error ? e.message : t('pricingSync.message.applyFailed'))
  } finally {
    applying.value = false
  }
}

async function rejectCurrent() {
  if (!current.value?.id) return
  try {
    await billingApi.rejectPricingSyncBatch(current.value.id)
    ElMessage.success(t('pricingSync.message.rejectSuccess'))
    reviewVisible.value = false
    await load()
  } catch (e: unknown) {
    ElMessage.error(e instanceof Error ? e.message : t('pricingSync.message.rejectFailed'))
  }
}

async function remove(row: PricingSyncBatch) {
  if (row.id == null) return
  try {
    await ElMessageBox.confirm(t('pricingSync.confirmDelete'), t('pricingSync.confirmDeleteTitle'))
    await billingApi.deletePricingSyncBatch(row.id)
    ElMessage.success(t('pricingSync.message.deleteSuccess'))
    await load()
  } catch {
    /* cancel */
  }
}

function statusType(status?: string): 'success' | 'warning' | 'info' | 'danger' {
  if (status === 'applied') return 'success'
  if (status === 'rejected') return 'danger'
  return 'warning'
}

function formatTime(v?: string): string {
  if (!v) return '-'
  const d = new Date(v)
  if (Number.isNaN(d.getTime())) return v
  return d.toLocaleString()
}

function priceUnit(): string {
  return cny.value ? '¥/M' : '$/M'
}

function priceHeader(kind: 'input' | 'output'): string {
  const base = kind === 'input' ? t('pricingSync.review.inputPrice') : t('pricingSync.review.outputPrice')
  return `${base} (${priceUnit()})`
}

function fmtPrice(v?: number): string {
  if (v == null) return '-'
  const n = cny.value ? v * USD_TO_CNY : v
  const rounded = Math.round(n * 10000) / 10000
  const sym = cny.value ? '¥' : '$'
  return sym + rounded.toLocaleString(undefined, { maximumFractionDigits: 4 })
}

onMounted(() => {
  load()
})
</script>

<style scoped>
.pricing-sync-panel {
  padding: 0;
  max-width: none;
}
.help-alert {
  margin-bottom: 16px;
}
.list-card {
  border-radius: 8px;
}
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}
.card-title {
  font-size: 16px;
  font-weight: 600;
}
.header-actions {
  display: flex;
  gap: 8px;
  align-items: center;
}
.review-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  margin-bottom: 14px;
}
.review-meta {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}
.batch-source {
  font-weight: 600;
  color: var(--el-text-color-primary);
  font-size: 14px;
}
.batch-time {
  color: var(--el-text-color-secondary);
  font-size: 13px;
}
.currency-switch {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}
.currency-label {
  font-size: 13px;
  color: var(--el-text-color-secondary);
}
.price-hint {
  margin-bottom: 12px;
}
.summary-chips {
  display: flex;
  gap: 8px;
  margin-bottom: 12px;
}
.unresolved-tip {
  margin-bottom: 12px;
}
.usage-tip {
  margin-top: 14px;
}
.price-cell {
  font-variant-numeric: tabular-nums;
  font-weight: 600;
  color: var(--el-color-primary);
}
.dialog-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  width: 100%;
}
.footer-actions {
  display: flex;
  gap: 8px;
}
</style>
