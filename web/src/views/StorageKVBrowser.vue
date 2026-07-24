<template>
  <div class="kv-browser">
    <el-card>
      <template #header>
        <div class="card-header">
          <span>{{ t('storageKVBrowser.kvDataBrowse') }}</span>
          <el-tag v-if="defaultKV" type="success" size="small">{{ t('storageKVBrowser.defaultStorage', { name: defaultKV }) }}</el-tag>
        </div>
      </template>

      <el-alert type="info" :closable="false" style="margin-bottom: 16px">
        {{ t('storageKVBrowser.browseTip') }}
      </el-alert>

      <!-- 工具栏 -->
      <div class="toolbar">
        <el-input
          v-model="filterPattern"
          :placeholder="t('storageKVBrowser.filterPlaceholder')"
          clearable
          style="width: 360px"
          @clear="loadKeys"
          @keyup.enter="loadKeys"
        >
          <template #prepend>pattern</template>
        </el-input>

        <el-button type="primary" @click="loadKeys" :loading="loading">
          <el-icon><Search /></el-icon> {{ t('storageKVBrowser.query') }}
        </el-button>

        <el-button @click="loadKeys" :loading="loading">
          <el-icon><Refresh /></el-icon> {{ t('storageKVBrowser.refresh') }}
        </el-button>

        <div style="flex: 1"></div>

        <span class="total-info">{{ t('storageKVBrowser.totalItems', { n: total }) }}</span>
      </div>

      <!-- 键列表 -->
      <el-table
        :data="keys.slice(pageStart, pageEnd)"
        v-loading="loading"
        stripe
        highlight-current-row
        @row-click="onKeyClick"
        style="margin-top: 16px; cursor: pointer"
        max-height="400"
      >
        <el-table-column prop="key" :label="t('storageKVBrowser.keyName')" min-width="300" show-overflow-tooltip />
        <el-table-column :label="t('storageKVBrowser.delete')" width="120" fixed="right">
          <template #default="{ row }">
            <el-button type="danger" text size="small" @click.stop="deleteKey(row.key)">
              <el-icon><Delete /></el-icon> {{ t('storageKVBrowser.delete') }}
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <!-- 分页 -->
      <div class="pagination" v-if="total > pageSize">
        <el-pagination
          v-model:current-page="currentPage"
          :page-size="pageSize"
          layout="prev, pager, next"
          :total="total"
          background
          small
        />
      </div>
    </el-card>

    <!-- 值详情 -->
    <el-card v-if="selectedKey" style="margin-top: 16px">
      <template #header>
        <div class="card-header">
          <span>{{ t('storageKVBrowser.keyDetail', { key: selectedKey }) }}</span>
          <el-button type="primary" text size="small" @click="copyValue">
            <el-icon><CopyDocument /></el-icon> {{ t('storageKVBrowser.copy') }}
          </el-button>
        </div>
      </template>

      <el-descriptions :column="2" border size="small" style="margin-bottom: 12px">
        <el-descriptions-item :label="t('storageKVBrowser.keyName')">{{ selectedKey }}</el-descriptions-item>
        <el-descriptions-item :label="t('storageKVBrowser.ttl')">{{ formatTTL(selectedTTL) }}</el-descriptions-item>
      </el-descriptions>

      <div class="value-section">
        <div class="value-label">{{ t('storageKVBrowser.valueJson') }}</div>
        <pre class="value-content">{{ formattedValue }}</pre>
      </div>
    </el-card>

    <!-- 空状态 -->
    <el-empty v-if="!loading && keys.length === 0 && !selectedKey" :description="t('storageKVBrowser.noData')" />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Search, Refresh, Delete, CopyDocument } from '@element-plus/icons-vue'
import { listKVKeys, getKVValue, deleteKVKey, getStorages } from '@/api'

const { t } = useI18n()

const loading = ref(false)
const keys = ref<{ key: string }[]>([])
const total = ref(0)
const filterPattern = ref('pipeline:*')
const selectedKey = ref('')
const selectedValue = ref<any>(null)
const selectedTTL = ref(0)
const defaultKV = ref('')

const pageSize = 20
const currentPage = ref(1)

const pageStart = computed(() => (currentPage.value - 1) * pageSize)
const pageEnd = computed(() => currentPage.value * pageSize)

const formattedValue = computed(() => {
  if (selectedValue.value == null) return ''
  try {
    return JSON.stringify(selectedValue.value, null, 2)
  } catch {
    return String(selectedValue.value)
  }
})

onMounted(async () => {
  try {
    const res: any = await getStorages()
    defaultKV.value = res?.default_kv || ''
  } catch { /* ignore */ }
  loadKeys()
})

async function loadKeys() {
  loading.value = true
  try {
    const params: any = {}
    if (filterPattern.value) params.pattern = filterPattern.value
    const res: any = await listKVKeys(params)
    keys.value = (res?.keys || []).map((k: string) => ({ key: k }))
    total.value = res?.total || 0
    currentPage.value = 1
  } catch (e: any) {
    ElMessage.error(t('storageKVBrowser.loadKeysFailed') + ': ' + (e?.response?.data?.message || e.message))
    keys.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

async function onKeyClick(row: { key: string }) {
  try {
    const res: any = await getKVValue({ key: row.key })
    selectedKey.value = row.key
    selectedValue.value = res?.value
    selectedTTL.value = res?.ttl_seconds || 0
  } catch (e: any) {
    ElMessage.error(t('storageKVBrowser.getValueFailed') + ': ' + (e?.response?.data?.message || e.message))
  }
}

async function deleteKey(key: string) {
  try {
    await ElMessageBox.confirm(t('storageKVBrowser.deleteKeyConfirm', { key }), t('storageKVBrowser.deleteConfirmTitle'), {
      type: 'warning',
    })
    await deleteKVKey({ key })
    ElMessage.success(t('storageKVBrowser.deleteSuccess'))
    if (selectedKey.value === key) {
      selectedKey.value = ''
      selectedValue.value = null
    }
    loadKeys()
  } catch (e: any) {
    if (e !== 'cancel') {
      ElMessage.error(t('storageKVBrowser.deleteFailed') + ': ' + (e?.response?.data?.message || e.message))
    }
  }
}

function copyValue() {
  if (formattedValue.value) {
    navigator.clipboard.writeText(formattedValue.value)
    ElMessage.success(t('storageKVBrowser.copiedToClipboard'))
  }
}

function formatTTL(seconds: number): string {
  if (seconds <= 0) return t('storageKVBrowser.permanent')
  if (seconds < 60) return t('storageKVBrowser.formatSeconds', { n: seconds })
  if (seconds < 3600) return t('storageKVBrowser.formatMinutes', { n: Math.floor(seconds / 60) })
  if (seconds < 86400) return t('storageKVBrowser.formatHours', { n: Math.floor(seconds / 3600) })
  return t('storageKVBrowser.formatDays', { n: Math.floor(seconds / 86400) })
}
</script>

<style scoped>
.kv-browser {
  padding: 16px;
  max-width: 1200px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.toolbar {
  display: flex;
  gap: 12px;
  align-items: center;
  flex-wrap: wrap;
}

.total-info {
  color: var(--el-text-color-secondary);
  font-size: 13px;
}

.pagination {
  display: flex;
  justify-content: flex-end;
  margin-top: 12px;
}

.value-section {
  background: var(--el-fill-color-light);
  border-radius: 6px;
  padding: 12px;
}

.value-label {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-bottom: 8px;
}

.value-content {
  margin: 0;
  padding: 12px;
  background: var(--el-bg-color);
  border-radius: 4px;
  font-size: 13px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-all;
  max-height: 600px;
  overflow: auto;
  font-family: 'SF Mono', 'Menlo', 'Monaco', monospace;
}
</style>
