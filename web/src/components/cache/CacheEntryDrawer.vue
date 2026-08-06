<template>
  <el-drawer
    :model-value="visible"
    :title="t('cache.detailDialog.title')"
    size="520px"
    @close="$emit('update:visible', false)"
  >
    <div v-loading="loading">
      <template v-if="data">
        <el-descriptions :column="1" border>
          <el-descriptions-item :label="t('cache.detailDialog.cacheKey')">
            <el-text style="font-family: monospace; font-size: 12px">{{ data.key }}</el-text>
            <el-button link type="primary" size="small" style="margin-left: 8px" @click="copyKey">
              {{ t('cache.detailDialog.copyKey') }}
            </el-button>
          </el-descriptions-item>
          <el-descriptions-item :label="t('cache.detailDialog.type')">
            <el-tag v-if="data.metadata?.save_only" type="warning" size="small">
              {{ t('cache.detailDialog.saveOnly') }}
            </el-tag>
            <el-tag v-else :type="typeTag" size="small">{{ typeLabel }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item :label="t('cache.table.session')">
            {{ data.session_id || data.metadata?.session_id || '-' }}
          </el-descriptions-item>
          <el-descriptions-item :label="t('cache.detailDialog.model')">
            {{ data.model || data.metadata?.model || '-' }}
          </el-descriptions-item>
          <el-descriptions-item :label="t('cache.detailDialog.storage')">
            {{ data.storage_backend || '-' }}
          </el-descriptions-item>
          <el-descriptions-item :label="t('cache.detailDialog.createdAt')">
            {{ formatDate(data.timestamp) }}
          </el-descriptions-item>
          <el-descriptions-item
            v-if="data.similarity !== undefined && data.similarity !== null"
            :label="t('cache.detailDialog.similarity')"
          >
            {{ (data.similarity * 100).toFixed(2) }}%
          </el-descriptions-item>
        </el-descriptions>

        <el-divider content-position="left">{{ t('cache.detailDialog.questionDivider') }}</el-divider>
        <div class="detail-content"><pre>{{ data.question || data.request || data.metadata?.request_text || '-' }}</pre></div>

        <el-divider content-position="left">{{ t('cache.detailDialog.responseDivider') }}</el-divider>
        <div class="detail-content"><pre>{{ data.response || '-' }}</pre></div>

        <template v-if="data.metadata && Object.keys(data.metadata).length">
          <el-divider content-position="left">{{ t('cache.detailDialog.metadataDivider') }}</el-divider>
          <div class="detail-content"><pre>{{ formatJson(data.metadata) }}</pre></div>
        </template>
      </template>
      <el-empty v-else-if="!loading" :description="t('cache.detailDialog.noData')" />
    </div>
    <template #footer>
      <el-button type="danger" :disabled="!data?.key" @click="$emit('delete', data)">
        {{ t('cache.table.delete') }}
      </el-button>
      <el-button @click="$emit('update:visible', false)">{{ t('cache.detailDialog.close') }}</el-button>
    </template>
  </el-drawer>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'

const props = defineProps<{
  visible: boolean
  loading?: boolean
  data: Record<string, any> | null
}>()

defineEmits<{ 'update:visible': [boolean]; delete: [row: any] }>()

const { t } = useI18n()

const typeLabel = computed(() => {
  const ct = props.data?.cache_type
  if (props.data?.metadata?.save_only) return t('cache.detailDialog.saveOnly')
  if (ct === 'semantic') return t('cache.detailDialog.semanticMatch')
  if (ct === 'external') return t('cache.filterOptions.external')
  return t('cache.detailDialog.exactMatch')
})

const typeTag = computed(() => {
  const ct = props.data?.cache_type
  if (ct === 'semantic') return 'success'
  if (ct === 'external') return 'warning'
  return 'primary'
})

function formatDate(timestamp: any) {
  if (!timestamp) return '-'
  return new Date(timestamp).toLocaleString()
}

function formatJson(value: any) {
  try {
    return JSON.stringify(value, null, 2)
  } catch {
    return String(value)
  }
}

async function copyKey() {
  const key = props.data?.key
  if (!key) return
  try {
    await navigator.clipboard.writeText(key)
    ElMessage.success(t('cache.message.copied'))
  } catch {
    ElMessage.error(t('cache.message.copyFailed'))
  }
}
</script>

<style scoped>
.detail-content {
  background: var(--color-gray-50);
  border: 1px solid var(--color-gray-200);
  border-radius: 4px;
  padding: 12px;
  max-height: 240px;
  overflow: auto;
}
.detail-content pre {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-all;
  font-family: Monaco, Menlo, Consolas, monospace;
  font-size: 13px;
  line-height: 1.5;
}
</style>
