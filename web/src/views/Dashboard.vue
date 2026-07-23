<template>
  <div class="dashboard" :class="rootClass">
    <div class="page-header" :class="{ 'page-header--actions-only': !sections.pageTitle }">
      <div v-if="sections.pageTitle">
        <h1 class="page-title">{{ isPersonal ? '首页' : '概览' }}</h1>
        <p class="page-description">{{ pageDescription }}</p>
      </div>
      <div class="page-header-actions">
        <div class="response-trace-toggle">
          <el-tooltip content="开启后在 AI 回复前附加流水线、后端、模型与降级信息" placement="bottom">
            <span class="toggle-label">响应追踪</span>
          </el-tooltip>
          <el-switch
            v-model="responseTraceBanner"
            :loading="responseTraceToggling"
            size="small"
            @change="toggleResponseTraceBanner"
          />
        </div>
        <template v-if="sections.headerActions">
          <el-button type="success" @click="openPipelineChat()">
            <el-icon><ChatDotRound /></el-icon>&nbsp;AI 对话
          </el-button>
          <el-button @click="securityDialogVisible = true">安全设置</el-button>
          <el-button @click="handleLogout">退出登录</el-button>
        </template>
      </div>
    </div>

    <!-- 主区：后端 / 流水线 / 状态 / 接入 —— 由 layout + sections 控制 -->
    <div class="dash-main" :class="'dash-main--' + sections.layout">
      <!-- 服务状态 -->
      <el-card v-if="sections.serviceStatus" class="info-card dash-card dash-card--status">
        <template #header>
          <div class="card-head">
            <el-icon class="card-icon service-color"><Monitor /></el-icon>
            <span>服务状态</span>
          </div>
        </template>
        <div class="info-rows">
          <template v-if="sections.serviceStatusCompact">
            <div class="personal-status-grid">
              <div class="personal-status-item">
                <span class="info-label">运行状态</span>
                <el-tag :type="status.status === 'healthy' ? 'success' : 'danger'" size="small" effect="light">
                  {{ status.status === 'healthy' ? '运行中' : '异常' }}
                </el-tag>
              </div>
              <div class="personal-status-item">
                <span class="info-label">默认后端</span>
                <div class="personal-backend-val">
                  <span class="info-val">{{ defaultBackendSummary?.name || '未配置' }}</span>
                  <el-tag v-if="defaultBackendSummary" size="small" effect="plain" type="info">
                    {{ defaultBackendSummary.type }}
                  </el-tag>
                </div>
              </div>
              <div class="personal-status-item">
                <span class="info-label">版本</span>
                <span class="info-val mono">{{ status.version || '--' }}</span>
              </div>
              <div class="personal-status-item">
                <span class="info-label">运行时长</span>
                <span class="info-val">{{ status.uptime || '--' }}</span>
              </div>
            </div>
          </template>
          <template v-else>
            <div class="info-row">
              <span class="info-label">运行状态</span>
              <el-tag :type="status.status === 'healthy' ? 'success' : 'danger'" size="small" effect="light">
                {{ status.status === 'healthy' ? '● 运行中' : '● 异常' }}
              </el-tag>
            </div>
          </template>

          <template v-if="sections.teamAccessInStatus && (status.external_url || status.status === 'healthy')">
            <div v-if="status.external_url" class="info-row">
              <span class="info-label">外部地址</span>
              <div class="external-url-row">
                <span class="info-val mono external-url-text">{{ status.external_url }}</span>
                <el-tooltip content="复制地址" placement="top">
                  <el-icon class="copy-icon" @click="copyExternalUrl"><CopyDocument /></el-icon>
                </el-tooltip>
              </div>
            </div>
            <el-divider style="margin: 8px 0" />
            <div class="section-label" style="margin-bottom: 6px;">客户端接入地址</div>
            <div v-for="ep in apiEndpoints" :key="ep.path" class="endpoint-row">
              <el-tag size="small" :type="ep.tagType || undefined" class="endpoint-tag">{{ ep.label }}</el-tag>
              <span class="endpoint-url mono">{{ baseUrl }}{{ ep.path }}</span>
              <el-tooltip :content="'复制 ' + ep.label + ' 地址'" placement="top">
                <el-icon class="copy-icon" @click="copyEndpoint(ep)"><CopyDocument /></el-icon>
              </el-tooltip>
            </div>
          </template>

          <template v-if="sections.pluginsStorage">
            <el-divider style="margin: 8px 0" />
            <div class="info-row section-title-row">
              <span class="section-label">插件详情列表</span>
              <span class="card-badge">{{ dashboard.plugin_running }} / {{ dashboard.plugin_count }} 运行中</span>
            </div>
            <div class="plugin-list">
              <div v-for="p in plugins" :key="p.name" class="plugin-item">
                <div class="plugin-left">
                  <el-tag
                    :type="p.status === 'running' ? 'success' : 'info'"
                    size="small"
                    effect="light"
                    class="plugin-status"
                  >{{ p.status === 'running' ? '运行中' : p.status }}</el-tag>
                  <div class="plugin-info">
                    <div class="plugin-name">{{ p.name }}</div>
                    <div class="plugin-meta">{{ p.type }} · v{{ p.version }}</div>
                  </div>
                </div>
              </div>
              <div v-if="!plugins.length" class="empty-tip">暂无插件</div>
            </div>

            <el-divider style="margin: 8px 0" />
            <div class="info-row section-title-row">
              <span class="section-label">存储于数据库</span>
              <span class="card-badge">{{ storages.length + 1 }} 项</span>
            </div>
            <div class="info-section">
              <div class="section-label info-section-head">数据库</div>
              <div class="info-row info-section-row">
                <span class="info-label">驱动类型</span>
                <el-tag :type="getDbDriverType(dashboard.database?.driver)" size="small" effect="light">
                  {{ formatDbDriver(dashboard.database?.driver) }}
                </el-tag>
              </div>
              <div class="info-row info-section-row">
                <span class="info-label">连接状态</span>
                <el-tag
                  :type="dashboard.database?.status === 'connected' ? 'success' : 'danger'"
                  size="small"
                  effect="light"
                >
                  {{ dashboard.database?.status === 'connected' ? '已连接' : '未连接' }}
                </el-tag>
              </div>
              <div class="info-row">
                <span class="info-label">连接地址</span>
                <span class="info-val mono">{{ dashboard.database?.address || '未知' }}</span>
              </div>
            </div>

            <el-divider style="margin: 8px 0" />
            <div class="info-section">
              <div class="section-label info-section-head">存储中间件</div>
              <div v-for="s in storages" :key="s.name" class="backend-item compact-backend-item">
                <div class="backend-left">
                  <el-tag
                    :type="!s.enabled ? 'info' : s.healthy ? 'success' : 'danger'"
                    size="small"
                    effect="light"
                    class="backend-status"
                  >{{ !s.enabled ? '禁用' : s.healthy ? '健康' : '异常' }}</el-tag>
                  <div class="backend-info">
                    <div class="backend-name">
                      {{ s.name }}
                      <el-tag v-if="s.is_default" type="warning" size="small" effect="plain" style="margin-left:6px">默认</el-tag>
                    </div>
                    <div class="backend-meta">{{ s.type }} · {{ s.description }}</div>
                  </div>
                </div>
              </div>
              <div v-if="!storages.length" class="empty-tip compact-empty-tip">暂无存储配置</div>
            </div>
          </template>

          <template v-if="sections.proxyControls">
            <el-divider style="margin: 8px 0" />
            <div class="info-row section-title-row">
              <span class="section-label">系统代理</span>
              <el-switch
                v-model="proxyStatus.enabled"
                :loading="proxyToggling"
                size="small"
                @change="toggleSystemProxy"
              />
            </div>
            <div v-if="proxyStatus.enabled" class="info-row">
              <span class="info-label">PAC 代理</span>
              <el-tag :type="proxyStatus.pac_enabled ? 'success' : 'info'" size="small" effect="plain">
                {{ proxyStatus.pac_enabled ? '已启用' : '未启用' }}
              </el-tag>
            </div>
            <div v-if="proxyStatus.pac_domains?.length" class="info-row">
              <span class="info-label">代理域名</span>
              <span class="info-val">{{ proxyStatus.pac_domains.length }} 个</span>
            </div>

            <el-divider style="margin: 8px 0" />
            <div class="info-row section-title-row">
              <span class="section-label">Host 代理</span>
              <el-switch
                v-model="hostProxy.enabled"
                :loading="hostProxyToggling"
                size="small"
                @change="toggleHostProxy"
              />
            </div>
            <template v-if="hostProxy.enabled">
              <div class="info-row">
                <span class="info-label">HTTP 端口</span>
                <span class="info-val mono">{{ hostProxy.http_port }}</span>
              </div>
              <div class="info-row">
                <span class="info-label">HTTPS 端口</span>
                <span class="info-val mono">{{ hostProxy.https_port }}</span>
              </div>
            </template>
            <div v-if="hostProxy.domains" class="info-row">
              <span class="info-label">代理域名</span>
              <span class="info-val">{{ Object.keys(hostProxy.domains || {}).length }} 个</span>
            </div>
          </template>
        </div>
      </el-card>

      <!-- API 接入 -->
      <el-card
        v-if="sections.accessPanel"
        class="info-card access-card dash-card dash-card--access"
        :class="{ 'access-card--compact': sections.accessCompact }"
        :shadow="sections.accessCompact ? 'never' : undefined"
      >
        <ApiAccessPanel :base-url="baseUrl" :compact="sections.accessCompact" />
        <!-- accessQuickLinks 已关闭：对话走流水线「测试」抽屉，后端在首页面板 -->
      </el-card>

      <!-- 后端 -->
      <el-card v-if="sections.backends" class="info-card config-card dash-card dash-card--backends">
        <template #header>
          <div class="card-head">
            <el-icon class="card-icon backend-color"><DataBoard /></el-icon>
            <span>后端配置</span>
            <span class="card-badge">{{ backends.length }} 个</span>
          </div>
        </template>
        <DashboardBackendList
          ref="backendListRef"
          :backends="backends"
          @backend-updated="patchBackend"
          @refresh="loadBackendsOnly"
        />
      </el-card>

      <!-- 流水线 -->
      <el-card v-if="sections.pipelines" class="info-card config-card dash-card dash-card--pipelines">
        <template #header>
          <div class="card-head" :class="{ 'card-head--actions': sections.pipelineCreateButton }">
            <div class="card-head-main">
              <el-icon class="card-icon pipeline-color"><Share /></el-icon>
              <span>流水线配置</span>
              <span v-if="sections.pipelineCreateButton" class="card-badge">{{ pipelineCount }} 个</span>
            </div>
            <div v-if="sections.pipelineCreateButton" class="card-actions">
              <el-button size="small" plain @click="pipelinePanelRef?.openImport()">
                导入
              </el-button>
              <el-button type="primary" size="small" @click="pipelinePanelRef?.openCreate()">
                + 创建流水线
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

    <!-- 用量与会话 / 计费（三版可开） -->
    <el-card v-if="sections.usageBilling" class="info-card usage-card mt-card">
      <el-collapse v-model="usageCollapse" class="usage-collapse">
        <el-collapse-item name="usage">
          <template #title>
            <div class="card-head usage-collapse-title">
              <el-icon class="card-icon service-color"><TrendCharts /></el-icon>
              <span>用量与会话</span>
              <span v-if="sections.usageEphemeralHint" class="card-badge">进程内 · 重启清零</span>
            </div>
          </template>
          <MinimalUsagePanel
            ref="usagePanelRef"
            :hint="usageHint"
          />
        </el-collapse-item>
      </el-collapse>
    </el-card>

    <!-- 团队运维统计 -->
    <el-card v-if="sections.opsStats" class="info-card mt-card stats-card">
      <template #header>
        <div class="card-head">
          <el-icon class="card-icon stats-color"><DataAnalysis /></el-icon>
          <span>运行统计</span>
        </div>
      </template>
      <div class="stats-grid">
        <div class="stat-cell">
          <div class="stat-icon-wrap request-bg"><el-icon :size="18"><Document /></el-icon></div>
          <div class="stat-num">{{ formatNumber(dashboard.request.total_requests) }}</div>
          <div class="stat-label">请求总数</div>
        </div>
        <div class="stat-cell">
          <div class="stat-icon-wrap hit-bg"><el-icon :size="18"><CircleCheck /></el-icon></div>
          <div class="stat-num">{{ formatNumber(dashboard.request.success_requests) }}</div>
          <div class="stat-label">成功请求</div>
        </div>
        <div class="stat-cell">
          <div class="stat-icon-wrap error-bg"><el-icon :size="18"><Warning /></el-icon></div>
          <div class="stat-num">{{ formatNumber(dashboard.request.error_requests) }}</div>
          <div class="stat-label">错误数</div>
        </div>
        <div class="stat-cell">
          <div class="stat-icon-wrap error-bg"><el-icon :size="18"><TrendCharts /></el-icon></div>
          <div class="stat-num">{{ dashboard.request.error_rate_percent?.toFixed(2) ?? '0' }}%</div>
          <div class="stat-label">错误率</div>
        </div>
        <div class="stat-cell">
          <div class="stat-icon-wrap qps-bg"><el-icon :size="18"><Timer /></el-icon></div>
          <div class="stat-num">{{ dashboard.request.qps?.toFixed(2) ?? '0' }}</div>
          <div class="stat-label">QPS</div>
        </div>
        <div class="stat-cell">
          <div class="stat-icon-wrap latency-bg"><el-icon :size="18"><Stopwatch /></el-icon></div>
          <div class="stat-num">{{ dashboard.request.avg_latency_ms ?? 0 }}ms</div>
          <div class="stat-label">平均延迟</div>
        </div>
      </div>
      <el-divider style="margin: 10px 0" />
      <div class="stats-grid">
        <div class="stat-cell">
          <div class="stat-icon-wrap hit-bg"><el-icon :size="18"><CircleCheck /></el-icon></div>
          <div class="stat-num">{{ formatNumber(dashboard.cache.hits) }}</div>
          <div class="stat-label">缓存命中</div>
        </div>
        <div class="stat-cell">
          <div class="stat-icon-wrap rate-bg"><el-icon :size="18"><TrendCharts /></el-icon></div>
          <div class="stat-num">{{ hitRate }}%</div>
          <div class="stat-label">命中率</div>
        </div>
        <div class="stat-cell">
          <div class="stat-icon-wrap entry-bg"><el-icon :size="18"><Coin /></el-icon></div>
          <div class="stat-num">{{ formatNumber(dashboard.cache.entries) }}</div>
          <div class="stat-label">缓存条目</div>
        </div>
        <div class="stat-cell">
          <div class="stat-icon-wrap request-bg"><el-icon :size="18"><DataLine /></el-icon></div>
          <div class="stat-num">{{ formatNumber(dashboard.cache.misses) }}</div>
          <div class="stat-label">未命中</div>
        </div>
      </div>
    </el-card>

    <el-card v-if="sections.opsStats && modelStatsList.length" class="info-card mt-card">
      <template #header>
        <div class="card-head">
          <el-icon class="card-icon stats-color"><Cpu /></el-icon>
          <span>模型统计</span>
        </div>
      </template>
      <el-table :data="modelStatsList" size="small" style="width: 100%">
        <el-table-column prop="name" label="模型" min-width="120">
          <template #default="{ row }">
            <el-tag size="small" :type="row.error_rate > 10 ? 'danger' : 'success'" effect="light">
              {{ row.name }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="total_requests" label="请求数" align="center" width="100">
          <template #default="{ row }">
            <span class="stat-num-sm">{{ formatNumber(row.total_requests) }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="avg_latency_ms" label="平均延迟" align="center" width="100">
          <template #default="{ row }">
            <span class="stat-num-sm">{{ row.avg_latency_ms }}ms</span>
          </template>
        </el-table-column>
        <el-table-column label="缓存命中" align="center" width="100">
          <template #default="{ row }">
            <span class="stat-num-sm">{{ formatNumber(row.cache_hits) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="命中率" align="center" width="80">
          <template #default="{ row }">
            {{ row.cache_hit_rate_percent?.toFixed(1) }}%
          </template>
        </el-table-column>
        <el-table-column label="错误率" align="center" width="80">
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
          <span>性能趋势</span>
          <el-radio-group v-model="chartTimeRange" size="small" style="margin-left: auto;">
            <el-radio-button value="1m">1分钟</el-radio-button>
            <el-radio-button value="5m">5分钟</el-radio-button>
            <el-radio-button value="15m">15分钟</el-radio-button>
          </el-radio-group>
        </div>
      </template>
      <v-chart :option="chartOption" :autoresize="true" style="height: 280px" />
    </el-card>

    <el-card v-if="sections.opsStats" class="info-card mt-card">
      <template #header>
        <div class="card-head">
          <el-icon class="card-icon log-color"><List /></el-icon>
          <span>实时请求</span>
          <el-button size="small" text style="margin-left: auto;" @click="requestLogs = []">清空</el-button>
        </div>
      </template>
      <el-table :data="requestLogs" size="small" max-height="240" style="width: 100%">
        <el-table-column label="时间" width="90">
          <template #default="{ row }">
            <span class="log-time">{{ row.time }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="model" label="模型" width="140">
          <template #default="{ row }">
            <el-tag size="small" effect="plain">{{ row.model || '-' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="70" align="center">
          <template #default="{ row }">
            <el-tag :type="row.status === 200 ? 'success' : 'danger'" size="small">{{ row.status }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="cacheStatus" label="缓存" width="90" align="center">
          <template #default="{ row }">
            <el-tag v-if="row.cacheStatus === 'HIT-EXACT'" type="success" size="small">精确命中</el-tag>
            <el-tag v-else-if="row.cacheStatus === 'HIT-SEMANTIC'" type="warning" size="small">语义命中</el-tag>
            <el-tag v-else-if="row.cacheStatus === 'MISS'" size="small">未命中</el-tag>
            <span v-else class="text-muted">-</span>
          </template>
        </el-table-column>
        <el-table-column label="延迟" width="80" align="right">
          <template #default="{ row }">
            <span :class="row.latency > 5000 ? 'text-warning' : ''">{{ row.latency }}ms</span>
          </template>
        </el-table-column>
        <el-table-column prop="prompt" label="请求内容" min-width="200">
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
    return '配置后端与默认流水线，复制 API 地址即可在客户端使用'
  }
  if (edition.value === 'team' && authStore.isAdmin) {
    return '管理共用后端/策略、用户与限额；业务干活请使用普通用户账号'
  }
  return 'Centag 服务运行状态与基础配置总览'
})
const usageHint = computed(() =>
  sections.value.usageEphemeralHint
    ? '本次进程内计量与成本估算（重启后清零）。'
    : '计量与成本估算（按当前服务存储策略保留）。'
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
/** 用量与会话默认折叠 */
const usageCollapse = ref<string[]>([])

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
    await ElMessageBox.confirm('确定要退出登录吗？', '退出登录', {
      confirmButtonText: '退出',
      cancelButtonText: '取消',
      type: 'warning'
    })
  } catch {
    return
  }
  await authStore.logout()
  router.push('/login')
}

// 注册 ECharts 组件
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
const responseTraceBanner = ref(false)
const responseTraceToggling = ref(false)

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
    ElMessage.success('已复制外部地址')
  } catch {
    ElMessage.error('复制失败，请手动复制')
  }
}

async function copyEndpoint(ep: { label: string; path: string }) {
  const url = baseUrl.value + ep.path
  try {
    await navigator.clipboard.writeText(url)
    ElMessage.success(`已复制 ${ep.label} 地址`)
  } catch {
    ElMessage.error('复制失败，请手动复制')
  }
}

const hitRate = computed(() => {
  const h = dashboard.value.cache?.hits || 0
  const m = dashboard.value.cache?.misses || 0
  if (!h && !m) return '0.00'
  return ((h / (h + m)) * 100).toFixed(2)
})

// 模型统计数据
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

// 趋势图表数据
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
      data: ['QPS', '延迟(ms)', '命中率(%)'],
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
        name: 'QPS/延迟',
        position: 'left',
        axisLabel: { fontSize: 10 }
      },
      {
        type: 'value',
        name: '命中率(%)',
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
        name: '延迟(ms)',
        type: 'line',
        smooth: true,
        data: latencyData,
        itemStyle: { color: '#409eff' }
      },
      {
        name: '命中率(%)',
        type: 'line',
        smooth: true,
        yAxisIndex: 1,
        data: hitRateData,
        itemStyle: { color: '#e6a23c' }
      }
    ]
  }
})

// 实时请求日志
const requestLogs = ref<{ time: string; model: string; status: number; cacheStatus: string; latency: number; prompt: string }[]>([])

// 添加新的请求日志
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
  
  // 保留最近50条
  if (requestLogs.value.length > 50) {
    requestLogs.value = requestLogs.value.slice(0, 50)
  }
}

// 切换「响应追踪」：统一在返回正文前附加流水线 / 后端 / 模型 / 降级信息
async function toggleResponseTraceBanner() {
  responseTraceToggling.value = true
  try {
    await api.put('/api/v1/config/proxy', {
      response_trace_banner: responseTraceBanner.value
    })
    ElMessage.success(responseTraceBanner.value ? '响应追踪已开启' : '响应追踪已关闭')
  } catch (error: any) {
    ElMessage.error('操作失败: ' + (error.message || '未知错误'))
    responseTraceBanner.value = !responseTraceBanner.value
  } finally {
    responseTraceToggling.value = false
  }
}

// 切换系统代理
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
    ElMessage.success(proxyStatus.value.enabled ? '系统代理已启用' : '系统代理已禁用')
  } catch (error: any) {
    ElMessage.error('操作失败: ' + error.message)
    proxyStatus.value.enabled = !proxyStatus.value.enabled
  } finally {
    proxyToggling.value = false
  }
}

// 切换 Host 代理
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
    ElMessage.success(hostProxy.value.enabled ? 'Host代理已启用' : 'Host代理已禁用')
  } catch (error: any) {
    ElMessage.error('操作失败: ' + error.message)
    hostProxy.value.enabled = !hostProxy.value.enabled
  } finally {
    hostProxyToggling.value = false
  }
}

// 格式化数据库驱动名称
function formatDbDriver(driver: string | undefined): string {
  if (!driver) return '未知'
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

// 获取数据库驱动标签类型
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
    ElMessage.error('加载后端数据失败: ' + (e.message || '未知错误'))
  }
}

function applyProxyConfig(data: any) {
  if (!data || typeof data !== 'object') return
  responseTraceBanner.value = !!data.response_trace_banner
}

async function load() {
  loading.value = true
  const gen = ++backendsLoadGen
  try {
    const sec = sections.value
    if (!sec.opsStats && !sec.serviceStatus && !sec.proxyControls) {
      const [statusRes, backendsRes, proxyCfgRes] = await Promise.allSettled([
        getStatus(),
        getBackends(),
        api.get('/api/v1/config/proxy')
      ])
      if (statusRes.status === 'fulfilled' && statusRes.value) {
        status.value = statusRes.value
        syncEditionFromStatus(statusRes.value)
      }
      if (backendsRes.status === 'fulfilled' && backendsRes.value && gen === backendsLoadGen) {
        backends.value = backendsRes.value || []
      }
      if (proxyCfgRes.status === 'fulfilled' && proxyCfgRes.value) {
        applyProxyConfig(proxyCfgRes.value)
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
      sec.proxyControls ? api.get('/api/v1/host-proxy/status') : Promise.resolve(null),
      api.get('/api/v1/config/proxy')
    ]
    const [dashRes, statusRes, backendsRes, storagesRes, pluginsRes, proxyRes, hostProxyRes, proxyCfgRes] = await Promise.allSettled(tasks)

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
    if (proxyCfgRes.status === 'fulfilled' && proxyCfgRes.value) {
      applyProxyConfig(proxyCfgRes.value)
    }

    pipelinePanelRef.value?.reload()

    // 添加历史数据点
    const now = new Date()
    const time = `${now.getHours().toString().padStart(2,'0')}:${now.getMinutes().toString().padStart(2,'0')}`
    const qps = dashboard.value.request?.qps || 0
    const latency = dashboard.value.request?.avg_latency_ms || 0
    const cache = dashboard.value.cache || {}
    const hitRateVal = cache.hits + cache.misses > 0 
      ? (cache.hits / (cache.hits + cache.misses)) * 100 
      : 0
    
    historyData.value.push({ time, qps, latency, hitRate: hitRateVal })
    
    // 根据时间范围保留数据
    const maxPoints = chartTimeRange.value === '1m' ? 6 : chartTimeRange.value === '5m' ? 30 : 90
    if (historyData.value.length > maxPoints) {
      historyData.value = historyData.value.slice(-maxPoints)
    }
  } catch (e: any) {
    ElMessage.error('加载数据失败: ' + (e.message || '未知错误'))
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

/* 图标色 */
.request-bg  { background: rgba(102,126,234,0.12); color: #667eea; }
.hit-bg      { background: rgba(16,185,129,0.12);  color: #10b981; }
.rate-bg     { background: rgba(245,158,11,0.12);  color: #f59e0b; }
.qps-bg      { background: rgba(59,130,246,0.12);  color: #3b82f6; }
.latency-bg  { background: rgba(139,92,246,0.12);  color: #8b5cf6; }
.error-bg    { background: rgba(239,68,68,0.12);   color: #ef4444; }
.entry-bg    { background: rgba(20,184,166,0.12);  color: #14b8a6; }

/* 统计网格（两行每行6/4列自适应） */
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

/* 主区布局：由 sections.layout 决定 */
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

.response-trace-toggle {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  margin-right: 4px;
  padding: 4px 10px;
  border-radius: 6px;
  background: var(--el-fill-color-light, #f5f7fa);
}

.response-trace-toggle .toggle-label {
  font-size: 13px;
  color: var(--el-text-color-regular, #606266);
  cursor: default;
  user-select: none;
}

.mt-card { margin-top: 16px; }

/* 信息卡片 */
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

/* Info rows */
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

/* 存储列表复用 backend-item 样式 */
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

/* 统计数字 */
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

/* 响应式 */
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

/* Plugin list */
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

/* 新增组件样式 */
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

/* 表格行悬停效果 */
:deep(.el-table__row:hover) {
  cursor: default;
}

/* 图表容器 */
:deep(.v-chart) {
  width: 100% !important;
}
</style>
