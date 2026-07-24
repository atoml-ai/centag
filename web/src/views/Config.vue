<template>
  <div class="config">
    <div class="header config-header">
      <div class="header-left">
        <h1 class="page-title">系统配置</h1>
        <p class="page-description">精简设置面：服务概览、代理默认、缓存与韧性；本机代理请走「接入」</p>
      </div>
      <div class="header-actions">
        <el-button :loading="loading" @click="load">
          <el-icon><Refresh /></el-icon>
          刷新
        </el-button>
        <el-button type="primary" :loading="saving" @click="save">
          <el-icon><Check /></el-icon>
          保存配置
        </el-button>
      </div>
    </div>

    <div class="config-tabs-wrapper" v-loading="loading">
      <el-tabs v-model="activeTab" type="border-card" class="config-tabs">
        <!-- 服务概览 -->
        <el-tab-pane label="服务概览" name="overview">
          <el-form label-width="120px">
            <el-divider content-position="left">监听信息（只读）</el-divider>
            <el-form-item label="服务主机">
              <el-input :model-value="config.server.host" disabled />
              <div class="form-tip">由安装 / 环境变量决定；此处保存不会热重启监听</div>
            </el-form-item>
            <el-form-item label="服务端口">
              <el-input-number :model-value="config.server.port" disabled style="width: 200px" />
              <div class="form-tip">改端口请修改启动配置（如 LLM_PROXY_SERVER_PORT）后重启</div>
            </el-form-item>

            <el-divider content-position="left">相关入口</el-divider>
            <div class="link-cards">
              <el-card shadow="never" class="link-card" @click="router.push('/dashboard')">
                <div class="link-title">默认后端 / 模型</div>
                <div class="link-desc">在首页后端面板设置，并显示于底部状态栏</div>
              </el-card>
              <el-card
                v-if="showSystemProxyLink"
                shadow="never"
                class="link-card"
                @click="router.push('/system-proxy')"
              >
                <div class="link-title">本机系统代理</div>
                <div class="link-desc">PAC / MITM 主入口在「接入 → 系统代理」</div>
              </el-card>
              <el-card shadow="never" class="link-card" @click="router.push('/profile')">
                <div class="link-title">账号与改密</div>
                <div class="link-desc">个人中心修改密码与 API Key</div>
              </el-card>
            </div>
          </el-form>
        </el-tab-pane>

        <!-- 代理默认 -->
        <el-tab-pane label="代理默认" name="proxy">
          <el-form label-width="140px">
            <el-alert
              type="info"
              :closable="false"
              show-icon
              class="section-alert"
              title="与流水线 / 请求头的关系"
              description="日常选路以首页默认流水线与请求头 X-Proxy-Mode 为准；此处为未指定时的全局回落默认。"
            />
            <el-form-item label="启用代理">
              <el-switch v-model="config.proxy.enabled" />
              <div class="form-tip">关闭后网关代理能力不可用（一般保持开启）</div>
            </el-form-item>
            <el-form-item label="默认模式">
              <el-select v-model="config.proxy.default_mode" style="width: 280px">
                <el-option label="指定默认后端" value="direct-backend" />
                <el-option label="智能调度" value="smart-scheduling" />
                <el-option label="透明模式（不注入 system prompt）" value="transparent-proxy" />
              </el-select>
              <div class="form-tip">
                对应 X-Proxy-Mode：direct-backend / smart-scheduling / transparent-proxy；固定出站跳板请用 fixed-egress（#j）
              </div>
            </el-form-item>
            <el-form-item label="代理超时">
              <el-input-number v-model="config.proxy.timeout" :min="1" :max="300" style="width: 200px" />
              <span class="unit">秒</span>
            </el-form-item>
            <el-form-item label="默认后端">
              <el-button @click="router.push('/dashboard')">去首页设置</el-button>
              <div class="form-tip">系统默认后端与模型在首页「后端」面板维护</div>
            </el-form-item>
          </el-form>
        </el-tab-pane>

        <!-- 缓存 -->
        <el-tab-pane label="缓存" name="cache">
          <el-form label-width="150px">
            <el-card shadow="never" class="config-card">
              <template #header><div class="card-header">功能开关</div></template>
              <el-form-item label="启用缓存">
                <el-switch v-model="config.cache.enabled" />
              </el-form-item>
              <el-form-item label="启用缓存命中">
                <el-switch v-model="config.cache.enable_cache_read" :disabled="!config.cache.enabled" />
                <div class="form-tip">关闭后不走命中流程，直接转发上游</div>
              </el-form-item>
            </el-card>

            <template v-if="config.cache.enabled">
              <el-card shadow="never" class="config-card">
                <template #header><div class="card-header">缓存写入</div></template>
                <el-form-item label="写入模式">
                  <el-radio-group v-model="cacheWriteMode">
                    <el-radio value="normal">正常缓存（可命中）</el-radio>
                    <el-radio value="save_only">仅保存（浏览用）</el-radio>
                    <el-radio value="disabled">关闭写入</el-radio>
                  </el-radio-group>
                </el-form-item>
              </el-card>

              <el-card v-if="cacheWriteMode === 'normal'" shadow="never" class="config-card">
                <template #header><div class="card-header">匹配策略</div></template>
                <el-form-item label="缓存策略">
                  <el-select v-model="config.cache.strategy" style="width: 220px">
                    <el-option label="仅精确匹配" value="exact" />
                    <el-option label="仅语义匹配" value="semantic" />
                    <el-option label="混合策略" value="hybrid" />
                  </el-select>
                </el-form-item>
                <el-form-item label="默认过期时间">
                  <el-input-number v-model="config.cache.default_ttl" :min="0" :max="86400" style="width: 200px" />
                  <span class="unit">秒</span>
                  <div class="form-tip">0 表示永不过期</div>
                </el-form-item>
              </el-card>

              <el-collapse v-if="cacheWriteMode === 'normal'" class="advanced-collapse">
                <el-collapse-item title="高级：语义缓存与嵌入" name="semantic">
                  <el-alert
                    type="info"
                    :closable="false"
                    show-icon
                    class="section-alert"
                    title="与对话「生成温度」无关"
                    description="语义命中阈值是向量相似度下限，不是大模型采样温度。"
                  />
                  <el-form-item label="自动向量化">
                    <el-switch v-model="config.cache.semantic.enable_auto_embedding" />
                  </el-form-item>
                  <el-form-item label="语义命中阈值">
                    <el-slider
                      v-model="config.cache.semantic.threshold"
                      :min="0"
                      :max="1"
                      :step="0.01"
                      :format-tooltip="(val: number) => (val * 100).toFixed(0) + '%'"
                      style="width: 280px"
                    />
                    <span class="slider-val">{{ (config.cache.semantic.threshold * 100).toFixed(0) }}%</span>
                  </el-form-item>
                  <el-form-item label="返回结果数">
                    <el-input-number v-model="config.cache.semantic.top_k" :min="1" :max="10" style="width: 160px" />
                  </el-form-item>
                  <el-form-item label="距离算法">
                    <el-select v-model="config.cache.semantic.distance_type" style="width: 200px">
                      <el-option label="余弦相似度" value="cosine" />
                      <el-option label="欧氏距离" value="euclidean" />
                      <el-option label="点积" value="dot_product" />
                    </el-select>
                  </el-form-item>
                  <el-divider content-position="left">嵌入模型</el-divider>
                  <el-form-item label="启用嵌入">
                    <el-switch v-model="config.embedding.enabled" />
                  </el-form-item>
                  <el-form-item label="嵌入后端">
                    <el-select
                      v-model="config.embedding.backend_id"
                      style="width: 300px"
                      :disabled="!config.embedding.enabled"
                      placeholder="选择后端"
                    >
                      <el-option
                        v-for="backend in enabledBackends"
                        :key="backend.id"
                        :label="`${backend.name} (${backend.type})`"
                        :value="backend.id"
                      />
                    </el-select>
                  </el-form-item>
                  <el-form-item label="向量化模型">
                    <el-select
                      v-model="config.embedding.model"
                      style="width: 300px"
                      :disabled="!config.embedding.backend_id || !config.embedding.enabled"
                      :loading="loadingEmbeddingModels"
                      filterable
                      allow-create
                    >
                      <el-option v-for="model in embeddingModels" :key="model" :label="model" :value="model" />
                    </el-select>
                  </el-form-item>
                </el-collapse-item>

                <el-collapse-item title="高级：容量与清理" name="capacity">
                  <el-form-item label="最大缓存数">
                    <el-input-number v-model="config.cache.max_cache_size" :min="0" :max="1000000" style="width: 200px" />
                    <div class="form-tip">0 表示无限制</div>
                  </el-form-item>
                  <el-form-item label="清理间隔">
                    <el-input-number v-model="config.cache.cleanup_interval" :min="0" :max="3600" style="width: 200px" />
                    <span class="unit">秒</span>
                  </el-form-item>
                </el-collapse-item>

                <el-collapse-item title="高级：输出拆分（实验）" name="qa_split">
                  <el-form-item label="启用拆分">
                    <el-switch v-model="config.qa_split.enabled" />
                    <div class="form-tip">将 LLM 输出拆为 Q&A 对写入缓存（实验能力）</div>
                  </el-form-item>
                  <template v-if="config.qa_split.enabled">
                    <el-form-item label="拆分后端">
                      <el-select v-model="config.qa_split.backend_id" style="width: 300px">
                        <el-option
                          v-for="backend in enabledBackends"
                          :key="backend.id"
                          :label="`${backend.name} (${backend.type})`"
                          :value="backend.id"
                        />
                      </el-select>
                    </el-form-item>
                    <el-form-item label="拆分模型">
                      <el-select
                        v-model="config.qa_split.model"
                        style="width: 300px"
                        :disabled="!config.qa_split.backend_id"
                        :loading="loadingQAModels"
                        filterable
                        allow-create
                      >
                        <el-option v-for="model in qaModels" :key="model" :label="model" :value="model" />
                      </el-select>
                    </el-form-item>
                  </template>
                </el-collapse-item>
              </el-collapse>
            </template>

            <p class="hint-line">
              缓存条目监控请到
              <el-link type="primary" @click="router.push('/cache')">更多 → 存储配置 → 缓存监控</el-link>
            </p>
          </el-form>
        </el-tab-pane>

        <!-- 韧性 -->
        <el-tab-pane label="韧性" name="resilience">
          <el-form label-width="140px">
            <el-alert
              type="warning"
              :closable="false"
              show-icon
              class="section-alert"
              title="与「降级策略」不同"
              description="本页配置 HTTP 重试与熔断（代理韧性）。模型/后端链路的降级策略请到「更多 → 系统 → 降级策略」。"
            />
            <div class="link-cards" style="margin-bottom: 16px">
              <el-card shadow="never" class="link-card" @click="router.push('/fallback-policies')">
                <div class="link-title">打开降级策略</div>
                <div class="link-desc">配置后端/模型失败时的链路切换</div>
              </el-card>
            </div>

            <el-divider content-position="left">HTTP 重试</el-divider>
            <el-form-item label="可重试状态码">
              <el-select
                v-model="config.proxy.retryable_status_codes"
                multiple
                filterable
                allow-create
                default-first-option
                style="width: 400px"
                placeholder="如 429, 500, 502, 503, 504"
              >
                <el-option
                  v-for="code in [400, 401, 403, 404, 408, 429, 500, 502, 503, 504]"
                  :key="code"
                  :label="String(code)"
                  :value="code"
                />
              </el-select>
              <div class="form-tip">上游返回这些状态码时触发重试（热生效）</div>
            </el-form-item>
            <el-form-item label="超时可重试">
              <el-switch v-model="config.proxy.timeout_retryable" />
            </el-form-item>
            <el-form-item label="网络错误可重试">
              <el-switch v-model="config.proxy.network_retryable" />
            </el-form-item>

            <el-divider content-position="left">熔断器</el-divider>
            <el-form-item label="失败阈值">
              <el-input-number
                v-model="config.proxy.circuit_breaker.failure_threshold"
                :min="1"
                :max="20"
                style="width: 150px"
              />
            </el-form-item>
            <el-form-item label="恢复成功数">
              <el-input-number
                v-model="config.proxy.circuit_breaker.success_threshold"
                :min="1"
                :max="10"
                style="width: 150px"
              />
            </el-form-item>
            <el-form-item label="熔断持续时间">
              <el-input-number
                v-model="config.proxy.circuit_breaker.timeout_sec"
                :min="10"
                :max="300"
                style="width: 150px"
              />
              <span class="unit">秒</span>
            </el-form-item>
            <el-form-item label="滑动窗口">
              <el-input-number
                v-model="config.proxy.circuit_breaker.window_sec"
                :min="10"
                :max="300"
                style="width: 150px"
              />
              <span class="unit">秒</span>
            </el-form-item>
            <el-form-item label="429 加重系数">
              <el-input-number
                v-model="config.proxy.circuit_breaker.rate_limit_weight"
                :min="1"
                :max="10"
                style="width: 150px"
              />
            </el-form-item>
          </el-form>
        </el-tab-pane>
      </el-tabs>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Refresh, Check } from '@element-plus/icons-vue'
import { getConfig, saveConfig, getBackends } from '@/api'
import { getBackendModels } from '@/api/backend'
import { useEdition } from '@/composables/useEdition'
import { useAuthStore } from '@/stores/auth'
import { getCapabilities } from '@/utils/capabilities'

const router = useRouter()
const authStore = useAuthStore()
const { edition } = useEdition()

const loading = ref(false)
const saving = ref(false)
const activeTab = ref('overview')
const backends = ref<any[]>([])
const embeddingModels = ref<string[]>([])
const qaModels = ref<string[]>([])
const loadingEmbeddingModels = ref(false)
const loadingQAModels = ref(false)
const isInitialLoad = ref(false)

const showSystemProxyLink = computed(
  () => getCapabilities(edition.value, authStore.isAdmin).localProxy
)
const enabledBackends = computed(() => backends.value.filter((b: any) => b.enabled))

/** 与后端 ProxyConfig.default_mode 一致；旧版页面曾使用 direct/cache/fallback */
const PROXY_DEFAULT_MODES = ['direct-backend', 'smart-scheduling', 'transparent-proxy'] as const

function normalizeProxyDefaultMode(mode: string | undefined) {
  if (!mode || (PROXY_DEFAULT_MODES as readonly string[]).includes(mode)) return mode
  const legacy: Record<string, string> = {
    direct: 'direct-backend',
    cache: 'smart-scheduling',
    fallback: 'transparent-proxy'
  }
  return legacy[mode] || 'transparent-proxy'
}

const config = ref<any>({
  server: {
    host: '0.0.0.0',
    port: 20060
  },
  proxy: {
    enabled: true,
    default_mode: 'transparent-proxy',
    timeout: 30,
    retryable_status_codes: [429, 500, 502, 503, 504],
    timeout_retryable: true,
    network_retryable: true,
    circuit_breaker: {
      failure_threshold: 3,
      success_threshold: 2,
      timeout_sec: 60,
      window_sec: 60,
      rate_limit_weight: 2
    }
  },
  system_proxy: {
    enabled: false,
    listen_port: 8080,
    pac_enabled: false
  },
  host_proxy: {
    enabled: false,
    http_port: 8081,
    https_port: 8082
  },
  qa_split: {
    enabled: false,
    backend_id: '',
    model: '',
    prompt: '',
    temperature: 0.3,
    max_tokens: 2000,
    timeout: 120
  },
  // 仍随 GET/PUT 完整读写，UI 已迁出（流水线节点侧配置）
  question_split: {
    enabled: false,
    fast_split_enabled: true,
    llm_split_enabled: false,
    split_strategy: 'rule',
    backend_id: '',
    model: '',
    synthesis_strategy: 'concat',
    synthesis_backend_id: '',
    synthesis_model: '',
    max_sub_questions: 5,
    timeout: 3,
    complexity_threshold: 0.2
  },
  embedding: {
    enabled: true,
    provider: 'ollama',
    backend_id: 'ollama-local',
    model: 'bge-m3:latest',
    base_url: 'http://localhost:21434'
  },
  cache: {
    enabled: true,
    enable_cache_read: true,
    enable_cache_write: true,
    save_only_mode: false,
    strategy: 'semantic',
    default_ttl: 3600,
    max_cache_size: 0,
    cleanup_interval: 300,
    semantic: {
      enable_auto_embedding: true,
      threshold: 0.8,
      top_k: 5,
      distance_type: 'cosine'
    }
  }
})

const cacheWriteMode = computed({
  get: () => {
    if (!config.value.cache.enabled) return 'disabled'
    if (config.value.cache.save_only_mode) return 'save_only'
    if (config.value.cache.enable_cache_write === false) return 'disabled'
    return 'normal'
  },
  set: (val: string) => {
    if (val === 'save_only') {
      config.value.cache.save_only_mode = true
      config.value.cache.enable_cache_write = true
    } else if (val === 'disabled') {
      config.value.cache.save_only_mode = false
      config.value.cache.enable_cache_write = false
    } else {
      config.value.cache.save_only_mode = false
      config.value.cache.enable_cache_write = true
    }
  }
})

watch(
  () => config.value.embedding.backend_id,
  async (newBackendId, oldBackendId) => {
    if (isInitialLoad.value) return
    if (newBackendId && newBackendId !== oldBackendId) {
      await loadEmbeddingModels(newBackendId)
      config.value.embedding.model = ''
    } else if (!newBackendId) {
      embeddingModels.value = []
      config.value.embedding.model = ''
    }
  }
)

watch(
  () => config.value.qa_split.backend_id,
  async (newBackendId, oldBackendId) => {
    if (isInitialLoad.value) return
    if (newBackendId && newBackendId !== oldBackendId) {
      await loadQAModels(newBackendId)
      config.value.qa_split.model = ''
    } else if (!newBackendId) {
      qaModels.value = []
      config.value.qa_split.model = ''
    }
  }
)

async function loadBackends() {
  try {
    const data = await getBackends()
    backends.value = Array.isArray(data) ? data : []
  } catch (error: any) {
    console.error('Failed to load backends:', error)
  }
}

async function loadEmbeddingModels(backendId: string) {
  if (!backendId) return
  loadingEmbeddingModels.value = true
  try {
    const models = await getBackendModels(backendId, 'embedding')
    embeddingModels.value = models && models.length > 0 ? models : []
  } catch (error: any) {
    console.error('Failed to load embedding models:', error)
    embeddingModels.value = []
  } finally {
    loadingEmbeddingModels.value = false
  }
}

async function loadQAModels(backendId: string) {
  if (!backendId) return
  loadingQAModels.value = true
  try {
    const models = await getBackendModels(backendId, 'chat')
    qaModels.value = models && models.length > 0 ? models : []
  } catch (error: any) {
    console.error('Failed to load QA models:', error)
    qaModels.value = []
  } finally {
    loadingQAModels.value = false
  }
}

async function load() {
  loading.value = true
  try {
    const data = await getConfig()
    isInitialLoad.value = true
    config.value = {
      server: { ...config.value.server, ...data.server },
      proxy: { ...config.value.proxy, ...data.proxy },
      system_proxy: { ...config.value.system_proxy, ...data.system_proxy },
      host_proxy: { ...config.value.host_proxy, ...data.host_proxy },
      qa_split: { ...config.value.qa_split, ...data.qa_split },
      question_split: { ...config.value.question_split, ...(data.question_split || {}) },
      embedding: { ...config.value.embedding, ...data.embedding },
      cache: {
        ...config.value.cache,
        ...data.cache,
        semantic: { ...config.value.cache.semantic, ...(data.cache?.semantic || {}) }
      }
    }
    config.value.proxy.default_mode = normalizeProxyDefaultMode(config.value.proxy.default_mode)
    if (config.value.embedding.backend_id) {
      await loadEmbeddingModels(config.value.embedding.backend_id)
    }
    if (config.value.qa_split.backend_id) {
      await loadQAModels(config.value.qa_split.backend_id)
    }
    await nextTick()
    isInitialLoad.value = false
  } catch (error: any) {
    console.error('Failed to load config:', error)
    ElMessage.error('加载失败: ' + (error.message || '未知错误'))
  } finally {
    loading.value = false
  }
}

async function save() {
  saving.value = true
  try {
    await saveConfig(config.value)
    ElMessage.success('配置已保存')
  } catch (error: any) {
    console.error('Failed to save config:', error)
    ElMessage.error('保存失败: ' + (error.message || '未知错误'))
  } finally {
    saving.value = false
  }
}

onMounted(async () => {
  await loadBackends()
  await load()
})
</script>

<style scoped>
.config-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
}

.header-left {
  flex: 1;
}

.header-actions {
  display: flex;
  gap: 8px;
  align-items: center;
  padding-top: 4px;
}

.config-tabs-wrapper {
  max-width: 1100px;
  margin: 0 auto;
  padding: 0 var(--spacing-sm);
}

.config-tabs {
  width: 100%;
}

.section-alert {
  margin-bottom: 16px;
}

.form-tip {
  font-size: 0.75rem;
  color: var(--color-gray-500);
  margin-top: var(--spacing-xs);
}

.unit {
  margin-left: var(--spacing-sm);
  color: var(--color-gray-600);
}

.slider-val {
  margin-left: 12px;
  color: #606266;
}

:deep(.el-divider__text) {
  font-weight: 600;
  color: var(--color-gray-700);
}

:deep(.el-tabs--border-card) {
  border: 1px solid var(--color-gray-200);
  box-shadow: none;
}

:deep(.el-tabs__content) {
  padding: var(--spacing-lg);
}

.config-card {
  margin-bottom: 16px;
}

.config-card :deep(.el-card__header) {
  padding: 12px 16px;
  background-color: var(--el-fill-color-light);
  border-bottom: 1px solid var(--el-border-color-light);
}

.config-card :deep(.el-card__body) {
  padding: 16px;
}

.card-header {
  font-weight: 600;
  color: var(--color-gray-700);
}

.advanced-collapse {
  margin-top: 8px;
  border: none;
}

.link-cards {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 12px;
}

.link-card {
  cursor: pointer;
  transition: border-color 0.15s ease, box-shadow 0.15s ease;
}

.link-card:hover {
  border-color: var(--el-color-primary-light-5);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
}

.link-title {
  font-weight: 600;
  color: var(--color-gray-800);
  margin-bottom: 4px;
}

.link-desc {
  font-size: 0.75rem;
  color: var(--color-gray-500);
  line-height: 1.4;
}

.hint-line {
  margin-top: 12px;
  font-size: 0.8rem;
  color: var(--color-gray-500);
}
</style>
