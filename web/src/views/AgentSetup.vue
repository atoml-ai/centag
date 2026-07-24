<template>
  <div class="agent-setup-page">
    <div class="page-header">
      <div>
        <h1 class="page-title">
          <el-icon><Link /></el-icon>
          Agent 接入
        </h1>
        <p class="page-description">将 Agent 工具接入 Centag 代理：选择流水线、写入配置并验证生效</p>
      </div>
    </div>

    <el-tabs v-model="activeTab" class="setup-tabs">
      <!-- Tab 1: 快速接入 -->
      <el-tab-pane label="快速接入" name="setup">
        <div class="section-block">
          <p class="section-hint">点击下方 Agent，通过向导接入 Centag 代理（只需选择流水线，后端由流水线内部指定）。</p>
          <el-row :gutter="16">
            <el-col
              v-for="agent in agentTypes"
              :key="agent.type"
              :xs="24" :sm="12" :md="8"
            >
              <div class="agent-card" @click="openWizard(agent)">
                <div class="agent-icon">
                  <el-icon :size="32">
                    <component :is="agentIcon(agent.type)" />
                  </el-icon>
                </div>
                <div class="agent-info">
                  <div class="agent-name">{{ agent.display_name }}</div>
                  <div class="agent-desc">{{ agent.description }}</div>
                </div>
                <div class="agent-actions" @click.stop>
                  <el-button type="primary" link @click="openWizard(agent)">
                    接入代理
                  </el-button>
                  <el-button
                    type="info"
                    link
                    :loading="restoringAgent === agent.type"
                    @click="restoreDefaults(agent)"
                  >
                    恢复默认
                  </el-button>
                </div>
              </div>
            </el-col>
          </el-row>
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

    <!-- 接入向导对话框 -->
    <el-dialog
      v-model="wizardVisible"
      :title="wizardTitle"
      width="720px"
      :close-on-click-modal="false"
      destroy-on-close
      class="agent-wizard-dialog"
      @closed="resetWizard"
    >
      <el-steps :active="wizardStep" finish-status="success" align-center class="wizard-steps">
        <el-step title="选择流水线" description="决定路由与模型" />
        <el-step title="写入配置" description="应用到 Agent" />
        <el-step title="验证生效" description="确认代理可用" />
      </el-steps>

      <!-- Step 1: 选择流水线 -->
      <div v-show="wizardStep === 0" class="wizard-step" v-loading="loadingPipelines">
        <el-alert type="info" :closable="false" show-icon class="step-alert">
          只需选择流水线。后端与模型由流水线内部节点决定，无需在此单独配置。
        </el-alert>

        <el-alert
          v-if="hasEnabledAPIKey === false"
          type="warning"
          :closable="false"
          show-icon
          class="step-alert"
        >
          <template #title>尚未配置可用的 Centag API Key</template>
          接入时会自动把当前账号下可解密的 <code>llmproxy_*</code> 密钥写入 Agent 配置。
          请先到个人中心创建 API Key，否则无法生成/写入配置。
          <div class="alert-actions">
            <el-button type="warning" size="small" @click="goProfileForAPIKey">前往创建 API Key</el-button>
          </div>
        </el-alert>
        <el-alert
          v-else-if="hasEnabledAPIKey === true"
          type="success"
          :closable="false"
          show-icon
          class="step-alert"
        >
          已检测到可用 API Key；下一步生成配置时会自动填入（不会在界面明文展示完整密钥）。
        </el-alert>

        <div v-if="pipelines.length === 0 && !loadingPipelines" class="empty-pipelines">
          <el-empty description="暂无可用流水线，请先在「策略管理」中添加">
            <el-button type="primary" @click="goPipelines">前往策略管理</el-button>
          </el-empty>
        </div>
        <div v-else>
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

          <div v-if="selectedPipeline" class="pipeline-summary">
            <el-tag type="success" size="default">流水线: {{ selectedPipelineName }}</el-tag>
            <el-tag type="info" size="default">
              <el-icon><Cpu /></el-icon>
              模型路由: centag/{{ selectedPipeline }}
            </el-tag>
            <span v-if="pipelineModel" class="model-hint">流水线内模型示意: {{ pipelineModel }}</span>
          </div>
        </div>
      </div>

      <!-- Step 2: 写入配置 -->
      <div v-show="wizardStep === 1" class="wizard-step" v-loading="loadingConfig">
        <el-alert type="info" :closable="false" show-icon class="step-alert">
          将 Centag 代理地址与当前账号的 API Key 写入 {{ currentAgentName }}。
          密钥来自「个人中心 → API Keys」中可解密的 <code>llmproxy_*</code> 密钥（服务端注入，界面预览已脱敏）。
        </el-alert>

        <div v-if="configResult" class="config-result">
          <div class="config-header">
            <h3>{{ configResult.description }}</h3>
            <el-tag>路由: {{ configResult.backend_name }}</el-tag>
          </div>

          <!-- 一键写入 -->
          <div class="config-section">
            <h4>一键配置</h4>
            <el-button type="primary" :loading="writingConfig" @click="writeToConfig">
              <el-icon class="el-icon--left"><Plus /></el-icon>
              写入配置文件
            </el-button>
            <p class="write-hint" v-if="writeResult">
              <span v-if="writeResult.success" style="color: #67c23a">✓ {{ writeResult.message }}</span>
              <span v-else style="color: #f56c6c">✗ {{ writeResult.message }}</span>
            </p>
          </div>

          <!-- 写入成功：只展示已写入预览，避免与「配置预览」重复 -->
          <div v-if="writeSucceeded && writePreviewFiles.length" class="config-section">
            <h4>已写入配置（已脱敏）</h4>
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

          <!-- 未写入成功时：展示配置预览（脱敏）；团队版另提供命令作为备选 -->
          <template v-if="!writeSucceeded">
            <div v-if="sanitizedConfigFiles.length" class="config-section">
              <h4>配置预览（已脱敏）</h4>
              <p class="section-subhint">确认无误后点击上方「写入配置文件」；团队版也可复制下方命令自行写入。</p>
              <el-collapse>
                <el-collapse-item
                  v-for="file in sanitizedConfigFiles"
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

            <div v-if="!isDesktopEdition && configResult.commands" class="config-section">
              <h4>配置命令（含密钥，请妥善保管）</h4>
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
          </template>
        </div>
        <el-empty v-else-if="!loadingConfig" description="配置生成失败，请返回上一步重试" />
      </div>

      <!-- Step 3: 验证生效 -->
      <div v-show="wizardStep === 2" class="wizard-step">
        <el-result
          icon="success"
          title="配置已就绪"
          :sub-title="`请按下列方式验证 ${currentAgentName} 是否已走 Centag 代理`"
        />

        <el-alert type="success" :closable="false" show-icon class="step-alert">
          验证通过后，该 Agent 的请求将经 Centag，并按所选流水线路由到其内部配置的后端与模型。
        </el-alert>

        <div v-if="configResult?.verify_cmd" class="config-section">
          <h4>验证命令</h4>
          <p class="verify-desc">在终端执行以下命令，确认能连通 Centag 并返回模型响应（需本机已安装对应 Agent CLI）：</p>
          <div class="code-block">
            <pre><code>{{ configResult.verify_cmd }}</code></pre>
            <el-button class="copy-btn" :icon="DocumentCopy" @click="copyText(configResult.verify_cmd)" />
          </div>
        </div>

        <div class="config-section">
          <h4>验证清单</h4>
          <ol class="verify-checklist">
            <li>重启或重新打开 {{ currentAgentName }}，确保加载了最新配置。</li>
            <li>发起一次简单对话或补全请求（例如询问当前模型）。</li>
            <li>在 Centag「请求日志 / 仪表盘」中确认出现对应请求，且流水线为 <code>{{ selectedPipeline }}</code>。</li>
            <li>若失败：检查 API Key 是否有效、Centag 服务是否运行、Agent 配置中的 Base URL 是否指向本机 Centag。</li>
          </ol>
        </div>
      </div>

      <template #footer>
        <div class="wizard-footer">
          <el-button @click="wizardVisible = false">取消</el-button>
          <div class="wizard-footer-right">
            <el-button :disabled="wizardStep === 0 || loadingConfig || writingConfig" @click="prevStep">
              上一步
            </el-button>
            <el-button
              v-if="wizardStep < 2"
              type="primary"
              :loading="loadingConfig"
              :disabled="!canGoNext"
              @click="nextStep"
            >
              下一步
            </el-button>
            <el-button v-else type="primary" @click="wizardVisible = false">
              完成
            </el-button>
          </div>
        </div>
      </template>
    </el-dialog>
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
import { listAPIKeys } from '@/api/user'
import api from '@/api'

const router = useRouter()
const authStore = useAuthStore()
const isAdmin = computed(() => authStore.isAdmin)

const activeTab = ref('setup')

/** 系统默认流水线：透明模式 */
const DEFAULT_PIPELINE_ID = 'transparent-proxy'

// ===================== 快速接入向导 =====================

const wizardVisible = ref(false)
const wizardStep = ref(0)
const selectedAgent = ref('')
const selectedAgentDisplay = ref('')
const selectedPipeline = ref('')
const pipelineModel = ref('')
const loadingPipelines = ref(false)
const loadingConfig = ref(false)
const writingConfig = ref(false)
const restoringAgent = ref('')
const platformTab = ref('macos')
const agentTypes = ref<Array<{ type: string; display_name: string; description: string }>>([])
const pipelines = ref<Array<{ id: string; name: string; description?: string; nodes?: any[] }>>([])
const configResult = ref<any>(null)
const writeResult = ref<{ success: boolean; message: string; written?: Array<{ path: string; content: string }> } | null>(null)
/** null=未检测；true/false=是否有启用中的 API Key（列表级提示，最终以服务端解密结果为准） */
const hasEnabledAPIKey = ref<boolean | null>(null)

const isDesktopEdition = computed(() => isPersonalEdition())

const wizardTitle = computed(() => {
  if (!selectedAgentDisplay.value) return '接入 Centag 代理'
  return `接入 Centag 代理 — ${selectedAgentDisplay.value}`
})

const currentAgentName = computed(() => selectedAgentDisplay.value || selectedAgent.value || 'Agent')

const selectedPipelineName = computed(() => {
  const pipe = pipelines.value.find(p => p.id === selectedPipeline.value)
  return pipe?.name || selectedPipeline.value
})

const writeSucceeded = computed(() => !!writeResult.value?.success)

const canGoNext = computed(() => {
  if (wizardStep.value === 0) {
    return !!selectedPipeline.value && pipelines.value.length > 0 && hasEnabledAPIKey.value !== false
  }
  if (wizardStep.value === 1) return !!configResult.value && !loadingConfig.value
  return true
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

const sanitizedConfigFiles = computed(() => {
  if (!Array.isArray(configResult.value?.files)) return []
  return configResult.value.files
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

function pickDefaultPipelineID(list: Array<{ id: string; name: string }>): string {
  const byID = list.find(p => p.id === DEFAULT_PIPELINE_ID)
  if (byID) return byID.id
  const byName = list.find(p => p.name === '透明模式' || /transparent/i.test(p.id))
  return byName?.id || ''
}

async function loadPipelines() {
  loadingPipelines.value = true
  try {
    const res: any = await api.get('/api/v1/pipelines')
    const data = res?.data || res
    pipelines.value = Array.isArray(data) ? data : []
    if (!selectedPipeline.value) {
      const defaultID = pickDefaultPipelineID(pipelines.value)
      if (defaultID) {
        selectedPipeline.value = defaultID
        onPipelineChange(defaultID)
      }
    }
  } catch (e: any) {
    pipelines.value = []
    console.warn('加载流水线列表失败:', e.message)
  } finally {
    loadingPipelines.value = false
  }
}

async function checkAPIKeys() {
  try {
    const keys = await listAPIKeys()
    hasEnabledAPIKey.value = Array.isArray(keys) && keys.some(k => k.enabled)
  } catch (e: any) {
    hasEnabledAPIKey.value = null
    console.warn('检测 API Key 失败:', e.message)
  }
}

function goProfileForAPIKey() {
  wizardVisible.value = false
  router.push('/profile')
}

async function restoreDefaults(agent: { type: string; display_name: string }) {
  try {
    await ElMessageBox.confirm(
      `将恢复 ${agent.display_name} 的本地配置为接入 Centag 之前的状态。\n若写入前有备份则还原备份；若配置由 Centag 新建则删除对应文件。`,
      '恢复默认配置',
      {
        confirmButtonText: '恢复默认',
        cancelButtonText: '取消',
        type: 'warning',
      }
    )
  } catch {
    return
  }

  restoringAgent.value = agent.type
  try {
    const res: any = await api.post('/api/v1/agent/configs/restore', {
      agent_type: agent.type,
    })
    if (res?.success) {
      ElMessage.success(res.message || '已恢复默认配置')
    } else {
      ElMessage.error(res?.message || '恢复失败')
    }
  } catch (e: any) {
    ElMessage.error('恢复默认失败：' + e.message)
  } finally {
    restoringAgent.value = ''
  }
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
  configResult.value = null
  writeResult.value = null
}

function openWizard(agent: { type: string; display_name: string }) {
  selectedAgent.value = agent.type
  selectedAgentDisplay.value = agent.display_name
  wizardStep.value = 0
  selectedPipeline.value = ''
  pipelineModel.value = ''
  configResult.value = null
  writeResult.value = null
  hasEnabledAPIKey.value = null
  wizardVisible.value = true
  loadPipelines()
  checkAPIKeys()
}

function resetWizard() {
  wizardStep.value = 0
  selectedAgent.value = ''
  selectedAgentDisplay.value = ''
  selectedPipeline.value = ''
  pipelineModel.value = ''
  configResult.value = null
  writeResult.value = null
  hasEnabledAPIKey.value = null
}

function goPipelines() {
  wizardVisible.value = false
  router.push('/pipelines')
}

async function nextStep() {
  if (wizardStep.value === 0) {
    if (!selectedPipeline.value) return
    const ok = await generateConfig()
    if (!ok) return
    wizardStep.value = 1
    return
  }
  if (wizardStep.value === 1 && configResult.value) {
    wizardStep.value = 2
  }
}

function prevStep() {
  if (wizardStep.value > 0) {
    wizardStep.value -= 1
  }
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
    wizardVisible.value = false
    router.push('/profile')
  } catch {
    // 用户取消时不做跳转
  }
  return true
}

async function generateConfig(): Promise<boolean> {
  if (!selectedAgent.value || !selectedPipeline.value) return false
  loadingConfig.value = true
  configResult.value = null
  writeResult.value = null
  try {
    const res: any = await api.post('/api/v1/agent/configs/generate', {
      agent_type: selectedAgent.value,
      pipeline_id: selectedPipeline.value,
    })
    configResult.value = res
    return true
  } catch (e: any) {
    if (await maybeHandleMissingProxyAPIKeyError(e.message)) return false
    ElMessage.error('生成配置失败：' + e.message)
    return false
  } finally {
    loadingConfig.value = false
  }
}

async function writeToConfig() {
  if (!selectedAgent.value || !selectedPipeline.value) return
  writingConfig.value = true
  writeResult.value = null
  try {
    const res: any = await api.post('/api/v1/agent/configs/write', {
      agent_type: selectedAgent.value,
      pipeline_id: selectedPipeline.value,
    })
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

.section-hint {
  color: #606266;
  font-size: 0.875rem;
  margin: 0 0 16px;
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
  border-color: var(--el-color-primary-light-3);
  background: #fafafa;
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

.agent-card:hover .agent-icon {
  background: var(--el-color-primary-light-8);
  color: var(--el-color-primary);
}

.agent-info {
  flex: 1;
  min-width: 0;
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

.agent-actions {
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 2px;
}

/* Wizard */
.wizard-steps {
  margin-bottom: 24px;
}

.wizard-step {
  min-height: 240px;
}

.step-alert {
  margin-bottom: 16px;
}

.alert-actions {
  margin-top: 8px;
}

.section-subhint {
  margin: 0 0 8px;
  font-size: 0.8rem;
  color: #909399;
}

.select-label {
  font-size: 0.9rem;
  font-weight: 600;
  color: #303133;
  margin: 0 0 8px;
}

.pipeline-option {
  display: flex;
  flex-direction: column;
}

.pipeline-desc {
  font-size: 0.75rem;
  color: #909399;
}

.pipeline-summary {
  margin-top: 12px;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}

.pipeline-summary .el-tag {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.model-hint {
  font-size: 0.8rem;
  color: #909399;
}

.empty-pipelines {
  padding: 24px 0;
}

.config-result {
  margin-top: 8px;
}

.config-header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 20px;
}

.config-header h3 {
  margin: 0;
  font-size: 1rem;
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

.verify-desc {
  margin: 0 0 8px;
  font-size: 0.875rem;
  color: #606266;
}

.verify-checklist {
  margin: 0;
  padding-left: 1.25rem;
  color: #303133;
  font-size: 0.875rem;
  line-height: 1.8;
}

.verify-checklist code {
  background: #f5f7fa;
  padding: 1px 6px;
  border-radius: 4px;
  font-size: 0.8rem;
}

.wizard-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  width: 100%;
}

.wizard-footer-right {
  display: flex;
  gap: 8px;
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
