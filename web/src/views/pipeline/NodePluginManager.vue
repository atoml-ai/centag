<template>
  <div class="node-plugin-manager">
    <div class="page-header">
      <h2>{{ t('nodePluginManager.title') }}</h2>
      <div class="header-actions">
        <el-button @click="loadPlugins" :loading="loading">
          <el-icon><Refresh /></el-icon>
          {{ t('nodePluginManager.refresh') }}
        </el-button>
      </div>
    </div>

    <div v-if="schemaVersion" class="schema-version-tip">
      {{ t('nodePluginManager.pluginInterfaceVersion', { version: schemaVersion.split('/').pop() }) }}
    </div>

    <NodePluginList
      :plugins="plugins"
      :loading="loading"
      @view="handleView"
      @test="handleTest"
    />

    <el-drawer
      v-model="detailVisible"
      :title="selectedPlugin?.name"
      size="60%"
    >
      <template v-if="selectedPlugin">
        <el-descriptions :column="2" border>
          <el-descriptions-item :label="t('nodePluginManager.implementation')">
            <code>{{ selectedPlugin.implementation }}</code>
          </el-descriptions-item>
          <el-descriptions-item :label="t('nodePluginManager.version')">
            {{ selectedPlugin.version }}
          </el-descriptions-item>
          <el-descriptions-item :label="t('nodePluginManager.kind')">
            {{ selectedPlugin.kind }}
          </el-descriptions-item>
          <el-descriptions-item :label="t('nodePluginManager.pluginType')">
            <el-tag v-if="selectedPlugin.remote" size="small">{{ t('nodePluginManager.remote') }}</el-tag>
            <el-tag v-else size="small">{{ t('nodePluginManager.builtin') }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item :label="t('nodePluginManager.streamSupport')">
            <el-tag :type="selectedPlugin.supports_stream ? 'success' : 'info'" size="small">
              {{ selectedPlugin.supports_stream ? t('nodePluginManager.yes') : t('nodePluginManager.no') }}
            </el-tag>
          </el-descriptions-item>
          <el-descriptions-item :label="t('nodePluginManager.deprecated')">
            <el-tag v-if="selectedPlugin.deprecated" type="warning" size="small">{{ t('nodePluginManager.deprecatedTag') }}</el-tag>
            <span v-else>-</span>
          </el-descriptions-item>
          <el-descriptions-item :label="t('nodePluginManager.description')" :span="2">
            {{ selectedPlugin.description || '-' }}
          </el-descriptions-item>
          <el-descriptions-item :label="t('nodePluginManager.permissions')" :span="2">
            <el-tag
              v-for="perm in selectedPlugin.permissions"
              :key="perm"
              size="small"
              type="info"
              class="permission-tag"
            >
              {{ perm }}
            </el-tag>
            <span v-if="!selectedPlugin.permissions?.length" class="text-muted">{{ t('nodePluginManager.none') }}</span>
          </el-descriptions-item>
          <el-descriptions-item :label="t('nodePluginManager.tags')" :span="2">
            <el-tag
              v-for="tag in selectedPlugin.tags"
              :key="tag"
              size="small"
              class="tag-item"
            >
              {{ tag }}
            </el-tag>
            <span v-if="!selectedPlugin.tags?.length" class="text-muted">{{ t('nodePluginManager.none') }}</span>
          </el-descriptions-item>
        </el-descriptions>

        <el-tabs style="margin-top: 16px">
          <el-tab-pane :label="t('nodePluginManager.configSchema')">
            <pre v-if="selectedPlugin.config_schema">{{ formatJson(selectedPlugin.config_schema) }}</pre>
            <el-empty v-else :description="t('nodePluginManager.noConfigSchema')" :image-size="60" />
          </el-tab-pane>
          <el-tab-pane :label="t('nodePluginManager.inputSchema')">
            <pre v-if="selectedPlugin.input_schema">{{ formatJson(selectedPlugin.input_schema) }}</pre>
            <el-empty v-else :description="t('nodePluginManager.noInputSchema')" :image-size="60" />
          </el-tab-pane>
          <el-tab-pane :label="t('nodePluginManager.outputSchema')">
            <pre v-if="selectedPlugin.output_schema">{{ formatJson(selectedPlugin.output_schema) }}</pre>
            <el-empty v-else :description="t('nodePluginManager.noOutputSchema')" :image-size="60" />
          </el-tab-pane>
        </el-tabs>
      </template>
    </el-drawer>

    <el-drawer
      v-model="testVisible"
      :title="t('nodePluginManager.testPlugin')"
      size="70%"
      :before-close="() => testVisible = false"
    >
      <NodeTestPanel
        :plugin="selectedPlugin"
        @close="testVisible = false"
      />
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { Refresh } from '@element-plus/icons-vue'
import NodePluginList from '@/components/pipeline/NodePluginList.vue'
import NodeTestPanel from '@/components/pipeline/NodeTestPanel.vue'
import { getNodePlugins, parseNodePluginsResponse } from '@/api/pipeline'

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
  config_schema?: Record<string, any>
  input_schema?: Record<string, any>
  output_schema?: Record<string, any>
  remote?: any
}

const plugins = ref<PluginDescriptor[]>([])
const loading = ref(false)
const schemaVersion = ref('')

const detailVisible = ref(false)
const testVisible = ref(false)
const selectedPlugin = ref<PluginDescriptor | null>(null)

onMounted(() => {
  loadPlugins()
})

async function loadPlugins() {
  loading.value = true
  try {
    const resp = await getNodePlugins()
    if (resp && typeof resp === 'object' && !Array.isArray(resp) && 'schema_version' in resp) {
      schemaVersion.value = String((resp as { schema_version?: string }).schema_version || '')
    } else {
      schemaVersion.value = ''
    }
    plugins.value = parseNodePluginsResponse(resp)
  } catch (e) {
    console.error('Failed to load plugins:', e)
    ElMessage.error(t('nodePluginManager.loadPluginsFailed'))
  } finally {
    loading.value = false
  }
}

function handleView(plugin: PluginDescriptor) {
  selectedPlugin.value = plugin
  detailVisible.value = true
}

function handleTest(plugin: PluginDescriptor) {
  selectedPlugin.value = plugin
  testVisible.value = true
}

function formatJson(data: any): string {
  try {
    return JSON.stringify(data, null, 2)
  } catch {
    return String(data)
  }
}
</script>

<style scoped>
.node-plugin-manager {
  padding: 20px;
  position: relative;
}
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}
.page-header h2 {
  margin: 0;
}
.header-actions {
  display: flex;
  gap: 8px;
}
.permission-tag {
  margin-right: 4px;
}
.tag-item {
  margin-right: 4px;
}
.text-muted {
  color: #909399;
  font-size: 12px;
}
.schema-version-tip {
  position: absolute;
  top: 70px;
  right: 20px;
  font-size: 11px;
  color: #c0c4cc;
  background: transparent;
  z-index: 1;
}
</style>
