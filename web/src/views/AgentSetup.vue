<template>
  <div class="agent-setup-page">
    <div class="page-header">
      <div>
        <h1 class="page-title">
          <el-icon><Link /></el-icon>
          Agent 接入
        </h1>
        <p class="page-description">管理 Agent 工具的接入配置和供应商路由</p>
      </div>
    </div>

    <el-tabs v-model="activeTab" class="setup-tabs">
      <!-- Tab 1: 快速接入 -->
      <el-tab-pane label="快速接入" name="setup">
        <!-- 选择 Agent -->
        <div class="section-block">
          <el-row :gutter="16">
            <el-col
              v-for="agent in agentTypes"
              :key="agent.type"
              :xs="24" :sm="12" :md="8"
            >
              <div
                class="agent-card"
                :class="{ active: selectedAgent === agent.type }"
                @click="selectedAgent = agent.type; onAgentChange()"
              >
                <div class="agent-icon">
                  <el-icon :size="32">
                    <component :is="agentIcon(agent.type)" />
                  </el-icon>
                </div>
                <div class="agent-info">
                  <div class="agent-name">{{ agent.display_name }}</div>
                  <div class="agent-desc">{{ agent.description }}</div>
                </div>
              </div>
            </el-col>
          </el-row>
        </div>

        <!-- 选择 Provider + 流水线 + 配置 -->
        <div v-if="selectedAgent" class="section-block">
          <el-card class="config-card">
            <div v-if="pipelines.length === 0 && !loadingPipelines" class="empty-backends">
              <el-empty description="暂无可用流水线，请先在「策略管理」中添加">
                <el-button type="primary" @click="$router.push('/pipelines')">前往策略管理</el-button>
              </el-empty>
            </div>
            <div v-else>
              <!-- Provider 选择 -->
              <div v-loading="loadingBackends" class="backend-section">
                <h4 class="select-label">选择 Provider（可选）</h4>
                <el-select
                  v-model="selectedBackend"
                  placeholder="自动选择默认 Provider"
                  filterable
                  clearable
                  style="width: 100%"
                  @change="onBackendChange"
                >
                  <el-option
                    v-for="backend in backends"
                    :key="backend.id"
                    :label="backend.name || backend.id"
                    :value="backend.id"
                  >
                    <div class="backend-option">
                      <span class="backend-name">{{ backend.name || backend.id }}</span>
                      <el-tag size="small" type="info">{{ backend.type }}</el-tag>
                    </div>
                  </el-option>
                </el-select>
                <div v-if="selectedBackend" class="backend-hint">
                  <el-tag type="success" size="small">
                    <el-icon><Connection /></el-icon>
                    已选择: {{ getBackendName(selectedBackend) }}
                  </el-tag>
                </div>
              </div>

              <!-- 流水线选择 -->
              <div v-loading="loadingPipelines" class="pipeline-section">
                <h4 class="select-label">选择流水线（必选）</h4>
                <el-select
                  v-model="selectedPipeline"
                  placeholder="请选择流水线"
                  filterable
                  style="width: 100%"
                  @change="onPipelineChange"
                >
                  <el-option
                    v-for="pipe in pipelines"
                    :key="pipe.id"
                    :label="pipe.name || pipe.id"
                    :value="pipe.id"
                  >
                    <div class="pipeline-option">
                      <span>{{ pipe.name || pipe.id }}</span>
                      <span v-if="pipe.description" class="pipeline-desc">{{ pipe.description }}</span>
                    </div>
                  </el-option>
                </el-select>

                <!-- 模型信息 -->
                <div v-if="displayModel" class="model-display">
                  <el-tag type="info" size="default">
                    <el-icon><Cpu /></el-icon>
                    模型: {{ displayModel }}
                  </el-tag>
                  <span v-if="selectedPipeline" class="model-hint">由流水线决定</span>
                </div>
              </div>

              <!-- 生成配置按钮 -->
              <div class="generate-section">
                <el-button
                  type="primary"
                  size="large"
                  :loading="loadingConfig"
                  :disabled="loadingConfig || !selectedPipeline"
                  @click="generateConfig"
                >
                  <el-icon class="el-icon--left"><DocumentCopy /></el-icon>
                  生成配置
                </el-button>
              </div>
            </div>

            <!-- 配置结果 -->
            <div v-if="configResult" class="config-result">
              <el-divider content-position="center">
                <el-icon><DocumentCopy /></el-icon>
                配置结果
              </el-divider>

              <div class="config-header">
                <h3>{{ configResult.description }}</h3>
                <el-tag>路由: {{ configResult.backend_name }}</el-tag>
              </div>

              <!-- 一键写入 -->
              <div v-if="configResult.commands" class="config-section">
                <h4>一键配置</h4>
                <el-button type="primary" :loading="writingConfig" @click="writeToConfig">
                  <el-icon class="el-icon--left"><Plus /></el-icon>
                  写入配置文件
                </el-button>
                <p class="write-hint" v-if="writeResult">
                  <span v-if="writeResult.success" style="color: #67c23a">✓ {{ writeResult.message }}</span>
                  <span v-else style="color: #f56c6c">✗ {{ writeResult.message }}</span>
                </p>
                <div v-if="writeResult?.success && writePreviewFiles.length" class="write-preview">
                  <h5>已写入配置关键片段（已脱敏）</h5>
                  <el-collapse>
                    <el-collapse-item
                      v-for="file in writePreviewFiles"
                      :key="file.path"
                      :title="file.path"
                    >
                      <div class="code-block">
                        <pre><code>{{ file.preview }}</code></pre>
                        <el-button class="copy-btn" :icon="DocumentCopy" @click="copyText(file.preview)" />
                      </div>
                    </el-collapse-item>
                  </el-collapse>
                </div>
              </div>

              <!-- 团队版：平台命令 -->
              <div v-if="!isDesktopEdition && configResult.commands" class="config-section">
                <h4>配置命令</h4>
                <el-tabs v-model="platformTab" type="border-card">
                  <el-tab-pane label="macOS" name="macos">
                    <div class="code-block">
                      <pre><code>{{ configResult.commands.macos }}</code></pre>
                      <el-button class="copy-btn" :icon="DocumentCopy" @click="copyText(configResult.commands.macos)" />
                    </div>
                  </el-tab-pane>
                  <el-tab-pane label="Linux" name="linux">
                    <div class="code-block">
                      <pre><code>{{ configResult.commands.linux }}</code></pre>
                      <el-button class="copy-btn" :icon="DocumentCopy" @click="copyText(configResult.commands.linux)" />
                    </div>
                  </el-tab-pane>
                  <el-tab-pane label="Windows" name="windows">
                    <div class="code-block">
                      <pre><code>{{ configResult.commands.windows }}</code></pre>
                      <el-button class="copy-btn" :icon="DocumentCopy" @click="copyText(configResult.commands.windows)" />
                    </div>
                  </el-tab-pane>
                </el-tabs>
              </div>

              <!-- 配置文件 -->
              <div v-if="configResult.files && configResult.files.length" class="config-section">
                <h4>配置文件</h4>
                <el-collapse>
                  <el-collapse-item v-for="file in configResult.files" :key="file.path" :title="file.path">
                    <div class="code-block">
                      <pre><code>{{ file.content }}</code></pre>
                      <el-button class="copy-btn" :icon="DocumentCopy" @click="copyText(file.content)" />
                    </div>
                  </el-collapse-item>
                </el-collapse>
              </div>

              <!-- 手动步骤 -->
              <div v-if="configResult.steps && configResult.steps.length" class="config-section">
                <h4>手动配置步骤</h4>
                <el-timeline>
                  <el-timeline-item v-for="(step, i) in configResult.steps" :key="i" :timestamp="step.title" placement="top">
                    <p v-if="step.description">{{ step.description }}</p>
                    <div v-if="step.code" class="code-block">
                      <pre><code>{{ step.code }}</code></pre>
                      <el-button class="copy-btn" :icon="DocumentCopy" @click="copyText(step.code)" />
                    </div>
                  </el-timeline-item>
                </el-timeline>
              </div>

              <!-- 验证命令 -->
              <div v-if="configResult.verify_cmd" class="config-section">
                <h4>验证连通性</h4>
                <div class="code-block">
                  <pre><code>{{ configResult.verify_cmd }}</code></pre>
                  <el-button class="copy-btn" :icon="DocumentCopy" @click="copyText(configResult.verify_cmd)" />
                </div>
              </div>
            </div>
          </el-card>

          <div v-if="configResult" class="step-actions">
            <el-button type="default" @click="resetWizard">
              重新开始
            </el-button>
          </div>
        </div>
      </el-tab-pane>

      <!-- Tab 2: 供应商配置（仅管理员，只读视图） -->
      <el-tab-pane label="供应商配置" name="providers" v-if="isAdmin">
        <el-card>
          <template #header>
            <div class="card-header">
              <span>内置供应商路由配置（系统预设，不可编辑）</span>
            </div>
          </template>

          <el-table :data="providers" v-loading="loadingProviders" stripe>
            <el-table-column prop="agent_type" label="Agent 类型" width="150" />
            <el-table-column prop="display_name" label="显示名称" width="150" />
            <el-table-column prop="backend_id" label="后端 ID" width="180">
              <template #default="{ row }">
                <el-tag v-if="row.backend_id" type="success">{{ row.backend_id }}</el-tag>
                <span v-else class="text-muted">默认</span>
              </template>
            </el-table-column>
            <el-table-column prop="pipeline_id" label="流水线 ID" width="180">
              <template #default="{ row }">
                <el-tag v-if="row.pipeline_id" type="warning">{{ row.pipeline_id }}</el-tag>
                <span v-else class="text-muted">默认</span>
              </template>
            </el-table-column>
            <el-table-column prop="model" label="模型覆盖" width="150">
              <template #default="{ row }">
                <span v-if="row.model">{{ row.model }}</span>
                <span v-else class="text-muted">-</span>
              </template>
            </el-table-column>
            <el-table-column prop="enabled" label="状态" width="80">
              <template #default="{ row }">
                <el-tag :type="row.enabled ? 'success' : 'info'" size="small">
                  {{ row.enabled ? '启用' : '禁用' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="120" fixed="right">
              <template #default="{ row }">
                <el-button size="small" type="primary" @click="handleHotSwap(row)">
                  设为默认
                </el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useRouter } from 'vue-router'
import {
  Link, DocumentCopy, Plus,
  Monitor, ChatDotRound, DataLine, Connection, Cpu
} from '@element-plus/icons-vue'
import { isPersonalEdition } from '@/utils/edition'
import { useAuthStore } from '@/stores/auth'
import api from '@/api'

const router = useRouter()
const authStore = useAuthStore()
const isAdmin = computed(() => authStore.isAdmin)

// --- Tab ---
const activeTab = ref('setup')

// ===================== 快速接入 (Wizard) =====================

const selectedAgent = ref('')
const selectedPipeline = ref('')
const selectedBackend = ref('')
const pipelineModel = ref('')
const loadingPipelines = ref(false)
const loadingBackends = ref(false)
const loadingConfig = ref(false)
const writingConfig = ref(false)
const platformTab = ref('macos')
const agentTypes = ref<Array<{ type: string; display_name: string; description: string }>>([])
const pipelines = ref<Array<{ id: string; name: string; description?: string; nodes?: any[] }>>([])
const backends = ref<Array<{ id: string; name: string; type: string }>>([])
const configResult = ref<any>(null)
const writeResult = ref<{ success: boolean; message: string; written?: Array<{ path: string; content: string }> } | null>(null)

const isDesktopEdition = computed(() => isPersonalEdition())

const displayModel = computed(() => {
  if (selectedPipeline.value) {
    return `pipeline.${selectedPipeline.value}`
  }
  return pipelineModel.value || ''
})

const writePreviewFiles = computed(() => {
  if (!writeResult.value?.success || !Array.isArray(writeResult.value.written)) return []
  return writeResult.value.written
    .filter((f: any) => f?.path && typeof f.content === 'string')
    .map((f: any) => ({
      path: f.path,
      preview: buildSanitizedConfigPreview(f.content),
    }))
})

async function loadAgentTypes() {
  try {
    const res: any = await api.get('/api/v1/agent/types')
    agentTypes.value = res.agent_types || []
  } catch (e: any) {
    ElMessage.error('加载 Agent 类型失败：' + e.message)
  }
}

async function loadBackends() {
  loadingBackends.value = true
  try {
    const res: any = await api.get('/api/v1/backends')
    const data = res?.data || res
    backends.value = Array.isArray(data) ? data.filter((b: any) => b.enabled) : []
  } catch (e: any) {
    backends.value = []
    console.warn('加载后端列表失败:', e.message)
  } finally {
    loadingBackends.value = false
  }
}

async function loadPipelines() {
  loadingPipelines.value = true
  try {
    const res: any = await api.get('/api/v1/pipelines')
    const data = res?.data || res
    pipelines.value = Array.isArray(data) ? data : []
  } catch (e: any) {
    pipelines.value = []
    console.warn('加载流水线列表失败:', e.message)
  } finally {
    loadingPipelines.value = false
  }
}

function getBackendName(backendId: string): string {
  const backend = backends.value.find(b => b.id === backendId)
  return backend?.name || backendId
}

function onBackendChange() {
  selectedPipeline.value = ''
  pipelineModel.value = ''
  loadPipelines()
}

function extractPipelineModel(pipe: any): string {
  if (!pipe?.nodes?.length) return ''
  for (const node of pipe.nodes) {
    const model = node?.config?.model || node?.model
    if (model) return model
  }
  return ''
}

function onPipelineChange(pipeId: string) {
  if (!pipeId) {
    pipelineModel.value = ''
    return
  }
  const pipe = pipelines.value.find(p => p.id === pipeId)
  pipelineModel.value = pipe ? extractPipelineModel(pipe) : ''
}

function onAgentChange() {
  configResult.value = null
  writeResult.value = null
  selectedPipeline.value = ''
  selectedBackend.value = ''
  pipelineModel.value = ''
  loadBackends()
  loadPipelines()
}

function buildSanitizedConfigPreview(content: string): string {
  const masked = content
    .replace(/llmproxy_[a-zA-Z0-9]+/g, 'llmproxy_***')
    .replace(/("api[_-]?key"\s*:\s*")[^"]*(")/gi, '$1***$2')
    .replace(/(api[_-]?key\s*:\s*")[^"]*(")/gi, '$1***$2')
    .replace(/(api[_-]?key\s*=\s*)[^\n]*/gi, '$1***')
  const lines = masked.split('\n')
  const maxLines = 28
  if (lines.length <= maxLines) return masked
  return `${lines.slice(0, maxLines).join('\n')}\n...`
}

function isMissingProxyAPIKeyError(message: string): boolean {
  return message.includes('未找到可用于 Agent 接入的 Centag API Key')
}

async function maybeHandleMissingProxyAPIKeyError(message: string): Promise<boolean> {
  if (!isMissingProxyAPIKeyError(message)) return false
  try {
    await ElMessageBox.confirm(
      '当前账号没有可用的 Centag API Key（llmproxy_*），请先到个人中心创建后再重试。',
      '缺少 Centag API Key',
      {
        confirmButtonText: '前往个人中心',
        cancelButtonText: '稍后',
        type: 'warning',
      }
    )
    router.push('/profile')
  } catch {
    // 用户取消时不做跳转
  }
  return true
}

async function generateConfig() {
  if (!selectedAgent.value || !selectedPipeline.value) return
  loadingConfig.value = true
  configResult.value = null
  try {
    const payload: any = {
      agent_type: selectedAgent.value,
      pipeline_id: selectedPipeline.value,
    }
    if (selectedBackend.value) payload.backend_id = selectedBackend.value
    if (!selectedPipeline.value && pipelineModel.value) payload.model = pipelineModel.value
    const res: any = await api.post('/api/v1/agent/configs/generate', payload)
    configResult.value = res
  } catch (e: any) {
    if (await maybeHandleMissingProxyAPIKeyError(e.message)) return
    ElMessage.error('生成配置失败：' + e.message)
  } finally {
    loadingConfig.value = false
  }
}

async function writeToConfig() {
  if (!selectedAgent.value || !selectedPipeline.value) return
  writingConfig.value = true
  writeResult.value = null
  try {
    const payload: any = {
      agent_type: selectedAgent.value,
      pipeline_id: selectedPipeline.value,
    }
    if (selectedBackend.value) payload.backend_id = selectedBackend.value
    if (!selectedPipeline.value && pipelineModel.value) payload.model = pipelineModel.value
    const res: any = await api.post('/api/v1/agent/configs/write', payload)
    writeResult.value = res
    if (res.success) {
      ElMessage.success('配置写入成功')
    } else {
      ElMessage.error('配置写入失败：' + res.message)
    }
  } catch (e: any) {
    writeResult.value = { success: false, message: e.message }
    if (await maybeHandleMissingProxyAPIKeyError(e.message)) return
    ElMessage.error('写入请求失败：' + e.message)
  } finally {
    writingConfig.value = false
  }
}

function copyText(text: string) {
  navigator.clipboard.writeText(text).then(() => {
    ElMessage.success('已复制到剪贴板')
  }).catch(() => {
    ElMessage.error('复制失败，请手动选择')
  })
}

function agentIcon(type: string) {
  const map: Record<string, any> = {
    'claude-code': ChatDotRound,
    'claude-desktop': ChatDotRound,
    'codex': Monitor,
    'gemini-cli': DataLine,
    'grok-build': Connection,
    'opencode': Connection,
    'openclaw': Connection,
    'hermes': Connection,
  }
  return map[type] || Connection
}

function resetWizard() {
  selectedAgent.value = ''
  selectedPipeline.value = ''
  selectedBackend.value = ''
  pipelineModel.value = ''
  configResult.value = null
}

// ===================== 供应商配置 (只读视图) =====================

interface AgentProviderConfig {
  id: string
  agent_type: string
  display_name: string
  backend_id: string
  pipeline_id: string
  model: string
  api_key: string
  enabled: boolean
  description: string
}

const loadingProviders = ref(false)
const providers = ref<AgentProviderConfig[]>([])

async function loadProviders() {
  loadingProviders.value = true
  try {
    const res: any = await api.get('/api/v1/agent-providers')
    providers.value = res.agent_providers || []
  } catch (e: any) {
    ElMessage.error('加载供应商配置失败：' + e.message)
  } finally {
    loadingProviders.value = false
  }
}

async function handleHotSwap(provider: AgentProviderConfig) {
  try {
    await api.post(`/api/v1/agent-providers/${provider.id}/hotswap`, {
      agent_type: provider.agent_type,
      backend_id: provider.backend_id,
    })
    ElMessage.success(`已将 ${provider.display_name || provider.agent_type} 设为默认`)
    loadProviders()
  } catch (e: any) {
    ElMessage.error('HotSwap 失败：' + e.message)
  }
}

// ===================== Init =====================

onMounted(() => {
  loadAgentTypes()
  if (isAdmin.value) {
    loadProviders()
  }
})
</script>

<style scoped>
.agent-setup-page {
  padding: 24px;
}

.page-header {
  margin-bottom: 24px;
}

.page-title {
  font-size: 1.5rem;
  font-weight: 600;
  margin: 0;
  display: flex;
  align-items: center;
  gap: 8px;
}

.page-description {
  color: #6b7280;
  font-size: 0.875rem;
  margin: 4px 0 0;
}

.setup-tabs {
  margin-top: 8px;
}

.step-actions {
  margin-top: 24px;
  display: flex;
  gap: 12px;
  justify-content: center;
}

/* Agent cards */
.agent-card {
  border: 2px solid #e4e7ed;
  border-radius: 12px;
  padding: 20px;
  margin-bottom: 16px;
  cursor: pointer;
  transition: all 0.2s;
  display: flex;
  align-items: center;
  gap: 16px;
}

.agent-card:hover {
  border-color: #c0c4cc;
  background: #fafafa;
}

.agent-card.active {
  border-color: var(--el-color-primary);
  background: #ecf5ff;
}

.agent-icon {
  width: 56px;
  height: 56px;
  border-radius: 12px;
  background: #f0f2f5;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #606266;
  flex-shrink: 0;
}

.agent-card.active .agent-icon {
  background: var(--el-color-primary-light-8);
  color: var(--el-color-primary);
}

.agent-name {
  font-weight: 600;
  font-size: 1rem;
  margin-bottom: 4px;
}

.agent-desc {
  color: #909399;
  font-size: 0.8rem;
  line-height: 1.4;
}

/* Backend selection */
.select-label {
  font-size: 0.9rem;
  font-weight: 600;
  color: #303133;
  margin: 16px 0 8px;
}

.select-label:first-child {
  margin-top: 0;
}

.backend-section {
  margin-bottom: 20px;
}

.backend-hint {
  margin-top: 8px;
  display: flex;
  align-items: center;
  gap: 8px;
}

.backend-option {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 0;
}

.backend-name {
  font-weight: 500;
}

/* Pipeline section */
.pipeline-section {
  margin-top: 20px;
}

.pipeline-option {
  display: flex;
  flex-direction: column;
}

.pipeline-desc {
  font-size: 0.75rem;
  color: #909399;
}

.model-display {
  margin-top: 12px;
  display: flex;
  align-items: center;
  gap: 8px;
}

.model-display .el-tag {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.model-hint {
  font-size: 0.8rem;
  color: #909399;
}

/* Generate button */
.generate-section {
  margin-top: 24px;
  text-align: center;
}

/* Config result */
.config-result {
  margin-top: 24px;
}

.config-header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 20px;
}

.config-header h3 {
  margin: 0;
}

.config-section {
  margin-bottom: 24px;
}

.config-section h4 {
  margin: 0 0 12px;
  font-size: 0.95rem;
  color: #303133;
}

.code-block {
  position: relative;
  background: #f5f7fa;
  border: 1px solid #e4e7ed;
  border-radius: 8px;
  overflow: hidden;
}

.code-block pre {
  margin: 0;
  padding: 16px;
  overflow-x: auto;
  font-size: 0.85rem;
  line-height: 1.6;
}

.code-block code {
  font-family: 'Menlo', 'Monaco', 'Consolas', monospace;
  color: #303133;
}

.copy-btn {
  position: absolute;
  top: 8px;
  right: 8px;
  background: white;
  border: 1px solid #dcdfe6;
  border-radius: 4px;
  padding: 4px 8px;
}

.copy-btn:hover {
  background: #f5f7fa;
}

.empty-backends {
  padding: 40px 0;
}

.write-hint {
  margin-top: 12px;
  font-size: 0.85rem;
}

.write-preview {
  margin-top: 12px;
}

.write-preview h5 {
  margin: 0 0 8px;
  font-size: 0.85rem;
  color: #606266;
}

/* Provider table */
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.text-muted {
  color: #909399;
  font-size: 0.85rem;
}
</style>
