<template>
  <div class="cache">
    <div class="header">
      <h1 class="page-title">{{ t('cache.pageTitle') }}</h1>
      <p class="page-description">{{ t('cache.pageDescription') }}</p>
    </div>

    <el-tabs v-model="activeTab" class="cache-tabs" @tab-change="onTabChange">
      <el-tab-pane :label="t('cache.tabs.data')" name="data">
        <el-row :gutter="16">
          <el-col :xs="24" :md="5" :lg="4">
            <CacheStatsPanel
              :stats="cacheStats"
              :backend="activeBackend"
              :loading="loading || listLoading"
              :clearing="clearing"
              @refresh="refreshAll"
              @clear="handleClear"
            />
          </el-col>
          <el-col :xs="24" :md="19" :lg="20">
            <CacheDataTable
              v-model:filters="filters"
              :rows="cacheList"
              :total="pagination.total"
              :storages="storages"
              :loading="listLoading"
              @refresh="refreshAll"
              @view="handleViewDetail"
              @delete="handleDeleteEntry"
            />
          </el-col>
        </el-row>
      </el-tab-pane>

      <el-tab-pane :label="t('cache.tabs.stats')" name="stats">
        <el-row :gutter="16">
          <el-col :xs="24" :md="8">
            <CacheStatsPanel
              :stats="cacheStats"
              :backend="activeBackend"
              :loading="loading"
              :clearing="clearing"
              @refresh="loadStats"
              @clear="handleClear"
            />
          </el-col>
          <el-col :xs="24" :md="16">
            <el-card>
              <template #header>
                <span class="card-title">{{ t('cache.tabs.stats') }}</span>
              </template>
              <el-descriptions :column="2" border>
                <el-descriptions-item :label="t('cache.hits')">{{ formatNumber(cacheStats.hits) }}</el-descriptions-item>
                <el-descriptions-item :label="t('cache.misses')">{{ formatNumber(cacheStats.misses) }}</el-descriptions-item>
                <el-descriptions-item :label="t('cache.backendLabel')">{{ activeBackend || 'exact' }}</el-descriptions-item>
                <el-descriptions-item :label="t('cache.listTotal')">{{ pagination.total }}</el-descriptions-item>
              </el-descriptions>
            </el-card>
          </el-col>
        </el-row>
      </el-tab-pane>

      <el-tab-pane :label="t('cache.tabs.config')" name="config">
        <CacheConfigPanel @saved="onConfigSaved" />
      </el-tab-pane>
    </el-tabs>

    <CacheEntryDrawer
      v-model:visible="detailVisible"
      :loading="detailLoading"
      :data="detailData"
      @delete="handleDeleteEntry"
    />
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  clearCache,
  deleteCacheEntry,
  getCacheEntry,
  getCacheList,
  getCacheStats,
  getConfig,
  getStorages,
  type CacheListParams
} from '@/api'
import { formatNumber } from '@/utils/format'
import CacheStatsPanel from '@/components/cache/CacheStatsPanel.vue'
import CacheDataTable from '@/components/cache/CacheDataTable.vue'
import CacheEntryDrawer from '@/components/cache/CacheEntryDrawer.vue'
import CacheConfigPanel from '@/components/cache/CacheConfigPanel.vue'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()

const activeTab = ref('data')
const activeBackend = ref('exact')
const loading = ref(false)
const listLoading = ref(false)
const clearing = ref(false)
const storages = ref<any[]>([])
const cacheList = ref<any[]>([])
const pagination = reactive({ total: 0 })
const detailVisible = ref(false)
const detailLoading = ref(false)
const detailData = ref<any>(null)

const filters = reactive<CacheListParams & { page: number; size: number }>({
  type: 'all',
  save_only: 'all',
  storage: 'all',
  session_id: '',
  model: '',
  q: '',
  from: '',
  to: '',
  page: 1,
  size: 20
})

const cacheStats = ref<any>({
  hits: 0,
  misses: 0,
  enabled: true,
  ttl: 3600,
  max_items: 10000,
  hit_rate: 0
})

function syncQueryToFilters() {
  const q = route.query
  if (typeof q.tab === 'string' && ['data', 'stats', 'config'].includes(q.tab)) {
    activeTab.value = q.tab
  }
  if (typeof q.session_id === 'string') filters.session_id = q.session_id
  if (typeof q.model === 'string') filters.model = q.model
  if (typeof q.type === 'string') filters.type = q.type
  if (typeof q.q === 'string') filters.q = q.q
}

function pushQuery() {
  const query: Record<string, string> = { tab: activeTab.value }
  if (filters.session_id) query.session_id = filters.session_id
  if (filters.model) query.model = filters.model
  if (filters.type && filters.type !== 'all') query.type = filters.type
  if (filters.q) query.q = filters.q
  router.replace({ path: '/cache', query })
}

function onTabChange() {
  pushQuery()
}

async function loadStats() {
  loading.value = true
  try {
    const res = await getCacheStats()
    const data = res?.data || res
    cacheStats.value = {
      hits: data.hits || 0,
      misses: data.misses || 0,
      enabled: true,
      ttl: 3600,
      max_items: 10000,
      hit_rate: data.hit_rate_percent || 0
    }
  } catch (error: any) {
    ElMessage.error(t('cache.message.loadFailed') + ': ' + (error.message || t('cache.message.unknownError')))
  } finally {
    loading.value = false
  }
}

async function loadBackend() {
  try {
    const res = await getConfig()
    const data = res?.data || res
    activeBackend.value = data?.cache?.backend || data?.cache?.strategy || 'exact'
  } catch {
    /* ignore */
  }
}

async function loadStorageList() {
  try {
    const res = await getStorages()
    const data = res?.data || res
    storages.value = Array.isArray(data?.storages) ? data.storages : []
  } catch {
    /* ignore */
  }
}

async function loadCacheList() {
  listLoading.value = true
  try {
    const res = await getCacheList({
      page: filters.page,
      size: filters.size,
      type: filters.type,
      save_only: filters.save_only,
      storage: filters.storage,
      session_id: filters.session_id || undefined,
      model: filters.model || undefined,
      q: filters.q || undefined,
      from: filters.from || undefined,
      to: filters.to || undefined
    })
    const data = res?.data || res
    cacheList.value = data.entries || []
    pagination.total = data.total_count ?? data.total ?? 0
  } catch (error: any) {
    ElMessage.error(t('cache.message.loadListFailed') + ': ' + (error.message || t('cache.message.unknownError')))
  } finally {
    listLoading.value = false
  }
}

async function refreshAll() {
  await Promise.all([loadStats(), loadStorageList(), loadCacheList(), loadBackend()])
}

async function handleClear() {
  try {
    await ElMessageBox.confirm(t('cache.confirm.clearMessage'), t('cache.confirm.clearTitle'), {
      type: 'warning',
      confirmButtonText: t('cache.confirm.confirm'),
      cancelButtonText: t('cache.confirm.cancel')
    })
    clearing.value = true
    await clearCache()
    ElMessage.success(t('cache.message.cleared'))
    await refreshAll()
  } catch (error: any) {
    if (error !== 'cancel') {
      ElMessage.error(t('cache.message.clearFailed') + ': ' + (error.message || t('cache.message.unknownError')))
    }
  } finally {
    clearing.value = false
  }
}

async function handleDeleteEntry(row: any) {
  if (!row?.key) return
  try {
    await ElMessageBox.confirm(t('cache.confirm.deleteMessage'), t('cache.confirm.deleteTitle'), {
      type: 'warning',
      confirmButtonText: t('cache.confirm.confirm'),
      cancelButtonText: t('cache.confirm.cancel')
    })
    await deleteCacheEntry(row.key, row.cache_type)
    ElMessage.success(t('cache.message.deleted'))
    detailVisible.value = false
    await refreshAll()
  } catch (error: any) {
    if (error !== 'cancel') {
      ElMessage.error(t('cache.message.deleteFailed') + ': ' + (error.message || t('cache.message.unknownError')))
    }
  }
}

async function handleViewDetail(row: any) {
  detailVisible.value = true
  detailLoading.value = true
  detailData.value = null
  try {
    const res = await getCacheEntry({ key: row.key, type: row.cache_type || 'exact' })
    const data = res?.data || res
    detailData.value = {
      ...row,
      ...(data || {}),
      question: data?.request || row.question || row.request || row.metadata?.request_text || row.key
    }
  } catch {
    detailData.value = {
      ...row,
      question: row.question || row.request || row.metadata?.request_text || row.key
    }
  } finally {
    detailLoading.value = false
  }
}

function onConfigSaved(backend: string) {
  activeBackend.value = backend || activeBackend.value
  loadStats()
}

watch(
  filters,
  () => {
    pushQuery()
    loadCacheList()
  },
  { deep: true }
)

onMounted(async () => {
  syncQueryToFilters()
  await refreshAll()
})
</script>

<style scoped>
.cache-tabs {
  margin-top: 8px;
}
.card-title {
  font-weight: 600;
}
</style>
