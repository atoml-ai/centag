<template>
  <div class="plugin-selector">
    <div class="plugin-selector-header">
      <span class="plugin-selector-title">{{ t('pluginSelector.title') }}</span>
      <el-button
        v-if="showAllPlugins"
        size="small"
        text
        @click="$emit('view-all')"
      >
        {{ t('pluginSelector.viewAll') }}
      </el-button>
    </div>

    <div class="plugin-selector-content">
      <el-form-item :label="t('pluginSelector.kindLabel')" v-if="showKindSelector">
        <el-select
          v-model="selectedKind"
          :placeholder="t('pluginSelector.kindPlaceholder')"
          style="width: 100%"
          @change="onKindChange"
        >
          <el-option
            v-for="kind in uniqueKinds"
            :key="kind"
            :label="kind"
            :value="kind"
          />
        </el-select>
        <div class="help-text">
          {{ t('pluginSelector.kindHelp') }}
        </div>
      </el-form-item>

      <el-form-item :label="t('pluginSelector.implLabel')">
        <el-select
          v-model="selectedImplementation"
          :placeholder="t('pluginSelector.implPlaceholder')"
          style="width: 100%"
          filterable
          @change="onImplementationChange"
        >
          <el-option
            v-for="plugin in filteredPlugins"
            :key="plugin.implementation"
            :label="plugin.name"
            :value="plugin.implementation"
          >
            <div class="plugin-option">
              <span class="plugin-name">{{ plugin.name }}</span>
              <span class="plugin-impl">{{ plugin.implementation }}</span>
            </div>
          </el-option>
        </el-select>
      </el-form-item>

      <div v-if="selectedPlugin" class="plugin-details">
        <el-alert
          type="info"
          :closable="false"
          style="margin-bottom: 12px"
        >
          <template #default>
            <div style="font-size: 13px; line-height: 1.5">
              <strong>{{ t('pluginSelector.pluginInfo') }}</strong><br>
              <span v-if="selectedPlugin.description">{{ selectedPlugin.description }}<br></span>
              <span>{{ t('pluginSelector.version', { version: selectedPlugin.version }) }}</span>
            </div>
          </template>
        </el-alert>

        <div v-if="selectedPlugin.permissions && selectedPlugin.permissions.length > 0" class="permissions-section">
          <div class="permissions-label">
            <el-icon><Lock /></el-icon>
            <span>{{ t('pluginSelector.permissions') }}</span>
          </div>
          <div class="permissions-list">
            <el-tooltip
              v-for="perm in selectedPlugin.permissions"
              :key="perm"
              placement="top"
              :content="getPermissionDescription(perm)"
            >
              <el-tag size="small" type="info" class="permission-tag">
                {{ perm }}
              </el-tag>
            </el-tooltip>
          </div>
        </div>

        <div v-if="selectedPlugin.min_centag_version" class="compatibility-section">
          <el-alert
            :type="checkVersionCompatibility(selectedPlugin.min_centag_version) ? 'success' : 'warning'"
            :closable="false"
            style="margin-top: 8px"
          >
            <template #default>
              <div style="font-size: 13px">
                <span v-if="checkVersionCompatibility(selectedPlugin.min_centag_version)">
                  {{ t('pluginSelector.compatible', { version: selectedPlugin.min_centag_version }) }}
                </span>
                <span v-else>
                  {{ t('pluginSelector.incompatible', { version: selectedPlugin.min_centag_version }) }}
                </span>
              </div>
            </template>
          </el-alert>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Lock } from '@element-plus/icons-vue'
import { PluginDescriptor } from '@/api/pipeline'

const { t } = useI18n()

const props = defineProps<{
  plugins: PluginDescriptor[]
  modelValue?: string
  kind?: string
  showKindSelector?: boolean
  showAllPlugins?: boolean
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: string | undefined)
  (e: 'update:kind', value: string | undefined)
  (e: 'view-all')
}>()

const selectedImplementation = ref<string | undefined>(props.modelValue)
const selectedKind = ref<string | undefined>(props.kind)

const uniqueKinds = computed(() => {
  const kinds = new Set(props.plugins.map(p => p.kind).filter(Boolean))
  return Array.from(kinds).sort()
})

const filteredPlugins = computed(() => {
  if (!selectedKind.value) {
    return props.plugins
  }
  const matched = props.plugins.filter(p => p.kind === selectedKind.value)
  return matched.length > 0 ? matched : props.plugins
})

const selectedPlugin = computed(() => {
  if (!selectedImplementation.value) return undefined
  return props.plugins.find(p => p.implementation === selectedImplementation.value)
})

const onKindChange = (kind: string | undefined) => {
  selectedKind.value = kind
  selectedImplementation.value = undefined
  emit('update:modelValue', undefined)
  emit('update:kind', kind)
}

const onImplementationChange = (impl: string | undefined) => {
  selectedImplementation.value = impl
  const plugin = props.plugins.find(p => p.implementation === impl)
  if (plugin) {
    selectedKind.value = plugin.kind
    emit('update:kind', plugin.kind)
  } else {
    selectedKind.value = undefined
    emit('update:kind', undefined)
  }
  emit('update:modelValue', impl)
}

watch(() => props.modelValue, (newVal) => {
  if (newVal !== selectedImplementation.value) {
    selectedImplementation.value = newVal
    const plugin = props.plugins.find(p => p.implementation === newVal)
    if (plugin) {
      selectedKind.value = plugin.kind
    }
  }
})

watch(() => props.kind, (newVal) => {
  if (newVal !== selectedKind.value) {
    selectedKind.value = newVal
  }
})

function checkVersionCompatibility(minVersion: string): boolean {
  const currentVersion = '1.0.0'
  const min = minVersion.split('.').map(Number)
  const curr = currentVersion.split('.').map(Number)
  for (let i = 0; i < 3; i++) {
    if (curr[i] < min[i]) return false
    if (curr[i] > min[i]) return true
  }
  return true
}

function getPermissionDescription(perm: string): string {
  const descriptions: Record<string, string> = {
    'llm.call': t('pluginSelector.permLlmCall'),
    'storage.read': t('pluginSelector.permStorageRead'),
    'storage.write': t('pluginSelector.permStorageWrite'),
    'memory.read': t('pluginSelector.permMemoryRead'),
    'memory.write': t('pluginSelector.permMemoryWrite'),
    'network.outbound': t('pluginSelector.permNetworkOutbound'),
    'system.admin': t('pluginSelector.permSystemAdmin'),
  }
  return descriptions[perm] || perm
}
</script>

<style scoped>
.plugin-selector {
  border: 1px solid #dcdfe6;
  border-radius: 6px;
  padding: 16px;
  margin-bottom: 16px;
  background: #f5f7fa;
}

.plugin-selector-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.plugin-selector-title {
  font-weight: 600;
  color: #303133;
  font-size: 14px;
}

.plugin-selector-content {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.plugin-option {
  display: flex;
  flex-direction: column;
}

.plugin-name {
  font-weight: 500;
  color: #303133;
}

.plugin-impl {
  font-size: 12px;
  color: #606266;
  font-family: monospace;
}

.help-text {
  font-size: 12px;
  color: #909399;
  margin-top: 4px;
}

.plugin-details {
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px solid #e4e7ed;
}

.permissions-section {
  margin-top: 12px;
}

.permissions-label {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: #606266;
  margin-bottom: 8px;
}

.permissions-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.permission-tag {
  cursor: pointer;
}

.compatibility-section {
  margin-top: 8px;
}
</style>
