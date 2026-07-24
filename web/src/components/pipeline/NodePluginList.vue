<template>
  <div class="node-plugin-list">
    <el-table :data="plugins" style="width: 100%" v-loading="loading">
      <el-table-column prop="name" :label="t('nodePluginList.nameColumn')" min-width="150">
        <template #default="{ row }">
          <div class="plugin-name">
            <span>{{ row.name }}</span>
            <el-tag v-if="row.deprecated" type="warning" size="small">{{ t('nodePluginList.deprecated') }}</el-tag>
          </div>
        </template>
      </el-table-column>
      <el-table-column prop="implementation" :label="t('nodePluginList.implColumn')" min-width="180">
        <template #default="{ row }">
          <code class="implementation-code">{{ row.implementation }}</code>
        </template>
      </el-table-column>
      <el-table-column prop="kind" :label="t('nodePluginList.kindColumn')" min-width="120" />
      <el-table-column prop="version" :label="t('nodePluginList.versionColumn')" width="100" />
      <el-table-column :label="t('nodePluginList.permissionsColumn')" min-width="150">
        <template #default="{ row }">
          <el-tooltip
            v-for="perm in row.permissions"
            :key="perm"
            placement="top"
            :content="getPermissionDescription(perm)"
          >
            <el-tag
              size="small"
              type="info"
              class="permission-tag"
            >
              {{ perm }}
            </el-tag>
          </el-tooltip>
        </template>
      </el-table-column>
      <el-table-column :label="t('nodePluginList.compatibilityColumn')" width="130">
        <template #default="{ row }">
          <el-tooltip v-if="row.min_centag_version" :content="t('nodePluginList.minVersion', { version: row.min_centag_version })">
            <el-tag :type="checkVersionCompatibility(row.min_centag_version) ? 'success' : 'danger'" size="small">
              {{ checkVersionCompatibility(row.min_centag_version) ? t('nodePluginList.compatible') : t('nodePluginList.incompatible') }}
            </el-tag>
          </el-tooltip>
          <span v-else class="text-muted">-</span>
        </template>
      </el-table-column>
      <el-table-column :label="t('nodePluginList.actionsColumn')" width="180" fixed="right">
        <template #default="{ row }">
          <el-button size="small" @click="$emit('view', row)">{{ t('nodePluginList.view') }}</el-button>
          <el-button size="small" type="primary" @click="$emit('test', row)">{{ t('nodePluginList.test') }}</el-button>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

interface PluginDescriptor {
  name: string
  implementation: string
  kind: string
  version: string
  description?: string
  permissions?: string[]
  deprecated?: boolean
  min_centag_version?: string
  supports_stream?: boolean
  tags?: string[]
}

defineProps<{
  plugins: PluginDescriptor[]
  loading?: boolean
}>()

defineEmits<{
  view: [plugin: PluginDescriptor]
  test: [plugin: PluginDescriptor]
}>()

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
    'llm.call': t('nodePluginList.permLlmCall'),
    'storage.read': t('nodePluginList.permStorageRead'),
    'storage.write': t('nodePluginList.permStorageWrite'),
    'memory.read': t('nodePluginList.permMemoryRead'),
    'memory.write': t('nodePluginList.permMemoryWrite'),
    'network.outbound': t('nodePluginList.permNetworkOutbound'),
    'secrets.read': t('nodePluginList.permSecretsRead'),
    'network.inbound': t('nodePluginList.permNetworkInbound'),
  }
  return descriptions[perm] || t('nodePluginList.permPrefix', { perm })
}
</script>

<style scoped>
.plugin-name {
  display: flex;
  align-items: center;
  gap: 8px;
}
.implementation-code {
  font-size: 12px;
  background: #f5f7fa;
  padding: 2px 6px;
  border-radius: 4px;
}
.permission-tag {
  margin-right: 4px;
}
.text-muted {
  color: #909399;
  font-size: 12px;
}
</style>
