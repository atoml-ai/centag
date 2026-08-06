<template>
  <el-card class="list-card" v-loading="loading">
    <template #header>
      <div class="card-header">
        <span class="card-title">{{ t('cache.cacheList') }}</span>
        <el-button :loading="loading" @click="$emit('refresh')">
          <el-icon><Refresh /></el-icon>
          {{ t('cache.refresh') }}
        </el-button>
      </div>
    </template>

    <div class="filter-bar">
      <el-select v-model="local.type" style="width: 130px" @change="emitChange">
        <el-option :label="t('cache.filterOptions.allTypes')" value="all" />
        <el-option :label="t('cache.filterOptions.exact')" value="exact" />
        <el-option :label="t('cache.filterOptions.semantic')" value="semantic" />
        <el-option :label="t('cache.filterOptions.external')" value="external" />
      </el-select>
      <el-select v-model="local.save_only" style="width: 120px" @change="emitChange">
        <el-option :label="t('cache.filterOptions.allData')" value="all" />
        <el-option :label="t('cache.filterOptions.saveOnly')" value="save_only" />
        <el-option :label="t('cache.filterOptions.cacheData')" value="cache" />
      </el-select>
      <el-select v-model="local.storage" clearable style="width: 140px" @change="emitChange">
        <el-option :label="t('cache.filterOptions.allStorage')" value="all" />
        <el-option v-for="s in storages" :key="s.name" :label="s.name" :value="s.name" />
      </el-select>
      <el-input
        v-model="local.session_id"
        clearable
        :placeholder="t('cache.filterPlaceholder.session')"
        style="width: 160px"
        @change="emitChange"
        @keyup.enter="emitChange"
      />
      <el-input
        v-model="local.model"
        clearable
        :placeholder="t('cache.filterPlaceholder.model')"
        style="width: 140px"
        @change="emitChange"
        @keyup.enter="emitChange"
      />
      <el-input
        v-model="local.q"
        clearable
        :placeholder="t('cache.filterPlaceholder.keyword')"
        style="width: 180px"
        @change="emitChange"
        @keyup.enter="emitChange"
      />
      <el-date-picker
        v-model="dateRange"
        type="daterange"
        value-format="YYYY-MM-DD"
        :start-placeholder="t('cache.filterPlaceholder.from')"
        :end-placeholder="t('cache.filterPlaceholder.to')"
        style="width: 260px"
        @change="onDateChange"
      />
    </div>

    <el-empty
      v-if="!loading && rows.length === 0 && local.type === 'external'"
      :description="t('cache.empty.external')"
    />
    <el-empty
      v-else-if="!loading && rows.length === 0 && local.type === 'semantic'"
      :description="t('cache.empty.semantic')"
    />
    <el-table v-else :data="rows" stripe :max-height="tableMaxHeight">
      <el-table-column :label="t('cache.table.type')" width="90">
        <template #default="{ row }">
          <el-tag v-if="row.metadata?.save_only" type="warning" size="small">{{ t('cache.table.save') }}</el-tag>
          <el-tag v-else-if="row.cache_type === 'semantic'" type="success" size="small">S2</el-tag>
          <el-tag v-else-if="row.cache_type === 'external'" type="warning" size="small">S3</el-tag>
          <el-tag v-else type="primary" size="small">S1</el-tag>
        </template>
      </el-table-column>
      <el-table-column :label="t('cache.table.question')" min-width="180" show-overflow-tooltip>
        <template #default="{ row }">
          {{ row.question || row.request || row.metadata?.request_text || row.key }}
        </template>
      </el-table-column>
      <el-table-column :label="t('cache.table.session')" width="120" show-overflow-tooltip>
        <template #default="{ row }">
          {{ row.session_id || row.metadata?.session_id || '-' }}
        </template>
      </el-table-column>
      <el-table-column :label="t('cache.table.model')" width="130" show-overflow-tooltip>
        <template #default="{ row }">
          {{ row.model || row.metadata?.model || '-' }}
        </template>
      </el-table-column>
      <el-table-column prop="storage_backend" :label="t('cache.table.storage')" width="90">
        <template #default="{ row }">
          <el-tag v-if="row.storage_backend" type="info" size="small">{{ row.storage_backend }}</el-tag>
          <span v-else>-</span>
        </template>
      </el-table-column>
      <el-table-column :label="t('cache.table.createdAt')" width="150">
        <template #default="{ row }">{{ formatDate(row.timestamp) }}</template>
      </el-table-column>
      <el-table-column :label="t('cache.table.actions')" width="110" fixed="right">
        <template #default="{ row }">
          <el-button type="primary" size="small" link @click="$emit('view', row)">
            <el-icon><View /></el-icon>
            {{ t('cache.table.view') }}
          </el-button>
          <el-button type="danger" size="small" link @click="$emit('delete', row)">
            <el-icon><Delete /></el-icon>
          </el-button>
        </template>
      </el-table-column>
    </el-table>

    <div class="pagination-wrapper">
      <el-pagination
        v-model:current-page="local.page"
        v-model:page-size="local.size"
        :page-sizes="[10, 20, 50, 100]"
        :total="total"
        layout="total, sizes, prev, pager, next, jumper"
        @size-change="emitChange"
        @current-change="emitChange"
      />
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Refresh, Delete, View } from '@element-plus/icons-vue'
import type { CacheListParams } from '@/api'

const props = defineProps<{
  filters: CacheListParams & { page?: number; size?: number }
  rows: any[]
  total: number
  storages: any[]
  loading?: boolean
}>()

const emit = defineEmits<{
  'update:filters': [CacheListParams & { page?: number; size?: number }]
  refresh: []
  view: [row: any]
  delete: [row: any]
}>()

const { t } = useI18n()

const local = reactive({
  type: props.filters.type || 'all',
  save_only: props.filters.save_only || 'all',
  storage: props.filters.storage || 'all',
  session_id: props.filters.session_id || '',
  model: props.filters.model || '',
  q: props.filters.q || '',
  from: props.filters.from || '',
  to: props.filters.to || '',
  page: props.filters.page || 1,
  size: props.filters.size || 20
})

const dateRange = computed({
  get: () => (local.from && local.to ? [local.from, local.to] : null) as any,
  set: () => {}
})

watch(
  () => props.filters,
  (f) => {
    Object.assign(local, {
      type: f.type || 'all',
      save_only: f.save_only || 'all',
      storage: f.storage || 'all',
      session_id: f.session_id || '',
      model: f.model || '',
      q: f.q || '',
      from: f.from || '',
      to: f.to || '',
      page: f.page || 1,
      size: f.size || 20
    })
  },
  { deep: true }
)

const tableMaxHeight = computed(() => window.innerHeight - 360)

function emitChange() {
  emit('update:filters', { ...local })
}

function onDateChange(val: string[] | null) {
  if (val && val.length === 2) {
    local.from = val[0]
    local.to = val[1]
  } else {
    local.from = ''
    local.to = ''
  }
  local.page = 1
  emitChange()
}

function formatDate(timestamp: any) {
  if (!timestamp) return '-'
  return new Date(timestamp).toLocaleString()
}
</script>

<style scoped>
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.card-title {
  font-weight: 600;
}
.filter-bar {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 12px;
}
.list-card {
  min-height: 420px;
}
.pagination-wrapper {
  margin-top: 12px;
  display: flex;
  justify-content: flex-end;
}
</style>
