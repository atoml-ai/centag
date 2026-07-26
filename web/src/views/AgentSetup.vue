<template>
  <div class="agent-setup-page">
    <div class="page-header">
      <div>
        <h1 class="page-title">
          <el-icon><Link /></el-icon>
          {{ $t('agentSetup.title') }}
        </h1>
        <p class="page-description">{{ $t('agentSetup.subtitle') }}</p>
      </div>
    </div>

    <el-tabs v-model="activeTab" class="setup-tabs">
      <!-- Tab 1: 快速接入 -->
      <el-tab-pane :label="$t('agentSetup.quickSetup')" name="setup">
        <div class="section-block">
          <p class="section-hint">{{ $t('agentSetup.quickSetupHint') }}</p>

          <div
            v-for="group in agentGroups"
            :key="group.id"
            class="agent-group"
          >
            <div class="agent-group-header">
              <h3 class="agent-group-title">{{ group.title }}</h3>
              <p v-if="group.hint" class="agent-group-hint">{{ group.hint }}</p>
            </div>
            <el-row :gutter="16" class="agent-card-row">
              <el-col
                v-for="agent in group.agents"
                :key="agent.type"
                :xs="24" :sm="12" :md="8"
                class="agent-card-col"
              >
                <div
                  class="agent-card"
                  :class="{ 'agent-card--verified': agent.verified }"
                >
                  <div class="agent-card-head">
                    <div class="agent-icon">
                      <el-icon :size="20">
                        <component :is="agentIcon(agent.type)" />
                      </el-icon>
                    </div>
                    <div class="agent-head-text">
                      <div class="agent-name">
                        <span class="agent-name-text">{{ agent.display_name }}</span>
                        <el-tag v-if="agent.verified" size="small" type="success" effect="plain" class="verified-tag">
                          {{ $t('agentSetup.verified') }}
                        </el-tag>
                      </div>
                      <div class="agent-desc">{{ agentLocalized(agent, 'description') }}</div>
                    </div>
                  </div>

                  <div class="access-methods" @click.stop>
                    <!-- 方式一：写入配置 -->
                    <div v-if="agent.write_mode !== 'none'" class="access-method">
                      <div class="access-method-head">
                        <div class="access-method-titles">
                          <span class="access-method-index">1</span>
                          <div>
                            <div class="access-method-title">{{ $t('agentSetup.methodWriteConfig') }}</div>
                            <p class="access-method-hint">{{ $t('agentSetup.methodWriteConfigHint') }}</p>
                          </div>
                        </div>
                        <el-button type="primary" size="small" @click="openWizard(agent)">
                          {{ $t('agentSetup.writeConfigAction') }}
                        </el-button>
                      </div>
                    </div>

                    <!-- 方式二：wrap 运行（无需改配置） -->
                    <div v-if="agent.wrap_command" class="access-method">
                      <div class="access-method-head">
                        <div class="access-method-titles">
                          <span class="access-method-index">{{ agent.write_mode !== 'none' ? '2' : '1' }}</span>
                          <div>
                            <div class="access-method-title">{{ $t('agentSetup.methodWrap') }}</div>
                            <p class="access-method-hint">{{ $t('agentSetup.methodWrapHint') }}</p>
                          </div>
                        </div>
                      </div>
                      <div class="wrap-cmd-row">
                        <code class="wrap-cmd">{{ agent.wrap_command }}</code>
                        <el-button
                          class="wrap-copy-btn"
                          link
                          type="primary"
                          :icon="DocumentCopy"
                          :title="$t('agentSetup.copyWrapCommand')"
                          @click="copyText(agent.wrap_command!)"
                        />
                      </div>
                    </div>

                    <!-- 内置 Agent：无本地配置、无 wrap -->
                    <div
                      v-if="agent.write_mode === 'none' && !agent.wrap_command"
                      class="access-method"
                    >
                      <div class="access-method-head">
                        <div class="access-method-titles">
                          <span class="access-method-index">1</span>
                          <div>
                            <div class="access-method-title">{{ $t('agentSetup.methodBuiltin') }}</div>
                            <p class="access-method-hint">{{ $t('agentSetup.methodBuiltinHint') }}</p>
                          </div>
                        </div>
                        <el-button type="primary" size="small" @click="openWizard(agent)">
                          {{ $t('agentSetup.connectProxy') }}
                        </el-button>
                      </div>
                    </div>
                  </div>

                  <el-collapse class="agent-meta-collapse" @click.stop>
                    <el-collapse-item :title="$t('agentSetup.moreDetails')" name="details">
                      <div class="meta-block">
                        <div class="meta-row">
                          <span class="meta-label">{{ $t('agentSetup.writeMode') }}</span>
                          <span>{{ writeModeLabel(agent.write_mode) }}</span>
                        </div>
                        <div v-if="agent.install_url || agentLocalized(agent, 'installHint')" class="meta-row meta-row-col">
                          <span class="meta-label">{{ $t('agentSetup.installGuide') }}</span>
                          <a
                            v-if="agent.install_url"
                            class="install-link"
                            :href="agent.install_url"
                            target="_blank"
                            rel="noopener noreferrer"
                          >{{ agent.install_url }}</a>
                          <span v-if="agentLocalized(agent, 'installHint')" class="install-hint">
                            {{ agentLocalized(agent, 'installHint') }}
                          </span>
                        </div>
                        <div v-if="agent.config_paths?.length" class="meta-row meta-row-col">
                          <span class="meta-label">{{ $t('agentSetup.configPaths') }}</span>
                          <code v-for="p in agent.config_paths" :key="p" class="meta-path">{{ p }}</code>
                        </div>
                        <div v-if="agent.key_fields?.length" class="meta-row meta-row-col">
                          <span class="meta-label">{{ $t('agentSetup.keyFields') }}</span>
                          <span class="meta-fields">{{ agent.key_fields.join(', ') }}</span>
                        </div>
                        <p class="meta-method">{{ agentLocalized(agent, 'configMethod') || $t('agentSetup.noConfigMethod') }}</p>
                        <div v-if="agent.write_mode !== 'none'" class="meta-actions">
                          <el-button
                            size="small"
                            :loading="restoringAgent === agent.type"
                            @click="restoreDefaults(agent)"
                          >
                            {{ $t('agentSetup.restoreDefault') }}
                          </el-button>
                        </div>
                      </div>
                    </el-collapse-item>
                  </el-collapse>
                </div>
              </el-col>
            </el-row>
          </div>
        </div>
      </el-tab-pane>

      <!-- Tab 2: 供应商配置（仅管理员，只读视图） -->
      <el-tab-pane :label="$t('agentSetup.providerConfig')" name="providers" v-if="isAdmin">
        <el-card>
          <template #header>
            <div class="card-header">
              <span>{{ $t('agentSetup.builtinProviderRoute') }}</span>
            </div>
          </template>

          <el-table :data="providers" v-loading="loadingProviders" stripe>
            <el-table-column prop="agent_type" :label="$t('agentSetup.agentType')" width="150" />
            <el-table-column prop="display_name" :label="$t('agentSetup.displayName')" width="150" />
            <el-table-column prop="backend_id" :label="$t('agentSetup.backendId')" width="180">
              <template #default="{ row }">
                <el-tag v-if="row.backend_id" type="success">{{ row.backend_id }}</el-tag>
                <span v-else class="text-muted">{{ $t('agentSetup.default') }}</span>
              </template>
            </el-table-column>
            <el-table-column prop="pipeline_id" :label="$t('agentSetup.pipelineId')" width="180">
              <template #default="{ row }">
                <el-tag v-if="row.pipeline_id" type="warning">{{ row.pipeline_id }}</el-tag>
                <span v-else class="text-muted">{{ $t('agentSetup.default') }}</span>
              </template>
            </el-table-column>
            <el-table-column prop="model" :label="$t('agentSetup.modelOverride')" width="150">
              <template #default="{ row }">
                <span v-if="row.model">{{ row.model }}</span>
                <span v-else class="text-muted">-</span>
              </template>
            </el-table-column>
            <el-table-column prop="enabled" :label="$t('agentSetup.status')" width="80">
              <template #default="{ row }">
                <el-tag :type="row.enabled ? 'success' : 'info'" size="small">
                  {{ row.enabled ? $t('agentSetup.enabled') : $t('agentSetup.disabled') }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column :label="$t('agentSetup.actions')" width="120" fixed="right">
              <template #default="{ row }">
                <el-button size="small" type="primary" @click="handleHotSwap(row)">
                  {{ $t('agentSetup.setDefault') }}
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
        <el-step :title="$t('agentSetup.selectPipeline')" :description="$t('agentSetup.decideRouteAndModel')" />
        <el-step :title="$t('agentSetup.writeConfig')" :description="$t('agentSetup.applyToAgent')" />
        <el-step :title="$t('agentSetup.verify')" :description="$t('agentSetup.confirmProxyAvailable')" />
      </el-steps>

      <!-- Step 1: 选择流水线 -->
      <div v-show="wizardStep === 0" class="wizard-step" v-loading="loadingPipelines">
        <el-alert type="info" :closable="false" show-icon class="step-alert">
          {{ $t('agentSetup.selectPipelineHint') }}
        </el-alert>

        <el-alert
          v-if="hasEnabledAPIKey === false"
          type="warning"
          :closable="false"
          show-icon
          class="step-alert"
        >
          <template #title>{{ $t('agentSetup.noApiKey') }}</template>
          {{ $t('agentSetup.apiKeyAutoWrite') }}
          {{ $t('agentSetup.apiKeyCreateFirst') }}
          <div class="alert-actions">
            <el-button type="warning" size="small" @click="goProfileForAPIKey">{{ $t('agentSetup.goToCreateApiKey') }}</el-button>
          </div>
        </el-alert>
        <el-alert
          v-else-if="hasEnabledAPIKey === true"
          type="success"
          :closable="false"
          show-icon
          class="step-alert"
        >
          {{ $t('agentSetup.apiKeyDetected') }}
        </el-alert>

        <div v-if="pipelines.length === 0 && !loadingPipelines" class="empty-pipelines">
          <el-empty :description="$t('agentSetup.noPipeline')">
            <el-button type="primary" @click="goPipelines">{{ $t('agentSetup.goToPolicy') }}</el-button>
          </el-empty>
        </div>
        <div v-else>
          <h4 class="select-label">{{ $t('agentSetup.selectPipelineLabel') }}</h4>
          <el-select
            v-model="selectedPipeline"
            :placeholder="$t('agentSetup.selectPipelinePlaceholder')"
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
            <el-tag type="success" size="default">{{ $t('agentSetup.pipeline') }} {{ selectedPipelineName }}</el-tag>
            <el-tag type="info" size="default">
              <el-icon><Cpu /></el-icon>
              {{ $t('agentSetup.modelRoute') }} centag/{{ selectedPipeline }}
            </el-tag>
            <span v-if="pipelineModel" class="model-hint">{{ $t('agentSetup.pipelineModelHint') }} {{ pipelineModel }}</span>
          </div>
        </div>
      </div>

      <!-- Step 2: 写入配置 -->
      <div v-show="wizardStep === 1" class="wizard-step" v-loading="loadingConfig">
        <el-alert type="info" :closable="false" show-icon class="step-alert">
          {{ $t('agentSetup.writeConfigHint', { agent: currentAgentName }) }}
          {{ $t('agentSetup.writeConfigKeySource') }}
        </el-alert>

        <div v-if="selectedAgentMeta" class="wizard-meta">
          <div class="meta-row">
            <span class="meta-label">{{ $t('agentSetup.writeMode') }}</span>
            <el-tag size="small" type="info">{{ writeModeLabel(selectedAgentMeta.write_mode) }}</el-tag>
          </div>
          <div v-if="selectedAgentMeta.config_paths?.length" class="meta-row">
            <span class="meta-label">{{ $t('agentSetup.configPaths') }}</span>
            <code v-for="p in selectedAgentMeta.config_paths" :key="p" class="meta-path">{{ p }}</code>
          </div>
          <p class="meta-method">{{ agentLocalized(selectedAgentMeta, 'configMethod') }}</p>
          <a
            v-if="selectedAgentMeta.install_url"
            class="install-link"
            :href="selectedAgentMeta.install_url"
            target="_blank"
            rel="noopener noreferrer"
          >
            {{ $t('agentSetup.installGuide') }}
          </a>
          <div v-if="agentLocalized(selectedAgentMeta, 'installHint')" class="install-hint">
            {{ agentLocalized(selectedAgentMeta, 'installHint') }}
          </div>
        </div>

        <div v-if="configResult" class="config-result">
          <div class="config-header">
            <h3>{{ configResult.description }}</h3>
            <el-tag>{{ $t('agentSetup.route') }} {{ configResult.backend_name }}</el-tag>
          </div>

          <!-- 一键写入 -->
          <div class="config-section">
            <h4>{{ $t('agentSetup.oneClickConfig') }}</h4>
            <el-button type="primary" :loading="writingConfig" @click="writeToConfig">
              <el-icon class="el-icon--left"><Plus /></el-icon>
              {{ $t('agentSetup.writeConfigFile') }}
            </el-button>
            <p class="write-hint" v-if="writeResult">
              <span v-if="writeResult.success" style="color: #67c23a">✓ {{ writeResult.message }}</span>
              <span v-else style="color: #f56c6c">✗ {{ writeResult.message }}</span>
            </p>
          </div>

          <!-- 写入成功：只展示已写入预览，避免与「配置预览」重复 -->
          <div v-if="writeSucceeded && writePreviewFiles.length" class="config-section">
            <h4>{{ $t('agentSetup.configWritten') }}</h4>
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
              <h4>{{ $t('agentSetup.configPreview') }}</h4>
              <p class="section-subhint">{{ $t('agentSetup.configPreviewHint') }}</p>
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
              <h4>{{ $t('agentSetup.configCommand') }}</h4>
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
        <el-empty v-else-if="!loadingConfig" :description="$t('agentSetup.configGenFailed')" />
      </div>

      <!-- Step 3: 验证生效 -->
      <div v-show="wizardStep === 2" class="wizard-step">
        <el-result
          icon="success"
          :title="$t('agentSetup.configReady')"
          :sub-title="$t('agentSetup.verifyHint', { agent: currentAgentName })"
        />

        <el-alert type="success" :closable="false" show-icon class="step-alert">
          {{ $t('agentSetup.verifySuccessHint') }}
        </el-alert>

        <div v-if="configResult?.verify_cmd" class="config-section">
          <h4>{{ $t('agentSetup.verifyCommand') }}</h4>
          <p class="verify-desc">{{ $t('agentSetup.verifyCommandHint') }}</p>
          <div class="code-block">
            <pre><code>{{ configResult.verify_cmd }}</code></pre>
            <el-button class="copy-btn" :icon="DocumentCopy" @click="copyText(configResult.verify_cmd)" />
          </div>
        </div>

        <div class="config-section">
          <h4>{{ $t('agentSetup.verifyChecklist') }}</h4>
          <ol class="verify-checklist">
            <li>{{ $t('agentSetup.verifyChecklistStep1', { agent: currentAgentName }) }}</li>
            <li>{{ $t('agentSetup.verifyChecklistStep2') }}</li>
            <li>{{ $t('agentSetup.verifyChecklistStep3', { pipeline: selectedPipeline }) }}</li>
            <li>{{ $t('agentSetup.verifyChecklistStep4') }}</li>
          </ol>
        </div>
      </div>

      <template #footer>
        <div class="wizard-footer">
          <el-button @click="wizardVisible = false">{{ $t('agentSetup.cancel') }}</el-button>
          <div class="wizard-footer-right">
            <el-button :disabled="wizardStep === 0 || loadingConfig || writingConfig" @click="prevStep">
              {{ $t('agentSetup.prevStep') }}
            </el-button>
            <el-button
              v-if="wizardStep < 2"
              type="primary"
              :loading="loadingConfig"
              :disabled="!canGoNext"
              @click="nextStep"
            >
              {{ $t('agentSetup.nextStep') }}
            </el-button>
            <el-button v-else type="primary" @click="wizardVisible = false">
              {{ $t('agentSetup.finish') }}
            </el-button>
          </div>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
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

const { t, te } = useI18n()
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
interface AgentTypeInfo {
  type: string
  display_name: string
  description: string
  category?: string
  write_mode?: string
  config_paths?: string[]
  key_fields?: string[]
  config_method?: string
  install_url?: string
  install_hint?: string
  verified?: boolean
  wrap_command?: string
}

const agentTypes = ref<AgentTypeInfo[]>([])
const pipelines = ref<Array<{ id: string; name: string; description?: string; nodes?: any[] }>>([])
const configResult = ref<any>(null)
const writeResult = ref<{ success: boolean; message: string; written?: Array<{ path: string; content: string }> } | null>(null)
/** null=未检测；true/false=是否有启用中的 API Key（列表级提示，最终以服务端解密结果为准） */
const hasEnabledAPIKey = ref<boolean | null>(null)

interface AgentGroup {
  id: string
  title: string
  hint: string
  agents: AgentTypeInfo[]
}

/** 仅按形态分组；组内已验证优先 */
const agentGroups = computed<AgentGroup[]>(() => {
  const sortInGroup = (agents: AgentTypeInfo[]) =>
    [...agents].sort((a, b) => {
      if (!!a.verified !== !!b.verified) return a.verified ? -1 : 1
      return a.type.localeCompare(b.type)
    })

  const sections: Array<{ id: string; cat: string; titleKey: string; hintKey: string }> = [
    { id: 'cli', cat: 'cli', titleKey: 'agentSetup.groupCli', hintKey: 'agentSetup.groupCliHint' },
    { id: 'desktop', cat: 'desktop', titleKey: 'agentSetup.groupDesktop', hintKey: 'agentSetup.groupDesktopHint' },
    { id: 'tui', cat: 'tui', titleKey: 'agentSetup.groupTui', hintKey: 'agentSetup.groupTuiHint' },
    { id: 'web', cat: 'web', titleKey: 'agentSetup.groupWeb', hintKey: 'agentSetup.groupWebHint' },
  ]

  const groups: AgentGroup[] = []
  const known = new Set(sections.map(s => s.cat))
  for (const s of sections) {
    const agents = sortInGroup(
      agentTypes.value.filter(a => (a.category || 'cli') === s.cat)
    )
    if (!agents.length) continue
    groups.push({
      id: s.id,
      title: t(s.titleKey),
      hint: t(s.hintKey),
      agents,
    })
  }
  const other = sortInGroup(
    agentTypes.value.filter(a => !known.has(a.category || 'cli'))
  )
  if (other.length) {
    groups.push({
      id: 'other',
      title: t('agentSetup.groupOther'),
      hint: t('agentSetup.groupOtherHint'),
      agents: other,
    })
  }
  return groups
})

const isDesktopEdition = computed(() => isPersonalEdition())

const wizardTitle = computed(() => {
  if (!selectedAgentDisplay.value) return t('agentSetup.connectCentagProxy')
  return `${t('agentSetup.connectCentagProxy')} — ${selectedAgentDisplay.value}`
})

const currentAgentName = computed(() => selectedAgentDisplay.value || selectedAgent.value || 'Agent')

const selectedAgentMeta = computed(() =>
  agentTypes.value.find(a => a.type === selectedAgent.value) || null
)

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
    ElMessage.error(t('agentSetup.loadAgentTypeFailed') + e.message)
  }
}

function pickDefaultPipelineID(list: Array<{ id: string; name: string }>): string {
  const byID = list.find(p => p.id === DEFAULT_PIPELINE_ID)
  if (byID) return byID.id
  const byName = list.find(p => p.name === 'transparent' || /transparent/i.test(p.id))
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
    console.warn(t('agentSetup.loadPipelineFailed'), e.message)
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
    console.warn(t('agentSetup.detectApiKeyFailed'), e.message)
  }
}

function goProfileForAPIKey() {
  wizardVisible.value = false
  router.push({ path: '/profile', query: { section: 'api-keys' } })
}

async function restoreDefaults(agent: { type: string; display_name: string }) {
  try {
    await ElMessageBox.confirm(
      t('agentSetup.restoreConfirm', { name: agent.display_name }),
      t('agentSetup.restoreDefaultConfig'),
      {
        confirmButtonText: t('agentSetup.restoreDefault'),
        cancelButtonText: t('agentSetup.cancel'),
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
      ElMessage.success(res.message || t('agentSetup.restoreSuccess'))
    } else {
      ElMessage.error(res?.message || t('agentSetup.restoreFailed'))
    }
  } catch (e: any) {
    ElMessage.error(t('agentSetup.restoreFailed') + e.message)
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
  return message.includes(t('agentSetup.noApiKeyForAgent'))
}

async function maybeHandleMissingProxyAPIKeyError(message: string): Promise<boolean> {
  if (!isMissingProxyAPIKeyError(message)) return false
  try {
    await ElMessageBox.confirm(
      t('agentSetup.noApiKeyHint'),
      t('agentSetup.missingApiKey'),
      {
        confirmButtonText: t('agentSetup.goToProfile'),
        cancelButtonText: t('agentSetup.later'),
        type: 'warning',
      }
    )
    wizardVisible.value = false
    router.push({ path: '/profile', query: { section: 'api-keys' } })
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
    ElMessage.error(t('agentSetup.genConfigFailed') + e.message)
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
      ElMessage.success(t('agentSetup.configWriteSuccess'))
    } else {
      ElMessage.error(t('agentSetup.configWriteFailed') + res.message)
    }
  } catch (e: any) {
    writeResult.value = { success: false, message: e.message }
    if (await maybeHandleMissingProxyAPIKeyError(e.message)) return
    ElMessage.error(t('agentSetup.writeRequestFailed') + e.message)
  } finally {
    writingConfig.value = false
  }
}

function copyText(text: string) {
  navigator.clipboard.writeText(text).then(() => {
    ElMessage.success(t('agentSetup.copiedToClipboard'))
  }).catch(() => {
    ElMessage.error(t('agentSetup.copyFailed'))
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
    'pi': Connection,
    'hermes': Connection,
    'codebuddy': Monitor,
    'workbuddy': Monitor,
    'trae': ChatDotRound,
  }
  return map[type] || Connection
}

function writeModeLabel(mode?: string): string {
  switch (mode) {
    case 'merge':
      return t('agentSetup.writeModeMerge')
    case 'none':
      return t('agentSetup.writeModeNone')
    case 'overwrite':
      return t('agentSetup.writeModeOverwrite')
    default:
      return mode || '-'
  }
}

/** 卡片/向导文案走前端 i18n；路径与 install_url 仍用后端。 */
function agentLocalized(
  agent: { type?: string; description?: string; config_method?: string; install_hint?: string; config_paths?: string[] } | null,
  field: 'description' | 'configMethod' | 'installHint'
): string {
  if (!agent?.type) return ''
  if (field === 'configMethod' && agent.type === 'claude-desktop' && (!agent.config_paths || agent.config_paths.length === 0)) {
    const unsupportedKey = 'agentSetup.agents.claude-desktop.configMethodUnsupported'
    if (te(unsupportedKey) || te(unsupportedKey, 'en')) return t(unsupportedKey)
  }
  const key = `agentSetup.agents.${agent.type}.${field}`
  // 当前语言缺失时回退 en，避免显示后端硬编码中文。
  // vue-i18n 会把未转义的 @ / | 当成链接/复数语法，解析失败时回退后端文案。
  try {
    if (te(key) || te(key, 'en')) return t(key)
  } catch {
    /* fall through */
  }
  if (field === 'description') return agent.description || ''
  if (field === 'configMethod') return agent.config_method || ''
  return agent.install_hint || ''
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
    ElMessage.error(t('agentSetup.loadProviderFailed') + e.message)
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
    ElMessage.success(t('agentSetup.setDefaultSuccess', { name: provider.display_name || provider.agent_type }))
    loadProviders()
  } catch (e: any) {
    ElMessage.error(t('agentSetup.hotSwapFailed') + e.message)
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
  min-height: 100%;
  width: 100%;
  padding: 0 0 24px;
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

.agent-group {
  margin-bottom: 32px;
  padding-top: 4px;
}

.agent-group + .agent-group {
  border-top: 1px solid #ebeef5;
  padding-top: 24px;
}

.agent-group-header {
  margin-bottom: 14px;
}

.agent-group-title {
  margin: 0;
  font-size: 1.05rem;
  font-weight: 600;
  color: #303133;
}

.agent-group-hint {
  margin: 4px 0 0;
  font-size: 0.8rem;
  color: #909399;
  line-height: 1.45;
  max-width: 720px;
}

/* Agent cards — 纵向间距放在列上，避免 height:100% 把卡片 margin 顶没 */
.agent-card-col {
  margin-bottom: 16px;
}

.agent-card {
  height: 100%;
  border: 1px solid #e4e7ed;
  border-radius: 10px;
  padding: 14px 16px 6px;
  cursor: default;
  transition: border-color 0.2s, box-shadow 0.2s, background 0.2s;
  display: flex;
  flex-direction: column;
  gap: 10px;
  background: #fff;
  box-sizing: border-box;
}

.agent-card:hover {
  border-color: var(--el-color-primary-light-5);
  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.04);
}

.agent-card--verified {
  border-color: var(--el-color-success-light-5);
  background: linear-gradient(180deg, var(--el-color-success-light-9) 0%, #fff 48px);
}

.agent-card--verified:hover {
  border-color: var(--el-color-success);
}

.agent-card-head {
  display: flex;
  align-items: flex-start;
  gap: 10px;
}

.agent-icon {
  width: 36px;
  height: 36px;
  border-radius: 8px;
  background: #f5f7fa;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #606266;
  flex-shrink: 0;
}

.agent-card:hover .agent-icon {
  background: var(--el-color-primary-light-9);
  color: var(--el-color-primary);
}

.agent-card--verified .agent-icon {
  background: var(--el-color-success-light-9);
  color: var(--el-color-success);
}

.agent-head-text {
  flex: 1;
  min-width: 0;
}

.agent-name {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
  margin-bottom: 2px;
}

.agent-name-text {
  font-weight: 600;
  font-size: 0.95rem;
  color: #303133;
  line-height: 1.3;
}

.verified-tag {
  font-weight: 500;
  height: 20px;
  padding: 0 6px;
}

.agent-desc {
  color: #909399;
  font-size: 0.78rem;
  line-height: 1.4;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.access-methods {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.access-method {
  padding: 8px 10px;
  border-radius: 8px;
  background: #f8f9fb;
  border: 1px solid #eef0f4;
}

.access-method-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 10px;
}

.access-method-titles {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  min-width: 0;
}

.access-method-index {
  flex-shrink: 0;
  width: 18px;
  height: 18px;
  margin-top: 1px;
  border-radius: 50%;
  background: #e4e7ed;
  color: #606266;
  font-size: 0.68rem;
  font-weight: 600;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  line-height: 1;
}

.access-method-title {
  font-size: 0.82rem;
  font-weight: 600;
  color: #303133;
  line-height: 1.3;
}

.access-method-hint {
  margin: 2px 0 0;
  font-size: 0.72rem;
  color: #909399;
  line-height: 1.4;
}

.wrap-cmd-row {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 8px;
  padding: 6px 8px;
  border-radius: 6px;
  background: #fff;
  border: 1px dashed #dcdfe6;
}

.wrap-cmd {
  flex: 1;
  min-width: 0;
  font-size: 0.72rem;
  line-height: 1.35;
  color: #303133;
  word-break: break-all;
  user-select: all;
}

.wrap-copy-btn {
  flex-shrink: 0;
  position: static;
  padding: 0 2px;
  height: auto;
  min-height: 0;
}

.install-link {
  display: inline-block;
  font-size: 0.78rem;
  color: var(--el-color-primary);
  text-decoration: none;
  word-break: break-all;
}

.install-link:hover {
  text-decoration: underline;
}

.install-hint {
  margin-top: 2px;
  font-size: 0.75rem;
  color: #a8abb2;
  line-height: 1.35;
  word-break: break-all;
}

.agent-meta-collapse {
  border: none;
  margin-top: auto;
  --el-collapse-header-height: 32px;
}

.agent-meta-collapse :deep(.el-collapse-item__header) {
  font-size: 0.78rem;
  color: #909399;
  border: none;
  background: transparent;
  height: 32px;
  line-height: 32px;
}

.agent-meta-collapse :deep(.el-collapse-item__wrap) {
  border: none;
  background: transparent;
}

.agent-meta-collapse :deep(.el-collapse-item__content) {
  padding-bottom: 8px;
}

.meta-block,
.wizard-meta {
  font-size: 0.8rem;
  color: #606266;
  line-height: 1.5;
}

.wizard-meta {
  margin-bottom: 16px;
  padding: 12px 14px;
  background: #f5f7fa;
  border-radius: 8px;
}

.meta-row {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 6px 8px;
  margin-bottom: 6px;
}

.meta-row-col {
  flex-direction: column;
  align-items: flex-start;
  gap: 2px;
}

.meta-actions {
  margin-top: 8px;
}

.meta-label {
  font-weight: 600;
  color: #303133;
  flex-shrink: 0;
}

.meta-path {
  font-size: 0.75rem;
  background: #eef1f6;
  padding: 1px 6px;
  border-radius: 4px;
  word-break: break-all;
}

.meta-fields {
  word-break: break-all;
  color: #909399;
}

.meta-method {
  margin: 4px 0 0;
  white-space: pre-wrap;
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
