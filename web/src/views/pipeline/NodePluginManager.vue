<template>
  <div class="node-plugin-manager">
    <div class="page-header">
      <h2>节点插件管理器</h2>
      <div class="header-actions">
        <el-button @click="loadPlugins" :loading="loading">
          <el-icon><Refresh /></el-icon>
          刷新
        </el-button>
      </div>
    </div>

    <div v-if="schemaVersion" class="schema-version-tip">
      插件接口版本：{{ schemaVersion.split('/').pop() }}
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
          <el-descriptions-item label="实现">
            <code>{{ selectedPlugin.implementation }}</code>
          </el-descriptions-item>
          <el-descriptions-item label="版本">
            {{ selectedPlugin.version }}
          </el-descriptions-item>
          <el-descriptions-item label="类型">
            {{ selectedPlugin.kind }}
          </el-descriptions-item>
          <el-descriptions-item label="插件类型">
            <el-tag v-if="selectedPlugin.remote" size="small">远程</el-tag>
            <el-tag v-else size="small">内置</el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="流式支持">
            <el-tag :type="selectedPlugin.supports_stream ? 'success' : 'info'" size="small">
              {{ selectedPlugin.supports_stream ? '是' : '否' }}
            </el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="已弃用">
            <el-tag v-if="selectedPlugin.deprecated" type="warning" size="small">已弃用</el-tag>
            <span v-else>-</span>
          </el-descriptions-item>
          <el-descriptions-item label="描述" :span="2">
            {{ selectedPlugin.description || '-' }}
          </el-descriptions-item>
          <el-descriptions-item label="权限" :span="2">
            <el-tag
              v-for="perm in selectedPlugin.permissions"
              :key="perm"
              size="small"
              type="info"
              class="permission-tag"
            >
              {{ perm }}
            </el-tag>
            <span v-if="!selectedPlugin.permissions?.length" class="text-muted">无</span>
          </el-descriptions-item>
          <el-descriptions-item label="标签" :span="2">
            <el-tag
              v-for="tag in selectedPlugin.tags"
              :key="tag"
              size="small"
              class="tag-item"
            >
              {{ tag }}
            </el-tag>
            <span v-if="!selectedPlugin.tags?.length" class="text-muted">无</span>
          </el-descriptions-item>
        </el-descriptions>

        <el-tabs style="margin-top: 16px">
          <el-tab-pane label="配置模式">
            <pre v-if="selectedPlugin.config_schema">{{ formatJson(selectedPlugin.config_schema) }}</pre>
            <el-empty v-else description="无配置模式" :image-size="60" />
          </el-tab-pane>
          <el-tab-pane label="输入模式">
            <pre v-if="selectedPlugin.input_schema">{{ formatJson(selectedPlugin.input_schema) }}</pre>
            <el-empty v-else description="无输入模式" :image-size="60" />
          </el-tab-pane>
          <el-tab-pane label="输出模式">
            <pre v-if="selectedPlugin.output_schema">{{ formatJson(selectedPlugin.output_schema) }}</pre>
            <el-empty v-else description="无输出模式" :image-size="60" />
          </el-tab-pane>
        </el-tabs>
      </template>
    </el-drawer>

    <el-drawer
      v-model="testVisible"
      title="测试插件"
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
import { ElMessage } from 'element-plus'
import { Refresh } from '@element-plus/icons-vue'
import NodePluginList from '@/components/pipeline/NodePluginList.vue'
import NodeTestPanel from '@/components/pipeline/NodeTestPanel.vue'
import { getNodePlugins, parseNodePluginsResponse } from '@/api/pipeline'

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
    ElMessage.error('加载节点插件失败，请检查后端服务和插件注册接口')
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