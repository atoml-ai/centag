<template>
  <div class="config-page">
    <div class="page-header">
      <div class="header-left">
        <h1 class="page-title">{{ t('config.title') }}</h1>
        <p class="page-description">{{ t('config.description') }}</p>
      </div>
      <div v-if="showConfigActions" class="header-actions">
        <el-button :loading="loading" @click="load">
          <el-icon><Refresh /></el-icon>
          {{ t('config.refresh') }}
        </el-button>
        <el-button type="primary" :loading="saving" @click="save">
          <el-icon><Check /></el-icon>
          {{ t('config.saveConfig') }}
        </el-button>
      </div>
    </div>

    <div class="config-layout" v-loading="loading">
      <aside class="config-nav" aria-label="Config sections">
        <nav class="nav-list">
          <button
            v-for="item in navItems"
            :key="item.id"
            type="button"
            class="nav-item"
            :class="{ 'is-active': activeSection === item.id }"
            @click="selectSection(item.id)"
          >
            <el-icon class="nav-icon"><component :is="item.icon" /></el-icon>
            <span class="nav-label">{{ t(item.labelKey) }}</span>
          </button>
        </nav>
      </aside>

      <main class="config-main">
        <!-- 服务概览 -->
        <section v-show="activeSection === 'overview'" class="section-panel overview-panel">
          <header class="section-header">
            <h2 class="section-title">{{ t('config.serviceOverview') }}</h2>
            <p class="section-desc">{{ t('config.serviceOverviewDesc') }}</p>
          </header>

          <div class="overview-block">
            <div class="overview-block-head">
              <span class="overview-block-title">{{ t('config.listenInfo') }}</span>
              <el-tag size="small" type="info" effect="plain">{{ t('config.readOnly') }}</el-tag>
            </div>
            <div class="listen-row">
              <div class="listen-field">
                <label class="field-label">{{ t('config.serviceHost') }}</label>
                <el-input :model-value="config.server.host" disabled class="listen-input" />
              </div>
              <div class="listen-field listen-field--port">
                <label class="field-label">{{ t('config.servicePort') }}</label>
                <el-input :model-value="String(config.server.port ?? '')" disabled class="listen-input" />
              </div>
              <p class="form-tip listen-tip">{{ t('config.listenInfoTip') }}</p>
            </div>
          </div>

          <div class="overview-block">
            <div class="overview-block-head">
              <span class="overview-block-title">{{ t('config.responseBehavior') }}</span>
            </div>
            <div class="setting-row">
              <div class="setting-copy">
                <div class="setting-label">{{ t('config.responseTrace') }}</div>
                <p class="form-tip">{{ t('config.responseTraceDesc') }}</p>
              </div>
              <el-switch v-model="config.proxy.response_trace_banner" />
            </div>
          </div>

          <div class="overview-block">
            <div class="overview-block-head">
              <span class="overview-block-title">{{ t('config.relatedEntries') }}</span>
            </div>
            <div class="link-cards">
              <button type="button" class="link-card" @click="router.push('/dashboard')">
                <div class="link-copy">
                  <div class="link-title">{{ t('config.defaultBackendModelPipeline') }}</div>
                  <div class="link-desc">{{ t('config.defaultBackendModelPipelineDesc') }}</div>
                </div>
                <el-icon class="link-arrow"><ArrowRight /></el-icon>
              </button>
              <button
                v-if="showSystemProxyLink"
                type="button"
                class="link-card"
                @click="router.push('/system-proxy')"
              >
                <div class="link-copy">
                  <div class="link-title">{{ t('config.systemProxyEntry') }}</div>
                  <div class="link-desc">{{ t('config.systemProxyEntryDesc') }}</div>
                </div>
                <el-icon class="link-arrow"><ArrowRight /></el-icon>
              </button>
              <button
                type="button"
                class="link-card"
                @click="router.push({ path: '/profile', query: { section: 'password' } })"
              >
                <div class="link-copy">
                  <div class="link-title">{{ t('config.accountAndPassword') }}</div>
                  <div class="link-desc">{{ t('config.accountAndPasswordDesc') }}</div>
                </div>
                <el-icon class="link-arrow"><ArrowRight /></el-icon>
              </button>
            </div>
          </div>
        </section>

        <!-- HTTP 重试与熔断 -->
        <section v-show="activeSection === 'http'" class="section-panel">
          <header class="section-header">
            <h2 class="section-title">{{ t('config.httpRetryAndCircuitBreaker') }}</h2>
          </header>

          <el-form label-width="140px">
            <el-alert
              type="info"
              :closable="false"
              show-icon
              class="section-alert"
              :title="t('config.proxyResilience')"
              :description="t('config.proxyResilienceDesc')"
            />

            <el-divider content-position="left">{{ t('config.httpRetry') }}</el-divider>
            <el-form-item :label="t('config.retryableStatusCodes')">
              <el-select
                v-model="config.proxy.retryable_status_codes"
                multiple
                filterable
                allow-create
                default-first-option
                style="width: 400px"
                :placeholder="t('config.retryableStatusCodesPlaceholder')"
              >
                <el-option
                  v-for="code in [400, 401, 403, 404, 408, 429, 500, 502, 503, 504]"
                  :key="code"
                  :label="String(code)"
                  :value="code"
                />
              </el-select>
              <div class="form-tip">{{ t('config.retryableStatusCodesTip') }}</div>
            </el-form-item>
            <el-form-item :label="t('config.timeoutRetry')">
              <el-switch v-model="config.proxy.timeout_retryable" />
            </el-form-item>
            <el-form-item :label="t('config.networkErrorRetry')">
              <el-switch v-model="config.proxy.network_retryable" />
            </el-form-item>

            <el-divider content-position="left">{{ t('config.circuitBreaker') }}</el-divider>
            <el-form-item :label="t('config.failureThreshold')">
              <el-input-number
                v-model="config.proxy.circuit_breaker.failure_threshold"
                :min="1"
                :max="20"
                style="width: 150px"
              />
            </el-form-item>
            <el-form-item :label="t('config.recoverySuccessCount')">
              <el-input-number
                v-model="config.proxy.circuit_breaker.success_threshold"
                :min="1"
                :max="10"
                style="width: 150px"
              />
            </el-form-item>
            <el-form-item :label="t('config.circuitBreakerDuration')">
              <el-input-number
                v-model="config.proxy.circuit_breaker.timeout_sec"
                :min="10"
                :max="300"
                style="width: 150px"
              />
              <span class="unit">{{ t('config.seconds') }}</span>
            </el-form-item>
            <el-form-item :label="t('config.slidingWindow')">
              <el-input-number
                v-model="config.proxy.circuit_breaker.window_sec"
                :min="10"
                :max="300"
                style="width: 150px"
              />
              <span class="unit">{{ t('config.seconds') }}</span>
            </el-form-item>
            <el-form-item :label="t('config.backoff429')">
              <el-input-number
                v-model="config.proxy.circuit_breaker.rate_limit_weight"
                :min="1"
                :max="10"
                style="width: 150px"
              />
            </el-form-item>
          </el-form>
        </section>

        <!-- 降级策略 -->
        <section v-show="activeSection === 'fallback'" class="section-panel">
          <header class="section-header">
            <h2 class="section-title">{{ t('config.fallbackPolicy') }}</h2>
            <p class="section-desc">{{ t('config.fallbackPolicyTip') }}</p>
          </header>
          <FallbackPolicyView embedded />
        </section>

        <!-- 部署与数据 -->
        <section v-show="activeSection === 'deployment'" class="section-panel">
          <header class="section-header">
            <h2 class="section-title">{{ t('config.deploymentTitle') }}</h2>
            <p class="section-desc">{{ t('config.deploymentDesc') }}</p>
          </header>

          <el-alert
            type="warning"
            :closable="false"
            show-icon
            class="section-alert"
            :title="t('config.deploymentRestartTip')"
          />

          <div class="overview-block">
            <div class="overview-block-head">
              <span class="overview-block-title">{{ t('config.deploymentDatabase') }}</span>
            </div>
            <el-form label-width="150px">
              <el-form-item :label="t('config.deploymentDbDriver')">
                <el-radio-group v-model="config.deployment.db_driver">
                  <el-radio value="sqlite">{{ t('config.deploymentDbSqlite') }}</el-radio>
                  <el-radio value="postgresql">{{ t('config.deploymentDbPostgresql') }}</el-radio>
                </el-radio-group>
              </el-form-item>
              <template v-if="config.deployment.db_driver === 'postgresql'">
                <el-form-item :label="t('config.deploymentPgHost')">
                  <el-input v-model="config.deployment.pg_host" style="width: 320px" />
                </el-form-item>
                <el-form-item :label="t('config.deploymentPgPort')">
                  <el-input v-model="config.deployment.pg_port" style="width: 160px" />
                </el-form-item>
                <el-form-item :label="t('config.deploymentPgUser')">
                  <el-input v-model="config.deployment.pg_user" style="width: 240px" />
                </el-form-item>
                <el-form-item :label="t('config.deploymentPgPassword')">
                  <el-input
                    v-model="config.deployment.pg_password"
                    type="password"
                    show-password
                    style="width: 240px"
                    :placeholder="config.deployment.pg_password === '***' ? t('config.deploymentPgPasswordMasked') : ''"
                  />
                </el-form-item>
                <el-form-item :label="t('config.deploymentPgDb')">
                  <el-input v-model="config.deployment.pg_db" style="width: 240px" />
                </el-form-item>
              </template>
            </el-form>
          </div>

          <div class="overview-block">
            <div class="overview-block-head">
              <span class="overview-block-title">{{ t('config.deploymentUninstall') }}</span>
            </div>
            <div class="setting-row">
              <div class="setting-copy">
                <div class="setting-label">{{ t('config.deploymentCleanData') }}</div>
                <p class="form-tip">{{ t('config.deploymentCleanDataDesc') }}</p>
              </div>
              <el-switch v-model="config.deployment.clean_data_on_uninstall" />
            </div>
          </div>
        </section>

        <!-- 系统更新 -->
        <section v-if="isTeamAdmin" v-show="activeSection === 'system-update'" class="section-panel">
          <div class="su-section-hd">
            <span class="su-section-icon upload-color"><el-icon><Upload /></el-icon></span>
            <div>
              <div class="su-section-title">{{ t('config.systemUpdateTitle') }}</div>
              <div class="su-section-sub">{{ t('config.systemUpdateDesc') }}</div>
            </div>
            <div style="margin-left:auto">
              <el-button :loading="suLoadingHistory" @click="suLoadHistory">
                <el-icon><Refresh /></el-icon>{{ t('config.systemUpdateRefresh') }}
              </el-button>
            </div>
          </div>

          <el-row :gutter="24">
            <el-col :xs="24">
              <el-card shadow="never" class="su-card" style="margin-bottom:16px">
                <template #header>
                  <div class="su-card-hd">
                    <span class="su-section-icon upload-color"><el-icon><Download /></el-icon></span>
                    <div>
                      <div class="su-section-title">{{ t('config.systemUpdateOnlineTitle') }}</div>
                      <div class="su-section-sub">{{ t('config.systemUpdateOnlineDesc') }}</div>
                    </div>
                    <div style="margin-left:auto">
                      <el-button :loading="suChecking" @click="suCheckUpdate">
                        <el-icon><Search /></el-icon>{{ t('config.systemUpdateCheck') }}
                      </el-button>
                    </div>
                  </div>
                </template>
                <div v-if="suCheckResult">
                  <div style="display:flex;gap:16px;flex-wrap:wrap;margin-bottom:12px">
                    <div>{{ t('config.systemUpdateCurrent') }}: <code class="ver-badge">{{ suCheckResult.current_version || '—' }}</code></div>
                    <div>{{ t('config.systemUpdateLatest') }}: <code class="ver-badge">{{ suCheckResult.remote_version || '—' }}</code></div>
                  </div>
                  <el-alert
                    :type="suCheckResult.update_available ? 'success' : 'info'"
                    :closable="false"
                    show-icon
                    :title="suCheckResult.update_available ? t('config.systemUpdateAvailable') : (suCheckResult.message || t('config.systemUpdateUpToDate'))"
                  />
                  <el-button
                    type="primary"
                    size="large"
                    :loading="suApplyingRemote"
                    :disabled="!suCheckResult.update_available"
                    @click="suApplyRemote"
                    style="width:100%;margin-top:16px"
                  >
                    {{ suApplyingRemote ? t('config.systemUpdateApplying') : t('config.systemUpdateApplyRemote') }}
                  </el-button>
                </div>
                <div v-else style="color:var(--el-text-color-secondary);font-size:.875rem">
                  {{ t('config.systemUpdateCheckHint') }}
                </div>
              </el-card>

              <el-card shadow="never" class="su-card">
                <template #header>
                  <div class="su-card-hd">
                    <span class="su-section-icon upload-color"><el-icon><Upload /></el-icon></span>
                    <div>
                      <div class="su-section-title">{{ t('config.systemUpdateUploadTitle') }}</div>
                      <div class="su-section-sub">{{ t('config.systemUpdateUploadDesc') }}</div>
                    </div>
                  </div>
                </template>

                <el-upload
                  ref="suUploadRef"
                  drag
                  :auto-upload="false"
                  :limit="1"
                  accept=".tar.gz,.tgz"
                  :on-change="suHandleFileChange"
                  :on-exceed="suHandleExceed"
                  :on-remove="() => (suSelectedFile = null)"
                >
                  <el-icon style="font-size:48px;color:#667eea;opacity:.75"><UploadFilled /></el-icon>
                  <div style="margin-top:12px;color:var(--el-text-color-regular)">
                    {{ t('config.systemUpdateDragTip') }}
                  </div>
                  <div style="margin-top:6px;font-size:12px;color:var(--el-text-color-secondary)">
                    {{ t('config.systemUpdateFileHint') }}
                  </div>
                </el-upload>

                <el-button
                  type="primary"
                  size="large"
                  :loading="suUpdating"
                  :disabled="!suSelectedFile"
                  @click="suDoUpdate"
                  style="width:100%;margin-top:16px"
                >
                  {{ suUpdating ? t('config.systemUpdateUpdating') : t('config.systemUpdateStart') }}
                </el-button>

                <div v-if="suUpdateLog" class="su-log">
                  <div class="su-log-hd"><el-icon><Document /></el-icon> {{ t('config.systemUpdateOutput') }}</div>
                  <pre class="su-log-body">{{ suUpdateLog }}</pre>
                </div>
              </el-card>
            </el-col>

            <el-col :xs="24">
              <el-card shadow="never" class="su-card">
                <template #header>
                  <div class="su-card-hd">
                    <span class="su-section-icon history-color"><el-icon><Clock /></el-icon></span>
                    <div>
                      <div class="su-section-title">{{ t('config.systemUpdateHistoryTitle') }}</div>
                      <div class="su-section-sub">{{ t('config.systemUpdateHistoryDesc') }}</div>
                    </div>
                  </div>
                </template>

                <el-table
                  :data="suHistory"
                  v-loading="suLoadingHistory"
                  :empty-text="t('config.systemUpdateNoRecords')"
                  stripe
                  size="large"
                >
                  <el-table-column :label="t('config.systemUpdateVersion')" prop="version" min-width="100">
                    <template #default="{ row }">
                      <code class="ver-badge">{{ row.version || '—' }}</code>
                    </template>
                  </el-table-column>
                  <el-table-column :label="t('config.systemUpdateTime')" prop="time" min-width="140" />
                  <el-table-column :label="t('config.systemUpdateStatus')" width="90" align="center">
                    <template #default="{ row }">
                      <el-tag :type="row.success ? 'success' : 'danger'" size="small" effect="light">
                        {{ row.success ? t('config.systemUpdateSuccess') : t('config.systemUpdateFailed') }}
                      </el-tag>
                    </template>
                  </el-table-column>
                  <el-table-column :label="t('config.systemUpdateAction')" width="80" align="center">
                    <template #default="{ row }">
                      <el-dropdown trigger="click">
                        <el-button type="primary" link>
                          <el-icon><MoreFilled /></el-icon>
                        </el-button>
                        <template #dropdown>
                          <el-dropdown-menu>
                            <el-dropdown-item v-if="row.can_rollback" @click="suRollback(row)">
                              <el-icon><RefreshLeft /></el-icon>{{ t('config.systemUpdateRollback') }}
                            </el-dropdown-item>
                            <el-dropdown-item :divided="row.can_rollback" @click="suDeleteRecord(row)">
                              <el-icon><Delete /></el-icon>{{ t('config.systemUpdateDelete') }}
                            </el-dropdown-item>
                          </el-dropdown-menu>
                        </template>
                      </el-dropdown>
                    </template>
                  </el-table-column>
                </el-table>
              </el-card>
            </el-col>
          </el-row>
        </section>
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch, type Component } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh, Check, Monitor, Connection, Switch, ArrowRight, Upload, UploadFilled, Clock, Document, RefreshLeft, Delete, MoreFilled, Search, Download, Box } from '@element-plus/icons-vue'
import { getConfig, saveConfig } from '@/api'
import { useEdition } from '@/composables/useEdition'
import { useAuthStore } from '@/stores/auth'
import { getCapabilities } from '@/utils/capabilities'
import api from '@/api/index'
import FallbackPolicyView from '@/views/FallbackPolicy.vue'
import type { UploadInstance, UploadFile } from 'element-plus'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const { edition } = useEdition()
const { t } = useI18n()

type ConfigSection = 'overview' | 'http' | 'fallback' | 'deployment' | 'system-update'

const isAdmin = computed(() => authStore.isAdmin)
const isTeamAdmin = computed(() => edition.value === 'team' && authStore.isAdmin)

const navItems = computed(() => {
  const items: Array<{ id: ConfigSection; labelKey: string; icon: Component }> = [
    { id: 'overview', labelKey: 'config.navOverview', icon: Monitor },
    { id: 'http', labelKey: 'config.navHttp', icon: Connection },
    { id: 'fallback', labelKey: 'config.navFallback', icon: Switch },
  ]
  if (authStore.isAdmin) {
    items.push({ id: 'deployment', labelKey: 'config.navDeployment', icon: Box })
  }
  if (isTeamAdmin.value) {
    items.push({ id: 'system-update', labelKey: 'config.navSystemUpdate', icon: Upload })
  }
  return items
})

const loading = ref(false)
const saving = ref(false)
const activeSection = ref<ConfigSection>('overview')

const showSystemProxyLink = computed(
  () => getCapabilities(edition.value, authStore.isAdmin).localProxy
)
/** 降级策略子页有独立保存，隐藏顶部「保存配置」避免误解 */
const showConfigActions = computed(() => activeSection.value !== 'fallback' && activeSection.value !== 'system-update')

function selectSection(id: ConfigSection) {
  activeSection.value = id
  if (id === 'overview') {
    router.replace({ query: { tab: 'overview' } })
  } else if (id === 'http') {
    router.replace({ query: { tab: 'resilience', sub: 'http' } })
  } else if (id === 'system-update') {
    router.replace({ query: { tab: 'resilience', sub: 'system-update' } })
    void suCheckUpdate()
    void suLoadHistory()
  } else if (id === 'deployment') {
    router.replace({ query: { tab: 'resilience', sub: 'deployment' } })
  } else {
    router.replace({ query: { tab: 'resilience', sub: 'fallback' } })
  }
}

function applyRouteQuery() {
  const tab = String(route.query.tab || '')
  const sub = String(route.query.sub || '')
  if (sub === 'fallback') {
    activeSection.value = 'fallback'
    return
  }
  if (sub === 'deployment') {
    activeSection.value = 'deployment'
    return
  }
  if (sub === 'system-update') {
    activeSection.value = 'system-update'
    void suCheckUpdate()
    void suLoadHistory()
    return
  }
  if (sub === 'http' || tab === 'resilience') {
    activeSection.value = 'http'
    return
  }
  if (tab === 'overview' || !tab) {
    activeSection.value = 'overview'
  }
}

/**
 * 完整配置对象：UI 仅编辑概览/韧性字段，其余（缓存、拆分等）随 GET/PUT 原样读写，避免保存时被清空。
 */
const config = ref<any>({
  server: {
    host: '0.0.0.0',
    port: 20060,
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
      rate_limit_weight: 2,
    },
  },
  system_proxy: {
    enabled: false,
    listen_port: 8080,
    pac_enabled: false,
  },
  host_proxy: {
    enabled: false,
    http_port: 8081,
    https_port: 8082,
  },
  qa_split: {
    enabled: false,
    backend_id: '',
    model: '',
    prompt: '',
    temperature: 0.3,
    max_tokens: 2000,
    timeout: 120,
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
    complexity_threshold: 0.2,
  },
  embedding: {
    enabled: true,
    provider: 'ollama',
    backend_id: 'ollama-local',
    model: 'bge-m3:latest',
    base_url: 'http://localhost:21434',
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
      distance_type: 'cosine',
    },
  },
  deployment: {
    clean_data_on_uninstall: false,
    db_driver: 'sqlite',
    pg_host: 'localhost',
    pg_port: '5432',
    pg_user: 'postgres',
    pg_password: '',
    pg_db: 'centag',
  },
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
        semantic: { ...config.value.cache.semantic, ...(data.cache?.semantic || {}) },
      },
      deployment: { ...config.value.deployment, ...(data.deployment || {}) },
    }
    if (typeof config.value.proxy.response_trace_banner !== 'boolean') {
      config.value.proxy.response_trace_banner = false
    }
  } catch (error: any) {
    console.error('Failed to load config:', error)
    ElMessage.error(t('config.loadFailed') + ': ' + (error.message || t('config.unknownError')))
  } finally {
    loading.value = false
  }
}

async function save() {
  saving.value = true
  try {
    await saveConfig(config.value)
    ElMessage.success(t('config.configSaved'))
  } catch (error: any) {
    console.error('Failed to save config:', error)
    ElMessage.error(t('config.saveFailed') + ': ' + (error.message || t('config.unknownError')))
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

// ── 系统更新 ──────────────────────────────────────────────────────────────────

const suUploadRef = ref<UploadInstance>()
const suSelectedFile = ref<File | null>(null)
const suUpdating = ref(false)
const suUpdateLog = ref('')
const suHistory = ref<Array<{ version: string; time: string; success: boolean; can_rollback: boolean; history_file?: string }>>([])
const suLoadingHistory = ref(false)
const suChecking = ref(false)
const suApplyingRemote = ref(false)
const suCheckResult = ref<{
  current_version?: string
  remote_version?: string
  update_available?: boolean
  message?: string
  asset_name?: string
} | null>(null)

function suHandleFileChange(file: UploadFile) { suSelectedFile.value = file.raw ?? null }
function suHandleExceed() { ElMessage.warning(t('config.systemUpdateOnlyOne')) }

async function suCheckUpdate() {
  suChecking.value = true
  try {
    const data = await api.get('/api/v1/system/update/check')
    suCheckResult.value = data?.check || data || null
  } catch (e: any) {
    suCheckResult.value = null
    ElMessage.error(e.message || t('config.systemUpdateCheckFailed'))
  } finally {
    suChecking.value = false
  }
}

async function suApplyRemote() {
  if (!suCheckResult.value?.update_available) return
  try {
    await ElMessageBox.confirm(
      t('config.systemUpdateApplyConfirm', { version: suCheckResult.value.remote_version || '' }),
      t('config.systemUpdateApplyTitle'),
      { confirmButtonText: t('config.systemUpdateApplyRemote'), cancelButtonText: t('config.cancel'), type: 'warning' },
    )
  } catch {
    return
  }
  suApplyingRemote.value = true
  suUpdateLog.value = ''
  try {
    const resp = await api.post('/api/v1/system/update/apply-remote')
    suUpdateLog.value = typeof resp === 'string' ? resp : JSON.stringify(resp, null, 2)
    if (resp?.success === false) {
      ElMessage.warning(resp.message || t('config.systemUpdateFailedMsg'))
    } else {
      ElMessage.success(t('config.systemUpdateApplySuccess'))
    }
    suLoadHistory()
    suCheckUpdate()
  } catch (e: any) {
    ElMessage.error(e.message || t('config.systemUpdateFailedMsg'))
    suUpdateLog.value = e.message || t('config.systemUpdateFailedMsg')
  } finally {
    suApplyingRemote.value = false
  }
}

async function suDoUpdate() {
  if (!suSelectedFile.value) return
  suUpdating.value = true; suUpdateLog.value = ''
  const fd = new FormData()
  fd.append('package', suSelectedFile.value)
  try {
    const resp = await api.post('/api/v1/system/update', fd, { headers: { 'Content-Type': 'multipart/form-data' } })
    suUpdateLog.value = typeof resp === 'string' ? resp : JSON.stringify(resp, null, 2)
    ElMessage.success(t('config.systemUpdateSuccessMsg'))
    suUploadRef.value?.clearFiles(); suSelectedFile.value = null
    suLoadHistory()
  } catch (e: any) {
    ElMessage.error(e.message || t('config.systemUpdateFailedMsg')); suUpdateLog.value = e.message || t('config.systemUpdateFailedMsg')
  } finally { suUpdating.value = false }
}

async function suLoadHistory() {
  suLoadingHistory.value = true
  try {
    const data = await api.get('/api/v1/system/update/history')
    const rawList = Array.isArray(data) ? data : (data?.history || [])
    suHistory.value = rawList.map((row: any) => ({
      version: row.version || '—',
      time: formatSuTime(row.start_time || row.end_time || ''),
      success: !!row.success,
      can_rollback: !!row.success && !!row.history_file,
      history_file: row.history_file,
    }))
  } catch { suHistory.value = [] }
  finally { suLoadingHistory.value = false }
}

async function suRollback(item: typeof suHistory.value[number]) {
  if (!item.history_file) { ElMessage.error(t('config.systemUpdateMissingFile')); return }
  try {
    await ElMessageBox.confirm(t('config.systemUpdateRollbackConfirm', { version: item.version }), t('config.systemUpdateRollbackTitle'), { confirmButtonText: t('config.systemUpdateRollbackBtn'), cancelButtonText: t('config.cancel'), type: 'warning' })
    const fd = new FormData()
    fd.append('history_file', item.history_file)
    await api.post('/api/v1/system/rollback', fd)
    ElMessage.success(t('config.systemUpdateRollbackSuccess')); suLoadHistory()
  } catch (e: any) { if (e !== 'cancel' && e?.message) ElMessage.error(e.message) }
}

async function suDeleteRecord(item: typeof suHistory.value[number]) {
  if (!item.history_file) { ElMessage.error(t('config.systemUpdateMissingFile')); return }
  try {
    await ElMessageBox.confirm(t('config.systemUpdateDeleteConfirm'), t('config.systemUpdateDeleteTitle'), { confirmButtonText: t('config.delete'), cancelButtonText: t('config.cancel'), type: 'warning' })
    const fd = new FormData()
    fd.append('history_file', item.history_file)
    await api.post('/api/v1/system/delete-update', fd)
    ElMessage.success(t('config.systemUpdateDeleteSuccess')); suLoadHistory()
  } catch (e: any) { if (e !== 'cancel' && e?.message) ElMessage.error(e.message) }
}

function formatSuTime(raw: string): string {
  if (!raw) return '—'
  const dt = new Date(raw)
  if (Number.isNaN(dt.getTime())) return raw
  return dt.toLocaleString()
}
</script>

<style scoped>
.config-page {
  max-width: 1100px;
  margin: 0 auto;
  width: 100%;
  padding: 0 0 24px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
  margin-bottom: 24px;
}

.header-left {
  flex: 1;
  min-width: 0;
}

.page-title {
  margin: 0;
  font-size: 1.5rem;
  font-weight: 600;
}

.page-description {
  margin: 6px 0 0;
  color: var(--el-text-color-secondary);
  font-size: 0.875rem;
}

.header-actions {
  display: flex;
  gap: 8px;
  align-items: center;
  flex-shrink: 0;
  padding-top: 4px;
}

.config-layout {
  display: grid;
  grid-template-columns: 220px minmax(0, 1fr);
  gap: 24px;
  align-items: start;
}

.config-nav {
  position: sticky;
  top: 16px;
}

.nav-list {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 8px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 10px;
  background: var(--el-bg-color);
}

.nav-item {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  border: none;
  background: transparent;
  text-align: left;
  padding: 10px 12px;
  border-radius: 8px;
  cursor: pointer;
  color: var(--el-text-color-regular);
  font-size: 0.875rem;
  transition: background 0.15s, color 0.15s;
}

.nav-item:hover {
  background: var(--el-fill-color-light);
  color: var(--el-text-color-primary);
}

.nav-item.is-active {
  background: var(--el-color-primary-light-9);
  color: var(--el-color-primary);
  font-weight: 600;
}

.nav-icon {
  font-size: 16px;
  flex-shrink: 0;
}

.config-main {
  min-width: 0;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 10px;
  background: var(--el-bg-color);
  padding: 24px;
}

.section-header {
  margin-bottom: 20px;
}

.section-title {
  margin: 0;
  font-size: 1.125rem;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.section-desc {
  margin: 6px 0 0;
  font-size: 0.8125rem;
  color: var(--el-text-color-secondary);
}

.section-alert {
  margin-bottom: 16px;
}

.overview-panel {
  max-width: 760px;
}

.overview-block {
  margin-bottom: 16px;
  padding: 16px 18px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 10px;
  background: var(--el-fill-color-blank);
}

.overview-block-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 14px;
}

.overview-block-title {
  font-size: 0.875rem;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.listen-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 140px;
  column-gap: 16px;
  row-gap: 10px;
  align-items: start;
}

.field-label {
  display: block;
  margin-bottom: 6px;
  font-size: 0.8125rem;
  color: var(--el-text-color-regular);
}

.listen-input {
  width: 100%;
}

.listen-tip {
  grid-column: 1 / -1;
  width: 100%;
  max-width: none;
  margin: 0;
  box-sizing: border-box;
}

.setting-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 20px;
}

.setting-copy {
  flex: 1;
  min-width: 0;
}

.setting-label {
  font-size: 0.875rem;
  font-weight: 500;
  color: var(--el-text-color-primary);
  margin-bottom: 4px;
}

.form-tip {
  font-size: 0.75rem;
  color: var(--el-text-color-secondary);
  margin: 6px 0 0;
  line-height: 1.45;
}

.unit {
  margin-left: 8px;
  color: var(--el-text-color-regular);
}

:deep(.el-divider__text) {
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.link-cards {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.link-card {
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;
  margin: 0;
  padding: 12px 14px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 8px;
  background: var(--el-bg-color);
  text-align: left;
  cursor: pointer;
  transition: border-color 0.15s ease, background 0.15s ease;
}

.link-card:hover {
  border-color: var(--el-color-primary-light-5);
  background: var(--el-color-primary-light-9);
}

.link-copy {
  flex: 1;
  min-width: 0;
}

.link-title {
  font-weight: 600;
  font-size: 0.875rem;
  color: var(--el-text-color-primary);
  margin-bottom: 2px;
}

.link-desc {
  font-size: 0.75rem;
  color: var(--el-text-color-secondary);
  line-height: 1.4;
}

.link-arrow {
  flex-shrink: 0;
  color: var(--el-text-color-secondary);
}

@media (max-width: 768px) {
  .page-header {
    flex-direction: column;
  }

  .config-layout {
    grid-template-columns: 1fr;
  }

  .config-nav {
    position: static;
  }

  .nav-list {
    flex-direction: row;
    overflow-x: auto;
    gap: 4px;
  }

  .nav-item {
    white-space: nowrap;
    flex-shrink: 0;
  }

  .config-main {
    padding: 16px;
  }

  .listen-row {
    grid-template-columns: 1fr;
  }

  .listen-field--port {
    max-width: 160px;
  }

  .overview-panel {
    max-width: none;
  }
}

/* ── 系统更新 ──────────────────────────────────────────────────────────────── */
.su-section-hd {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 20px;
}

.su-card { width: 100%; }

.su-card-hd {
  display: flex;
  align-items: center;
  gap: 12px;
}

.su-section-icon {
  width: 36px;
  height: 36px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 18px;
  flex-shrink: 0;
}

.upload-color {
  background: rgba(102,126,234,.12);
  color: #667eea;
}

.history-color {
  background: rgba(16,185,129,.12);
  color: #10b981;
}

.su-section-title {
  font-size: .9375rem;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.su-section-sub {
  font-size: .8125rem;
  color: var(--el-text-color-secondary);
  margin-top: 2px;
}

.su-log {
  margin-top: 16px;
  border: 1px solid var(--el-border-color-light);
  border-radius: 8px;
  overflow: hidden;
}

.su-log-hd {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 14px;
  background: var(--el-fill-color-light);
  font-size: .8125rem;
  font-weight: 500;
  color: var(--el-text-color-secondary);
  border-bottom: 1px solid var(--el-border-color-light);
}

.su-log-body {
  padding: 12px 14px;
  margin: 0;
  font-size: .8125rem;
  font-family: 'Menlo', 'Monaco', 'Consolas', monospace;
  white-space: pre-wrap;
  word-break: break-all;
  max-height: 200px;
  overflow-y: auto;
  color: var(--el-text-color-regular);
}

.ver-badge {
  font-family: monospace;
  font-size: .8125rem;
  background: var(--el-fill-color-light);
  padding: 2px 8px;
  border-radius: 4px;
}
</style>
