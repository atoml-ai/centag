<template>
  <div class="pipeline-canvas">
    <!-- 工具栏 -->
    <div class="canvas-toolbar">
      <el-button-group>
        <el-button size="small" @click="fitView">
          <el-icon><FullScreen /></el-icon> {{ t('pipelineCanvas.fitCanvas') }}
        </el-button>
        <el-button size="small" @click="autoLayout">
          <el-icon><Rank /></el-icon> {{ t('pipelineCanvas.autoLayout') }}
        </el-button>
        <el-button size="small" type="success" @click="addNewNode">
          <el-icon><Plus /></el-icon> {{ t('pipelineCanvas.addNode') }}
        </el-button>
        <el-button size="small" @click="exportPipeline">
          <el-icon><Download /></el-icon> {{ t('pipelineCanvas.export') }}
        </el-button>
        <el-button size="small" @click="triggerImport">
          <el-icon><Upload /></el-icon> {{ t('pipelineCanvas.import') }}
        </el-button>
        <el-button size="small" type="primary" @click="savePipeline">
          <el-icon><DocumentAdd /></el-icon> {{ t('pipelineCanvas.savePipeline') }}
        </el-button>
        <el-button size="small" @click="showGlobalConfig = true">
          <el-icon><Setting /></el-icon> {{ t('pipelineCanvas.globalConfig') }}
        </el-button>
          <el-button size="small" type="warning" @click="showTestPanel = true">
          <el-icon><VideoPlay /></el-icon> {{ t('pipelineCanvas.test') }}
        </el-button>
      </el-button-group>
      <input
        ref="importInputRef"
        type="file"
        accept=".yaml,.yml,text/yaml"
        style="display: none"
        @change="handleImportFile"
      />

      <div class="legend">
        <span class="node-type generator">{{ t('pipelineCanvas.generator') }}</span>
        <span class="node-type processor">{{ t('pipelineCanvas.processor') }}</span>
        <span class="node-type reviewer">{{ t('pipelineCanvas.moderator') }}</span>
        <span class="node-type router">{{ t('pipelineCanvas.router') }}</span>
        <span class="node-type aggregator">{{ t('pipelineCanvas.aggregator') }}</span>
        <span class="node-type parallel">{{ t('pipelineCanvas.parallel') }}</span>
        <span class="node-type cache">{{ t('pipelineCanvas.cache') }}</span>
        <span class="node-type token_usage">{{ t('pipelineCanvas.meter') }}</span>
        <span class="node-type tool_call_injector">{{ t('pipelineCanvas.inject') }}</span>
      </div>
    </div>

    <div class="canvas-body">
      <!-- 画布容器 -->
      <div class="flow-wrapper">
      <VueFlow
        v-model:nodes="nodes"
        v-model:edges="edges"
        :fit-view-on-init="false"
        :default-viewport="{ zoom: 0.85, x: 0, y: 0 }"
        :min-zoom="0.25"
        :max-zoom="2"
        class="vue-flow-container"
        @node-click="onNodeClick"
        @nodes-change="onNodesChange"
        @connect="onConnect"
        :nodes-connectable="true"
        :edges-updatable="true"
      >
        <template #node-pipeline-node="nodeProps">
          <PipelineNode
            v-bind="nodeProps"
            @delete="deleteNode(nodeProps.id)"
          />
        </template>

        <!-- MiniMap：仅在节点数据就绪后渲染，避免空节点导致 Vue Flow 内部崩溃 -->
        <MiniMap
          v-if="nodes.length > 0"
          :node-color="getNodeColor"
          :node-stroke-color="'#555'"
          pannable
          zoomable
        />

        <!-- 控制栏 -->
        <Controls
          :show-interactive="true"
          position="top-left"
        />
      </VueFlow>
    </div>

      <!-- 测试抽屉 -->
      <el-drawer
        v-model="showTestPanel"
        :title="t('pipelineCanvas.pipelineTest')"
        size="1200px"
        direction="rtl"
      >
        <div class="test-drawer-body">
          <el-input
            v-model="testContent"
            type="textarea"
            :rows="4"
            :placeholder="t('pipelineCanvas.testPlaceholder')"
            :disabled="testing"
            @keydown.enter.prevent="testing || runTest()"
          />
          <div class="test-actions">
            <el-button type="primary" :loading="testing" @click="runTest">
              <el-icon><VideoPlay /></el-icon> {{ t('pipelineCanvas.execute') }}
            </el-button>
          </div>

          <div v-if="testResult" class="test-result">
            <el-divider style="margin: 16px 0 12px;">{{ t('pipelineCanvas.executionResult') }}</el-divider>
            <el-alert
              v-if="testResult.success"
              type="success"
              :title="t('pipelineCanvas.executionSuccess')"
              show-icon
              :closable="false"
            />
            <el-alert
              v-else-if="testResult.error"
              type="error"
              :title="testResult.error"
              show-icon
              :closable="false"
            />
            <el-alert
              v-else
              type="warning"
              :title="t('pipelineCanvas.executionPartialFail')"
              show-icon
              :closable="false"
            />

            <div v-if="testResult.content" class="result-content">
              <div class="result-label">{{ t('pipelineCanvas.outputContent') }}</div>
              <div class="result-text">{{ testResult.content }}</div>
            </div>

            <div v-if="testResult.execution_log" class="result-log-summary">
              <span>{{ t('pipelineCanvas.elapsed') }}：{{ testResult.execution_log.duration_ms }} ms</span>
              <span>{{ t('pipelineCanvas.tokenCount') }}：{{ formatTokens(testResult.execution_log.total_tokens) }}</span>
              <span>{{ t('pipelineCanvas.nodeCount') }}：{{ testResult.execution_log.node_logs?.length || 0 }}</span>
            </div>

            <el-collapse style="margin-top: 12px">
              <el-collapse-item :title="t('pipelineCanvas.viewFullJson')">
                <pre class="result-json">{{ JSON.stringify(testResult, null, 2) }}</pre>
              </el-collapse-item>
            </el-collapse>
          </div>
        </div>
      </el-drawer>
    </div>

    <!-- 右侧配置抽屉 -->
    <NodeConfigDrawer
      v-model:visible="drawerVisible"
      :node="selectedNode"
      :all-nodes="nodes"
      :backends="backends"
      :storages="storages"
      :plugins="nodePlugins"
      :pipeline-id="props.pipeline?.id"
      @update:node="updateNode"
      @close="drawerVisible = false"
    />

    <!-- 全局配置抽屉 -->
    <el-drawer
      v-model="showGlobalConfig"
      :title="t('pipelineCanvas.pipelineGlobalConfig')"
      size="600px"
      direction="rtl"
    >
      <div style="padding: 20px">
        <el-form label-position="top">
          <el-divider>{{ t('pipelineCanvas.pipelineInfo') }}</el-divider>
          <el-form-item :label="t('pipelineCanvas.id')">
            <el-input :model-value="pipelineInfo.id" :readonly="!isCreate" :placeholder="t('pipelineCanvas.pipelineIdPlaceholder')" />
            <div v-if="!isCreate" style="font-size: 12px; color: #666; margin-top: 4px">
              {{ t('pipelineCanvas.cannotModifyAfterCreate') }}
            </div>
          </el-form-item>
          <el-form-item :label="t('pipelineCanvas.name')">
            <el-input v-model="pipelineInfo.name" :placeholder="t('pipelineCanvas.pipelineNamePlaceholder')" />
          </el-form-item>
          <el-form-item :label="t('pipelineCanvas.description')">
            <el-input v-model="pipelineInfo.description" type="textarea" :rows="2" :placeholder="t('pipelineCanvas.descriptionOptional')" />
          </el-form-item>
          <el-form-item :label="t('pipelineCanvas.version')">
            <el-input v-model="pipelineInfo.version" placeholder="1.0" />
          </el-form-item>
          <el-form-item :label="t('pipelineCanvas.shortcutCode')">
            <el-input v-model="pipelineInfo.shortcut_code" :placeholder="t('pipelineCanvas.shortcutPlaceholder')" />
          </el-form-item>

          <el-divider>{{ t('pipelineCanvas.runConfig') }}</el-divider>
          <el-form-item :label="t('pipelineCanvas.timeoutSeconds')">
            <el-input-number 
              v-model="globalConfig.timeout" 
              :min="10" :max="600" 
              style="width: 100%"
            />
          </el-form-item>
          <el-form-item :label="t('pipelineCanvas.maxRetries')">
            <el-input-number 
              v-model="globalConfig.max_retries" 
              :min="0" :max="10" 
              style="width: 100%"
            />
          </el-form-item>
          <el-form-item :label="t('pipelineCanvas.parallelLimit')">
            <el-input-number 
              v-model="globalConfig.parallel_limit" 
              :min="1" :max="20" 
              style="width: 100%"
            />
            <div style="font-size: 12px; color: #666; margin-top: 4px">
              {{ t('pipelineCanvas.parallelLimitDesc') }}
            </div>
          </el-form-item>
          <el-form-item :label="t('pipelineCanvas.streamMode')">
            <el-switch v-model="globalConfig.stream_mode" />
          </el-form-item>
          <el-form-item :label="t('pipelineCanvas.bypassOnError')">
            <el-switch v-model="globalConfig.bypass_on_error" />
            <div style="font-size: 12px; color: #666; margin-top: 4px">
              {{ t('pipelineCanvas.bypassOnErrorDesc') }}
            </div>
          </el-form-item>

          <el-divider>{{ t('pipelineCanvas.fallbackGroups') }}</el-divider>
          <!-- 全局降级策略 -->
          <el-form-item :label="t('pipelineCanvas.defaultFallbackPolicy')">
            <el-select
              v-model="globalConfig.fallback_policy_id"
              clearable
              :placeholder="t('pipelineCanvas.autoFallbackPolicy')"
              style="width: 100%"
            >
              <el-option :label="t('pipelineCanvas.autoFallbackPolicy')" value="" />
              <el-option
                v-for="policy in fallbackPolicies"
                :key="policy.id"
                :label="`${policy.name} (${policy.id})`"
                :value="policy.id"
              />
            </el-select>
            <div style="font-size: 12px; color: #666; margin-top: 4px">
              {{ t('pipelineCanvas.fallbackPolicyDesc') }}
            </div>
          </el-form-item>

          <el-alert type="info" :closable="false" style="margin-bottom: 16px">
            <template #default>
              <div style="font-size: 13px; line-height: 1.5">
                <strong>{{ t('pipelineCanvas.fallbackGroupLegacyNote') }}</strong><br>
                {{ t('pipelineCanvas.fallbackGroupLegacyDesc1') }}<br>
                {{ t('pipelineCanvas.fallbackGroupLegacyDesc2') }}
              </div>
            </template>
          </el-alert>

          <div v-for="(fg, idx) in globalConfig.fallback_groups" :key="idx" class="fallback-group-item">
            <el-card shadow="hover" style="margin-bottom: 16px">
              <template #header>
                <div style="display: flex; justify-content: space-between; align-items: center">
                  <span>{{ t('pipelineCanvas.fallbackGroup') }} {{ idx + 1 }}</span>
                  <el-button type="danger" text @click="removeFallbackGroup(idx)">
                      <el-icon><Delete /></el-icon> {{ t('pipelineCanvas.delete') }}
                  </el-button>
                </div>
              </template>
              
              <el-form-item :label="t('pipelineCanvas.primaryNode')">
                <el-select v-model="fg.primary_node_id" style="width: 100%">
                  <el-option 
                    v-for="n in nodes" 
                    :key="n.id" 
                    :label="n.data?.name || n.id" 
                    :value="n.id"
                  />
                </el-select>
              </el-form-item>

              <el-form-item :label="t('pipelineCanvas.fallbackNodes')">
                <el-select v-model="fg.fallback_nodes" multiple style="width: 100%">
                  <el-option 
                    v-for="n in nodes" 
                    :key="n.id" 
                    :label="n.data?.name || n.id" 
                    :value="n.id"
                  />
                </el-select>
              </el-form-item>

              <el-form-item :label="t('pipelineCanvas.maxAttempts')">
                <el-input-number 
                  v-model="fg.max_attempts" 
                  :min="1" :max="10" 
                  style="width: 100%"
                />
                <div style="font-size: 12px; color: #666; margin-top: 4px">
                  {{ t('pipelineCanvas.maxAttemptsDesc') }}
                </div>
              </el-form-item>
            </el-card>
          </div>

          <el-button type="primary" @click="addFallbackGroup" style="margin-bottom: 20px">
            <el-icon><Plus /></el-icon> {{ t('pipelineCanvas.addFallbackGroup') }}
          </el-button>

          <el-divider>{{ t('pipelineCanvas.storageHook') }}</el-divider>
          <el-alert type="info" :closable="false" style="margin-bottom: 16px">
            <template #default>
              <div style="font-size: 13px; line-height: 1.5">
                {{ t('pipelineCanvas.storageHookDesc') }}
              </div>
            </template>
          </el-alert>

          <el-form-item :label="t('pipelineCanvas.enableStorageHook')">
            <el-switch v-model="globalConfig.storage_config.enabled" />
          </el-form-item>

          <template v-if="globalConfig.storage_config.enabled">
            <el-form-item :label="t('pipelineCanvas.namespace')">
              <el-input v-model="globalConfig.storage_config.namespace" :placeholder="t('pipelineCanvas.namespacePlaceholder')" />
              <div style="font-size: 12px; color: #666; margin-top: 4px">
                {{ t('pipelineCanvas.namespaceDesc') }}
              </div>
            </el-form-item>

            <el-form-item :label="t('pipelineCanvas.autoSave')">
              <el-switch v-model="globalConfig.storage_config.auto_save" />
              <div style="font-size: 12px; color: #666; margin-top: 4px">
                {{ t('pipelineCanvas.autoSaveDesc') }}
              </div>
            </el-form-item>

            <el-form-item :label="t('pipelineCanvas.saveInterval')">
              <el-input-number
                v-model="globalConfig.storage_config.save_interval"
                :min="10" :max="600"
                style="width: 100%"
              />
            </el-form-item>

            <el-form-item :label="t('pipelineCanvas.retentionDays')">
              <el-input-number
                v-model="globalConfig.storage_config.retention_days"
                :min="1" :max="365"
                style="width: 100%"
              />
              <div style="font-size: 12px; color: #666; margin-top: 4px">
                {{ t('pipelineCanvas.retentionDaysDesc') }}
              </div>
            </el-form-item>

            <el-divider>{{ t('pipelineCanvas.hookBehavior') }}</el-divider>
            <el-alert type="warning" :closable="false" style="margin-bottom: 16px">
              <template #default>
                <div style="font-size: 13px; line-height: 1.5">
                  {{ t('pipelineCanvas.hookBehaviorDesc') }}
                </div>
              </template>
            </el-alert>

            <div v-for="(hook, hIdx) in globalConfig.hooks" :key="hIdx" class="hook-item">
              <el-card shadow="hover" style="margin-bottom: 16px">
                <template #header>
                  <div style="display: flex; justify-content: space-between; align-items: center">
                    <span>{{ t('pipelineCanvas.hook') }} {{ hIdx + 1 }}</span>
                    <el-button type="danger" text @click="removeHook(hIdx)">
                    <el-icon><Delete /></el-icon> {{ t('pipelineCanvas.delete') }}
                    </el-button>
                  </div>
                </template>

                <el-form-item :label="t('pipelineCanvas.hookType')">
                  <el-select v-model="hook.type" style="width: 100%">
                    <el-option label="storage" value="storage" />
                  </el-select>
                </el-form-item>

                <el-form-item :label="t('pipelineCanvas.hookTriggerOn')">
                  <el-select v-model="hook.on" multiple style="width: 100%">
                    <el-option :label="t('pipelineCanvas.hookOnNodeStart')" value="node_start" />
                    <el-option :label="t('pipelineCanvas.hookOnNodeComplete')" value="node_complete" />
                    <el-option :label="t('pipelineCanvas.hookOnNodeError')" value="node_error" />
                    <el-option :label="t('pipelineCanvas.hookOnPipelineComplete')" value="pipeline_complete" />
                    <el-option :label="t('pipelineCanvas.hookOnPipelineError')" value="pipeline_error" />
                  </el-select>
                </el-form-item>

                <el-form-item :label="t('pipelineCanvas.targetStorage')">
                  <el-select v-model="hook.storage_name" clearable :placeholder="t('pipelineCanvas.defaultStoragePlaceholder')" style="width: 100%">
                    <el-option
                      v-for="s in storages"
                      :key="s.name"
                      :label="s.name + ' (' + s.type + ')'"
                      :value="s.name"
                    />
                  </el-select>
                  <div style="font-size: 12px; color: #666; margin-top: 4px">
                    {{ t('pipelineCanvas.targetStorageDesc') }}
                  </div>
                </el-form-item>

                <el-form-item :label="t('pipelineCanvas.storageType')">
                  <el-select v-model="hook.storage_type" style="width: 100%">
                    <el-option :label="t('pipelineCanvas.storageTypeKv')" value="kv" />
                    <el-option :label="t('pipelineCanvas.storageTypeKnowledge')" value="knowledge" />
                    <el-option :label="t('pipelineCanvas.storageTypeVector')" value="vector" />
                  </el-select>
                  <div style="font-size: 12px; color: #666; margin-top: 4px">
                    {{ t('pipelineCanvas.storageTypeDesc') }}
                  </div>
                </el-form-item>

                <el-form-item :label="t('pipelineCanvas.storageBehavior')">
                  <el-checkbox-group
                    :model-value="getHookBehaviorFlags(hook)"
                    @update:model-value="(vals: string[]) => setHookBehaviorFlags(hook, vals)"
                    style="display: flex; flex-direction: column; gap: 8px"
                  >
                    <el-checkbox label="save_user_progress">{{ t('pipelineCanvas.behaviorSaveUserProgress') }}</el-checkbox>
                    <el-checkbox label="save_conversation_history">{{ t('pipelineCanvas.behaviorSaveConversation') }}</el-checkbox>
                    <el-checkbox label="save_scene_context">{{ t('pipelineCanvas.behaviorSaveSceneContext') }}</el-checkbox>
                    <el-checkbox label="save_code_snippets">{{ t('pipelineCanvas.behaviorSaveCodeSnippets') }}</el-checkbox>
                    <el-checkbox label="save_solutions">{{ t('pipelineCanvas.behaviorSaveSolutions') }}</el-checkbox>
                    <el-checkbox label="track_file_changes">{{ t('pipelineCanvas.behaviorTrackFileChanges') }}</el-checkbox>
                  </el-checkbox-group>
                </el-form-item>
              </el-card>
            </div>

            <el-button type="primary" @click="addHook" style="margin-bottom: 20px">
              <el-icon><Plus /></el-icon> {{ t('pipelineCanvas.addHook') }}
            </el-button>
          </template>
        </el-form>

        <div style="display: flex; justify-content: flex-end; gap: 12px; margin-top: 20px">
          <el-button @click="showGlobalConfig = false">{{ t('pipelineCanvas.cancel') }}</el-button>
          <el-button type="primary" @click="saveGlobalConfig">{{ t('pipelineCanvas.saveGlobalConfig') }}</el-button>
        </div>
      </div>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import { VueFlow, useVueFlow, Position, type Node, type Edge, type NodeChange, type EdgeChange, type Connection } from '@vue-flow/core'
import { MiniMap } from '@vue-flow/minimap'
import { Controls } from '@vue-flow/controls'
import PipelineNode from './PipelineNode.vue'
import NodeConfigDrawer from './NodeConfigDrawer.vue'
import { ElMessage } from 'element-plus'
import { dump as yamlDump, load as yamlLoad } from 'js-yaml'
import { FullScreen, Rank, DocumentAdd, Plus, Upload, Download, VideoPlay, Setting, Delete } from '@element-plus/icons-vue'
import type { PipelineNodeConfig, PluginDescriptor } from '@/api/pipeline'
import { executePipelineDirect, getNodePlugins, parseNodePluginsResponse } from '@/api/pipeline'
import { formatTokens } from '@/utils/format'

const { t } = useI18n()

const props = defineProps<{
  pipeline: any
  backends: any[]
  storages?: any[]
  isCreate?: boolean
}>()

const emit = defineEmits(['update:pipeline', 'save', 'dirty-change'])

/** 画布节点卡片尺寸（与 PipelineNode.vue 保持一致） */
const NODE_WIDTH = 176
const NODE_HEIGHT = 76

const nodeCardStyle = {
  width: `${NODE_WIDTH}px`,
  height: `${NODE_HEIGHT}px`,
  borderRadius: '10px',
  border: '2px solid',
}

const nodes = ref<Node[]>([])
const edges = ref<Edge[]>([])
const nodePlugins = ref<PluginDescriptor[]>([])
const selectedNode = ref<any>(null)
const drawerVisible = ref(false)
const importInputRef = ref<HTMLInputElement | null>(null)
const isDirty = ref(false)
const showTestPanel = ref(false)
const testContent = ref('')
const testing = ref(false)
const testResult = ref<any>(null)

const { fitView: vueFlowFitView, updateNode: vueFlowUpdateNode } = useVueFlow()

// 全局配置状态
const showGlobalConfig = ref(false)
const pipelineInfo = ref({
  id: '',
  name: '',
  description: '',
  version: '1.0',
  shortcut_code: ''
})
const globalConfig = ref({
  timeout: 120,
  max_retries: 3,
  bypass_on_error: true,
  stream_mode: false,
  parallel_limit: 4,
  log_level: 'info',
  fallback_policy_id: '',
  fallback_groups: [] as Array<{
    primary_node_id: string
    fallback_nodes: string[]
    max_attempts: number
  }>,
  storage_config: {
    enabled: false,
    namespace: '',
    auto_save: true,
    save_interval: 60,
    retention_days: 30
  },
  hooks: [] as Array<{
    type: string
    on: string[]
    storage_name?: string
    config?: Record<string, any>
  }>
})

// 降级策略列表
const fallbackPolicies = ref<any[]>([])
const loadingFallbackPolicies = ref(false)

async function loadFallbackPolicies() {
  loadingFallbackPolicies.value = true
  try {
    const { getFallbackPolicies } = await import('../api/fallback')
    const res = await getFallbackPolicies()
    fallbackPolicies.value = Array.isArray(res) ? res : Array.isArray((res as any)?.data) ? (res as any).data : []
  } catch (err) {
    console.error('Failed to load fallback policies', err)
  } finally {
    loadingFallbackPolicies.value = false
  }
}

// 从 pipeline 加载基础信息与全局配置
const loadPipelineInfo = () => {
  pipelineInfo.value = {
    id: props.pipeline?.id || '',
    name: props.pipeline?.name || '',
    description: props.pipeline?.description || '',
    version: props.pipeline?.version || '1.0',
    shortcut_code: props.pipeline?.shortcut_code || ''
  }
}

const loadGlobalConfig = () => {
  loadPipelineInfo()
  const gc = props.pipeline?.global_config || {}
  globalConfig.value = {
    timeout: gc.timeout ?? 120,
    max_retries: gc.max_retries ?? 3,
    bypass_on_error: gc.bypass_on_error ?? true,
    stream_mode: gc.stream_mode ?? false,
    parallel_limit: gc.parallel_limit ?? 4,
    log_level: gc.log_level ?? 'info',
    fallback_policy_id: gc.fallback_policy_id ?? '',
    fallback_groups: gc.fallback_groups || [],
    storage_config: gc.storage ? {
      enabled: gc.storage.enabled ?? false,
      namespace: gc.storage.namespace ?? '',
      auto_save: gc.storage.auto_save ?? true,
      save_interval: gc.storage.save_interval ?? 60,
      retention_days: gc.storage.retention_days ?? 30,
    } : { enabled: false, namespace: '', auto_save: true, save_interval: 60, retention_days: 30 },
    hooks: gc.hooks || []
  }
}

// 保存全局配置
const saveGlobalConfig = () => {
  const code = (pipelineInfo.value.shortcut_code || '').trim()
  if (code && !code.startsWith('#')) {
    ElMessage.warning(t('pipelineCanvas.shortcutMustStartWithHash'))
    return
  }

  const updatedPipeline = {
    ...props.pipeline,
    id: pipelineInfo.value.id.trim() || props.pipeline?.id,
    name: pipelineInfo.value.name.trim(),
    description: pipelineInfo.value.description.trim(),
    version: pipelineInfo.value.version.trim() || '1.0',
    shortcut_code: code,
    global_config: {
      ...globalConfig.value,
      fallback_groups: globalConfig.value.fallback_groups.filter(
        fg => fg.primary_node_id && fg.fallback_nodes.length > 0
      ),
      storage: globalConfig.value.storage_config.enabled ? {
        ...globalConfig.value.storage_config
      } : undefined,
      hooks: globalConfig.value.hooks.length > 0 ? [...globalConfig.value.hooks] : undefined
    }
  }
  emit('update:pipeline', updatedPipeline)
  showGlobalConfig.value = false
  markDirty()
  ElMessage.success(t('pipelineCanvas.configUpdated'))
}

// 添加降级组
const addFallbackGroup = () => {
  globalConfig.value.fallback_groups.push({
    primary_node_id: '',
    fallback_nodes: [],
    max_attempts: 2
  })
}

// 添加钩子
const addHook = () => {
  globalConfig.value.hooks.push({
    type: 'storage',
    on: ['node_complete', 'pipeline_complete'],
    storage_name: '',
    storage_type: 'kv',
    config: {
      save_user_progress: true,
      save_conversation_history: true,
      save_scene_context: false,
      save_code_snippets: false,
      save_solutions: false,
      track_file_changes: false
    }
  })
}

// 删除钩子
const removeHook = (idx: number) => {
  globalConfig.value.hooks.splice(idx, 1)
}

// 读取 hook.config 中已启用的行为标志（返回选中的 label 数组）
const getHookBehaviorFlags = (hook: Record<string, any>): string[] => {
  if (!hook.config) hook.config = {}
  const flags: string[] = []
  const keys = ['save_user_progress', 'save_conversation_history', 'save_scene_context', 'save_code_snippets', 'save_solutions', 'track_file_changes']
  for (const k of keys) {
    if (hook.config[k] === true) flags.push(k)
  }
  return flags
}

// 根据选中的 label 数组更新 hook.config
const setHookBehaviorFlags = (hook: Record<string, any>, vals: string[]) => {
  if (!hook.config) hook.config = {}
  const keys = ['save_user_progress', 'save_conversation_history', 'save_scene_context', 'save_code_snippets', 'save_solutions', 'track_file_changes']
  for (const k of keys) {
    hook.config[k] = vals.includes(k)
  }
}

// 删除降级组
const removeFallbackGroup = (idx: number) => {
  globalConfig.value.fallback_groups.splice(idx, 1)
}

async function loadNodePlugins() {
  try {
    const res = await getNodePlugins()
    nodePlugins.value = parseNodePluginsResponse(res)
  } catch (err) {
    console.error('Failed to preload node plugins:', err)
    nodePlugins.value = []
  }
}

// 初始化时加载全局配置与插件列表
onMounted(() => {
  loadGlobalConfig()
  loadNodePlugins()
  loadFallbackPolicies()
})

// 流水线切换或基础信息变更时重新加载（不仅 id，避免创建时名称/快捷码未同步）
watch(
  () => [
    props.pipeline?.id,
    props.pipeline?.name,
    props.pipeline?.description,
    props.pipeline?.version,
    props.pipeline?.shortcut_code
  ],
  () => {
    if (props.pipeline) {
      loadGlobalConfig()
    }
  }
)

// 颜色映射（MiniMap 的 node-color prop，需防御 null node）
const getNodeColor = (node: any) => {
  if (!node) return '#6b7280'
  const type = node?.data?.type || node?.type || 'generator'
  switch (type) {
    case 'generator': return '#3b82f6'
    case 'processor': return '#10b981'
    case 'reviewer': return '#f59e0b'
    case 'router': return '#ef4444'
    case 'aggregator': return '#8b5cf6'
    case 'parallel': return '#06b67f'
    case 'cache': return '#0ea5e9'  // 缓存节点使用天蓝色
    case 'token_usage': return '#a855f7'
    case 'transparent_forward': return '#14b8a6'
    case 'tool_call_injector': return '#f97316'
    default: return '#6b7280'
  }
}

const markDirty = () => {
  if (!isDirty.value) {
    isDirty.value = true
    emit('dirty-change', true)
  }
}

const clearDirty = () => {
  if (isDirty.value) {
    isDirty.value = false
    emit('dirty-change', false)
  }
}

const buildPipelineSnapshot = () => {
  const code = (pipelineInfo.value.shortcut_code || '').trim()
  return {
    ...props.pipeline,
    id: (pipelineInfo.value.id || props.pipeline?.id || '').trim(),
    name: (pipelineInfo.value.name || props.pipeline?.name || '').trim(),
    description: (pipelineInfo.value.description ?? props.pipeline?.description ?? '').trim(),
    version: (pipelineInfo.value.version || props.pipeline?.version || '1.0').trim(),
    shortcut_code: code,
    nodes: nodes.value.map(n => n.data || n),
    global_config: {
      ...(props.pipeline?.global_config || {}),
      ...globalConfig.value,
      fallback_groups: globalConfig.value.fallback_groups.filter(
        fg => fg.primary_node_id && fg.fallback_nodes.length > 0
      ),
      storage: globalConfig.value.storage_config.enabled ? {
        ...globalConfig.value.storage_config
      } : undefined,
      hooks: globalConfig.value.hooks.length > 0 ? [...globalConfig.value.hooks] : undefined
    },
    metadata: {
      ...props.pipeline?.metadata,
      lastUpdated: new Date().toISOString(),
      layoutVersion: '1.1'
    }
  }
}

// 将 Pipeline 数据转换为 VueFlow 格式
const convertToFlow = (pipeline: any) => {
  if (!pipeline?.nodes || !Array.isArray(pipeline.nodes)) {
    nodes.value = []
    edges.value = []
    return
  }

  const flowNodes: Node[] = []
  const flowEdges: Edge[] = []

  pipeline.nodes.forEach((nodeConfig: any, index: number) => {
    // 跳过空节点配置，防止 MiniMap 等组件渲染时访问 null 属性崩溃
    if (!nodeConfig || !nodeConfig.id) return

    const pos = nodeConfig.metadata?.position || {
      x: 80 + Math.floor(index / 4) * (NODE_WIDTH + 80),
      y: 60 + (index % 4) * (NODE_HEIGHT + 48)
    }

    flowNodes.push({
      id: nodeConfig.id,
      type: 'pipeline-node',
      position: pos,
      data: nodeConfig,
      style: { ...nodeCardStyle },
      class: `node-type-${nodeConfig.type || 'generator'}`
    })
  })

  // 生成连线
  pipeline.nodes.forEach((nodeConfig: any) => {
    if (!nodeConfig || !nodeConfig.id) return
    if (nodeConfig.next_nodes && Array.isArray(nodeConfig.next_nodes)) {
      nodeConfig.next_nodes.forEach((targetId: string) => {
        flowEdges.push({
          id: `e-${nodeConfig.id}-${targetId}`,
          source: nodeConfig.id,
          target: targetId,
          type: 'smoothstep',
          animated: true,
          style: { stroke: '#3b82f6', strokeWidth: 2 }
        })
      })
    }
  })

  nodes.value = flowNodes
  edges.value = flowEdges
  clearDirty()
  nextTick(() => vueFlowFitView({ padding: 0.25, maxZoom: 1, duration: 200 }))
}

const reloadFromPipeline = (pipeline: any) => {
  if (!pipeline) return
  convertToFlow(pipeline)
}

const onNodeClick = (event: any) => {
  if (event.node) {
    selectedNode.value = { ...event.node.data, id: event.node.id }
    drawerVisible.value = true
  }
}

const openNodeConfig = (nodeData: any) => {
  selectedNode.value = { ...nodeData }
  drawerVisible.value = true
}

const updateNode = (updatedNode: any) => {
  const index = nodes.value.findIndex(n => n.id === updatedNode.id)
  if (index !== -1) {
    // 使用 VueFlow 的 updateNode API 确保内部状态同步，避免 event.node.data 返回旧引用
    vueFlowUpdateNode(updatedNode.id, { data: { ...updatedNode } })
    // 同时更新本地 ref（VueFlow updateNode 可能不会立即同步 v-model 数组引用）
    const node = nodes.value[index]
    nodes.value[index] = { ...node, data: { ...updatedNode } }
  }

  // 更新父组件
  const updatedPipeline = {
    ...props.pipeline,
    nodes: nodes.value.map(n => n.data)
  }
  emit('update:pipeline', updatedPipeline)
  markDirty()
}

const deleteNode = async (nodeId: string) => {
  // 直接操作 nodes/edges ref（VueFlow v-model 会自动同步到画布）
  nodes.value = nodes.value.filter(n => n.id !== nodeId)
  edges.value = edges.value.filter(e => e.source !== nodeId && e.target !== nodeId)

  // 等待 VueFlow 完成内部图状态同步
  await nextTick()

  // 清理其他节点对该节点的引用
  nodes.value.forEach(n => {
    if (n.data.next_nodes) {
      n.data.next_nodes = n.data.next_nodes.filter((id: string) => id !== nodeId)
    }
    if (n.data.depends_on) {
      n.data.depends_on = n.data.depends_on.filter((id: string) => id !== nodeId)
    }
  })

  // 再次等待，确保清理后的数据已同步到 VueFlow
  await nextTick()

  const updatedPipeline = {
    ...props.pipeline,
    nodes: nodes.value.map(n => n.data)
  }
  emit('update:pipeline', updatedPipeline)
  markDirty()
  ElMessage.success(t('pipelineCanvas.nodeDeleted'))
}

const onNodesChange = (changes: NodeChange[]) => {
  changes.forEach(change => {
    if (change.type === 'position' && change.position) {
      const node = nodes.value.find(n => n.id === change.id)
      if (node && node.data) {
        if (!node.data.metadata) node.data.metadata = {}
        node.data.metadata.position = change.position
      }
    }
  })
  // VueFlow 初始渲染时也会触发 position 变更事件（dragging 未置为 true），
  // 只有用户真正拖拽节点（dragging === true）时才标记脏状态，避免无操作打开即提示保存。
  const hasUserDrag = changes.some((c: any) => c.type === 'position' && c.dragging === true)
  if (hasUserDrag) markDirty()
}

const onEdgesChange = (changes: EdgeChange[]) => {
  // 可以在这里同步 edges 到 next_nodes
  console.log('Edges changed:', changes)
}

const onConnect = (connection: Connection) => {
  const newEdge: Edge = {
    id: `e-${connection.source}-${connection.target}`,
    source: connection.source,
    target: connection.target,
    type: 'smoothstep',
    animated: true,
    style: { stroke: '#3b82f6', strokeWidth: 2 }
  }
  // 直接操作 edges ref（VueFlow v-model 会自动同步到画布）
  edges.value = [...edges.value, newEdge]

  // 同步到 source 节点的 next_nodes
  const sourceNode = nodes.value.find(n => n.id === connection.source)
  if (sourceNode?.data) {
    const next = sourceNode.data.next_nodes || []
    if (!next.includes(connection.target)) {
      sourceNode.data.next_nodes = [...next, connection.target]
    }
  }

  // 同步到 target 节点的 depends_on（连接线表示数据依赖关系）
  const targetNode = nodes.value.find(n => n.id === connection.target)
  if (targetNode?.data) {
    const deps = targetNode.data.depends_on || []
    if (!deps.includes(connection.source)) {
      targetNode.data.depends_on = [...deps, connection.source]
    }
  }

  emit('update:pipeline', {
    ...props.pipeline,
    nodes: nodes.value.map(n => n.data)
  })
  markDirty()
}

const addNewNode = () => {
  const newId = `node-${Date.now()}`

  // 后端和模型留空，由用户手动选择（避免使用可能不存在或未启用的默认值）
  const newNodeConfig = {
    id: newId,
    name: t('pipelineCanvas.newNode'),
    type: 'generator',
    backend: '',
    model: '',
    config: {},
    next_nodes: [],
    depends_on: [],
    retry: { max_attempts: 3, backoff_strategy: 'exponential', initial_delay: 1000, max_delay: 30000 },
    timeout: 60
  }

  const newNode: Node = {
    id: newId,
    type: 'pipeline-node',
    position: { x: 200, y: 200 },
    data: newNodeConfig,
    style: { ...nodeCardStyle },
    class: 'node-type-generator'
  }

  // 直接操作 nodes ref（VueFlow v-model 会自动同步到画布）
  nodes.value = [...nodes.value, newNode]
  nextTick(() => {
    emit('update:pipeline', {
      ...props.pipeline,
      nodes: nodes.value.map(n => n.data)
    })
  })
  markDirty()

  ElMessage.success(t('pipelineCanvas.newNodeAdded'))
}

const fitView = () => {
  nextTick(() => vueFlowFitView({ padding: 0.2, maxZoom: 1 }))
}

const autoLayout = () => {
  ElMessage.info(t('pipelineCanvas.autoLayoutInDevelopment'))
}

const savePipeline = () => {
  emit('save', buildPipelineSnapshot())
}

const runTest = async () => {
  if (testing.value) return // 防止重复提交（尤其是快速按 Enter 时）
  if (!testContent.value.trim()) {
    ElMessage.warning(t('pipelineCanvas.enterTestContent'))
    return
  }
  testing.value = true
  testResult.value = null
  try {
    const pipelineDef = buildPipelineSnapshot()
    const output = await executePipelineDirect({
      pipeline: pipelineDef,
      content: testContent.value
    })
    const pipelineOutput: any = output || {}
    const allNodesSucceeded = pipelineOutput.execution_log?.success !== false
    testResult.value = { success: allNodesSucceeded, ...pipelineOutput }
    if (allNodesSucceeded) {
      ElMessage.success(t('pipelineCanvas.testCompleted'))
    } else {
      ElMessage.warning(t('pipelineCanvas.testPartialFail'))
    }
  } catch (error: any) {
    testResult.value = { success: false, error: error.message || t('pipelineCanvas.executionFailed') }
    ElMessage.error(t('pipelineCanvas.testFailedPrefix') + (error.message || error))
  } finally {
    testing.value = false
  }
}

const exportPipeline = () => {
  try {
    const snapshot = buildPipelineSnapshot()
    const content = yamlDump(snapshot)
    const blob = new Blob([content], { type: 'text/yaml;charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    const safeId = (snapshot.id || 'pipeline').toString().replace(/[^a-zA-Z0-9_-]/g, '-')
    a.href = url
    a.download = `${safeId}.yaml`
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(url)
    ElMessage.success(t('pipelineCanvas.pipelineExported'))
  } catch (error: any) {
    ElMessage.error(t('pipelineCanvas.exportFailed') + (error?.message || error))
  }
}

const triggerImport = () => {
  if (importInputRef.value) {
    importInputRef.value.value = ''
    importInputRef.value.click()
  }
}

const handleImportFile = async (event: Event) => {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return

  try {
    const text = await file.text()
    const imported = yamlLoad(text)
    if (!imported || typeof imported !== 'object') {
      throw new Error(t('pipelineCanvas.invalidYaml'))
    }
    if (!Array.isArray(imported.nodes)) {
      throw new Error(t('pipelineCanvas.missingNodesArray'))
    }
    if (!imported.id || !imported.name || !imported.version) {
      throw new Error(t('pipelineCanvas.missingFields'))
    }

    const normalized = {
      ...imported,
      global_config: imported.global_config || {
        timeout: 120,
        max_retries: 3,
        bypass_on_error: true,
        stream_mode: false,
        parallel_limit: 4,
        log_level: 'info'
      },
      metadata: imported.metadata && typeof imported.metadata === 'object' ? imported.metadata : {}
    }

    emit('update:pipeline', normalized)
    convertToFlow(normalized)
    markDirty()
    ElMessage.success(t('pipelineCanvas.pipelineImported') + normalized.name)
  } catch (error: any) {
    ElMessage.error(t('pipelineCanvas.importFailed') + (error?.message || error))
  }
}

// 初始化
onMounted(() => {
  if (props.pipeline) {
    convertToFlow(props.pipeline)
  }
})

// 只在切换到不同流水线（id 变化）时重新初始化画布。
// 不用 deep watch 整个对象，避免 canvas 内部 emit('update:pipeline') 回写后
// 触发 convertToFlow → clearDirty，破坏脏状态追踪。
watch(() => props.pipeline?.id, (newId, oldId) => {
  if (newId && newId !== oldId && props.pipeline) {
    convertToFlow(props.pipeline)
  }
})

defineExpose({ clearDirtyState: clearDirty, reloadFromPipeline })
</script>

<style scoped>
.pipeline-canvas {
  height: 100%;
  min-height: 400px;
  flex: 1;
  display: flex;
  flex-direction: column;
  background: #f8fafc;
  border-radius: 8px;
  overflow: hidden;
}

.canvas-body {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.canvas-toolbar {
  padding: 12px 16px;
  background: white;
  border-bottom: 1px solid #e2e8f0;
  display: flex;
  justify-content: space-between;
  align-items: center;
  z-index: 10;
  border-radius: 8px 8px 0 0;
}

.legend {
  display: flex;
  gap: 12px;
  font-size: 12px;
}

.node-type {
  padding: 3px 10px;
  border-radius: 9999px;
  font-weight: 500;
  font-size: 11px;
}

.generator { background: #dbeafe; color: #1e40af; }
.processor { background: #d1fae5; color: #166534; }
.reviewer { background: #fef3c7; color: #854d0e; }
.router { background: #fee2e2; color: #991b1b; }
.aggregator { background: #ede9fe; color: #5b21b6; }
.parallel { background: #cffafe; color: #155e75; }
.cache { background: #e0f2fe; color: #075985; }
.token_usage { background: #f3e8ff; color: #6b21a8; }
.tool_call_injector { background: #fff7ed; color: #9a3412; }

.flow-wrapper {
  flex: 1;
  min-height: 0;
  border: 2px solid #e2e8f0;
  border-radius: 8px;
  overflow: hidden;
  background: 
    linear-gradient(#f1f5f9 1px, transparent 1px),
    linear-gradient(90deg, #f1f5f9 1px, transparent 1px);
  background-size: 24px 24px;
}

.vue-flow-container {
  width: 100%;
  height: 100%;
}

:deep(.vue-flow__node) {
  width: auto !important;
  height: auto !important;
}

:deep(.vue-flow__node-pipeline-node) {
  width: auto !important;
  height: auto !important;
  max-width: 176px;
}

.test-drawer-body {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.test-actions {
  margin-top: 12px;
}

.test-result {
  margin-top: 4px;
}

.result-content {
  margin-top: 8px;
  border: 1px solid #e2e8f0;
  border-radius: 6px;
  overflow: hidden;
}

.result-label {
  padding: 4px 10px;
  font-size: 12px;
  font-weight: 600;
  color: #64748b;
  background: #f1f5f9;
  border-bottom: 1px solid #e2e8f0;
}

.result-text {
  padding: 10px;
  font-size: 14px;
  line-height: 1.7;
  color: #1e293b;
  white-space: pre-wrap;
  word-break: break-word;
  max-height: 300px;
  overflow: auto;
}

.result-log-summary {
  display: flex;
  gap: 16px;
  margin-top: 8px;
  font-size: 13px;
  color: #64748b;
}

.result-json {
  background: #f8fafc;
  padding: 12px;
  border-radius: 0 0 8px 8px;
  font-family: ui-monospace, monospace;
  font-size: 13px;
  line-height: 1.5;
  max-height: 400px;
  overflow: auto;
  border: 1px solid #e2e8f0;
  white-space: pre-wrap;
  word-break: break-all;
  margin: 0;
}
</style>
