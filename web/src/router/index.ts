import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { getStatus } from '@/api'
import {
  isPersonalEdition,
  isMinimalEdition,
  isTeamOnlyRoute,
  isMinimalAllowedRoute,
  syncEditionFromStatus
} from '@/utils/edition'

// Lazy-load all page components for better chunking.
const Login = () => import('@/views/Login.vue')
const Dashboard = () => import('@/views/Dashboard.vue')
const Backends = () => import('@/views/Backends.vue')
const Config = () => import('@/views/Config.vue')
const Cache = () => import('@/views/Cache.vue')
const Evaluation = () => import('@/views/Evaluation.vue')
const Chat = () => import('@/views/Chat.vue')
const MinimalChat = () => import('@/views/MinimalChat.vue')
const Storage = () => import('@/views/Storage.vue')
const DataStores = () => import('@/views/DataStores.vue')
const HostProxy = () => import('@/views/HostProxy.vue')
const SystemProxy = () => import('@/views/SystemProxy.vue')
const Profile = () => import('@/views/Profile.vue')
const Users = () => import('@/views/system/Users.vue')
const SystemUpdate = () => import('@/views/system/SystemUpdate.vue')
const ClashRules = () => import('@/views/ClashRules.vue')
const LogViewer = () => import('@/views/LogViewer.vue')
const RequestTrace = () => import('@/views/RequestTrace.vue')
const TokenUsage = () => import('@/views/TokenUsage.vue')
const CostDashboard = () => import('@/views/CostDashboard.vue')
const AbComparison = () => import('@/views/AbComparison.vue')
const Memory = () => import('@/views/Memory.vue')
const ProxyModes = () => import('@/views/ProxyModes.vue')
const SessionMode = () => import('@/views/SessionMode.vue')
const NodePluginManager = () => import('@/views/pipeline/NodePluginManager.vue')
const PipelineModes = () => import('@/views/PipelineModes.vue')
const PluginRegistry = () => import('@/views/plugin/PluginRegistry.vue')
const TenantList = () => import('@/views/tenant/TenantList.vue')
const TenantMy = () => import('@/views/tenant/TenantMy.vue')
const AgentSetup = () => import('@/views/AgentSetup.vue')
const AgentProviders = () => import('@/views/AgentProviders.vue')
const VirtualKeys = () => import('@/views/VirtualKeys.vue')
const StorageKVBrowser = () => import('@/views/StorageKVBrowser.vue')
const Settings = () => import('@/views/Settings.vue')
const routes = [
  // ── Public ────────────────────────────────────────────────────────────────
  {
    path: '/login',
    name: 'Login',
    component: Login,
    meta: { title: '登录', public: true }
  },

  // ── Common (all authenticated users) ─────────────────────────────────────
  { path: '/', redirect: '/dashboard' },
  { path: '/dashboard', name: 'Dashboard', component: Dashboard, meta: { title: '概览' } },
  { path: '/backends', name: 'Backends', component: Backends, meta: { title: '后端管理' } },
  { path: '/settings', name: 'Settings', component: Settings, meta: { title: '设置' } },
  { path: '/cache', name: 'Cache', component: Cache, meta: { title: '缓存管理' } },
  { path: '/evaluation', name: 'Evaluation', component: Evaluation, meta: { title: '缓存评估管理' } },
  { path: '/chat', name: 'Chat', component: isMinimalEdition() ? MinimalChat : Chat, meta: { title: 'AI 对话' } },
  { path: '/agent-setup', name: 'AgentSetup', component: AgentSetup, meta: { title: 'Agent 快速接入' } },
  { path: '/agent-providers', name: 'AgentProviders', component: AgentProviders, meta: { title: 'Agent 供应商管理' } },
  {
    path: '/proxy-modes',
    name: 'ProxyModes',
    component: ProxyModes,
    meta: { title: '代理模式配置' }
  },
  {
    path: '/session-mode',
    name: 'SessionMode',
    component: SessionMode,
    meta: { title: '会话模式测试' }
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
    meta: { title: '策略管理' }
  },
  {
    path: '/pipelines/create',
    name: 'PipelineCreate',
    component: PipelineModes,
    meta: { title: '创建流水线' }
  },
  {
    path: '/pipelines/:id',
    name: 'PipelineEdit',
    component: PipelineModes,
    meta: { title: '编辑流水线' }
  },
  {
    path: '/pipeline/node-plugins',
    name: 'NodePluginManager',
    component: NodePluginManager,
    meta: { title: '节点插件管理' }
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
    meta: { title: '插件市场' }
  },

  // ── Tenant Management ────────────────────────────────────────────────────
  {
    path: '/my-tenant',
    name: 'TenantMy',
    component: TenantMy,
    meta: { title: '我的租户' }
  },
  {
    path: '/tenants',
    name: 'TenantList',
    component: TenantList,
    meta: { title: '租户管理', requiresAdmin: true }
  },
  { path: '/host-proxy', name: 'HostProxy', component: HostProxy, meta: { title: 'Host 代理管理' } },
  {
    path: '/system-proxy',
    name: 'SystemProxy',
    component: SystemProxy,
    meta: { title: '系统代理管理' }
  },
  { path: '/profile', name: 'Profile', component: Profile, meta: { title: '个人中心' } },
  { path: '/clash-rules', name: 'ClashRules', component: ClashRules, meta: { title: 'Clash 订阅管理' } },
  { path: '/logs', name: 'LogViewer', component: LogViewer, meta: { title: '日志查看器' } },
  {
    path: '/traces/:requestId',
    name: 'RequestTrace',
    component: RequestTrace,
    meta: { title: '请求追踪' }
  },
  { path: '/token-usage', name: 'TokenUsage', component: TokenUsage, meta: { title: 'Token 统计' } },
  {
    path: '/cost',
    name: 'CostDashboard',
    component: CostDashboard,
    meta: { title: '成本看板', requiresAdmin: true }
  },
  {
    path: '/ab-comparison',
    name: 'AbComparison',
    component: AbComparison,
    meta: { title: 'A/B 对比', requiresAdmin: true }
  },
  { path: '/memory', name: 'Memory', component: Memory, meta: { title: '云记忆管理' } },

  // ── Admin only ────────────────────────────────────────────────────────────
  {
    path: '/config',
    name: 'Config',
    component: Config,
    meta: { title: '系统配置', requiresAdmin: true }
  },
  {
    path: '/storage',
    name: 'Storage',
    component: Storage,
    meta: { title: '存储管理', requiresAdmin: true }
  },
  {
    path: '/storage/kv',
    name: 'StorageKVBrowser',
    component: StorageKVBrowser,
    meta: { title: 'KV 数据浏览', requiresAdmin: true }
  },
  {
    path: '/data-stores',
    name: 'DataStores',
    component: DataStores,
    meta: { title: '数据存储管理', requiresAdmin: true }
  },
  {
    path: '/system/users',
    name: 'Users',
    component: Users,
    meta: { title: '用户管理', requiresAdmin: true }
  },
  {
    path: '/system/update',
    name: 'SystemUpdate',
    component: SystemUpdate,
    meta: { title: '系统更新', requiresAdmin: true }
  },
  {
    path: '/system/virtual-keys',
    name: 'VirtualKeys',
    component: VirtualKeys,
    meta: { title: '虚拟密钥管理', requiresAdmin: true }
  },

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
  document.title = to.meta.title ? `${to.meta.title} - Centag` : 'Centag'

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

  // Personal / team edition guards.
  if (isPersonalEdition() && isTeamOnlyRoute(to.path)) {
    return next('/dashboard')
  }

  // Minimal lite: only allow a small route set.
  if (isMinimalEdition() && !isMinimalAllowedRoute(to.path)) {
    return next('/dashboard')
  }

  next()
})

export default router
