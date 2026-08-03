<template>
  <div class="api-access-panel" :class="{ 'is-compact': compact }">
    <div class="access-row">
      <el-select
        v-model="selectedPath"
        class="protocol-select"
        size="small"
        :teleported="true"
      >
        <el-option
          v-for="ep in endpoints"
          :key="ep.path"
          :label="ep.label"
          :value="ep.path"
        >
          <div class="option-row">
            <span>{{ ep.label }}</span>
            <span v-if="ep.hint" class="option-hint">{{ ep.hint }}</span>
          </div>
        </el-option>
      </el-select>
      <code class="access-url" :title="selectedUrl">{{ selectedUrl }}</code>
      <el-button size="small" type="primary" @click="copySelected">
        <el-icon><CopyDocument /></el-icon>
        {{ t('apiAccessPanel.copy') }}
      </el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { CopyDocument } from '@element-plus/icons-vue'
import { API_ENDPOINTS, buildEndpointUrl, type ApiEndpoint } from '@/utils/apiBaseUrl'

const { t } = useI18n()

const OPENAI_CHAT_PATH = '/v1/chat/completions'

const props = withDefaults(
  defineProps<{
    baseUrl: string
    endpoints?: ApiEndpoint[]
    showBaseCopy?: boolean
    compact?: boolean
  }>(),
  {
    endpoints: () => API_ENDPOINTS,
    showBaseCopy: true,
    compact: false
  }
)

const selectedPath = ref(OPENAI_CHAT_PATH)

watch(
  () => props.endpoints,
  (list) => {
    if (!list.some((ep) => ep.path === selectedPath.value)) {
      selectedPath.value = list[0]?.path || OPENAI_CHAT_PATH
    }
  },
  { immediate: true }
)

const selectedEndpoint = computed(
  () => props.endpoints.find((ep) => ep.path === selectedPath.value) || props.endpoints[0]
)

const selectedUrl = computed(() =>
  buildEndpointUrl(props.baseUrl, selectedEndpoint.value?.path || OPENAI_CHAT_PATH)
)

async function copySelected() {
  const label = selectedEndpoint.value?.label || t('apiAccessPanel.endpointLabel')
  const { copyToClipboard } = await import('@/utils/clipboard')
  if (await copyToClipboard(selectedUrl.value)) {
    ElMessage.success(t('apiAccessPanel.copySuccess', { label }))
  } else {
    ElMessage.error(t('apiAccessPanel.copyFailed'))
  }
}
</script>

<style scoped>
.api-access-panel {
  min-width: 0;
}

.access-row {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.protocol-select {
  width: 148px;
  flex-shrink: 0;
}

.access-url {
  flex: 1;
  min-width: 0;
  font-family: var(--font-mono);
  font-size: 0.8125rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  padding: 6px 10px;
  border-radius: var(--radius-md);
  background: var(--shell-sidebar-bg);
  border: 1px solid var(--shell-sidebar-border);
  color: var(--shell-sidebar-text);
}

.api-access-panel:not(.is-compact) .access-url {
  white-space: normal;
  word-break: break-all;
}

.option-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.option-hint {
  font-size: 0.75rem;
  color: var(--el-text-color-secondary);
}
</style>
