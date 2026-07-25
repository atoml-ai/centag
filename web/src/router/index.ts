import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { getStatus } from '@/api'
import {
  currentEdition,
  resolveEditionRouteRedirect,
  syncEditionFromStatus
} from '@/utils/edition'
import { teamPackRoutes } from '@team-pack'
import { applyDocumentTitle } from '@/i18n/document-title'

// Lazy-load all page components for better chunking.
const Login = () => import('@/views/Login.vue')
const Dashboard = () => import('@/views/Dashboard.vue')
const Backends = () => import('@/views/Backends.vue')
const Config = () => import('@/views/Config.vue')
const Cache = () => import('@/views/Cache.vue')
const Evaluation = () => import('@/views/Evaluation.vue')
const Chat = () => import('@/views/Chat.vue')
const Storage = () => import('@/views/Storage.vue')
const DataStores = () => import('@/views/DataStores.vue')
const HostProxy = () => import('@/views/HostProxy.vue')
const SystemProxy = () => import('@/views/SystemProxy.vue')
const Profile = () => import('@/views/Profile.vue')
const ClashRules = () => import('@/views/ClashRules.vue')
const LogViewer = () => import('@/views/LogViewer.vue')
const RequestTrace = () => import('@/views/RequestTrace.vue')
const TokenUsage = () => import('@/views/TokenUsage.vue')
const Conversations = () => import('@/views/Conversations.vue')
const CostDashboard = () => import('@/views/CostDashboard.vue')
const BillingRules = () => import('@/views/BillingRules.vue')
const Memory = () => import('@/views/Memory.vue')
const ProxyModes = () => import('@/views/ProxyModes.vue')
const SessionMode = () => import('@/views/SessionMode.vue')
const NodePluginManager = () => import('@/views/pipeline/NodePluginManager.vue')
const PipelineModes = () => import('@/views/PipelineModes.vue')
const PluginRegistry = () => import('@/views/plugin/PluginRegistry.vue')
const AgentSetup = () => import('@/views/AgentSetup.vue')
const AgentRun = () => import('@/views/AgentRun.vue')
const StorageKVBrowser = () => import('@/views/StorageKVBrowser.vue')

const routes = [
  // ── Public ────────────────────────────────────────────────────────────────
  {
    path: '/login',
    name: 'Login',
    component: Login,
    meta: { titleKey: 'route.login', public: true }
  },

  // ── Common (all authenticated users) ─────────────────────────────────────
  { path: '/', redirect: '/dashboard' },
  { path: '/dashboard', name: 'Dashboard', component: Dashboard, meta: { titleKey: 'route.dashboard' } },
  { path: '/backends', name: 'Backends', component: Backends, meta: { titleKey: 'route.backends' } },
  // 降级策略已并入系统配置 → 韧性 → 降级策略
  {
    path: '/fallback-policies',
    redirect: { path: '/config', query: { tab: 'resilience', sub: 'fallback' } }
  },
  // 旧 Minimal 设置页已废弃：改密/服务信息并入个人中心与首页
  { path: '/settings', redirect: '/profile' },
  { path: '/cache', name: 'Cache', component: Cache, meta: { titleKey: 'route.cache' } },
  { path: '/evaluation', name: 'Evaluation', component: Evaluation, meta: { titleKey: 'route.evaluation' } },
  { path: '/chat', name: 'Chat', component: Chat, meta: { titleKey: 'route.chat' } },
  { path: '/agent-setup', name: 'AgentSetup', component: AgentSetup, meta: { titleKey: 'route.agentSetup' } },
  { path: '/agent-run', name: 'AgentRun', component: AgentRun, meta: { titleKey: 'route.agentRun' } },
  { path: '/agent-providers', redirect: '/agent-setup' },
  {
    path: '/proxy-modes',
    name: 'ProxyModes',
    component: ProxyModes,
    meta: { titleKey: 'route.proxyModes' }
  },
  {
    path: '/session-mode',
    name: 'SessionMode',
    component: SessionMode,
    meta: { titleKey: 'route.sessionMode' }
  },
  {
    path: '/pipeline-modes',
    redirect: '/pipelines'
  },
  // ── Pipeline ─────────────────────────────────────────────────────────────
  {
    path: '/pipelines',
    name: 'PipelineList',
    component: PipelineModes,
    meta: { titleKey: 'route.pipelines' }
  },
  {
    path: '/pipelines/create',
    name: 'PipelineCreate',
    component: PipelineModes,
    meta: { titleKey: 'route.pipelineCreate' }
  },
  {
    path: '/pipelines/:id',
    name: 'PipelineEdit',
    component: PipelineModes,
    meta: { titleKey: 'route.pipelineEdit' }
  },
  {
    path: '/pipeline/node-plugins',
    name: 'NodePluginManager',
    component: NodePluginManager,
    meta: { titleKey: 'route.nodePlugins' }
  },
  {
    path: '/pipeline/defaults',
    redirect: '/dashboard'
  },

  // ── Plugin Registry ──────────────────────────────────────────────────────
  {
    path: '/plugins',
    name: 'PluginRegistry',
    component: PluginRegistry,
    meta: { titleKey: 'route.plugins' }
  },

  { path: '/host-proxy', name: 'HostProxy', component: HostProxy, meta: { titleKey: 'route.hostProxy' } },
  {
    path: '/system-proxy',
    name: 'SystemProxy',
    component: SystemProxy,
    meta: { titleKey: 'route.systemProxy' }
  },
  { path: '/profile', name: 'Profile', component: Profile, meta: { titleKey: 'route.profile' } },
  { path: '/clash-rules', name: 'ClashRules', component: ClashRules, meta: { titleKey: 'route.clashRules' } },
  { path: '/logs', name: 'LogViewer', component: LogViewer, meta: { titleKey: 'route.logs' } },
  {
    path: '/traces/:requestId',
    name: 'RequestTrace',
    component: RequestTrace,
    meta: { titleKey: 'route.requestTrace' }
  },
  { path: '/token-usage', name: 'TokenUsage', component: TokenUsage, meta: { titleKey: 'route.tokenUsage' } },
  {
    path: '/conversations',
    name: 'Conversations',
    component: Conversations,
    meta: { titleKey: 'route.conversations' }
  },

  {
    path: '/billing',
    name: 'BillingRules',
    component: BillingRules,
    meta: { titleKey: 'route.billing', requiresAdmin: true }
  },
  {
    path: '/cost',
    name: 'CostDashboard',
    component: CostDashboard,
    meta: { titleKey: 'route.cost', requiresAdmin: true }
  },
  { path: '/memory', name: 'Memory', component: Memory, meta: { titleKey: 'route.memory' } },

  // ── Admin only ────────────────────────────────────────────────────────────
  {
    path: '/config',
    name: 'Config',
    component: Config,
    meta: { titleKey: 'route.config', requiresAdmin: true }
  },
  {
    path: '/storage',
    name: 'Storage',
    component: Storage,
    meta: { titleKey: 'route.storage' }
  },
  {
    path: '/storage/kv',
    name: 'StorageKVBrowser',
    component: StorageKVBrowser,
    meta: { titleKey: 'route.storageKv' }
  },
  {
    path: '/data-stores',
    name: 'DataStores',
    component: DataStores,
    meta: { titleKey: 'route.dataStores' }
  },
  // E3: team-only pages come from @team-pack (stub empty for personal/minimal).
  ...teamPackRoutes,
  { path: '/:pathMatch(.*)*', name: 'NotFound', redirect: '/dashboard' }
]

const router = createRouter({
  history: createWebHistory('/static/'),
  routes,
  scrollBehavior(to, from, savedPosition) {
    if (savedPosition) return savedPosition
    return { top: 0 }
  }
})

// ── Navigation guard ─────────────────────────────────────────────────────────
// Tracks whether the session restore attempt has been made this page load.
let sessionRestored = false
let editionSynced = false

router.beforeEach(async (to, _from, next) => {
  applyDocumentTitle(to.meta.titleKey)

  if (!editionSynced) {
    editionSynced = true
    try {
      const status = await getStatus()
      syncEditionFromStatus(status)
    } catch {
      // keep DOM / default edition
    }
  }

  // Always allow public routes.
  if (to.meta.public) {
    return next()
  }

  const authStore = useAuthStore()

  // On first navigation after a page load, try to restore session from the
  // stored refresh token before deciding whether to redirect to /login.
  if (!sessionRestored) {
    sessionRestored = true
    if (!authStore.isAuthenticated) {
      await authStore.restoreSession()
    }
  }

  if (!authStore.isAuthenticated) {
    return next({ path: '/login', query: { redirect: to.fullPath } })
  }

  // Admin-only routes.
  if (to.meta.requiresAdmin && !authStore.isAdmin) {
    return next('/dashboard')
  }

  // Capability / edition 路由意图（列表 redirect、深链放行、无独立对话页等）
  const redirect = resolveEditionRouteRedirect(to.path, currentEdition(), authStore.isAdmin)
  if (redirect) {
    return next(redirect)
  }

  next()
})

export default router
