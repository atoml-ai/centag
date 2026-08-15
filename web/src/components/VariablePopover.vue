<template>
  <el-popover
    :width="320"
    trigger="click"
    placement="bottom-start"
  >
    <template #reference>
      <el-button
        class="var-trigger"
        size="small"
        circle
        :title="t('nodeConfig.insertVariable')"
      >
        <span class="var-icon">{}</span>
      </el-button>
    </template>
    <div class="var-popover">
      <div class="var-section-label">{{ t('nodeConfig.systemModelVars') }}</div>
      <div
        v-for="v in systemModelVars"
        :key="v.name"
        class="var-item"
        @click="handleSelect(v.name)"
      >
        <code class="var-name">{{ v.name }}</code>
        <span class="var-desc">{{ v.desc }}</span>
      </div>
      <template v-if="userVariables.length">
        <el-divider style="margin: 8px 0" />
        <div class="var-section-label">{{ t('nodeConfig.userVars') }}</div>
        <div
          v-for="v in userVariables"
          :key="v.name"
          class="var-item"
          @click="handleSelect(v.name)"
        >
          <code class="var-name">{{ v.name }}</code>
          <span class="var-value">{{ v.value }}</span>
        </div>
      </template>
    </div>
  </el-popover>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useModelConfigStore } from '@/stores/model-config'

const { t } = useI18n()
const store = useModelConfigStore()

const emit = defineEmits<{
  select: [variableName: string]
}>()

const systemModelVars = [
  { name: 'system.default_backend', desc: '默认后端' },
  { name: 'system.default_model', desc: '默认模型' },
  { name: 'system.fallback_backend', desc: '备用后端' },
  { name: 'system.fallback_model', desc: '备用模型' },
  { name: 'system.embedding_backend', desc: '向量化后端' },
  { name: 'system.embedding_model', desc: '向量化模型' },
  { name: 'system.rerank_backend', desc: '重排序后端' },
  { name: 'system.rerank_model', desc: '重排序模型' },
]

const userVariables = ref<{ name: string; value: string }[]>([])

function handleSelect(name: string) {
  emit('select', name)
}

onMounted(() => {
  if (!store.systemVariables.length) {
    store.loadConfig()
  }
  store.$subscribe(() => {
    userVariables.value = store.userVariables.map(v => ({ name: v.name, value: v.value }))
  })
  userVariables.value = store.userVariables.map(v => ({ name: v.name, value: v.value }))
})
</script>

<style scoped>
.var-trigger {
  margin-left: 4px;
  flex-shrink: 0;
}
.var-icon {
  font-family: 'Monaco', 'Menlo', 'Consolas', monospace;
  font-size: 12px;
  font-weight: 700;
  color: #409eff;
}
.var-popover {
  max-height: 320px;
  overflow-y: auto;
}
.var-section-label {
  font-size: 12px;
  color: #909399;
  margin-bottom: 6px;
  font-weight: 500;
}
.var-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 8px;
  border-radius: 4px;
  cursor: pointer;
  transition: background 0.15s;
}
.var-item:hover {
  background: #f0f7ff;
}
.var-item .var-name {
  font-family: 'Monaco', 'Menlo', 'Consolas', monospace;
  font-size: 12px;
  color: #409eff;
  flex-shrink: 0;
}
.var-item .var-desc,
.var-item .var-value {
  font-size: 12px;
  color: #909399;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
