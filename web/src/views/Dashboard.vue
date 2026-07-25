<template>
  <div class="dashboard" :class="rootClass">
    <div class="page-header" :class="{ 'page-header--actions-only': !sections.pageTitle }">
      <div v-if="sections.pageTitle">
        <h1 class="page-title">{{ isPersonal ? $t('nav.dashboard') : $t('nav.overview') }}</h1>
        <p class="page-description">{{ pageDescription }}</p>
      </div>
      <div class="page-header-actions">
        <template v-if="sections.headerActions">
          <el-button type="success" @click="openPipelineChat()">
            <el-icon><ChatDotRound /></el-icon>&nbsp;{{ $t('nav.chat') }}
          </el-button>
          <el-button @click="securityDialogVisible = true">{{ $t('dashboard.config') }}</el-button>
          <el-button @click="handleLogout">{{ $t('dashboard.logout') }}</el-button>
        </template>
      </div>
    </div>

    <div class="dash-main" :class="'dash-main--' + sections.layout">
      <el-card v-if="sections.serviceStatus" class="info-card dash-card dash-card--status">
        <template #header>
          <div class="card-head">
            <el-icon class="card-icon service-color"><Monitor /></el-icon>
            <span>{{ $t('dashboard.serviceStatus') }}</span>
          </div>
        </template>
        <div class="info-rows">
          <template v-if="sections.serviceStatusCompact">
            <div class="personal-status-grid">
              <div class="personal-status-item">
                <span class="info-label">{{ $t('dashboard.running') }}</span>
                <el-tag :type="status.status === 'healthy' ? 'success' : 'danger'" size="small" effect="light">
                  {{ status.status === 'healthy' ? $t('dashboard.running') : $t('dashboard.abnormal') }}
                </el-tag>
              </div>
              <div class="personal-status-item">
                <span class="info-label">{{ $t('dashboard.defaultBackend') }}</span>
                <div class="personal-backend-val">
                  <span class="info-val">{{ defaultBackendSummary?.name || $t('dashboard.notConfigured') }}</span>
                  <el-tag v-if="defaultBackendSummary" size="small" effect="plain" type="info">
                    {{ defaultBackendSummary.type }}
                  </el-tag>
                </div>
              </div>
              <div class="personal-status-item">
                <span class="info-label">{{ $t('dashboard.version') }}</span>
                <span class="info-val mono">{{ status.version || '--' }}</span>
              </div>
              <div class="personal-status-item">
                <span class="info-label">{{ $t('dashboard.runtime') }}</span>
                <span class="info-val">{{ formatUptime(status.uptime) }}</span>
              </div>
            </div>
          </template>
          <template v-else>
            <div class="info-row">
              <span class="info-label">{{ $t('dashboard.running') }}</span>
              <el-tag :type="status.status === 'healthy' ? 'success' : 'danger'" size="small" effect="light">
                {{ status.status === 'healthy' ? '● ' + $t('dashboard.running') : '● ' + $t('dashboard.abnormal') }}
              </el-tag>
            </div>
          </template>

          <template v-if="sections.teamAccessInStatus && (status.external_url || status.status === 'healthy')">
            <div v-if="status.external_url" class="info-row">
              <span class="info-label">{{ $t('dashboard.externalAddress') }}</span>
              <div class="external-url-row">
                <span class="info-val mono external-url-text">{{ status.external_url }}</span>
                <el-tooltip :content="$t('dashboard.copyAddress')" placement="top">
                  <el-icon class="copy-icon" @click="copyExternalUrl"><CopyDocument /></el-icon>
                </el-tooltip>
              </div>
            </div>
            <el-divider style="margin: 8px 0" />
            <div class="section-label" style="margin-bottom: 6px;">{{ $t('dashboard.clientAccess') }}</div>
            <div v-for="ep in apiEndpoints" :key="ep.path" class="endpoint-row">
              <el-tag size="small" :type="ep.tagType || undefined" class="endpoint-tag">{{ ep.label }}</el-tag>
              <span class="endpoint-url mono">{{ baseUrl }}{{ ep.path }}</span>
              <el-tooltip :content="$t('dashboard.copyAddress') + ' ' + ep.label" placement="top">
                <el-icon class="copy-icon" @click="copyEndpoint(ep)"><CopyDocument /></el-icon>
              </el-tooltip>
            </div>
          </template>

          <template v-if="sections.pluginsStorage">
            <el-divider style="margin: 8px 0" />
            <div class="info-row section-title-row">
              <span class="section-label">{{ $t('dashboard.pluginList') }}</span>
              <span class="card-badge">{{ dashboard.plugin_running }} / {{ dashboard.plugin_count }} {{ $t('dashboard.pluginsRunning') }}</span>
            </div>
            <div class="plugin-list">
              <div v-for="p in plugins" :key="p.name" class="plugin-item">
                <div class="plugin-left">
                  <el-tag
                    :type="p.status === 'running' ? 'success' : 'info'"
                    size="small"
                    effect="light"
                    class="plugin-status"
                  >{{ p.status === 'running' ? $t('dashboard.running') : p.status }}</el-tag>
                  <div class="plugin-info">
                    <div class="plugin-name">{{ p.name }}</div>
                    <div class="plugin-meta">{{ p.type }} · v{{ p.version }}</div>
                  </div>
                </div>
              </div>
              <div v-if="!plugins.length" class="empty-tip">{{ $t('dashboard.noPlugins') }}</div>
            </div>

            <el-divider style="margin: 8px 0" />
            <div class="info-row section-title-row">
              <span class="section-label">{{ $t('dashboard.storedInDatabase') }}</span>
              <span class="card-badge">{{ $t('dashboard.items', { count: storages.length + 1 }) }}</span>
            </div>
            <div class="info-section">
              <div class="section-label info-section-head">{{ $t('dashboard.database') }}</div>
              <div class="info-row info-section-row">
                <span class="info-label">{{ $t('dashboard.driverType') }}</span>
                <el-tag :type="getDbDriverType(dashboard.database?.driver)" size="small" effect="light">
                  {{ formatDbDriver(dashboard.database?.driver) }}
                </el-tag>
              </div>
              <div class="info-row info-section-row">
                <span class="info-label">{{ $t('dashboard.connectionStatus') }}</span>
                <el-tag
                  :type="dashboard.database?.status === 'connected' ? 'success' : 'danger'"
                  size="small"
                  effect="light"
                >
                  {{ dashboard.database?.status === 'connected' ? $t('dashboard.connected') : $t('dashboard.notConnected') }}
                </el-tag>
              </div>
              <div class="info-row">
                <span class="info-label">{{ $t('dashboard.connectionAddress') }}</span>
                <span class="info-val mono">{{ dashboard.database?.address || $t('dashboard.unknown') }}</span>
              </div>
            </div>

            <el-divider style="margin: 8px 0" />
            <div class="info-section">
              <div class="section-label info-section-head">{{ $t('dashboard.storageMiddleware') }}</div>
              <div v-for="s in storages" :key="s.name" class="backend-item compact-backend-item">
                <div class="backend-left">
                  <el-tag
                    :type="!s.enabled ? 'info' : s.healthy ? 'success' : 'danger'"
                    size="small"
                    effect="light"
                    class="backend-status"
                  >{{ !s.enabled ? $t('dashboard.disabled') : s.healthy ? $t('dashboard.healthy') : $t('dashboard.abnormal') }}</el-tag>
                  <div class="backend-info">
                    <div class="backend-name">
                      {{ s.name }}
                      <el-tag v-if="s.is_default" type="warning" size="small" effect="plain" style="margin-left:6px">{{ $t('dashboard.defaultTag') }}</el-tag>
                    </div>
                    <div class="backend-meta">{{ s.type }} · {{ s.description }}</div>
                  </div>
                </div>
              </div>
              <div v-if="!storages.length" class="empty-tip compact-empty-tip">{{ $t('dashboard.noStorageConfig') }}</div>
            </div>
          </template>

          <template v-if="sections.proxyControls">
            <el-divider style="margin: 8px 0" />
            <div class="info-row section-title-row">
              <span class="section-label">{{ $t('dashboard.systemProxy') }}</span>
              <el-switch
                v-model="proxyStatus.enabled"
                :loading="proxyToggling"
                size="small"
                @change="toggleSystemProxy"
              />
            </div>
            <div v-if="proxyStatus.enabled" class="info-row">
              <span class="info-label">{{ $t('dashboard.systemProxy') }}</span>
              <el-tag :type="proxyStatus.pac_enabled ? 'success' : 'info'" size="small" effect="plain">
                {{ proxyStatus.pac_enabled ? $t('dashboard.enabled') : $t('dashboard.disabled') }}
              </el-tag>
            </div>
            <div v-if="proxyStatus.pac_domains?.length" class="info-row">
              <span class="info-label">{{ $t('dashboard.proxyDomain') }}</span>
              <span class="info-val">{{ $t('dashboard.items', { count: proxyStatus.pac_domains.length }) }}</span>
            </div>

            <el-divider style="margin: 8px 0" />
            <div class="info-row section-title-row">
              <span class="section-label">{{ $t('dashboard.hostProxy') }}</span>
              <el-switch
                v-model="hostProxy.enabled"
                :loading="hostProxyToggling"
                size="small"
                @change="toggleHostProxy"
              />
            </div>
            <template v-if="hostProxy.enabled">
              <div class="info-row">
                <span class="info-label">{{ $t('dashboard.httpPort') }}</span>
                <span class="info-val mono">{{ hostProxy.http_port }}</span>
              </div>
              <div class="info-row">
                <span class="info-label">{{ $t('dashboard.httpsPort') }}</span>
                <span class="info-val mono">{{ hostProxy.https_port }}</span>
              </div>
            </template>
            <div v-if="hostProxy.domains" class="info-row">
              <span class="info-label">{{ $t('dashboard.proxyDomain') }}</span>
              <span class="info-val">{{ $t('dashboard.items', { count: Object.keys(hostProxy.domains || {}).length }) }}</span>
            </div>
          </template>
        </div>
      </el-card>

      <el-card
        v-if="sections.accessPanel"
        class="info-card access-card dash-card dash-card--access"
        :class="{ 'access-card--compact': sections.accessCompact }"
        :shadow="sections.accessCompact ? 'never' : undefined"
      >
        <ApiAccessPanel :base-url="baseUrl" :compact="sections.accessCompact" />
      </el-card>

      <el-card v-if="sections.backends" class="info-card config-card dash-card dash-card--backends">
        <template #header>
          <div class="card-head">
            <el-icon class="card-icon backend-color"><DataBoard /></el-icon>
            <span>{{ $t('dashboard.backendConfig') }}</span>
            <span class="card-badge">{{ $t('dashboard.items', { count: backends.length }) }}</span>
          </div>
        </template>
        <DashboardBackendList
          ref="backendListRef"
          :backends="backends"
          @backend-updated="patchBackend"
          @refresh="loadBackendsOnly"
        />
      </el-card>

      <el-card v-if="sections.pipelines" class="info-card config-card dash-card dash-card--pipelines">
        <template #header>
          <div class="card-head" :class="{ 'card-head--actions': sections.pipelineCreateButton }">
            <div class="card-head-main">
              <el-icon class="card-icon pipeline-color"><Share /></el-icon>
              <span>{{ $t('dashboard.pipelineConfig') }}</span>
              <span v-if="sections.pipelineCreateButton" class="card-badge">{{ $t('dashboard.items', { count: pipelineCount }) }}</span>
            </div>
            <div v-if="sections.pipelineCreateButton" class="card-actions">
              <el-button size="small" plain @click="pipelinePanelRef?.openImport()">
                {{ $t('dashboard.importBtn') }}
              </el-button>
              <el-button type="primary" size="small" @click="pipelinePanelRef?.openCreate()">
                + {{ $t('dashboard.createPipeline') }}
              </el-button>
            </div>
          </div>
        </template>
        <HomePipelineCard
          ref="pipelinePanelRef"
          @update:count="pipelineCount = $event"
          @test="openPipelineChat"
        />
      </el-card>
    </div>

    <el-card v-if="sections.usageBilling" class="info-card usage-card mt-card">
      <el-collapse v-model="usageCollapse" class="usage-collapse">
        <el-collapse-item name="usage">
          <template #title>
            <div class="card-head usage-collapse-title">
              <el-icon class="card-icon service-color"><TrendCharts /></el-icon>
              <span>{{ $t('dashboard.usageAndSessions') }}</span>
              <span v-if="sections.usageEphemeralHint" class="card-badge">{{ $t('dashboard.processInternal') }}</span>
            </div>
          </template>
          <MinimalUsagePanel
            ref="usagePanelRef"
            :hint="usageHint"
          />
        </el-collapse-item>
      </el-collapse>
    </el-card>

    <el-card v-if="sections.opsStats" class="info-card mt-card stats-card">
      <template #header>
        <div class="card-head">
          <el-icon class="card-icon stats-color"><DataAnalysis /></el-icon>
          <span>{{ $t('dashboard.runStats') }}</span>
        </div>
      </template>
      <div class="stats-grid">
        <div class="stat-cell">
          <div class="stat-icon-wrap request-bg"><el-icon :size="18"><Document /></el-icon></div>
          <div class="stat-num">{{ formatNumber(dashboard.request.total_requests) }}</div>
          <div class="stat-label">{{ $t('dashboard.totalRequests') }}</div>
        </div>
        <div class="stat-cell">
          <div class="stat-icon-wrap hit-bg"><el-icon :size="18"><CircleCheck /></el-icon></div>
          <div class="stat-num">{{ formatNumber(dashboard.request.success_requests) }}</div>
          <div class="stat-label">{{ $t('dashboard.successRequests') }}</div>
        </div>
        <div class="stat-cell">
          <div class="stat-icon-wrap error-bg"><el-icon :size="18"><Warning /></el-icon></div>
          <div class="stat-num">{{ formatNumber(dashboard.request.error_requests) }}</div>
          <div class="stat-label">{{ $t('dashboard.errorCount') }}</div>
        </div>
        <div class="stat-cell">
          <div class="stat-icon-wrap error-bg"><el-icon :size="18"><TrendCharts /></el-icon></div>
          <div class="stat-num">{{ dashboard.request.error_rate_percent?.toFixed(2) ?? '0' }}%</div>
          <div class="stat-label">{{ $t('dashboard.errorRate') }}</div>
        </div>
        <div class="stat-cell">
          <div class="stat-icon-wrap qps-bg"><el-icon :size="18"><Timer /></el-icon></div>
          <div class="stat-num">{{ dashboard.request.qps?.toFixed(2) ?? '0' }}</div>
          <div class="stat-label">QPS</div>
        </div>
        <div class="stat-cell">
          <div class="stat-icon-wrap latency-bg"><el-icon :size="18"><Stopwatch /></el-icon></div>
          <div class="stat-num">{{ dashboard.request.avg_latency_ms ?? 0 }}ms</div>
          <div class="stat-label">{{ $t('dashboard.avgLatency') }}</div>
        </div>
      </div>
      <el-divider style="margin: 10px 0" />
      <div class="stats-grid">
        <div class="stat-cell">
          <div class="stat-icon-wrap hit-bg"><el-icon :size="18"><CircleCheck /></el-icon></div>
          <div class="stat-num">{{ formatNumber(dashboard.cache.hits) }}</div>
          <div class="stat-label">{{ $t('dashboard.cacheHit') }}</div>
        </div>
        <div class="stat-cell">
          <div class="stat-icon-wrap rate-bg"><el-icon :size="18"><TrendCharts /></el-icon></div>
          <div class="stat-num">{{ hitRate }}%</div>
          <div class="stat-label">{{ $t('dashboard.hitRate') }}</div>
        </div>
        <div class="stat-cell">
          <div class="stat-icon-wrap entry-bg"><el-icon :size="18"><Coin /></el-icon></div>
          <div class="stat-num">{{ formatNumber(dashboard.cache.entries) }}</div>
          <div class="stat-label">{{ $t('dashboard.cacheEntries') }}</div>
        </div>
        <div class="stat-cell">
          <div class="stat-icon-wrap request-bg"><el-icon :size="18"><DataLine /></el-icon></div>
          <div class="stat-num">{{ formatNumber(dashboard.cache.misses) }}</div>
          <div class="stat-label">{{ $t('dashboard.missed') }}</div>
        </div>
      </div>
    </el-card>

    <el-card v-if="sections.opsStats && modelStatsList.length" class="info-card mt-card">
      <template #header>
        <div class="card-head">
          <el-icon class="card-icon stats-color"><Cpu /></el-icon>
          <span>{{ $t('dashboard.modelStats') }}</span>
        </div>
      </template>
      <el-table :data="modelStatsList" size="small" style="width: 100%">
        <el-table-column prop="name" :label="$t('dashboard.model')" min-width="120">
          <template #default="{ row }">
            <el-tag size="small" :type="row.error_rate > 10 ? 'danger' : 'success'" effect="light">
              {{ row.name }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="total_requests" :label="$t('dashboard.requests')" align="center" width="100">
          <template #default="{ row }">
            <span class="stat-num-sm">{{ formatNumber(row.total_requests) }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="avg_latency_ms" :label="$t('dashboard.avgLatency')" align="center" width="100">
          <template #default="{ row }">
            <span class="stat-num-sm">{{ row.avg_latency_ms }}ms</span>
          </template>
        </el-table-column>
        <el-table-column :label="$t('dashboard.cacheHit')" align="center" width="100">
          <template #default="{ row }">
            <span class="stat-num-sm">{{ formatNumber(row.cache_hits) }}</span>
          </template>
        </el-table-column>
        <el-table-column :label="$t('dashboard.hitRate')" align="center" width="80">
          <template #default="{ row }">
            {{ row.cache_hit_rate_percent?.toFixed(1) }}%
          </template>
        </el-table-column>
        <el-table-column :label="$t('dashboard.errorRate')" align="center" width="80">
          <template #default="{ row }">
            <span :class="row.error_rate > 10 ? 'text-danger' : ''">{{ row.error_rate?.toFixed(1) }}%</span>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-card v-if="sections.opsStats" class="info-card mt-card">
      <template #header>
        <div class="card-head">
          <el-icon class="card-icon chart-color"><TrendCharts /></el-icon>
          <span>{{ $t('dashboard.performance') }}</span>
          <el-radio-group v-model="chartTimeRange" size="small" style="margin-left: auto;">
            <el-radio-button value="1m">{{ $t('dashboard.oneMinute') }}</el-radio-button>
            <el-radio-button value="5m">{{ $t('dashboard.fiveMinutes') }}</el-radio-button>
            <el-radio-button value="15m">{{ $t('dashboard.fifteenMinutes') }}</el-radio-button>
          </el-radio-group>
        </div>
      </template>
      <v-chart :option="chartOption" :autoresize="true" style="height: 280px" />
    </el-card>

    <el-card v-if="sections.opsStats" class="info-card mt-card">
      <template #header>
        <div class="card-head">
          <el-icon class="card-icon log-color"><List /></el-icon>
          <span>{{ $t('dashboard.realTimeRequests') }}</span>
          <el-button size="small" text style="margin-left: auto;" @click="requestLogs = []">{{ $t('dashboard.clear') }}</el-button>
        </div>
      </template>
      <el-table :data="requestLogs" size="small" max-height="240" style="width: 100%">
        <el-table-column :label="$t('dashboard.time')" width="90">
          <template #default="{ row }">
            <span class="log-time">{{ row.time }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="model" :label="$t('dashboard.model')" width="140">
          <template #default="{ row }">
            <el-tag size="small" effect="plain">{{ row.model || '-' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="status" :label="$t('dashboard.status')" width="70" align="center">
          <template #default="{ row }">
            <el-tag :type="row.status === 200 ? 'success' : 'danger'" size="small">{{ row.status }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="cacheStatus" :label="$t('dashboard.cache')" width="90" align="center">
          <template #default="{ row }">
            <el-tag v-if="row.cacheStatus === 'HIT-EXACT'" type="success" size="small">{{ $t('dashboard.exactHit') }}</el-tag>
            <el-tag v-else-if="row.cacheStatus === 'HIT-SEMANTIC'" type="warning" size="small">{{ $t('dashboard.semanticHit') }}</el-tag>
            <el-tag v-else-if="row.cacheStatus === 'MISS'" size="small">{{ $t('dashboard.uncached') }}</el-tag>
            <span v-else class="text-muted">-</span>
          </template>
        </el-table-column>
        <el-table-column :label="$t('dashboard.latency')" width="80" align="right">
          <template #default="{ row }">
            <span :class="row.latency > 5000 ? 'text-warning' : ''">{{ row.latency }}ms</span>
          </template>
        </el-table-column>
        <el-table-column prop="prompt" :label="$t('dashboard.requestContent')" min-width="200">
          <template #default="{ row }">
            <el-text size="small" class="log-prompt" :line-clamp="1">{{ row.prompt || '-' }}</el-text>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <SecuritySettingsDialog v-if="sections.headerActions" v-model="securityDialogVisible" />
    <MinimalChat
      v-if="sections.liteChatDrawer"
      v-model="chatDialogVisible"
      :initial-pipeline-id="chatPipelineId"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Document, CircleCheck, TrendCharts, Warning,
  Timer, Stopwatch, Coin, Monitor, DataBoard, DataLine, DataAnalysis,
    CopyDocument, Cpu, List, Share, ChatDotRound
  } from '@element-plus/icons-vue'
import VChart from 'vue-echarts'
import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { LineChart } from 'echarts/charts'
import { GridComponent, TooltipComponent, LegendComponent } from 'echarts/components'
import { getDashboard, getStatus, getBackends, getStorages, getPlugins, api } from '@/api'
import { useEdition } from '@/composables/useEdition'
import { syncEditionFromStatus } from '@/utils/edition'
import { getDashboardSections } from '@/utils/dashboard-sections'
import { API_ENDPOINTS, resolveApiBaseUrl } from '@/utils/apiBaseUrl'
import { formatUptime } from '@/utils/format'
import ApiAccessPanel from '@/components/dashboard/ApiAccessPanel.vue'
import SecuritySettingsDialog from '@/components/dashboard/SecuritySettingsDialog.vue'
import HomePipelineCard from '@/components/dashboard/HomePipelineCard.vue'
import DashboardBackendList from '@/components/dashboard/DashboardBackendList.vue'
import MinimalChat from '@/views/MinimalChat.vue'
import MinimalUsagePanel from '@/components/dashboard/MinimalUsagePanel.vue'
import { mergeBackendUpdate } from '@/utils/backendTest'
import { getPipelineDefaults } from '@/api/pipeline'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const { t } = useI18n()
const router = useRouter()
const authStore = useAuthStore()
const { edition, isPersonal } = useEdition()
const sections = computed(() =>
  getDashboardSections(edition.value, edition.value === 'team' && authStore.isAdmin)
)
const rootClass = computed(() => ({
  'dashboard--personal': sections.value.layout === 'personal' || sections.value.layout === 'lite',
  'dashboard--lite': sections.value.layout === 'lite',
  'dashboard--team': sections.value.layout === 'team'
}))
const pageDescription = computed(() => {
  if (isPersonal.value || (edition.value === 'team' && !authStore.isAdmin)) {
    return t('dashboard.pageDescriptionUser')
  }
  if (edition.value === 'team' && authStore.isAdmin) {
    return t('dashboard.pageDescriptionAdmin')
  }
  return t('dashboard.pageDescriptionDefault')
})
const usageHint = computed(() =>
  sections.value.usageEphemeralHint
    ? t('dashboard.usageHintDefault')
    : t('dashboard.usageHintAdmin')
)
const pipelinePanelRef = ref<{
  reload: () => void
  openCreate: () => void
  openImport: () => void
} | null>(null)
const backendListRef = ref<{ openCreate: () => void; reloadDefault: () => void } | null>(null)
const usagePanelRef = ref<{ reload: () => void } | null>(null)
const pipelineCount = ref(0)
const securityDialogVisible = ref(false)
const chatDialogVisible = ref(false)
const chatPipelineId = ref('')
const usageCollapse = ref<string[]>(['usage'])

async function openPipelineChat(pipelineId = '') {
  let id = (pipelineId || '').trim()
  if (!id) {
    try {
      const res: any = await getPipelineDefaults()
      const data = res?.data ?? res
      id = String(data?.default_pipeline_id || '').trim()
    } catch {
      id = ''
    }
  }
  chatPipelineId.value = id
  chatDialogVisible.value = true
}

watch(chatDialogVisible, (open, wasOpen) => {
  if (wasOpen && !open) {
    chatPipelineId.value = ''
    usagePanelRef.value?.reload()
  }
})

watch(usageCollapse, (names) => {
  if (names.includes('usage')) {
    usagePanelRef.value?.reload()
  }
})

async function handleLogout() {
  try {
    await ElMessageBox.confirm(t('appHeader.logoutConfirm'), t('appHeader.logoutTitle'), {
      confirmButtonText: t('appHeader.confirm'),
      cancelButtonText: t('appHeader.cancel'),
      type: 'warning'
    })
  } catch {
    return
  }
  await authStore.logout()
  router.push('/login')
}

use([CanvasRenderer, LineChart, GridComponent, TooltipComponent, LegendComponent])

function formatNumber(n: number | undefined | null): string {
  if (!n) return '0'
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M'
  if (n >= 1_000) return (n / 1_000).toFixed(1) + 'K'
  return String(n)
}

const loading = ref(false)

const dashboard = ref<any>({
  request: { total_requests: 0, success_requests: 0, error_requests: 0, qps: 0, avg_latency_ms: 0, error_rate_percent: 0 },
  cache: { hits: 0, misses: 0, hit_rate_percent: 0, entries: 0 },
  plugin_count: 0,
  plugin_running: 0
})

const status = ref<any>({})
const backends = ref<any[]>([])
const storages = ref<any[]>([])
const plugins = ref<any[]>([])
const proxyStatus = ref<any>({ enabled: false, pac_enabled: false, pac_domains: [] })
const hostProxy = ref<any>({ enabled: false })
const proxyToggling = ref(false)
const hostProxyToggling = ref(false)
const baseUrl = computed(() => resolveApiBaseUrl(status.value))

const apiEndpoints = API_ENDPOINTS

const defaultBackend = computed(() => {
  const enabled = backends.value.filter((b: { enabled?: boolean }) => b.enabled)
  if (!enabled.length) return null
  return enabled.reduce((best: any, b: any) => ((b.weight ?? 0) > (best.weight ?? 0) ? b : best))
})

const defaultBackendSummary = computed(() => {
  const backend = defaultBackend.value
  if (!backend) return null
  return { name: backend.name as string, type: backend.type as string }
})

async function copyExternalUrl() {
  const url = status.value.external_url
  if (!url) return
  try {
    await navigator.clipboard.writeText(url)
    ElMessage.success(t('dashboard.copiedExternalAddress'))
  } catch {
    ElMessage.error(t('dashboard.copyFailed'))
  }
}

async function copyEndpoint(ep: { label: string; path: string }) {
  const url = baseUrl.value + ep.path
  try {
    await navigator.clipboard.writeText(url)
    ElMessage.success(t('dashboard.copiedExternalAddress'))
  } catch {
    ElMessage.error(t('dashboard.copyFailed'))
  }
}

const hitRate = computed(() => {
  const h = dashboard.value.cache?.hits || 0
  const m = dashboard.value.cache?.misses || 0
  if (!h && !m) return '0.00'
  return ((h / (h + m)) * 100).toFixed(2)
})

const modelStatsList = computed(() => {
  const stats = dashboard.value.request?.model_stats || {}
  return Object.entries(stats).map(([name, data]: [string, any]) => ({
    name,
    total_requests: data.total_requests || 0,
    avg_latency_ms: data.avg_latency_ms || 0,
    error_requests: data.error_requests || 0,
    cache_hits: data.cache_hits || 0,
    cache_misses: data.cache_misses || 0,
    cache_hit_rate_percent: data.cache_hit_rate_percent || 0,
    error_rate: data.error_rate_percent || 0
  }))
})

const chartTimeRange = ref('1m')
const historyData = ref<{ time: string; qps: number; latency: number; hitRate: number }[]>([])

const chartOption = computed(() => {
  const times = historyData.value.map(d => d.time)
  const qpsData = historyData.value.map(d => d.qps)
  const latencyData = historyData.value.map(d => d.latency)
  const hitRateData = historyData.value.map(d => d.hitRate)

  return {
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'cross' }
    },
    legend: {
      data: ['QPS', t('dashboard.latencyMs'), t('dashboard.hitRatePercent')],
      top: 0
    },
    grid: {
      left: '3%',
      right: '4%',
      bottom: '3%',
      containLabel: true
    },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: times,
      axisLabel: { fontSize: 10 }
    },
    yAxis: [
      {
        type: 'value',
        name: t('dashboard.qpsLatency'),
        position: 'left',
        axisLabel: { fontSize: 10 }
      },
      {
        type: 'value',
        name: t('dashboard.hitRatePercent'),
        min: 0,
        max: 100,
        position: 'right',
        axisLabel: { fontSize: 10, formatter: '{value}%' }
      }
    ],
    series: [
      {
        name: 'QPS',
        type: 'line',
        smooth: true,
        data: qpsData,
        itemStyle: { color: '#67c23a' },
        areaStyle: { color: 'rgba(103,194,58,0.1)' }
      },
      {
        name: t('dashboard.latencyMs'),
        type: 'line',
        smooth: true,
        data: latencyData,
        itemStyle: { color: '#409eff' }
      },
      {
        name: t('dashboard.hitRatePercent'),
        type: 'line',
        smooth: true,
        yAxisIndex: 1,
        data: hitRateData,
        itemStyle: { color: '#e6a23c' }
      }
    ]
  }
})

const requestLogs = ref<{ time: string; model: string; status: number; cacheStatus: string; latency: number; prompt: string }[]>([])

function addRequestLog(data: any) {
  const now = new Date()
  const time = `${now.getHours().toString().padStart(2,'0')}:${now.getMinutes().toString().padStart(2,'0')}:${now.getSeconds().toString().padStart(2,'0')}`

  requestLogs.value.unshift({
    time,
    model: data.model || '-',
    status: data.status_code || 200,
    cacheStatus: data.cache_status || '-',
    latency: data.latency || 0,
    prompt: data.prompt || '-'
  })

  if (requestLogs.value.length > 50) {
    requestLogs.value = requestLogs.value.slice(0, 50)
  }
}

async function toggleSystemProxy() {
  proxyToggling.value = true
  try {
    await api.put('/api/v1/config', {
      system_proxy: {
        enabled: proxyStatus.value.enabled,
        listen_port: proxyStatus.value.listen_port || 8080,
        pac_enabled: proxyStatus.value.pac_enabled
      }
    })
    ElMessage.success(proxyStatus.value.enabled ? t('dashboard.systemProxyEnabled') : t('dashboard.systemProxyDisabled'))
  } catch (error: any) {
    ElMessage.error(t('dashboard.operationFailed', { msg: error.message }))
    proxyStatus.value.enabled = !proxyStatus.value.enabled
  } finally {
    proxyToggling.value = false
  }
}

async function toggleHostProxy() {
  hostProxyToggling.value = true
  try {
    await api.put('/api/v1/config', {
      host_proxy: {
        enabled: hostProxy.value.enabled,
        http_port: hostProxy.value.http_port || 8080,
        https_port: hostProxy.value.https_port || 8443
      }
    })
    ElMessage.success(hostProxy.value.enabled ? t('dashboard.hostProxyEnabled') : t('dashboard.hostProxyDisabled'))
  } catch (error: any) {
    ElMessage.error(t('dashboard.operationFailed', { msg: error.message }))
    hostProxy.value.enabled = !hostProxy.value.enabled
  } finally {
    hostProxyToggling.value = false
  }
}

function formatDbDriver(driver: string | undefined): string {
  if (!driver) return t('dashboard.unknown')
  switch (driver.toLowerCase()) {
    case 'postgresql':
      return 'PostgreSQL'
    case 'sqlite':
      return 'SQLite'
    case 'mysql':
      return 'MySQL'
    default:
      return driver.charAt(0).toUpperCase() + driver.slice(1)
  }
}

function getDbDriverType(driver: string | undefined): string {
  if (!driver) return 'info'
  switch (driver.toLowerCase()) {
    case 'postgresql':
      return 'primary'
    case 'sqlite':
      return 'success'
    case 'mysql':
      return 'warning'
    default:
      return 'info'
  }
}

let backendsLoadGen = 0

function patchBackend(updated: any) {
  mergeBackendUpdate(backends.value, updated)
}

async function loadBackendsOnly() {
  const gen = ++backendsLoadGen
  try {
    const backendsRes = await getBackends()
    if (gen !== backendsLoadGen) return
    backends.value = backendsRes || []
    backendListRef.value?.reloadDefault()
  } catch (e: any) {
    if (gen !== backendsLoadGen) return
    ElMessage.error(t('dashboard.loadBackendDataFailed', { msg: e.message || t('dashboard.unknownError') }))
  }
}

async function load() {
  loading.value = true
  const gen = ++backendsLoadGen
  try {
    const sec = sections.value
    if (!sec.opsStats && !sec.serviceStatus && !sec.proxyControls) {
      const [statusRes, backendsRes] = await Promise.allSettled([getStatus(), getBackends()])
      if (statusRes.status === 'fulfilled' && statusRes.value) {
        status.value = statusRes.value
        syncEditionFromStatus(statusRes.value)
      }
      if (backendsRes.status === 'fulfilled' && backendsRes.value && gen === backendsLoadGen) {
        backends.value = backendsRes.value || []
      }
      pipelinePanelRef.value?.reload()
      return
    }

    const tasks: Promise<any>[] = [
      sec.opsStats ? getDashboard() : Promise.resolve(null),
      getStatus(),
      getBackends(),
      sec.pluginsStorage ? getStorages() : Promise.resolve(null),
      sec.pluginsStorage ? getPlugins() : Promise.resolve(null),
      sec.proxyControls ? api.get('/api/v1/proxy/status') : Promise.resolve(null),
      sec.proxyControls ? api.get('/api/v1/host-proxy/status') : Promise.resolve(null)
    ]
    const [dashRes, statusRes, backendsRes, storagesRes, pluginsRes, proxyRes, hostProxyRes] = await Promise.allSettled(tasks)

    if (dashRes.status === 'fulfilled' && dashRes.value) {
      dashboard.value = dashRes.value
    }
    if (statusRes.status === 'fulfilled' && statusRes.value) {
      status.value = statusRes.value
      syncEditionFromStatus(statusRes.value)
    }
    if (backendsRes.status === 'fulfilled' && backendsRes.value && gen === backendsLoadGen) {
      backends.value = backendsRes.value || []
    }
    if (storagesRes.status === 'fulfilled' && storagesRes.value) {
      const d = storagesRes.value as any
      storages.value = d?.storages || d || []
    }
    if (pluginsRes.status === 'fulfilled' && pluginsRes.value) {
      const d = pluginsRes.value as any
      plugins.value = d?.plugins || d || []
    }
    if (proxyRes.status === 'fulfilled' && proxyRes.value) {
      proxyStatus.value = proxyRes.value
    }
    if (hostProxyRes.status === 'fulfilled' && hostProxyRes.value) {
      hostProxy.value = hostProxyRes.value
    }
    pipelinePanelRef.value?.reload()

    const now = new Date()
    const time = `${now.getHours().toString().padStart(2,'0')}:${now.getMinutes().toString().padStart(2,'0')}`
    const qps = dashboard.value.request?.qps || 0
    const latency = dashboard.value.request?.avg_latency_ms || 0
    const cache = dashboard.value.cache || {}
    const hitRateVal = cache.hits + cache.misses > 0
      ? (cache.hits / (cache.hits + cache.misses)) * 100
      : 0

    historyData.value.push({ time, qps, latency, hitRate: hitRateVal })

    const maxPoints = chartTimeRange.value === '1m' ? 6 : chartTimeRange.value === '5m' ? 30 : 90
    if (historyData.value.length > maxPoints) {
      historyData.value = historyData.value.slice(-maxPoints)
    }
  } catch (e: any) {
    ElMessage.error(t('dashboard.loadDataFailed', { msg: e.message || t('dashboard.unknownError') }))
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  load()
})
</script>

<style scoped>
.dashboard {
  width: 100%;
  max-width: 100%;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 20px;
}

.page-header--actions-only {
  justify-content: flex-end;
  align-items: center;
  margin-bottom: 12px;
}

.request-bg  { background: rgba(102,126,234,0.12); color: #667eea; }
.hit-bg      { background: rgba(16,185,129,0.12);  color: #10b981; }
.rate-bg     { background: rgba(245,158,11,0.12);  color: #f59e0b; }
.qps-bg      { background: rgba(59,130,246,0.12);  color: #3b82f6; }
.latency-bg  { background: rgba(139,92,246,0.12);  color: #8b5cf6; }
.error-bg    { background: rgba(239,68,68,0.12);   color: #ef4444; }
.entry-bg    { background: rgba(20,184,166,0.12);  color: #14b8a6; }

.stats-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(120px, 1fr));
  gap: 8px;
}

.stats-card .stat-cell {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 10px 8px;
  border-radius: 8px;
  background: #f9fafb;
}

.stat-icon-wrap {
  width: 32px;
  height: 32px;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 6px;
}

.dash-main {
  display: grid;
  gap: 16px;
}

.dash-main--lite {
  grid-template-columns: 1fr 1fr;
  grid-template-areas:
    "access access"
    "backends pipelines";
}

.dash-main--personal {
  grid-template-columns: minmax(0, 1fr) minmax(0, 2fr);
  grid-template-areas:
    "status access"
    "backends pipelines";
}

.dash-main--team {
  grid-template-columns: repeat(3, minmax(0, 1fr));
  grid-template-areas: "status backends pipelines";
}

.dash-card--status { grid-area: status; }
.dash-card--access { grid-area: access; }
.dash-card--backends { grid-area: backends; }
.dash-card--pipelines { grid-area: pipelines; }

@media (max-width: 960px) {
  .dash-main--lite,
  .dash-main--personal,
  .dash-main--team {
    grid-template-columns: 1fr;
    grid-template-areas:
      "status"
      "access"
      "backends"
      "pipelines";
  }
}

.card-head--actions {
  width: 100%;
  justify-content: space-between;
}

.card-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.card-head-main {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.access-card {
  display: flex;
  flex-direction: column;
}

.access-card--compact :deep(.el-card__body) {
  padding: 10px 14px;
}

.usage-card {
  padding-bottom: 4px;
}

.usage-collapse {
  border: none;
}

.usage-collapse :deep(.el-collapse-item__header) {
  height: auto;
  min-height: 48px;
  line-height: 1.4;
  border: none;
  padding: 0 4px;
}

.usage-collapse :deep(.el-collapse-item__wrap) {
  border: none;
}

.usage-collapse :deep(.el-collapse-item__content) {
  padding: 8px 4px 12px;
}

.usage-collapse-title {
  gap: 8px;
}

.access-keys {
  margin-top: var(--spacing-md);
  padding-top: var(--spacing-md);
  border-top: 1px solid var(--shell-sidebar-border, #e4e7ed);
}

.hero-actions {
  display: flex;
  gap: var(--spacing-sm);
  flex-wrap: wrap;
  margin-top: var(--spacing-md);
  padding-top: var(--spacing-md);
  border-top: 1px solid var(--shell-sidebar-border, #e4e7ed);
}

.page-header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.mt-card { margin-top: 16px; }

.info-card {
  border: 1px solid #e4e7ed;
  box-shadow: 0 1px 4px rgba(0,0,0,0.04);
  display: flex;
  flex-direction: column;
}

.info-card :deep(.el-card__body) {
  flex: 1;
  min-height: 0;
}

.config-card :deep(.el-card__body) {
  overflow: hidden;
}

.config-card :deep(.home-pipeline-card),
.config-card :deep(.backend-list) {
  height: 100%;
  min-height: 0;
}

.card-head {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 500;
  font-size: 0.9rem;
  color: #374151;
}

.card-icon {
  font-size: 16px;
}
.service-color { color: #667eea; }
.proxy-color   { color: #3b82f6; }
.backend-color { color: #10b981; }
.pipeline-color { color: #6366f1; }
.storage-color { color: #f59e0b; }
.stats-color   { color: #8b5cf6; }
.plugin-color  { color: #ec4899; }
.chart-color   { color: #06b6d4; }
.log-color     { color: #f97316; }

.card-badge {
  margin-left: auto;
  font-size: 0.75rem;
  color: #9ca3af;
  font-weight: 400;
}

.personal-status-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 12px 16px;
}

.personal-status-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.personal-backend-val {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.info-rows {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.info-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 0.8375rem;
}

.section-title-row {
  margin-top: 4px;
}

.section-label {
  font-weight: 500;
  color: #374151;
  font-size: 0.8375rem;
}

.info-section {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.info-section-head {
  padding-bottom: 4px;
  border-bottom: 1px solid #eee;
}

.info-section-row {
  margin-bottom: 2px;
}

.info-label {
  color: #6b7280;
}

.info-val {
  color: #111827;
  font-weight: 500;
}

.mono {
  font-family: 'SF Mono', 'Fira Code', monospace;
  font-size: 0.8rem;
}

.external-url-row {
  display: flex;
  align-items: center;
  gap: 6px;
  flex: 1;
  min-width: 0;
}

.endpoint-row {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 3px 0;
  min-width: 0;
}

.endpoint-tag {
  flex-shrink: 0;
  width: 88px;
  text-align: center;
}

.endpoint-url {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 0.75rem;
  color: #606266;
}

.external-url-text {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
}

.copy-icon {
  cursor: pointer;
  color: #909399;
  flex-shrink: 0;
  transition: color 0.2s;
}

.copy-icon:hover {
  color: #409eff;
}

.backend-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 12px;
  background: #f9fafb;
  border-radius: 6px;
  border: 1px solid #f3f4f6;
}

.compact-backend-item {
  padding: 8px 0;
  border: none;
  background: transparent;
}

.backend-left {
  display: flex;
  align-items: center;
  gap: 10px;
  flex: 1;
  min-width: 0;
}

.backend-status { flex-shrink: 0; }

.backend-info { flex: 1; min-width: 0; }

.backend-name {
  font-size: 0.875rem;
  font-weight: 500;
  color: #111827;
  display: flex;
  align-items: center;
}

.backend-meta {
  font-size: 0.75rem;
  color: #9ca3af;
  margin-top: 2px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.empty-tip {
  text-align: center;
  color: #9ca3af;
  font-size: 0.8rem;
  padding: 16px 0;
}

.compact-empty-tip {
  padding: 8px 0;
}

.stat-num {
  font-size: 1.35rem;
  font-weight: 600;
  color: #111827;
  line-height: 1.2;
}

.stat-label {
  font-size: 0.72rem;
  color: #6b7280;
  margin-top: 3px;
  text-align: center;
}

@media (max-width: 1400px) {
  .grid-layout {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 900px) {
  .grid-layout {
    grid-template-columns: 1fr;
  }

  .lite-row--config,
  .personal-top-layout,
  .personal-config-layout {
    grid-template-columns: 1fr;
  }
  .metrics-bar {
    flex-wrap: wrap;
    gap: 8px;
  }
  .metric-divider { display: none; }
  .metric-item {
    flex: 0 0 calc(33% - 8px);
    padding: 8px 12px;
  }
}

.plugin-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.plugin-item {
  display: flex;
  align-items: center;
  padding: 8px 12px;
  background: #f9fafb;
  border-radius: 6px;
  border: 1px solid #f3f4f6;
}

.plugin-left {
  display: flex;
  align-items: center;
  gap: 10px;
  flex: 1;
  min-width: 0;
}

.plugin-status {
  flex-shrink: 0;
}

.plugin-info {
  flex: 1;
  min-width: 0;
}

.plugin-name {
  font-weight: 500;
  font-size: 0.8375rem;
  color: #374151;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.plugin-meta {
  font-size: 0.7875rem;
  color: #6b7280;
  margin-top: 2px;
}

.stat-num-sm {
  font-weight: 600;
  font-size: 0.9rem;
  color: #374151;
}

.text-danger { color: #f56c6c; }
.text-success { color: #67c23a; }
.text-warning { color: #e6a23c; }
.text-muted { color: #9ca3af; }

.log-time {
  font-family: 'SF Mono', 'Fira Code', monospace;
  font-size: 0.75rem;
  color: #6b7280;
}

.log-prompt {
  font-family: 'SF Mono', 'Fira Code', monospace;
  font-size: 0.75rem;
  color: #4b5563;
}

:deep(.el-table__row:hover) {
  cursor: default;
}

:deep(.v-chart) {
  width: 100% !important;
}
</style>
