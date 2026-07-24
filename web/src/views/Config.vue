<template>
  <div class="config">
    <div class="header config-header">
      <div class="header-left">
        <h1 class="page-title">系统配置</h1>
        <p class="page-description">服务概览与韧性（HTTP 重试/熔断 + 降级策略）；默认后端/模型在首页设置</p>
      </div>
      <div v-if="showConfigActions" class="header-actions">
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

            <el-divider content-position="left">响应行为</el-divider>
            <el-form-item label="响应追踪">
              <el-switch v-model="config.proxy.response_trace_banner" />
              <div class="form-tip">开启后在 AI 回复前附加流水线、后端、模型与降级信息（热生效）</div>
            </el-form-item>

            <el-divider content-position="left">相关入口</el-divider>
            <div class="link-cards">
              <el-card shadow="never" class="link-card" @click="router.push('/dashboard')">
                <div class="link-title">默认后端 / 模型 / 流水线</div>
                <div class="link-desc">在首页面板设置，并显示于底部状态栏</div>
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

        <!-- 韧性：HTTP 重试/熔断 + 降级策略 -->
        <el-tab-pane label="韧性" name="resilience">
          <el-tabs v-model="resilienceSubTab" class="resilience-sub-tabs">
            <el-tab-pane label="HTTP 重试与熔断" name="http">
              <el-form label-width="140px">
                <el-alert
                  type="info"
                  :closable="false"
                  show-icon
                  class="section-alert"
                  title="代理韧性（请求层）"
                  description="配置上游失败时的 HTTP 重试与熔断。模型/后端切换链路请使用旁边的「降级策略」页签。"
                />

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
            <el-tab-pane label="降级策略" name="fallback">
              <FallbackPolicyView embedded />
            </el-tab-pane>
          </el-tabs>
        </el-tab-pane>
      </el-tabs>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Refresh, Check } from '@element-plus/icons-vue'
import { getConfig, saveConfig } from '@/api'
import { useEdition } from '@/composables/useEdition'
import { useAuthStore } from '@/stores/auth'
import { getCapabilities } from '@/utils/capabilities'
import FallbackPolicyView from '@/views/FallbackPolicy.vue'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const { edition } = useEdition()

const loading = ref(false)
const saving = ref(false)
const activeTab = ref('overview')
const resilienceSubTab = ref<'http' | 'fallback'>('http')

const showSystemProxyLink = computed(
  () => getCapabilities(edition.value, authStore.isAdmin).localProxy
)
/** 降级策略子页有独立保存，隐藏顶部「保存配置」避免误解 */
const showConfigActions = computed(
  () => !(activeTab.value === 'resilience' && resilienceSubTab.value === 'fallback')
)

function applyRouteQuery() {
  const tab = String(route.query.tab || '')
  const sub = String(route.query.sub || '')
  if (tab === 'resilience' || tab === 'overview') {
    activeTab.value = tab
  }
  if (sub === 'fallback' || sub === 'http') {
    activeTab.value = 'resilience'
    resilienceSubTab.value = sub
  }
}

/**
 * 完整配置对象：UI 仅编辑概览/韧性字段，其余（缓存、拆分等）随 GET/PUT 原样读写，避免保存时被清空。
 * 缓存 / 代理默认等 Tab 暂隐藏，待能力稳定后再开放。
 */
const config = ref<any>({
  server: {
    host: '0.0.0.0',
    port: 20060
  },
  proxy: {
    enabled: true,
    default_mode: 'transparent-proxy',
    timeout: 30,
    response_trace_banner: false,
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

async function load() {
  loading.value = true
  try {
    const data = await getConfig()
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
    if (typeof config.value.proxy.response_trace_banner !== 'boolean') {
      config.value.proxy.response_trace_banner = false
    }
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

watch(
  () => [route.query.tab, route.query.sub],
  () => applyRouteQuery()
)

onMounted(() => {
  applyRouteQuery()
  void load()
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

.resilience-sub-tabs :deep(.el-tabs__header) {
  margin-bottom: 16px;
}

.resilience-sub-tabs :deep(.el-tabs__content) {
  padding: 0;
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
</style>
