import type { Capabilities } from '@/utils/capabilities'
import type { NavItem } from './types'

/**
 * 导航叶子与分组工厂（personal / team / minimal 共用，避免重复 path/icon）。
 * 普通用户菜单由 buildWorkerNav(capabilities) 同源生成。
 */

type LeafOpts = { requiresAdmin?: boolean; requiresTeam?: boolean; label?: string }

function withOpts(item: NavItem, opts?: LeafOpts): NavItem {
  if (!opts) return item
  return {
    ...item,
    ...(opts.label ? { label: opts.label } : {}),
    ...(opts.requiresAdmin ? { requiresAdmin: true } : {}),
    ...(opts.requiresTeam ? { requiresTeam: true } : {})
  }
}

export function dashboardNav(label: '首页' | '概览'): NavItem {
  return { id: 'dashboard', label, icon: 'Home', path: '/dashboard' }
}

export function backendsNav(opts?: LeafOpts): NavItem {
  return withOpts({ id: 'backends', label: '后端', icon: 'Connection', path: '/backends' }, opts)
}

export function fallbackPolicyNav(opts?: LeafOpts): NavItem {
  return withOpts({ id: 'fallback-policies', label: '降级策略', icon: 'Switch', path: '/fallback-policies' }, opts)
}

export function pipelinesNav(opts?: LeafOpts): NavItem {
  return withOpts({ id: 'pipelines', label: '策略', icon: 'Share', path: '/pipelines' }, opts)
}

export function agentSetupNav(opts?: LeafOpts): NavItem {
  return withOpts({ id: 'agent-setup', label: 'Agent 接入', icon: 'Link', path: '/agent-setup' }, opts)
}

export function agentProvidersNav(opts?: LeafOpts): NavItem {
  return withOpts(
    { id: 'agent-providers', label: 'Agent 供应商', icon: 'Connection', path: '/agent-providers' },
    opts
  )
}

export function nodePluginsNav(opts?: LeafOpts): NavItem {
  return withOpts(
    {
      id: 'node-plugins',
      label: '节点插件',
      icon: 'Connection',
      path: '/pipeline/node-plugins'
    },
    opts
  )
}

/** @deprecated 独立对话导航已取消；仅保留供兼容引用 */
export function chatNav(): NavItem {
  return { id: 'chat', label: '对话', icon: 'ChatDotRound', path: '/chat' }
}

export function conversationsNav(): NavItem {
  return { id: 'conversations', label: '会话记录', icon: 'ChatLineSquare', path: '/conversations' }
}

export function tokenUsageNav(label = '用量统计'): NavItem {
  return { id: 'token-usage', label, icon: 'TrendCharts', path: '/token-usage' }
}

/** @deprecated 计费规则已并入「用量与计费」页 */
export function billingRulesNav(): NavItem {
  return {
    id: 'billing-rules',
    label: '计费规则',
    icon: 'Coin',
    path: '/token-usage',
    requiresAdmin: true
  }
}

export function costDashboardNav(): NavItem {
  return {
    id: 'cost-dashboard',
    label: '成本看板',
    icon: 'Coin',
    path: '/cost',
    requiresAdmin: true
    // D1: not requiresTeam — personal admin can open /cost
  }
}

export function abComparisonNav(): NavItem {
  return {
    id: 'ab-comparison',
    label: 'A/B 对比',
    icon: 'DataAnalysis',
    path: '/ab-comparison',
    requiresAdmin: true,
    requiresTeam: true
  }
}

export function logsNav(): NavItem {
  return { id: 'logs', label: '日志', icon: 'Document', path: '/logs' }
}

export function cacheNav(opts?: LeafOpts): NavItem {
  return withOpts({ id: 'cache', label: '缓存监控', icon: 'Coin', path: '/cache' }, opts)
}

export function evaluationNav(opts?: LeafOpts): NavItem {
  return withOpts({ id: 'evaluation', label: '缓存评估', icon: 'TrendCharts', path: '/evaluation' }, opts)
}

export function storageNav(opts?: LeafOpts): NavItem {
  return withOpts({ id: 'storage', label: '存储管理', icon: 'FolderOpened', path: '/storage' }, opts)
}

export function dataStoresNav(opts?: LeafOpts): NavItem {
  return withOpts({ id: 'data-stores', label: '数据存储管理', icon: 'Coin', path: '/data-stores' }, opts)
}

export function memoryNav(): NavItem {
  return { id: 'memory', label: '记忆', icon: 'Folder', path: '/memory' }
}

export function hostProxyNav(opts?: LeafOpts): NavItem {
  return withOpts({ id: 'host-proxy', label: '主机代理', icon: 'Link', path: '/host-proxy' }, opts)
}

export function systemProxyNav(opts?: LeafOpts): NavItem {
  return withOpts({ id: 'system-proxy', label: '系统代理', icon: 'Connection', path: '/system-proxy' }, opts)
}

export function clashRulesNav(opts?: LeafOpts): NavItem {
  return withOpts({ id: 'clash-rules', label: 'Clash 规则', icon: 'Document', path: '/clash-rules' }, opts)
}

export function configNav(opts?: LeafOpts): NavItem {
  return withOpts({ id: 'config-basic', label: '系统配置', icon: 'Setting', path: '/config' }, opts)
}

/** 通用分组（可嵌套） */
export function navGroup(
  id: string,
  label: string,
  icon: string,
  children: NavItem[],
  path?: string
): NavItem {
  return {
    id,
    label,
    icon,
    path: path ?? children[0]?.path,
    children
  }
}

/** 用量：会话 + 用量与计费 */
export function usageNavGroup(): NavItem {
  return navGroup(
    'usage',
    '用量',
    'TrendCharts',
    [tokenUsageNav('用量与计费'), conversationsNav()],
    '/token-usage'
  )
}

/** 接入：系统代理 + Agent 快速接入 */
export function accessNavGroup(): NavItem {
  return navGroup(
    'access',
    '接入',
    'Link',
    [systemProxyNav(), agentSetupNav()],
    '/system-proxy'
  )
}

export function storageConfigNavGroup(options?: {
  includeDataStores?: boolean
  includeEvaluation?: boolean
}): NavItem {
  const includeDataStores = options?.includeDataStores ?? true
  const includeEvaluation = options?.includeEvaluation ?? true
  const children: NavItem[] = [storageNav()]
  if (includeDataStores) children.push(dataStoresNav())
  children.push(cacheNav())
  if (includeEvaluation) children.push(evaluationNav())
  return navGroup('storage-config', '存储配置', 'FolderOpened', children, children[0]?.path)
}

/**
 * 「更多」实验入口收纳区（存储/记忆/本机高级代理/日志等）。
 * personal：条目收齐后整体不挂顶栏（navMoreMenu=false），系统配置改走用户菜单。
 */
export function buildMoreNavChildren(caps: Capabilities): NavItem[] {
  const moreChildren: NavItem[] = []

  // personal：把未成熟入口也收纳进 more 结构，便于日后整体开放顶栏「更多」
  const stashExperimental = caps.role === 'personal'

  if (caps.storageConfig) {
    moreChildren.push(
      storageConfigNavGroup({
        includeDataStores: caps.navDataStores || stashExperimental,
        includeEvaluation: caps.navEvaluation || stashExperimental
      })
    )
  }
  if (caps.memoryQuery || stashExperimental) {
    moreChildren.push(memoryNav())
  }
  if (caps.navHostProxyTools || stashExperimental) {
    moreChildren.push(hostProxyNav())
    moreChildren.push(clashRulesNav())
  }
  moreChildren.push(logsNav())

  const systemChildren: NavItem[] = []
  if (caps.myTenant) {
    systemChildren.push({
      id: 'my-tenant',
      label: '我的租户',
      icon: 'OfficeBuilding',
      path: '/my-tenant',
      requiresTeam: true
    })
  }
  // 系统配置：personal 走右上角用户菜单，不进「更多」
  if (caps.systemConfig && caps.role !== 'personal') {
    systemChildren.push(configNav())
  }
  // 独立降级导航仅 team_admin 等仍可能使用；personal 已并入系统配置韧性页
  if (caps.navFallbackPolicy) {
    systemChildren.push(fallbackPolicyNav())
  }
  if (systemChildren.length) {
    moreChildren.push(
      navGroup('more-system', '系统', 'Setting', systemChildren, systemChildren[0]?.path)
    )
  }

  return moreChildren
}

/**
 * Personal / Team User 同源导航（由 capabilities 裁剪节点）。
 * 无独立「对话」；无侧栏后端/策略列表（主入口在首页）。
 */
export function buildWorkerNav(caps: Capabilities): NavItem[] {
  const items: NavItem[] = [dashboardNav('首页')]

  if (caps.usageBilling) {
    items.push(usageNavGroup())
  }
  if (caps.localProxy || caps.agentSetup) {
    items.push(accessNavGroup())
  }

  if (caps.navMoreMenu) {
    const moreChildren = buildMoreNavChildren(caps)
    if (moreChildren.length) {
      items.push(navGroup('more', '更多', 'MoreFilled', moreChildren, moreChildren[0]?.path))
    }
  }

  return items
}

/** @deprecated 使用 buildWorkerNav(getCapabilities(...)) */
export function personalConfigGroup(): NavItem {
  return navGroup(
    'personal-config',
    '配置',
    'Connection',
    [backendsNav({ label: '后端管理' }), pipelinesNav({ label: '策略管理' })],
    '/backends'
  )
}

/** @deprecated */
export function personalAppGroup(): NavItem {
  return navGroup(
    'personal-app',
    '应用',
    'Grid',
    [conversationsNav(), tokenUsageNav('用量与计费'), logsNav()],
    '/conversations'
  )
}

/** @deprecated */
export function personalMoreGroup(options?: { teamUser?: boolean }): NavItem {
  // 兼容旧调用：直接拼 more 子树（不依赖顶栏是否展示）
  const children = buildMoreNavChildren({
    role: options?.teamUser ? 'team_user' : 'personal',
    manageBackends: true,
    managePipelines: true,
    homeBackendsPanel: true,
    homePipelinesPanel: true,
    navBackendsPage: false,
    navPipelinesPage: false,
    pipelineTestChat: true,
    navChatPage: false,
    localProxy: true,
    storageConfig: !options?.teamUser,
    navMoreMenu: true,
    navHostProxyTools: false,
    navDataStores: false,
    navEvaluation: false,
    navFallbackPolicy: false,
    memoryQuery: !!options?.teamUser,
    memoryFull: false,
    usageBilling: true,
    agentSetup: true,
    systemConfig: !options?.teamUser,
    myTenant: !!options?.teamUser,
    userAdmin: false,
    liteHome: true
  })
  return navGroup('more', '更多', 'MoreFilled', children, children[0]?.path)
}

/** 代理策略：后端 + 流水线；团队版含节点插件且仅管理员可见 */
export function proxyStrategyGroup(options?: { includeNodePlugins?: boolean; adminOnly?: boolean }): NavItem {
  const admin = options?.adminOnly ? ({ requiresAdmin: true } as LeafOpts) : undefined
  const children: NavItem[] = [
    backendsNav({ ...admin, label: '后端管理' }),
    pipelinesNav({ ...admin, label: '策略管理' }),
    agentSetupNav(admin),
    agentProvidersNav(admin)
  ]
  if (options?.includeNodePlugins) {
    children.push(nodePluginsNav())
  }
  return {
    id: 'proxy-strategy',
    label: '代理策略',
    icon: 'Connection',
    path: '/backends',
    ...(options?.adminOnly ? { requiresAdmin: true } : {}),
    children
  }
}

export function appGroup(options?: { tokenUsageLabel?: string }): NavItem {
  return {
    id: 'app',
    label: '应用',
    icon: 'Grid',
    path: '/token-usage',
    children: [
      conversationsNav(),
      tokenUsageNav(options?.tokenUsageLabel ?? '用量与计费'),
      costDashboardNav(),
      abComparisonNav(),
      logsNav()
    ]
  }
}

/** 缓存与记忆：云记忆全员可见；缓存/存储类仅管理员（团队 Web） */
export function cacheMemoryGroup(options?: { adminTools?: boolean }): NavItem {
  const admin = options?.adminTools ? ({ requiresAdmin: true } as LeafOpts) : undefined
  return {
    id: 'cache-memory',
    label: '缓存与记忆',
    icon: 'Coin',
    path: '/memory',
    children: [cacheNav(admin), evaluationNav(admin), storageNav(admin), dataStoresNav(admin), memoryNav()]
  }
}

export function accessGroup(options?: { adminOnly?: boolean }): NavItem {
  const admin = options?.adminOnly ? ({ requiresAdmin: true } as LeafOpts) : undefined
  return {
    id: 'access',
    label: '接入',
    icon: 'Link',
    path: '/host-proxy',
    ...(options?.adminOnly ? { requiresAdmin: true } : {}),
    children: [hostProxyNav(admin), systemProxyNav(admin), clashRulesNav(admin)]
  }
}

/** 系统管理：团队 Web 含租户/更新；桌面版用户菜单为子集（配置项全员可见） */
export function systemAdminGroup(options?: { teamExtras?: boolean; relaxedAccess?: boolean }): NavItem {
  const admin = !(options?.relaxedAccess ?? false)
  const leafAdmin = admin ? ({ requiresAdmin: true } as LeafOpts) : undefined
  const children: NavItem[] = [
    {
      id: 'my-tenant',
      label: '我的租户',
      icon: 'OfficeBuilding',
      path: '/my-tenant',
      requiresTeam: true
    },
    configNav(leafAdmin),
    {
      id: 'system-users',
      label: '用户管理',
      icon: 'UserFilled',
      path: '/system/users',
      requiresAdmin: true,
      requiresTeam: true
    }
  ]
  if (options?.teamExtras) {
    children.push(
      {
        id: 'tenants',
        label: '租户管理',
        icon: 'OfficeBuilding',
        path: '/tenants',
        requiresAdmin: true,
        requiresTeam: true
      },
      {
        id: 'system-update',
        label: '系统更新',
        icon: 'Upload',
        path: '/system/update',
        requiresAdmin: true,
        requiresTeam: true
      }
    )
  }
  return {
    id: 'system-admin',
    label: '系统管理',
    icon: 'Setting',
    path: '/config',
    ...(admin ? { requiresAdmin: true } : {}),
    children
  }
}
