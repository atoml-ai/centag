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
        <div class="var-head">
          <code class="var-name">{{ v.name }}</code>
          <span class="var-desc">{{ v.desc }}</span>
        </div>
        <div class="var-usage">{{ v.usage }}</div>
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
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useModelConfigStore } from '@/stores/model-config'

const { t } = useI18n()
const store = useModelConfigStore()

const emit = defineEmits<{
  select: [variableName: string]
}>()

const systemModelVars = computed(() => [
  { name: 'system.default_backend', desc: t('nodeConfig.systemVarInfo.default_backend.desc'), usage: t('nodeConfig.systemVarInfo.default_backend.usage') },
  { name: 'system.default_model', desc: t('nodeConfig.systemVarInfo.default_model.desc'), usage: t('nodeConfig.systemVarInfo.default_model.usage') },
  { name: 'system.fallback_backend', desc: t('nodeConfig.systemVarInfo.fallback_backend.desc'), usage: t('nodeConfig.systemVarInfo.fallback_backend.usage') },
  { name: 'system.fallback_model', desc: t('nodeConfig.systemVarInfo.fallback_model.desc'), usage: t('nodeConfig.systemVarInfo.fallback_model.usage') },
  { name: 'system.classify_backend', desc: t('nodeConfig.systemVarInfo.classify_backend.desc'), usage: t('nodeConfig.systemVarInfo.classify_backend.usage') },
  { name: 'system.classify_model', desc: t('nodeConfig.systemVarInfo.classify_model.desc'), usage: t('nodeConfig.systemVarInfo.classify_model.usage') },
  { name: 'system.embedding_backend', desc: t('nodeConfig.systemVarInfo.embedding_backend.desc'), usage: t('nodeConfig.systemVarInfo.embedding_backend.usage') },
  { name: 'system.embedding_model', desc: t('nodeConfig.systemVarInfo.embedding_model.desc'), usage: t('nodeConfig.systemVarInfo.embedding_model.usage') },
  { name: 'system.rerank_backend', desc: t('nodeConfig.systemVarInfo.rerank_backend.desc'), usage: t('nodeConfig.systemVarInfo.rerank_backend.usage') },
  { name: 'system.rerank_model', desc: t('nodeConfig.systemVarInfo.rerank_model.desc'), usage: t('nodeConfig.systemVarInfo.rerank_model.usage') },
])

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
  padding: 6px 8px;
  border-radius: 4px;
  cursor: pointer;
  transition: background 0.15s;
}
.var-item:hover {
  background: #f0f7ff;
}
.var-item .var-head {
  display: flex;
  align-items: center;
  gap: 8px;
}
.var-item .var-name {
  font-family: 'Monaco', 'Menlo', 'Consolas', monospace;
  font-size: 12px;
  color: #409eff;
  flex-shrink: 0;
}
.var-item .var-usage {
  margin-top: 2px;
  font-size: 11px;
  color: #a8abb2;
  line-height: 1.5;
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
