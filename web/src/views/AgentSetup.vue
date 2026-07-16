<template>
  <div class="agent-setup-page">
    <div class="page-header">
      <div>
        <h1 class="page-title">
          <el-icon><Link /></el-icon>
          Agent 快速接入
        </h1>
        <p class="page-description">选择你的 Agent 工具，一键生成接入 Centag 的配置</p>
      </div>
    </div>

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

    <!-- 选择流水线 + 配置 -->
    <div v-if="selectedAgent" class="section-block">
      <el-card class="config-card">
        <div v-if="pipelines.length === 0 && !loadingPipelines" class="empty-backends">
          <el-empty description="暂无可用流水线，请先在「流水线管理」中添加">
            <el-button type="primary" @click="$router.push('/pipelines')">前往流水线管理</el-button>
          </el-empty>
        </div>
        <div v-else>
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

            <!-- 模型信息（只读 label） -->
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

        <!-- 配置结果（内联展示） -->
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
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useRouter } from 'vue-router'
import {
  Link, DocumentCopy, Plus,
  Monitor, ChatDotRound, DataLine, Connection, Cpu
} from '@element-plus/icons-vue'
import { isPersonalEdition } from '@/utils/edition'
import api from '@/api'

// --- State ---
const router = useRouter()
const selectedAgent = ref('')
const selectedPipeline = ref('')
const pipelineModel = ref('')
const loadingPipelines = ref(false)
const loadingConfig = ref(false)
const writingConfig = ref(false)
const platformTab = ref('macos')
const agentTypes = ref<Array<{ type: string; display_name: string; description: string }>>([])
const pipelines = ref<Array<{ id: string; name: string; description?: string; nodes?: any[] }>>([])
const configResult = ref<any>(null)
const writeResult = ref<{ success: boolean; message: string; written?: Array<{ path: string; content: string }> } | null>(null)

const isDesktopEdition = computed(() => isPersonalEdition())

// 用于展示的模型名：有流水线时仅显示 pipeline.<id>（具体模型由流水线决定）
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

// --- Load agent types ---
async function loadAgentTypes() {
  try {
    const res: any = await api.get('/api/v1/agent/types')
    agentTypes.value = res.agent_types || []
  } catch (e: any) {
    ElMessage.error('加载 Agent 类型失败：' + e.message)
  }
}

// --- Load pipelines ---
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

// --- Extract model from pipeline ---
function extractPipelineModel(pipe: any): string {
  if (!pipe?.nodes?.length) return ''
  // 取第一个有 model 的节点
  for (const node of pipe.nodes) {
    const model = node?.config?.model || node?.model
    if (model) return model
  }
  return ''
}

// --- Pipeline changed ---
function onPipelineChange(pipeId: string) {
  if (!pipeId) {
    pipelineModel.value = ''
    return
  }
  const pipe = pipelines.value.find(p => p.id === pipeId)
  pipelineModel.value = pipe ? extractPipelineModel(pipe) : ''
}

// --- Agent changed ---
function onAgentChange() {
  configResult.value = null
  writeResult.value = null
  selectedPipeline.value = ''
  pipelineModel.value = ''
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

// --- Generate config ---
async function generateConfig() {
  if (!selectedAgent.value || !selectedPipeline.value) return
  loadingConfig.value = true
  configResult.value = null
  try {
    const payload: any = {
      agent_type: selectedAgent.value,
      pipeline_id: selectedPipeline.value,
    }
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

// --- Write config to local files ---
async function writeToConfig() {
  if (!selectedAgent.value || !selectedPipeline.value) return
  writingConfig.value = true
  writeResult.value = null
  try {
    const payload: any = {
      agent_type: selectedAgent.value,
      pipeline_id: selectedPipeline.value,
    }
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

// --- Copy to clipboard ---
function copyText(text: string) {
  navigator.clipboard.writeText(text).then(() => {
    ElMessage.success('已复制到剪贴板')
  }).catch(() => {
    ElMessage.error('复制失败，请手动选择')
  })
}

// --- Agent icon mapping ---
function agentIcon(type: string) {
  const map: Record<string, any> = {
    'claude-code': ChatDotRound,
    'claude-desktop': ChatDotRound,
    'codex': Monitor,
    'gemini-cli': DataLine,
    'opencode': Connection,
    'openclaw': Connection,
    'hermes': Connection,
  }
  return map[type] || Connection
}

// --- Reset ---
function resetWizard() {
  selectedAgent.value = ''
  selectedPipeline.value = ''
  pipelineModel.value = ''
  configResult.value = null
}

// --- Init ---
onMounted(() => {
  loadAgentTypes()
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

.backend-radio-group {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.backend-radio {
  height: auto !important;
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

/* Config result (inline) */
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
</style>
