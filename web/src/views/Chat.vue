<template>
  <div class="chat">
    <div class="header">
      <h1 class="page-title">{{ t('chat.pageTitle') }}</h1>
      <p class="page-description">
        {{ t('chat.pageDescription') }}
      </p>
    </div>

    <div class="chat-body">
      <!-- 左侧配置面板 -->
      <div class="params-panel">
        <!-- 接入方式 -->
        <div class="access-section access-section-primary">
          <div class="params-panel-title">{{ t('chat.sectionRecommended') }}</div>
          <p class="access-section-hint">{{ t('chat.recommendedHint') }}</p>
          <el-radio-group v-model="accessMode" class="source-radio-group">
            <el-tooltip :content="t('chat.tooltipDefault')" placement="right">
              <el-radio value="default">
                <span class="radio-label">
                  <span class="radio-icon">🔗</span>
                  <span>{{ t('chat.modeDefault') }}</span>
                </span>
              </el-radio>
            </el-tooltip>
            <el-tooltip :content="t('chat.tooltipKeyword')" placement="right">
              <el-radio value="keyword">
                <span class="radio-label">
                  <span class="radio-icon">✏️</span>
                  <span>{{ t('chat.modeKeyword') }}</span>
                </span>
              </el-radio>
            </el-tooltip>
          </el-radio-group>
        </div>

        <div class="access-section access-section-advanced">
          <div class="params-panel-title">
            {{ t('chat.sectionAdvanced') }}
            <el-tag size="small" type="warning" effect="plain" class="access-badge">{{ t('chat.badgeAdvanced') }}</el-tag>
          </div>
          <p class="access-section-hint">{{ t('chat.advancedHint') }}</p>
          <el-radio-group v-model="accessMode" class="source-radio-group">
            <el-tooltip :content="t('chat.tooltipModel')" placement="right">
              <el-radio value="model">
                <span class="radio-label">
                  <span class="radio-icon">🏷️</span>
                  <span>{{ t('chat.modeModel') }}</span>
                </span>
              </el-radio>
            </el-tooltip>
            <el-tooltip :content="t('chat.tooltipHeader')" placement="right">
              <el-radio value="header">
                <span class="radio-label">
                  <span class="radio-icon">📋</span>
                  <span>{{ t('chat.modeHeader') }}</span>
                </span>
              </el-radio>
            </el-tooltip>
          </el-radio-group>
        </div>

        <!-- 默认方式说明 -->
        <div v-if="accessMode === 'default'" class="mode-explanation">
          <div class="explanation-title">
            <el-icon><InfoFilled /></el-icon>
            {{ t('chat.modeDefault') }}
          </div>
          <p class="explanation-text">
            {{ t('chat.defaultExplain') }}
          </p>
          <div class="default-pipeline-card" v-loading="pipelinesLoading">
            <div class="default-pipeline-row">
              <span class="default-pipeline-label">{{ t('chat.currentDefault') }}</span>
              <span class="default-pipeline-name">{{ defaultPipeline?.name || defaultPipelineId || t('chat.unset') }}</span>
            </div>
            <div v-if="defaultPipelineId" class="default-pipeline-id mono">{{ defaultPipelineId }}</div>
            <div class="default-pipeline-actions">
              <el-link type="primary" size="small" @click="$router.push('/pipelines')">{{ t('chat.managePipeline') }}</el-link>
              <el-link type="primary" size="small" @click="$router.push('/dashboard')">{{ t('chat.changeDefault') }}</el-link>
            </div>
          </div>
          <div class="example-block">
            <div class="example-label">{{ t('chat.clientExample') }}</div>
            <pre class="example-code">Base URL: https://your-proxy/v1
API Key: &lt;your-key&gt;
{{ t('chat.clientExampleModel') }}</pre>
          </div>
        </div>

        <!-- 关键码方式说明 -->
        <div v-if="accessMode === 'keyword'" class="mode-explanation">
          <div class="explanation-title">
            <el-icon><InfoFilled /></el-icon>
            {{ t('chat.modeKeyword') }}
          </div>
          <p class="explanation-text">
            {{ t('chat.keywordExplain') }}
          </p>
        </div>

        <!-- 模型名方式说明 -->
        <div v-if="accessMode === 'model'" class="mode-explanation">
          <div class="explanation-title">
            <el-icon><InfoFilled /></el-icon>
            {{ t('chat.modeModel') }}
          </div>
          <p class="explanation-text">
            {{ t('chat.modelExplain') }}
          </p>
        </div>

        <!-- 请求头方式说明 -->
        <div v-if="accessMode === 'header'" class="mode-explanation">
          <div class="explanation-title">
            <el-icon><InfoFilled /></el-icon>
            {{ t('chat.modeHeader') }}
          </div>
          <p class="explanation-text">
            {{ t('chat.headerExplain') }}
          </p>
        </div>

        <!-- 模型名 / 请求头：选择流水线 -->
        <template v-if="accessMode === 'model' || accessMode === 'header'">
          <div class="params-divider"></div>
          <div class="params-panel-title">{{ t('chat.selectPipeline') }}</div>
          <el-form label-position="top" size="small" class="panel-form">
            <el-form-item>
              <el-select
                v-model="selectedPipelineId"
                :placeholder="t('chat.selectPipelinePlaceholder')"
                style="width: 100%"
                :loading="pipelinesLoading"
              >
                <el-option
                  v-for="p in pipelines"
                  :key="p.id"
                  :label="p.name || p.id"
                  :value="p.id"
                >
                  <span>{{ p.name || p.id }}</span>
                  <span v-if="p.description" class="pipeline-option-desc">{{ p.description }}</span>
                </el-option>
              </el-select>
              <div class="form-tip">
                {{ t('chat.pipelineFormHint') }}
                <el-link type="primary" size="small" style="margin-left: 6px" @click="$router.push('/pipelines')">{{ t('chat.editPipeline') }}</el-link>
              </div>
            </el-form-item>
            <div v-if="accessMode === 'model'" class="example-block">
              <div class="example-label">{{ t('chat.modelFieldLabel') }}</div>
              <pre class="example-code">{{ modelFieldPreview }}</pre>
            </div>
            <div v-if="accessMode === 'header'" class="example-block">
              <div class="example-label">{{ t('chat.headerFieldLabel') }}</div>
              <pre class="example-code">X-Pipeline-ID: {{ selectedPipelineId || t('chat.pipelineIdPlaceholder') }}</pre>
              <div class="example-label" style="margin-top: 8px">{{ t('chat.bodyModelLabel') }}</div>
              <pre class="example-code">auto</pre>
            </div>
          </el-form>
        </template>

        <!-- 关键码快捷按钮 -->
        <div v-if="accessMode === 'keyword'" class="params-divider"></div>
        <div v-if="accessMode === 'keyword'" class="params-panel-title">{{ t('chat.builtinKeywords') }}</div>
        <div v-if="accessMode === 'keyword'" class="keyword-buttons">
          <el-tooltip
            v-for="item in builtinShortcuts"
            :key="item.code"
            :content="item.label"
            placement="top"
          >
            <el-tag
              @click="insertKeyword(item.code + ' ')"
              class="keyword-tag"
              :type="item.type || undefined"
            >{{ item.code }}</el-tag>
          </el-tooltip>
        </div>
        <div v-if="accessMode === 'keyword' && pipelineShortcuts.length > 0" class="params-panel-title" style="margin-top: 8px">
          {{ t('chat.pipelineShortcuts') }}
        </div>
        <div v-if="accessMode === 'keyword' && pipelineShortcuts.length > 0" class="keyword-buttons">
          <el-tooltip
            v-for="item in pipelineShortcuts"
            :key="item.code"
            :content="`${item.name} (${item.pipelineId})`"
            placement="top"
          >
            <el-tag
              @click="insertKeyword(item.code + ' ')"
              class="keyword-tag"
              type="primary"
            >{{ item.code }}</el-tag>
          </el-tooltip>
        </div>
        <p v-if="accessMode === 'keyword'" class="params-panel-hint">
          {{ t('chat.keywordHint') }}
        </p>

        <div class="params-divider"></div>

        <!-- 模型生成参数 -->
        <div class="params-panel-title">{{ t('chat.generationParams') }}</div>
        <el-form :model="settings" label-position="top" size="small" class="panel-form">
          <el-form-item>
            <template #label>
              <span class="label-with-tip">
                {{ t('chat.temperature') }}
                <el-tooltip placement="top" :show-after="400" max-width="300">
                  <template #content>
                    <div>
                      {{ t('chat.temperatureTip') }}
                    </div>
                  </template>
                  <el-icon class="label-help-icon" :size="14"><QuestionFilled /></el-icon>
                </el-tooltip>
              </span>
            </template>
            <el-slider v-model="settings.temperature" :min="0" :max="2" :step="0.1" />
            <span class="param-value">{{ settings.temperature }}</span>
          </el-form-item>
          <el-form-item :label="t('chat.maxToken')">
            <el-input-number
              v-model="settings.max_tokens"
              :min="1"
              :max="32768"
              :step="100"
              style="width: 100%"
              controls-position="right"
            />
          </el-form-item>
          <el-form-item :label="t('chat.streamOutput')">
            <el-switch v-model="settings.stream" :active-text="t('chat.on')" :inactive-text="t('chat.off')" />
          </el-form-item>
        </el-form>
      </div>

      <!-- 右侧对话区 -->
      <div class="chat-container">
        <div class="chat-main">
        <div class="chat-messages" ref="messagesContainer">
          <!-- HTTP 请求预览 -->
          <div v-if="accessMode === 'header' || accessMode === 'model'" class="http-preview">
            <div class="http-preview-title">
              <el-icon><Document /></el-icon>
              {{ accessMode === 'model' ? t('chat.httpModelExample') : t('chat.httpHeaderPreview') }}
            </div>
            <p v-if="accessMode === 'model'" class="http-preview-hint">
              {{ t('chat.httpPreviewHint') }}
            </p>
            <div class="http-preview-content">
              <div class="http-line method-line">
                <span class="http-label">POST</span>
                <span class="http-value">/v1/chat/completions</span>
              </div>
              <div class="http-section">
                <div class="http-section-title">Headers:</div>
                <pre class="http-code" v-html="formattedHeaders"></pre>
              </div>
              <div class="http-section">
                <div class="http-section-title">Body:</div>
                <pre class="http-code">{{ formattedBodyPreview }}</pre>
              </div>
            </div>
          </div>

          <!-- 关键字方式说明卡片 -->
          <div v-if="accessMode === 'keyword'" class="keyword-guide">
            <div class="keyword-guide-title">
              <el-icon><MagicStick /></el-icon>
              {{ t('chat.keywordGuide') }}
            </div>
            <div class="keyword-guide-content">
              <p>{{ t('chat.keywordGuideDesc') }}</p>
              <div class="keyword-table">
                <div class="keyword-row header-row">
                  <span>{{ t('chat.keyword') }}</span>
                  <span>{{ t('chat.proxyMode') }}</span>
                  <span>{{ t('chat.description') }}</span>
                </div>
                <div class="keyword-row">
                  <code>#d</code>
                  <span>direct</span>
                  <span>{{ t('chat.directDesc') }}</span>
                </div>
                <div class="keyword-row">
                  <code>#s</code>
                  <span>smart</span>
                  <span>{{ t('chat.smartDesc') }}</span>
                </div>
                <div class="keyword-row">
                  <code>#m</code>
                  <span>model-match</span>
                  <span>{{ t('chat.modelMatchDesc') }}</span>
                </div>
                <div class="keyword-row">
                  <code>#c</code>
                  <span>classify</span>
                  <span>{{ t('chat.classifyDesc') }}</span>
                </div>
                <div class="keyword-row">
                  <code>#t</code>
                  <span>transparent</span>
                  <span>{{ t('chat.transparentDesc') }}</span>
                </div>
                <div class="keyword-row">
                  <code>#a</code>
                  <span>audit</span>
                  <span>{{ t('chat.auditDesc') }}</span>
                </div>
                <div class="keyword-row">
                  <code>#f</code>
                  <span>fallback</span>
                  <span>{{ t('chat.fallbackDesc') }}</span>
                </div>
              </div>
              <div class="keyword-example">
                <div class="example-label">{{ t('chat.sendExample') }}</div>
                <pre class="example-code">#c 你使用PPIO的deepseek模型回答</pre>
                <div class="example-arrow">↓</div>
                <div class="example-label">{{ t('chat.actualSend') }}</div>
                <pre class="example-code">#c 你使用PPIO的deepseek模型回答</pre>
                <div class="example-hint">{{ t('chat.exampleHint') }}</div>
              </div>
            </div>
          </div>

          <div
            v-for="(message, index) in messages"
            :key="index"
            :class="['message', message.role]"
          >
            <div class="message-avatar">
              <el-icon v-if="message.role === 'user'" :size="24"><User /></el-icon>
              <el-icon v-else :size="24"><ChatDotRound /></el-icon>
            </div>
            <div class="message-content">
              <div class="message-role">
                {{ message.role === 'user' ? t('chat.userLabel') : t('chat.assistantLabel') }}
              </div>
              <div class="message-text" v-html="formatMessage(message.content)"></div>
              <!-- 显示该消息使用的代理模式（关键字方式时） -->
              <div v-if="message.role === 'user' && message.proxyMode" class="message-mode-tag">
                <el-tag :type="getModeTagType(message.proxyMode)" size="small">
                  {{ t('chat.keywordModeLabel', { mode: message.proxyMode }) }}
                </el-tag>
              </div>
              <span v-if="message.streaming" class="streaming-cursor">▋</span>
              <!-- 回答元信息 -->
              <div v-if="message.role === 'assistant' && !message.streaming && message.meta" class="message-meta">
                <el-icon class="meta-icon"><InfoFilled /></el-icon>
                <!-- 代理模式 -->
                <span class="meta-item">
                  <span class="meta-label">{{ t('chat.metaMode') }}</span>
                  <el-tag :type="getModeTagType(message.meta.proxyMode)" size="small" effect="plain">
                    {{ modeNames[message.meta.proxyMode] || message.meta.proxyMode || '—' }}
                  </el-tag>
                </span>
                <span class="meta-sep">·</span>
                <!-- 服务 -->
                <span class="meta-item">
                  <span class="meta-label">{{ t('chat.metaService') }}</span>
                  <span class="meta-value">{{ resolveDisplayBackend(message.meta) }}</span>
                </span>
                <span class="meta-sep">·</span>
                <!-- 模型（流水线节点实际使用） -->
                <span class="meta-item">
                  <span class="meta-label">{{ t('chat.metaModel') }}</span>
                  <span class="meta-value">{{ resolveDisplayModel(message.meta) }}</span>
                </span>
                <!-- 审核模式额外信息 -->
                <template v-if="message.meta.proxyMode === '#a'">
                  <span class="meta-sep">·</span>
                  <span class="meta-item">
                    <span class="meta-label">{{ t('chat.metaExecute') }}</span>
                    <span class="meta-value">{{ resolveDisplayModel(message.meta) }}</span>
                  </span>
                  <template v-if="message.meta.auditorBackend">
                    <span class="meta-sep">·</span>
                    <span class="meta-item">
                      <span class="meta-label">{{ t('chat.metaAudit') }}</span>
                      <span class="meta-value">{{ message.meta.auditorBackend }}</span>
                    </span>
                    <template v-if="message.meta.auditorModel">
                      <span class="meta-sep">/</span>
                      <span class="meta-value">{{ message.meta.auditorModel }}</span>
                    </template>
                  </template>
                  <!-- 审核结果 -->
                  <template v-if="message.meta.auditPassed !== undefined">
                    <span class="meta-sep">·</span>
                    <span class="meta-item">
                      <el-tag
                        :type="message.meta.auditPassed === 'true' || message.meta.auditPassed === true ? 'success' : 'danger'"
                        size="small"
                        effect="plain"
                      >
                        {{ message.meta.auditPassed === 'true' || message.meta.auditPassed === true ? t('chat.auditPassed') : t('chat.auditFailed') }}
                      </el-tag>
                    </span>
                    <template v-if="message.meta.auditScore">
                      <span class="meta-sep">·</span>
                      <span class="meta-item">
                        <span class="meta-label">{{ t('chat.metaScore') }}</span>
                        <span class="meta-value">{{ parseFloat(message.meta.auditScore).toFixed(2) }}</span>
                      </span>
                    </template>
                    <template v-if="message.meta.auditFeedback">
                      <el-tooltip :content="message.meta.auditFeedback" placement="top" effect="light">
                        <span class="meta-sep">·</span>
                        <span class="meta-item">
                          <span class="meta-label">{{ t('chat.metaFeedback') }}</span>
                          <span class="meta-value audit-feedback">{{ message.meta.auditFeedback.substring(0, 30) }}{{ message.meta.auditFeedback.length > 30 ? '...' : '' }}</span>
                        </span>
                      </el-tooltip>
                    </template>
                  </template>
                </template>
                <!-- 模型匹配模式额外信息 -->
                <template v-if="message.meta.proxyMode === '#m' && message.meta.analyzerBackend">
                  <span class="meta-sep">·</span>
                  <span class="meta-item">
                    <span class="meta-label">{{ t('chat.metaAnalyze') }}</span>
                    <span class="meta-value">{{ message.meta.analyzerBackend }}</span>
                    <template v-if="message.meta.analyzerModel">
                      <span class="meta-sep">/</span>
                      <span class="meta-value">{{ message.meta.analyzerModel }}</span>
                    </template>
                  </span>
                  <template v-if="message.meta.matchStrategy">
                    <span class="meta-sep">·</span>
                    <span class="meta-item">
                      <span class="meta-label">{{ t('chat.metaStrategy') }}</span>
                      <el-tag size="small" effect="plain">{{ message.meta.matchStrategy }}</el-tag>
                    </span>
                  </template>
                </template>
                
                <!-- 优化模式额外信息 -->
                <template v-if="message.meta.proxyMode === '#o'">
                  <span class="meta-sep">·</span>
                  <span class="meta-item">
                    <span class="meta-label">{{ t('chat.metaExecute') }}</span>
                    <span class="meta-value">{{ resolveDisplayModel(message.meta) }}</span>
                  </span>
                  <template v-if="message.meta.optimizerBackend">
                    <span class="meta-sep">·</span>
                    <span class="meta-item">
                      <span class="meta-label">{{ t('chat.metaOptimize') }}</span>
                      <span class="meta-value">{{ message.meta.optimizerBackend }}</span>
                      <template v-if="message.meta.optimizerModel">
                        <span class="meta-sep">/</span>
                        <span class="meta-value">{{ message.meta.optimizerModel }}</span>
                      </template>
                    </span>
                  </template>
                  <template v-if="message.meta.optimizeApplied !== undefined">
                    <span class="meta-sep">·</span>
                    <span class="meta-item">
                      <el-tag
                        :type="message.meta.optimizeApplied === 'true' ? 'success' : 'warning'"
                        size="small"
                        effect="plain"
                      >
                        {{ message.meta.optimizeApplied === 'true' ? t('chat.optimizeSuccess') : t('chat.optimizeFailed') }}
                      </el-tag>
                    </span>
                  </template>
                </template>
                <!-- 流水线执行信息 -->
                <template v-if="message.meta.pipelineId">
                  <span class="meta-sep">·</span>
                  <span class="meta-item">
                    <span class="meta-label">{{ t('chat.metaPipeline') }}</span>
                    <span class="meta-value mono">{{ message.meta.pipelineId }}</span>
                  </span>
                  <template v-if="message.meta.pipelineDuration">
                    <span class="meta-sep">·</span>
                    <span class="meta-item">
                      <span class="meta-label">{{ t('chat.metaDuration') }}</span>
                      <span class="meta-value">{{ message.meta.pipelineDuration }}ms</span>
                    </span>
                  </template>
                </template>
                <span class="meta-sep">·</span>
                <span class="meta-item">
                  <el-tag
                    :type="getCacheTagType(message.meta)"
                    size="small"
                    effect="plain"
                  >{{ getCacheTagText(message.meta) }}</el-tag>
                </span>
              </div>

              <!-- 流水线调试面板：展开完整响应数据体 -->
              <div
                v-if="message.role === 'assistant' && !message.streaming && message.meta?.rawData"
                class="pipeline-debug-panel"
              >
                <button
                  class="pipeline-debug-toggle"
                  @click="rawDataExpanded[index] = !rawDataExpanded[index]"
                >
                  <span class="debug-toggle-icon">{{ rawDataExpanded[index] ? '▾' : '▸' }}</span>
                  <span>{{ t('chat.fullResponseData') }}</span>
                  <span v-if="!rawDataExpanded[index]" class="debug-toggle-hint">
                    · passed={{ message.meta.rawData.passed }}
                    <template v-if="message.meta.rawData.score != null">
                      · score={{ typeof message.meta.rawData.score === 'number' ? message.meta.rawData.score.toFixed(2) : message.meta.rawData.score }}
                    </template>
                  </span>
                </button>
                <div v-if="rawDataExpanded[index]" class="pipeline-debug-body">
                  <pre class="pipeline-debug-json">{{ JSON.stringify(message.meta.rawData, null, 2) }}</pre>
                </div>
              </div>
            </div>
          </div>

          <div v-if="loading" class="message assistant">
            <div class="message-avatar">
              <el-icon :size="24"><ChatDotRound /></el-icon>
            </div>
            <div class="message-content">
              <div class="message-role">{{ t('chat.assistantLabel') }}</div>
              <div class="message-text">
                <span class="typing-indicator">{{ settings.stream ? t('chat.thinking') : t('chat.typing') }}</span>
              </div>
            </div>
          </div>

          <!-- 空状态提示 -->
          <div v-if="messages.length === 0 && !loading" class="empty-messages">
            <div class="empty-icon">💬</div>
            <div class="empty-title">{{ t('chat.emptyTitle') }}</div>
            <div class="empty-description">{{ emptyStateHint }}</div>
          </div>
        </div>

        <div class="chat-input-area">
          <el-input
            v-model="inputMessage"
            type="textarea"
            :rows="3"
            :placeholder="inputPlaceholder"
            @keydown="handleKeydown"
            :disabled="loading"
          ></el-input>
          <div class="input-actions">
            <el-button @click="clearMessages" :disabled="loading || messages.length === 0">
              <el-icon><Delete /></el-icon>
              {{ t('chat.clearChat') }}
            </el-button>
            <el-button type="primary" @click="sendMessage" :loading="loading" :disabled="!inputMessage.trim()">
              <el-icon><Promotion /></el-icon>
              {{ t('chat.send') }}
            </el-button>
          </div>
        </div>
      </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'
import { ElMessage } from 'element-plus'
import { User, ChatDotRound, Delete, Promotion, InfoFilled, QuestionFilled, Document, MagicStick } from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'

import { useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useChatPipelines } from '@/composables/useChatPipelines'
import {
  extractResponseMeta,
  mergeCentagMeta,
  resolveDisplayBackend,
  resolveDisplayModel
} from '@/composables/useChatResponseMeta'
import {
  BUILTIN_SHORTCUTS,
  MODE_NAMES,
  buildKeywordToModeMap,
  buildPipelineModelField,
  extractProxyMode,
  getModeTagType,
  type ChatAccessMode
} from '@/utils/chat-access'
import {
  extractStreamDeltaContent,
  isStreamRolePlaceholder
} from '@/utils/chat-stream'

const route = useRoute()
const authStore = useAuthStore()
const { t } = useI18n()

const accessMode = ref<ChatAccessMode>('default')

const {
  pipelines,
  defaultPipelineId,
  defaultPipeline,
  pipelineShortcuts,
  pipelinesLoading,
  loadPipelines
} = useChatPipelines()

const selectedPipelineId = ref('')

const modelFieldPreview = computed(() => buildPipelineModelField(selectedPipelineId.value))

const builtinShortcuts = BUILTIN_SHORTCUTS
const modeNames = MODE_NAMES

const loading = ref(false)
const inputMessage = ref('')
const messages = ref<any[]>([])
const messagesContainer = ref<HTMLElement | null>(null)
// 调试面板：记录每条消息是否展开原始数据体（key = 消息在 messages 中的 index）
const rawDataExpanded = ref<Record<number, boolean>>({})

const settings = ref({
  temperature: 0.7,
  max_tokens: 16384,
  stream: true
})

const keywordToModeMap = computed(() => buildKeywordToModeMap(pipelineShortcuts.value))

const emptyStateHint = computed(() => {
  switch (accessMode.value) {
    case 'default':
      return t('chat.emptyHintDefault')
    case 'keyword':
      return t('chat.emptyHintKeyword')
    case 'model':
      return t('chat.emptyHintModel', { model: modelFieldPreview.value })
    case 'header':
      return t('chat.emptyHintHeader')
    default:
      return t('chat.emptyHint')
  }
})

const inputPlaceholder = computed(() => {
  if (accessMode.value === 'keyword') {
    return t('chat.placeholderKeyword')
  }
  if (accessMode.value === 'model') {
    return t('chat.placeholderModel')
  }
  return t('chat.placeholderDefault')
})

watch(defaultPipelineId, (id) => {
  if (id && !selectedPipelineId.value) {
    selectedPipelineId.value = id
  }
}, { immediate: true })

// 计算格式化后的请求头预览（用于 HTTP 请求预览）
const formattedHeaders = computed(() => {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    'Authorization': 'Bearer ${API_KEY}',
  }

  if (accessMode.value === 'header' && selectedPipelineId.value) {
    headers['X-Pipeline-ID'] = selectedPipelineId.value
  }
  // default / keyword / model：仅标准头；model 方式路由在请求体 model 字段

  // 将 headers 对象格式化为可显示的字符串
  let result = ''
  for (const [key, value] of Object.entries(headers)) {
    result += `<span class="http-header-key">${key}</span>: <span class="http-header-value">${value}</span>\n`
  }
  return result
})

// 计算格式化后的请求体预览
const previewRequestModel = computed(() => {
  if (accessMode.value === 'model') return modelFieldPreview.value
  return 'auto'
})

const formattedBodyPreview = computed(() => {
  const body = {
    model: previewRequestModel.value,
    messages: [
      { role: 'user', content: accessMode.value === 'keyword' ? t('chat.messageContentPreviewKeyword') : t('chat.messageContentPreview') }
    ],
    temperature: settings.value.temperature,
    max_tokens: settings.value.max_tokens,
    stream: settings.value.stream
  }
  return JSON.stringify(body, null, 2)
})

watch(() => route.path, async (newPath) => {
  if (newPath === '/chat') {
    await loadPipelines()
  }
})

function scrollToBottom() {
  nextTick(() => {
    if (messagesContainer.value) {
      messagesContainer.value.scrollTop = messagesContainer.value.scrollHeight
    }
  })
}

function handleKeydown(event: KeyboardEvent) {
  if (event.key === 'Enter' && event.ctrlKey) {
    event.preventDefault()
    sendMessage()
  }
}

function insertKeyword(keyword: string) {
  inputMessage.value = keyword + inputMessage.value
}



function buildBaseHeaders(): Record<string, string> {
  return {
    'Content-Type': 'application/json',
    'Authorization': `Bearer ${authStore.accessToken || ''}`
  }
}

function resolveChatRequest(rawContent: string): {
  requestModel: string
  effectiveProxyMode: string
  usedKeyword: string | null
  headers: Record<string, string>
} | null {
  let requestModel = 'auto'
  let effectiveProxyMode = 'default'
  let usedKeyword: string | null = null
  let headers = buildBaseHeaders()

  if (accessMode.value === 'default') {
    requestModel = 'auto'
    effectiveProxyMode = 'default'
  } else if (accessMode.value === 'keyword') {
    const result = extractProxyMode(rawContent, keywordToModeMap.value)
    effectiveProxyMode = result.proxyMode || 'default'
    requestModel = 'auto'
    if (result.proxyMode) {
      usedKeyword = result.proxyMode
      ElMessage.success(t('chat.keywordRecognized', { mode: modeNames[result.proxyMode] || result.proxyMode }))
    }
  } else if (accessMode.value === 'model') {
    if (!selectedPipelineId.value) {
      ElMessage.warning(t('chat.selectPipelineFirst'))
      return null
    }
    requestModel = buildPipelineModelField(selectedPipelineId.value)
    effectiveProxyMode = 'pipeline'
  } else {
    if (!selectedPipelineId.value) {
      ElMessage.warning(t('chat.selectPipelineFirst'))
      return null
    }
    headers['X-Pipeline-ID'] = selectedPipelineId.value
    requestModel = 'auto'
    effectiveProxyMode = 'pipeline'
  }

  return { requestModel, effectiveProxyMode, usedKeyword, headers }
}

async function sendMessage() {
  const rawContent = inputMessage.value.trim()
  if (!rawContent) return

  const resolved = resolveChatRequest(rawContent)
  if (!resolved) return

  const { requestModel, effectiveProxyMode, usedKeyword, headers } = resolved

  messages.value.push({
    role: 'user',
    content: rawContent,
    proxyMode: usedKeyword
  })
  inputMessage.value = ''
  scrollToBottom()

  loading.value = true

  // 确定发送给API的消息内容
  // 关键字模式：发送原始内容，让后端解析关键字
  // 请求头模式：发送原始内容
  const apiContent = rawContent

  // 如果开启流式输出
  if (settings.value.stream) {
      let assistantMessageIndex = -1
      let streamCentag: Record<string, any> | null = null

      try {
      // 构建发送给API的消息列表（最后一条用户消息使用原始内容）
      const apiMessages = messages.value.map((m, idx) => {
        if (idx === messages.value.length - 1 && m.role === 'user') {
          return { role: m.role, content: apiContent }  // 发送原始内容
        }
        return { role: m.role, content: m.content }
      })

      const response = await fetch('/v1/chat/completions', {
        method: 'POST',
        headers: headers,
        body: JSON.stringify({
          model: requestModel,
          messages: apiMessages,
          temperature: settings.value.temperature,
          max_tokens: settings.value.max_tokens,
          stream: true
        })
      })

      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`)
      }

      const responseMeta = extractResponseMeta(response.headers)

      const reader = response.body?.getReader()
      const decoder = new TextDecoder()
      
      if (!reader) {
        throw new Error(t('chat.cannotReadStream'))
      }

      let buffer = ''
      while (true) {
        const { done, value } = await reader.read()
        
        if (done) break
        
        buffer += decoder.decode(value, { stream: true })
        const lines = buffer.split('\n')
        
        // 保留最后一个可能不完整的行
        buffer = lines.pop() || ''
        
        for (const line of lines) {
          const trimmedLine = line.trim()
          if (!trimmedLine || trimmedLine === 'data: [DONE]') continue

          if (trimmedLine.startsWith('data: ')) {
            try {
              const jsonStr = trimmedLine.slice(6)
              const data = JSON.parse(jsonStr)

              if (data.centag && typeof data.centag === 'object') {
                streamCentag = data.centag
              }

              // 处理后端返回的错误事件 {"error": "..."}
              if (data.error && !data.choices) {
                loading.value = false
                const errMsg = typeof data.error === 'string' ? data.error : JSON.stringify(data.error)
                if (assistantMessageIndex === -1) {
                  assistantMessageIndex = messages.value.length
                  messages.value.push({
                    role: 'assistant',
                    content: t('chat.backendError', { error: errMsg }),
                    streaming: false,
                    meta: responseMeta
                  })
                } else {
                  messages.value[assistantMessageIndex].content += `\n\n${t('chat.backendError', { error: errMsg })}`
                  messages.value[assistantMessageIndex].streaming = false
                }
                continue
              }

              const choice = data.choices?.[0] as Record<string, unknown> | undefined
              const deltaText = extractStreamDeltaContent(choice)
              const hasContent = deltaText.length > 0

              if (hasContent) {
                // 收到第一个有内容的 chunk 时，移除 loading 并添加 assistant 消息
                if (assistantMessageIndex === -1) {
                  loading.value = false
                  assistantMessageIndex = messages.value.length
                  messages.value.push({
                    role: 'assistant',
                    content: deltaText,
                    streaming: true,
                    meta: responseMeta
                  })
                } else {
                  messages.value[assistantMessageIndex].content += deltaText
                }
                scrollToBottom()
              } else if (assistantMessageIndex === -1 && isStreamRolePlaceholder(choice)) {
                // 首 chunk 仅有 role 无 content：占位，等待后续正文
                loading.value = false
                assistantMessageIndex = messages.value.length
                messages.value.push({
                  role: 'assistant',
                  content: '',
                  streaming: true,
                  meta: responseMeta
                })
              }
            } catch (e) {
              console.warn('Failed to parse SSE data:', trimmedLine, e)
            }
          }
        }
      }

      // 流结束：若从未创建过 assistant 消息（如后端只发了 [DONE] 或全是空 content），补一条占位
      if (assistantMessageIndex === -1) {
        assistantMessageIndex = messages.value.length
        messages.value.push({
          role: 'assistant',
          content: t('chat.noModelReply'),
          streaming: false
        })
      } else {
        // 若已创建过消息但内容为空，显示友好提示
        if (!messages.value[assistantMessageIndex].content?.trim()) {
          messages.value[assistantMessageIndex].content =
            t('chat.noModelReplyHint')
        }
        messages.value[assistantMessageIndex].streaming = false
        if (streamCentag) {
          messages.value[assistantMessageIndex].meta = mergeCentagMeta(
            messages.value[assistantMessageIndex].meta,
            streamCentag
          )
        }
      }
    } catch (error: any) {
      console.error('Failed to send message:', error)
      
      // 提取详细的错误信息
      let detailedError = error.message || t('chat.unknownError')
      
      // 尝试从响应体中提取错误信息
      if (error.response?.data) {
        try {
          const errData = typeof error.response.data === 'string' 
            ? JSON.parse(error.response.data) 
            : error.response.data
          if (errData?.error) {
            detailedError = typeof errData.error === 'string' 
              ? errData.error 
              : JSON.stringify(errData.error)
          } else if (errData?.message) {
            detailedError = errData.message
          }
        } catch {
          // 解析失败，使用原始错误信息
        }
      }
      
      // 尝试从 error 对象直接提取 error 字段
      if (error.error && !detailedError.includes(error.error)) {
        detailedError = typeof error.error === 'string' ? error.error : JSON.stringify(error.error)
      }
      
      ElMessage.error(t('chat.sendFailed', { error: detailedError }))
      if (assistantMessageIndex >= 0 && messages.value[assistantMessageIndex]) {
        messages.value[assistantMessageIndex].content = `**${t('chat.requestFailed')}**\n\n${detailedError}`
        messages.value[assistantMessageIndex].streaming = false
      } else {
        messages.value.push({
          role: 'assistant',
          content: `**${t('chat.requestFailed')}**\n\n${detailedError}`,
          streaming: false
        })
      }
    } finally {
      loading.value = false
      scrollToBottom()
    }
  } else {
    // 非流式输出
    try {
      // 构建发送给API的消息列表（最后一条用户消息使用原始内容）
      const apiMessages = messages.value.map((m, idx) => {
        if (idx === messages.value.length - 1 && m.role === 'user') {
          return { role: m.role, content: apiContent }  // 发送原始内容
        }
        return { role: m.role, content: m.content }
      })

      const response = await fetch('/v1/chat/completions', {
        method: 'POST',
        headers,
        body: JSON.stringify({
          model: requestModel,
          messages: apiMessages,
          temperature: settings.value.temperature,
          max_tokens: settings.value.max_tokens,
          stream: false
        })
      })

      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`)
      }

      const data = await response.json()
      const choice = data?.choices?.[0] as Record<string, unknown> | undefined
      const assistantMessage =
        extractStreamDeltaContent(choice) ||
        t('chat.noModelReplyHint')
      const nonStreamMeta = extractResponseMeta(response.headers)
      messages.value.push({
        role: 'assistant',
        content: assistantMessage,
        meta: mergeCentagMeta(nonStreamMeta, data?.centag)
      })
    } catch (error: any) {
      console.error('Failed to send message:', error)
      
      // 提取详细的错误信息
      let detailedError = error.message || t('chat.unknownError')
      
      // 尝试从响应体中提取错误信息
      if (error.response?.data) {
        try {
          const errData = typeof error.response.data === 'string' 
            ? JSON.parse(error.response.data) 
            : error.response.data
          if (errData?.error) {
            detailedError = typeof errData.error === 'string' 
              ? errData.error 
              : JSON.stringify(errData.error)
          } else if (errData?.message) {
            detailedError = errData.message
          }
        } catch {
          // 解析失败，使用原始错误信息
        }
      }
      
      // 尝试从 error 对象直接提取 error 字段
      if (error.error && !detailedError.includes(error.error)) {
        detailedError = typeof error.error === 'string' ? error.error : JSON.stringify(error.error)
      }
      
      ElMessage.error(t('chat.sendFailed', { error: detailedError }))
      messages.value.push({
        role: 'assistant',
        content: `**${t('chat.requestFailed')}**\n\n${detailedError}`
      })
    } finally {
      loading.value = false
      scrollToBottom()
    }
  }
}

// 根据缓存状态返回标签颜色类型
function getCacheTagType(meta: any): string {
  const status = meta.cacheStatus || ''
  if (status === 'HIT-SPLIT-ALL') return 'success'
  if (status === 'HIT-SPLIT-PARTIAL') return 'warning'
  if (status.startsWith('HIT')) return 'success'
  return 'info'
}

// 根据缓存状态返回标签显示文字
function getCacheTagText(meta: any): string {
  const status = meta.cacheStatus || ''
  if (status === 'HIT-SPLIT-ALL') {
    return meta.splitTotal > 0 ? t('chat.cacheAllHit', { hits: meta.splitHits, total: meta.splitTotal }) : t('chat.cacheAllHitSimple')
  }
  if (status === 'HIT-SPLIT-PARTIAL') {
    return meta.splitTotal > 0 ? t('chat.cachePartialHit', { hits: meta.splitHits, total: meta.splitTotal }) : t('chat.cachePartialHitSimple')
  }
  if (status.startsWith('HIT')) return t('chat.cacheHit')
  return t('chat.cacheRealtime')
}

function clearMessages() {
  messages.value = []
  ElMessage.success(t('chat.chatCleared'))
}

function formatMessage(content: string) {
  // 简单的Markdown格式化
  return content
    .replace(/\n/g, '<br>')
    .replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>')
    .replace(/\*(.*?)\*/g, '<em>$1</em>')
    .replace(/`(.*?)`/g, '<code>$1</code>')
    .replace(/```([\s\S]*?)```/g, '<pre class="code-block">$1</pre>')
}

</script>

<style scoped>
.chat {
  display: flex;
  flex-direction: column;
  height: 100%;
  flex: 1;
}

.header {
  margin-bottom: var(--spacing-md);
  flex-shrink: 0;
}

/* 左侧配置面板 */
.chat-body {
  flex: 1;
  display: flex;
  flex-direction: row;
  gap: var(--spacing-md);
  min-height: 0;
}

.params-panel {
  width: 210px;
  flex-shrink: 0;
  background: white;
  border-radius: var(--radius-lg);
  border: 1px solid var(--color-gray-200);
  padding: var(--spacing-md);
  display: flex;
  flex-direction: column;
  gap: var(--spacing-sm);
  overflow-y: auto;
}

.params-panel-title {
  font-size: 0.72rem;
  font-weight: 600;
  color: var(--color-gray-400);
  text-transform: uppercase;
  letter-spacing: 0.06em;
  flex-shrink: 0;
}

.params-divider {
  height: 1px;
  background: var(--color-gray-100);
  margin: var(--spacing-xs) 0;
  flex-shrink: 0;
}

.params-panel-hint {
  font-size: 0.75rem;
  line-height: 1.45;
  color: var(--color-gray-400);
  margin: 0 0 var(--spacing-sm) 0;
  flex-shrink: 0;
}

.params-panel-hint strong {
  color: var(--color-gray-500);
  font-weight: 600;
}

.params-panel-hint--tight {
  margin-top: 6px;
  margin-bottom: 0;
}

.label-with-tip {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.label-help-icon {
  cursor: help;
  color: var(--color-gray-400);
  vertical-align: middle;
}

.label-help-icon:hover {
  color: var(--color-gray-600);
}

/* 模式单选组 */
.mode-radio-group {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.mode-radio-group :deep(.el-radio) {
  height: 30px;
  display: flex;
  align-items: center;
  margin-right: 0;
  padding: 0 4px;
  border-radius: var(--radius-sm);
  transition: background 0.15s;
}

.mode-radio-group :deep(.el-radio:hover) {
  background: var(--color-gray-50);
}

.mode-radio-group :deep(.el-radio__label) {
  font-size: 0.85rem;
  padding-left: 6px;
}

/* 指定方式单选组 */
.source-radio-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.source-radio-group :deep(.el-radio) {
  height: auto;
  display: flex;
  align-items: flex-start;
  margin-right: 0;
  padding: 8px;
  border-radius: var(--radius-sm);
  background: var(--color-gray-50);
  transition: all 0.15s;
}

.source-radio-group :deep(.el-radio:hover) {
  background: var(--color-gray-100);
}

.source-radio-group :deep(.el-radio.is-checked) {
  background: #ecf5ff;
  border-color: var(--color-primary);
}

.source-radio-group :deep(.el-radio__label) {
  font-size: 0.85rem;
  padding-left: 8px;
  white-space: normal;
  line-height: 1.4;
}

.radio-label {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.radio-icon {
  font-size: 1.1rem;
}

.access-section {
  margin-bottom: 10px;
}

.access-section-hint {
  font-size: 0.75rem;
  color: var(--color-gray-500);
  margin: 0 0 8px;
  line-height: 1.4;
}

.access-section-advanced {
  padding-top: 8px;
  border-top: 1px dashed var(--color-gray-200);
}

.access-badge {
  margin-left: 6px;
  vertical-align: middle;
}

.default-pipeline-card {
  margin-top: 10px;
  padding: 10px;
  border-radius: var(--radius-sm);
  background: #fff;
  border: 1px solid var(--color-gray-200);
}

.default-pipeline-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.default-pipeline-label {
  font-size: 0.75rem;
  color: var(--color-gray-500);
}

.default-pipeline-name {
  font-size: 0.85rem;
  font-weight: 600;
  color: var(--color-gray-800);
}

.default-pipeline-id {
  margin-top: 4px;
  font-size: 0.75rem;
  color: var(--color-gray-500);
}

.default-pipeline-actions {
  display: flex;
  gap: 12px;
  margin-top: 8px;
}

.mono {
  font-family: 'Courier New', monospace;
}

/* 模式说明区块 */
.mode-explanation {
  background: #f8f9fa;
  border-radius: var(--radius-md);
  padding: 12px;
  margin-top: 8px;
}

.explanation-title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 0.85rem;
  font-weight: 600;
  color: var(--color-gray-700);
  margin-bottom: 8px;
}

.explanation-text {
  font-size: 0.8rem;
  color: var(--color-gray-600);
  line-height: 1.5;
  margin: 0;
}

.explanation-text code {
  background: #e9ecef;
  padding: 2px 6px;
  border-radius: 4px;
  font-family: 'Courier New', monospace;
  font-size: 0.85em;
}

.example-block {
  margin-top: 10px;
}

.example-label {
  font-size: 0.75rem;
  color: var(--color-gray-500);
  margin-bottom: 4px;
}

.example-code {
  background: #2d3748;
  color: #e2e8f0;
  padding: 10px;
  border-radius: 6px;
  font-family: 'Courier New', monospace;
  font-size: 0.8rem;
  margin: 0;
  overflow-x: auto;
}

/* 关键字指南 */
.keyword-guide {
  background: linear-gradient(135deg, #667eea15 0%, #764ba215 100%);
  border: 1px solid #667eea30;
  border-radius: var(--radius-lg);
  padding: 20px;
  margin-bottom: 20px;
}

.keyword-guide-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 1rem;
  font-weight: 600;
  color: #4f46e5;
  margin-bottom: 12px;
}

.keyword-guide-content {
  font-size: 0.85rem;
  color: var(--color-gray-600);
  line-height: 1.6;
}

.keyword-guide-content p {
  margin: 0 0 12px 0;
}

.keyword-guide-content code {
  background: #e9ecef;
  padding: 2px 6px;
  border-radius: 4px;
  font-family: 'Courier New', monospace;
  font-size: 0.9em;
  color: #e63946;
}

.keyword-table {
  background: white;
  border-radius: var(--radius-md);
  overflow: hidden;
  margin: 12px 0;
  font-size: 0.8rem;
}

.keyword-row {
  display: grid;
  grid-template-columns: 60px 1fr 1fr;
  gap: 10px;
  padding: 8px 12px;
  border-bottom: 1px solid var(--color-gray-100);
}

.keyword-row:last-child {
  border-bottom: none;
}

.keyword-row.header-row {
  background: var(--color-gray-100);
  font-weight: 600;
  color: var(--color-gray-600);
}

.keyword-row code {
  background: #fee2e2;
  color: #dc2626;
  padding: 2px 6px;
  border-radius: 4px;
  font-family: 'Courier New', monospace;
}

.keyword-example {
  background: white;
  border-radius: var(--radius-md);
  padding: 12px;
  margin-top: 12px;
}

.keyword-example .example-label {
  font-size: 0.75rem;
  color: var(--color-gray-500);
  margin-bottom: 4px;
}

.keyword-example .example-code {
  background: #2d3748;
  color: #e2e8f0;
  padding: 10px;
  border-radius: 6px;
  font-family: 'Courier New', monospace;
  font-size: 0.8rem;
  margin: 4px 0;
}

.keyword-example .example-arrow {
  text-align: center;
  color: var(--color-gray-400);
  font-size: 1.2rem;
  padding: 4px 0;
}

.keyword-example .example-hint {
  font-size: 0.75rem;
  color: #10b981;
  margin-top: 6px;
}

/* HTTP 请求预览 */
.http-preview {
  background: #1a1a2e;
  border-radius: var(--radius-lg);
  padding: 16px;
  margin-bottom: 20px;
}

.http-preview-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 0.9rem;
  font-weight: 600;
  color: #e2e8f0;
  margin-bottom: 12px;
}

.http-preview-hint {
  font-size: 0.78rem;
  color: #94a3b8;
  line-height: 1.5;
  margin: 0 0 12px;
}

.http-preview-hint code {
  background: rgba(255, 255, 255, 0.08);
  padding: 1px 4px;
  border-radius: 3px;
}

.http-preview-content {
  font-family: 'Courier New', monospace;
  font-size: 0.8rem;
}

.http-line {
  color: #e2e8f0;
  margin-bottom: 12px;
}

.http-label {
  color: #f97316;
  font-weight: bold;
  margin-right: 10px;
}

.http-value {
  color: #22d3ee;
}

.http-section {
  margin-bottom: 12px;
}

.http-section-title {
  color: #a78bfa;
  margin-bottom: 6px;
  font-size: 0.75rem;
}

.http-code {
  background: #0f0f1a;
  color: #a5f3fc;
  padding: 12px;
  border-radius: 6px;
  margin: 0;
  white-space: pre-wrap;
  overflow-x: auto;
  line-height: 1.6;
}

.http-code :deep(.http-header-key) {
  color: #f472b6;
}

.http-code :deep(.http-header-value) {
  color: #a5f3fc;
}

/* 消息中的模式标签 */
.message-mode-tag {
  margin-top: 6px;
}

/* 关键字按钮 */
.keyword-buttons {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.keyword-tag {
  cursor: pointer;
  font-weight: 600;
  font-family: 'Courier New', monospace;
  transition: all 0.15s;
}

.keyword-tag:hover {
  transform: scale(1.1);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.15);
}

/* 流水线选项描述 */
.pipeline-option-desc {
  margin-left: 8px;
  font-size: 0.75rem;
  color: var(--color-gray-400);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 140px;
  display: inline-block;
  vertical-align: middle;
}

/* 面板内表单 */
.panel-form {
  width: 100%;
}

.panel-form :deep(.el-form-item) {
  margin-bottom: var(--spacing-sm);
}

.panel-form :deep(.el-form-item:last-child) {
  margin-bottom: 0;
}

.panel-form :deep(.el-form-item__label) {
  font-size: 0.75rem;
  color: var(--color-gray-500);
  line-height: 1.4;
  padding-bottom: 3px;
  height: auto;
}

.panel-form :deep(.el-slider) {
  width: 100%;
}

.param-value {
  display: block;
  text-align: right;
  font-size: 0.78rem;
  font-weight: 600;
  color: var(--color-primary);
  margin-top: 1px;
}

.chat-container {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.chat-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  background: white;
  border-radius: var(--radius-lg);
  border: 1px solid var(--color-gray-200);
  overflow: hidden;
  min-height: 0;
}

.chat-messages {
  flex: 1;
  overflow-y: auto;
  padding: var(--spacing-lg);
  display: flex;
  flex-direction: column;
  gap: var(--spacing-lg);
  min-height: 0;
}

.message {
  display: flex;
  gap: var(--spacing-md);
  align-items: flex-start;
}

.message.user {
  flex-direction: row-reverse;
}

.message-avatar {
  width: 40px;
  height: 40px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.message.user .message-avatar {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  color: white;
}

.message.assistant .message-avatar {
  background: linear-gradient(135deg, #10b981 0%, #059669 100%);
  color: white;
}

.message-content {
  flex: 1;
  max-width: 70%;
}

.message.user .message-content {
  text-align: right;
}

.message-role {
  font-size: 0.75rem;
  color: var(--color-gray-500);
  margin-bottom: var(--spacing-xs);
}

.message-text {
  background: var(--color-gray-100);
  padding: var(--spacing-md);
  border-radius: var(--radius-md);
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-word;
}

.message.user .message-text {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  color: white;
}

.message.assistant .message-text {
  background: var(--color-gray-100);
}

.message-text :deep(.code-block) {
  background: var(--color-gray-800);
  color: var(--color-gray-100);
  padding: var(--spacing-md);
  border-radius: var(--radius-sm);
  overflow-x: auto;
  margin: var(--spacing-sm) 0;
}

.message-text :deep(code) {
  background: var(--color-gray-200);
  padding: 2px 6px;
  border-radius: 4px;
  font-family: 'Courier New', monospace;
  font-size: 0.9em;
}

.typing-indicator {
  display: inline-block;
  animation: typing 1.5s infinite;
}

@keyframes typing {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}

.message-meta {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 4px;
  margin-top: var(--spacing-sm);
  padding-top: var(--spacing-xs);
  border-top: 1px dashed var(--color-gray-200);
  font-size: 0.75rem;
  color: var(--color-gray-400);
  line-height: 1.4;
}

.meta-icon {
  font-size: 12px;
  color: var(--color-gray-400);
  flex-shrink: 0;
}

.meta-item {
  display: inline-flex;
  align-items: center;
  gap: 3px;
}

.meta-label {
  font-size: 0.7rem;
  color: var(--color-gray-400);
}

.meta-value {
  color: var(--color-gray-600);
  font-weight: 500;
  max-width: 200px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.meta-cache-type {
  text-transform: uppercase;
  font-size: 0.68rem;
  letter-spacing: 0.03em;
}

.meta-sep {
  color: var(--color-gray-300);
  font-size: 0.7rem;
}

.streaming-cursor {
  display: inline-block;
  margin-left: 2px;
  animation: blink 1s infinite;
  color: var(--color-primary);
  font-weight: bold;
}

@keyframes blink {
  0%, 49% { opacity: 1; }
  50%, 100% { opacity: 0; }
}

.empty-messages {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  padding: var(--spacing-xl);
  min-height: 300px;
}

.empty-icon {
  font-size: 4rem;
  margin-bottom: var(--spacing-md);
  opacity: 0.5;
}

.empty-title {
  font-size: 1.25rem;
  font-weight: 600;
  color: var(--color-gray-700);
  margin-bottom: var(--spacing-sm);
}

.empty-description {
  font-size: 0.875rem;
  color: var(--color-gray-500);
  max-width: 400px;
}

.chat-input-area {
  padding: var(--spacing-lg);
  border-top: 1px solid var(--color-gray-200);
  background: white;
  flex-shrink: 0;
}

.chat-input-area :deep(.el-textarea__inner) {
  resize: none;
}

.input-actions {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-top: var(--spacing-md);
}

@media (max-width: 768px) {
  .message-content {
    max-width: 85%;
  }

  /* 小屏幕下配置面板折叠到底部 */
  .chat-body {
    flex-direction: column-reverse;
  }

  .params-panel {
    width: 100%;
    flex-direction: row;
    flex-wrap: wrap;
    gap: var(--spacing-lg);
  }

  .mode-radio-group {
    flex-direction: row;
    flex-wrap: wrap;
    gap: var(--spacing-sm);
  }

  .params-divider {
    width: 100%;
  }
}

/* ── 流水线调试面板 ────────────────────────────────────────────── */
.pipeline-debug-panel {
  margin-top: 8px;
  border: 1px solid var(--el-border-color-lighter, #e4e7ed);
  border-radius: 6px;
  overflow: hidden;
  font-size: 12px;
}

.pipeline-debug-toggle {
  display: flex;
  align-items: center;
  gap: 4px;
  width: 100%;
  padding: 5px 10px;
  background: var(--el-fill-color-extra-light, #f9fafb);
  border: none;
  cursor: pointer;
  color: var(--el-text-color-secondary, #909399);
  font-size: 12px;
  text-align: left;
  transition: background 0.15s;

  &:hover {
    background: var(--el-fill-color-light, #f5f7fa);
    color: var(--el-text-color-regular, #606266);
  }
}

.debug-toggle-icon {
  font-size: 10px;
  width: 12px;
  flex-shrink: 0;
}

.debug-toggle-hint {
  color: var(--el-text-color-placeholder, #c0c4cc);
  margin-left: 2px;
}

.pipeline-debug-body {
  border-top: 1px solid var(--el-border-color-lighter, #e4e7ed);
  background: var(--el-bg-color-page, #f2f3f5);
  max-height: 400px;
  overflow-y: auto;
}

.pipeline-debug-json {
  margin: 0;
  padding: 12px 14px;
  font-family: 'JetBrains Mono', 'Fira Code', 'Consolas', monospace;
  font-size: 11.5px;
  line-height: 1.6;
  color: var(--el-text-color-primary, #303133);
  white-space: pre;
  word-break: normal;
  overflow-x: auto;
}
</style>
