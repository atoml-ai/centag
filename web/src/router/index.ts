import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { getStatus } from '@/api'
import {
  currentEdition,
  resolveEditionRouteRedirect,
  syncEditionFromStatus
} from '@/utils/edition'
import { teamPackRoutes } from '@team-pack'

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
const StorageKVBrowser = () => import('@/views/StorageKVBrowser.vue')
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
  // 降级策略已并入系统配置 → 韧性 → 降级策略
  {
    path: '/fallback-policies',
    redirect: { path: '/config', query: { tab: 'resilience', sub: 'fallback' } }
  },
  // 旧 Minimal 设置页已废弃：改密/服务信息并入个人中心与首页
  { path: '/settings', redirect: '/profile' },
  { path: '/cache', name: 'Cache', component: Cache, meta: { title: '缓存管理' } },
  { path: '/evaluation', name: 'Evaluation', component: Evaluation, meta: { title: '缓存评估管理' } },
  { path: '/chat', name: 'Chat', component: Chat, meta: { title: 'AI 对话' } },
  { path: '/agent-setup', name: 'AgentSetup', component: AgentSetup, meta: { title: 'Agent 接入' } },
  { path: '/agent-providers', redirect: '/agent-setup' },
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
  { path: '/token-usage', name: 'TokenUsage', component: TokenUsage, meta: { title: '用量与计费' } },
  {
    path: '/conversations',
    name: 'Conversations',
    component: Conversations,
    meta: { title: '会话记录' }
  },

  {
    path: '/billing',
    name: 'BillingRules',
    component: BillingRules,
    meta: { title: '计费规则', requiresAdmin: true }
  },
  {
    path: '/cost',
    name: 'CostDashboard',
    component: CostDashboard,
    meta: { title: '成本看板', requiresAdmin: true }
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
    meta: { title: '存储管理' }
  },
  {
    path: '/storage/kv',
    name: 'StorageKVBrowser',
    component: StorageKVBrowser,
    meta: { title: 'KV 数据浏览' }
  },
  {
    path: '/data-stores',
    name: 'DataStores',
    component: DataStores,
    meta: { title: '数据存储管理' }
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

  // Capability / edition 路由意图（列表 redirect、深链放行、无独立对话页等）
  const redirect = resolveEditionRouteRedirect(to.path, currentEdition(), authStore.isAdmin)
  if (redirect) {
    return next(redirect)
  }

  next()
})

export default router
