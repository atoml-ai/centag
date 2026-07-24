<template>
  <div class="cache">
    <div class="header">
      <h1 class="page-title">{{ t('cache.pageTitle') }}</h1>
      <p class="page-description">{{ t('cache.pageDescription') }}</p>
    </div>

    <div class="content-wrapper full-width">
      <el-row :gutter="16">
        <!-- 左侧：统计信息 -->
        <el-col :xs="24" :sm="24" :md="5" :lg="4" :xl="3">
          <div class="left-panel">
            <!-- 命中率展示 -->
            <el-card class="stats-card">
              <template #header>
                <span class="card-title">{{ t('cache.hitRate') }}</span>
              </template>
              <div class="stats-content">
                <el-progress
                  type="dashboard"
                  :percentage="parseFloat(hitRate)"
                  :color="hitRateColor"
                  :width="120"
                >
                  <template #default="{ percentage }">
                    <span class="percentage-value">{{ percentage }}%</span>
                  </template>
                </el-progress>
                <div class="stats-numbers">
                  <div class="stat-item">
                    <span class="stat-label">{{ t('cache.hits') }}</span>
                    <span class="stat-value hit">{{ formatNumber(cacheStats.hits) }}</span>
                  </div>
                  <div class="stat-item">
                    <span class="stat-label">{{ t('cache.misses') }}</span>
                    <span class="stat-value miss">{{ formatNumber(cacheStats.misses) }}</span>
                  </div>
                </div>
              </div>
            </el-card>

            <!-- 缓存状态 -->
            <el-card class="info-card">
              <template #header>
                <span class="card-title">{{ t('cache.cacheStatus') }}</span>
              </template>
              <div class="status-list">
                <div class="status-item">
                  <span class="status-label">{{ t('cache.enabledStatus') }}</span>
                  <el-tag :type="cacheStats.enabled ? 'success' : 'info'" size="small">
                    {{ cacheStats.enabled ? t('cache.enabled') : t('cache.disabled') }}
                  </el-tag>
                </div>
                <div class="status-item">
                  <span class="status-label">{{ t('cache.cacheType') }}</span>
                  <span>{{ cacheStats.type || t('cache.memoryCache') }}</span>
                </div>
                <div class="status-item">
                  <span class="status-label">{{ t('cache.expiry') }}</span>
                  <span>{{ cacheStats.ttl ? `${cacheStats.ttl}s` : t('cache.permanent') }}</span>
                </div>
                <div class="status-item">
                  <span class="status-label">{{ t('cache.maxEntries') }}</span>
                  <span>{{ cacheStats.max_items || t('cache.unlimited') }}</span>
                </div>
              </div>
            </el-card>

            <!-- 操作按钮 -->
            <el-card class="actions-card">
              <el-button :loading="loading || listLoading" @click="refreshAll" style="width: 100%">
                <el-icon><Refresh /></el-icon>
                {{ t('cache.refreshData') }}
              </el-button>
              <el-button
                type="danger"
                :loading="clearing"
                @click="handleClear"
                style="width: 100%; margin-left: 0; margin-top: 8px"
              >
                <el-icon><Delete /></el-icon>
                {{ t('cache.clearCache') }}
              </el-button>
            </el-card>
          </div>
        </el-col>

        <!-- 右侧：缓存列表 -->
        <el-col :xs="24" :sm="24" :md="19" :lg="20" :xl="21">
          <el-card class="list-card" v-loading="listLoading">
            <template #header>
              <div class="card-header">
                <span class="card-title">{{ t('cache.cacheList') }}</span>
                <div class="filter-bar">
                  <el-select v-model="cacheType" :placeholder="t('cache.filterPlaceholder.cacheType')" style="width: 120px" @change="loadCacheList">
                    <el-option :label="t('cache.filterOptions.allTypes')" value="all"></el-option>
                    <el-option :label="t('cache.filterOptions.exact')" value="exact"></el-option>
                    <el-option :label="t('cache.filterOptions.semantic')" value="semantic"></el-option>
                  </el-select>
                  <el-select v-model="saveOnlyFilter" :placeholder="t('cache.filterPlaceholder.dataSource')" style="width: 120px; margin-left: 8px" @change="loadCacheList">
                    <el-option :label="t('cache.filterOptions.allData')" value="all"></el-option>
                    <el-option :label="t('cache.filterOptions.saveOnly')" value="save_only"></el-option>
                    <el-option :label="t('cache.filterOptions.cacheData')" value="cache"></el-option>
                  </el-select>
                  <el-select v-model="storageFilter" :placeholder="t('cache.filterPlaceholder.storage')" style="width: 140px; margin-left: 8px" @change="loadCacheList" clearable>
                    <el-option :label="t('cache.filterOptions.allStorage')" value="all"></el-option>
                    <el-option v-for="storage in storages" :key="storage.name" :label="storage.name" :value="storage.name">
                      <span>{{ storage.name }}</span>
                      <el-tag v-if="storage.is_default" type="success" size="small" style="margin-left: 4px">{{ t('cache.filterOptions.default') }}</el-tag>
                    </el-option>
                  </el-select>
                  <el-button :loading="listLoading || loading" @click="refreshAll" style="margin-left: 8px">
                    <el-icon><Refresh /></el-icon>
                    {{ t('cache.refresh') }}
                  </el-button>
                </div>
              </div>
            </template>
            <el-table :data="cacheList" stripe :max-height="tableMaxHeight">
              <el-table-column prop="cache_type" :label="t('cache.table.type')" width="70">
                <template #default="{ row }">
                  <el-tag v-if="row.metadata?.save_only" type="warning" size="small">
                    {{ t('cache.table.save') }}
                  </el-tag>
                  <el-tag v-else :type="row.cache_type === 'exact' ? 'primary' : 'success'" size="small">
                    {{ row.cache_type === 'exact' ? t('cache.table.exact') : t('cache.table.semantic') }}
                  </el-tag>
                </template>
              </el-table-column>
              <!-- 问题列 -->
              <el-table-column :label="t('cache.table.question')" min-width="200" show-overflow-tooltip>
                <template #default="{ row }">
                  <span :title="row.question || row.key">{{ row.question || row.key }}</span>
                </template>
              </el-table-column>
              <el-table-column :label="t('cache.table.cacheKey')" width="140" show-overflow-tooltip>
                <template #default="{ row }">
                  <el-text type="info" size="small" style="font-family: monospace; font-size: 11px;">
                    {{ row.key }}
                  </el-text>
                </template>
              </el-table-column>
              <el-table-column prop="model" :label="t('cache.table.model')" width="140" show-overflow-tooltip></el-table-column>
              <el-table-column prop="storage_backend" :label="t('cache.table.storage')" width="70">
                <template #default="{ row }">
                  <el-tag v-if="row.storage_backend" type="info" size="small">
                    {{ row.storage_backend }}
                  </el-tag>
                  <span v-else>-</span>
                </template>
              </el-table-column>
              <el-table-column prop="timestamp" :label="t('cache.table.createdAt')" width="150">
                <template #default="{ row }">
                  {{ formatDate(row.timestamp) }}
                </template>
              </el-table-column>
              <el-table-column prop="similarity" :label="t('cache.table.similarity')" width="70">
                <template #default="{ row }">
                  <span v-if="row.similarity !== null && row.similarity !== undefined">{{ (row.similarity * 100).toFixed(1) }}%</span>
                  <span v-else>-</span>
                </template>
              </el-table-column>
              <el-table-column :label="t('cache.table.actions')" width="100" fixed="right">
                <template #default="{ row }">
                  <el-button type="primary" size="small" link @click="handleViewDetail(row)">
                    <el-icon><View /></el-icon>
                    {{ t('cache.table.view') }}
                  </el-button>
                  <el-button type="danger" size="small" link @click="handleDeleteEntry(row)">
                    <el-icon><Delete /></el-icon>
                  </el-button>
                </template>
              </el-table-column>
            </el-table>
            <div class="pagination-wrapper">
              <el-pagination
                v-model:current-page="pagination.page"
                v-model:page-size="pagination.size"
                :page-sizes="[10, 20, 50, 100]"
                :total="pagination.total"
                layout="total, sizes, prev, pager, next, jumper"
                @size-change="loadCacheList"
                @current-change="loadCacheList"
              />
            </div>
          </el-card>
        </el-col>
      </el-row>

      <!-- 详情对话框 -->
      <el-dialog
        v-model="detailDialogVisible"
        :title="t('cache.detailDialog.title')"
        width="700px"
        :close-on-click-modal="true"
      >
        <div v-loading="detailLoading">
          <template v-if="detailData">
            <el-descriptions :column="2" border>
              <el-descriptions-item :label="t('cache.detailDialog.cacheKey')">
                <el-text style="font-family: monospace; font-size: 12px;">{{ detailData.key }}</el-text>
              </el-descriptions-item>
              <el-descriptions-item :label="t('cache.detailDialog.type')">
                <el-tag v-if="detailData.metadata?.save_only" type="warning" size="small">
                  {{ t('cache.detailDialog.saveOnly') }}
                </el-tag>
                <el-tag v-else :type="detailData.cache_type === 'exact' ? 'primary' : 'success'" size="small">
                  {{ detailData.cache_type === 'exact' ? t('cache.detailDialog.exactMatch') : t('cache.detailDialog.semanticMatch') }}
                </el-tag>
              </el-descriptions-item>
              <el-descriptions-item :label="t('cache.detailDialog.model')">{{ detailData.model }}</el-descriptions-item>
              <el-descriptions-item :label="t('cache.detailDialog.storage')">{{ detailData.storage_backend }}</el-descriptions-item>
              <el-descriptions-item :label="t('cache.detailDialog.createdAt')">{{ formatDate(detailData.timestamp) }}</el-descriptions-item>
              <el-descriptions-item :label="t('cache.detailDialog.expiresAt')">
                {{ detailData.expires_at ? formatDate(detailData.expires_at) : t('cache.detailDialog.neverExpires') }}
              </el-descriptions-item>
              <el-descriptions-item :label="t('cache.detailDialog.similarity')" v-if="detailData.similarity !== undefined && detailData.similarity !== null">
                {{ (detailData.similarity * 100).toFixed(2) }}%
              </el-descriptions-item>
            </el-descriptions>

            <el-divider content-position="left">{{ t('cache.detailDialog.questionDivider') }}</el-divider>
            <div class="detail-content">
              <pre>{{ detailData.question }}</pre>
            </div>

            <el-divider content-position="left">{{ t('cache.detailDialog.responseDivider') }}</el-divider>
            <div class="detail-content">
              <pre>{{ detailData.response }}</pre>
            </div>

            <template v-if="detailData.metadata && Object.keys(detailData.metadata).length > 0">
              <el-divider content-position="left">{{ t('cache.detailDialog.metadataDivider') }}</el-divider>
              <div class="detail-content">
                <pre>{{ formatJson(detailData.metadata) }}</pre>
              </div>
            </template>
          </template>
          <el-empty v-else-if="!detailLoading" :description="t('cache.detailDialog.noData')" />
        </div>
        <template #footer>
          <el-button @click="detailDialogVisible = false">{{ t('cache.detailDialog.close') }}</el-button>
        </template>
      </el-dialog>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Collection, CircleCheck, CircleClose, Coin, Refresh, Delete, View } from '@element-plus/icons-vue'
import { getCacheStats, clearCache, getCacheList, deleteCacheEntry, getStorages } from '@/api'
import { formatNumber, formatBytes } from '@/utils/format'

const { t } = useI18n()

const loading = ref(false)
const clearing = ref(false)
const listLoading = ref(false)
const cacheType = ref('all')
const saveOnlyFilter = ref('all')
const storageFilter = ref('all')
const storages = ref<any[]>([])
const cacheList = ref<any[]>([])
const pagination = ref({
  page: 1,
  size: 20,
  total: 0
})

// 详情对话框
const detailDialogVisible = ref(false)
const detailLoading = ref(false)
const detailData = ref<any>(null)

// 动态计算表格最大高度
const tableMaxHeight = computed(() => {
  return window.innerHeight - 280
})

const cacheStats = ref<any>({
  total: 0,
  hits: 0,
  misses: 0,
  size: 0,
  enabled: true,
  type: 'memory',
  ttl: 3600,
  max_items: 10000,
  max_size: 104857600 // 100MB
})

const hitRate = computed(() => {
  // 优先使用后端返回的hit_rate
  if (cacheStats.value.hit_rate !== undefined) {
    return cacheStats.value.hit_rate.toFixed(2)
  }
  // 否则计算
  const total = cacheStats.value.hits + cacheStats.value.misses
  if (!total) return '0.00'
  return ((cacheStats.value.hits / total) * 100).toFixed(2)
})

const hitRateColor = computed(() => {
  const rate = parseFloat(hitRate.value)
  if (rate >= 80) return '#10b981'
  if (rate >= 50) return '#f59e0b'
  return '#ef4444'
})

async function load() {
  loading.value = true
  try {
    const res = await getCacheStats()
    console.log('Cache stats response:', res)

    // res 现在是 {success: true, data: {...}} 格式
    const data = res?.data || res

    // 根据后端返回的CacheStats结构解析
    cacheStats.value = {
      total: data.total_entries || 0,
      hits: data.hits || 0,
      misses: data.misses || 0,
      size: 0, // CacheStats中没有size字段
      enabled: true,
      type: 'memory',
      ttl: 3600,
      max_items: 10000,
      max_size: 104857600,
      hit_rate: data.hit_rate_percent || 0
    }
  } catch (error: any) {
    console.error('Failed to load cache stats:', error)
    ElMessage.error(t('cache.message.loadFailed') + ': ' + (error.message || t('cache.message.unknownError')))
  } finally {
    loading.value = false
  }
}

async function refreshAll() {
  await Promise.all([load(), loadStorageList(), loadCacheList()])
}

async function loadStorageList() {
  try {
    const res = await getStorages()
    const data = res?.data || res
    storages.value = Array.isArray(data?.storages) ? data.storages : []
  } catch (error: any) {
    console.error('Failed to load storage list:', error)
  }
}

async function loadCacheList() {
  listLoading.value = true
  try {
    const res = await getCacheList({
      page: pagination.value.page,
      size: pagination.value.size,
      type: cacheType.value,
      save_only: saveOnlyFilter.value,
      storage: storageFilter.value
    })
    const data = res?.data || res
    cacheList.value = data.entries || []
    pagination.value.total = data.total_count ?? data.total ?? 0
  } catch (error: any) {
    console.error('Failed to load cache list:', error)
    ElMessage.error(t('cache.message.loadListFailed') + ': ' + (error.message || t('cache.message.unknownError')))
  } finally {
    listLoading.value = false
  }
}

async function handleClear() {
  try {
    await ElMessageBox.confirm(
      t('cache.confirm.clearMessage'),
      t('cache.confirm.clearTitle'),
      {
        type: 'warning',
        confirmButtonText: t('cache.confirm.confirm'),
        cancelButtonText: t('cache.confirm.cancel')
      }
    )

    clearing.value = true
    await clearCache()
    ElMessage.success(t('cache.message.cleared'))
    await load()
    await loadCacheList()
  } catch (error: any) {
    if (error !== 'cancel') {
      console.error('Failed to clear cache:', error)
      ElMessage.error(t('cache.message.clearFailed') + ': ' + (error.message || t('cache.message.unknownError')))
    }
  } finally {
    clearing.value = false
  }
}

async function handleDeleteEntry(row: any) {
  try {
    await ElMessageBox.confirm(
      t('cache.confirm.deleteMessage'),
      t('cache.confirm.deleteTitle'),
      {
        type: 'warning',
        confirmButtonText: t('cache.confirm.confirm'),
        cancelButtonText: t('cache.confirm.cancel')
      }
    )
    await deleteCacheEntry(row.key)
    ElMessage.success(t('cache.message.deleted'))
    await load()
    await loadCacheList()
  } catch (error: any) {
    if (error !== 'cancel') {
      console.error('Failed to delete entry:', error)
      ElMessage.error(t('cache.message.deleteFailed') + ': ' + (error.message || t('cache.message.unknownError')))
    }
  }
}

async function handleViewDetail(row: any) {
  detailDialogVisible.value = true
  detailLoading.value = true
  detailData.value = null
  
  try {
    // 直接使用行数据，补充格式化信息
    detailData.value = {
      key: row.key,
      question: row.question || row.key,
      response: row.response || '',
      model: row.model || '-',
      cache_type: row.cache_type || 'exact',
      storage_backend: row.storage_backend || '-',
      timestamp: row.timestamp,
      expires_at: row.expires_at,
      similarity: row.similarity,
      metadata: row.metadata || {}
    }
  } catch (error: any) {
    console.error('Failed to load detail:', error)
    ElMessage.error(t('cache.message.detailLoadFailed') + ': ' + (error.message || t('cache.message.unknownError')))
  } finally {
    detailLoading.value = false
  }
}

function formatJson(value: any): string {
  if (!value) return '-'
  if (typeof value === 'string') {
    try {
      return JSON.stringify(JSON.parse(value), null, 2)
    } catch {
      return value
    }
  }
  return JSON.stringify(value, null, 2)
}

function formatDate(timestamp: any) {
  if (!timestamp) return '-'
  const date = new Date(timestamp)
  return date.toLocaleString('zh-CN')
}

onMounted(() => {
  load()
  loadStorageList()
  loadCacheList()
})
</script>

<style scoped>
.content-wrapper.full-width {
  max-width: 100%;
  padding: 0 16px;
}

.content-wrapper.full-width :deep(.el-row) {
  width: 100%;
}

.left-panel {
  display: flex;
  flex-direction: column;
  gap: 12px;
  height: calc(100vh - 240px);
}

.stats-card,
.info-card,
.actions-card {
  border: 1px solid var(--color-gray-200);
  box-shadow: var(--shadow-sm);
}

.stats-card :deep(.el-card__body) {
  padding: 16px;
}

.stats-content {
  display: flex;
  flex-direction: column;
  align-items: center;
}

.stats-numbers {
  display: flex;
  justify-content: space-around;
  width: 100%;
  margin-top: 12px;
}

.stat-item {
  display: flex;
  flex-direction: column;
  align-items: center;
}

.stat-label {
  font-size: 12px;
  color: var(--color-gray-600);
}

.stat-value {
  font-size: 16px;
  font-weight: 600;
}

.stat-value.hit {
  color: var(--color-success);
}

.stat-value.miss {
  color: var(--color-error);
}

.status-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.status-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 13px;
}

.status-label {
  color: var(--color-gray-600);
}

.card-title {
  font-weight: 600;
  color: var(--color-gray-900);
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.filter-bar {
  display: flex;
  align-items: center;
}

.percentage-value {
  font-size: 1rem;
  font-weight: 600;
  color: var(--color-gray-900);
}

.list-card {
  height: calc(100vh - 240px);
  display: flex;
  flex-direction: column;
}

.list-card :deep(.el-card__body) {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.list-card :deep(.el-table) {
  flex: 1;
}

.pagination-wrapper {
  margin-top: var(--spacing-md);
  padding-top: var(--spacing-md);
  border-top: 1px solid var(--color-gray-200);
  display: flex;
  justify-content: flex-end;
}

@media (max-width: 1024px) {
  .left-panel {
    height: auto;
  }

  .list-card {
    height: 500px;
  }
}

@media (max-width: 768px) {
  .content-wrapper.full-width :deep(.el-col) {
    width: 100% !important;
    flex: 0 0 100% !important;
  }

  .chart-legend {
    flex-direction: column;
    gap: var(--spacing-xs);
  }
}

.detail-content {
  background: var(--color-gray-50);
  border: 1px solid var(--color-gray-200);
  border-radius: 4px;
  padding: 12px;
  max-height: 300px;
  overflow: auto;
}

.detail-content pre {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-all;
  font-family: 'Monaco', 'Menlo', 'Consolas', monospace;
  font-size: 13px;
  line-height: 1.5;
}
</style>
