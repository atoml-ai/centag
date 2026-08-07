import type { Capabilities } from '@/utils/capabilities'
import type { NavItem } from './types'

/**
 * 导航叶子与分组工厂（personal / team / minimal 共用，避免重复 path/icon）。
 * 普通用户菜单由 buildWorkerNav(capabilities) 同源生成。
 */

type LeafOpts = { requiresAdmin?: boolean; requiresTeam?: boolean; labelKey?: string }

function withOpts(item: NavItem, opts?: LeafOpts): NavItem {
  if (!opts) return item
  return {
    ...item,
    ...(opts.labelKey ? { labelKey: opts.labelKey } : {}),
    ...(opts.requiresAdmin ? { requiresAdmin: true } : {}),
    ...(opts.requiresTeam ? { requiresTeam: true } : {})
  }
}

export function dashboardNav(labelKey: 'nav.dashboard' | 'nav.overview' = 'nav.dashboard'): NavItem {
  return { id: 'dashboard', labelKey, icon: 'House', path: '/dashboard' }
}

export function backendsNav(opts?: LeafOpts): NavItem {
  return withOpts({ id: 'backends', labelKey: 'nav.backends', icon: 'Connection', path: '/backends' }, opts)
}

export function fallbackPolicyNav(opts?: LeafOpts): NavItem {
  return withOpts({ id: 'fallback-policies', labelKey: 'nav.fallbackPolicies', icon: 'Switch', path: '/fallback-policies' }, opts)
}

export function pipelinesNav(opts?: LeafOpts): NavItem {
  return withOpts({ id: 'pipelines', labelKey: 'nav.pipelines', icon: 'Share', path: '/pipelines' }, opts)
}

export function agentSetupNav(opts?: LeafOpts): NavItem {
  return withOpts({ id: 'agent-setup', labelKey: 'nav.agentSetup', icon: 'Link', path: '/agent-setup' }, opts)
}

export function agentProvidersNav(opts?: LeafOpts): NavItem {
  return withOpts(
    { id: 'agent-providers', labelKey: 'nav.agentProviders', icon: 'Connection', path: '/agent-providers' },
    opts
  )
}

export function nodePluginsNav(opts?: LeafOpts): NavItem {
  return withOpts(
    {
      id: 'node-plugins',
      labelKey: 'nav.nodePlugins',
      icon: 'Connection',
      path: '/pipeline/node-plugins'
    },
    opts
  )
}

/** @deprecated 独立对话导航已取消；仅保留供兼容引用 */
export function chatNav(): NavItem {
  return { id: 'chat', labelKey: 'nav.chat', icon: 'ChatDotRound', path: '/chat' }
}

export function conversationsNav(): NavItem {
  return { id: 'conversations', labelKey: 'nav.conversations', icon: 'ChatLineSquare', path: '/conversations' }
}

export function tokenUsageNav(labelKey = 'nav.tokenUsage'): NavItem {
  return { id: 'token-usage', labelKey, icon: 'TrendCharts', path: '/token-usage' }
}

/** @deprecated 计费规则已并入「用量与计费」页 */
export function billingRulesNav(): NavItem {
  return {
    id: 'billing-rules',
    labelKey: 'nav.billingRules',
    icon: 'Coin',
    path: '/token-usage',
    requiresAdmin: true
  }
}

export function costDashboardNav(): NavItem {
  return {
    id: 'cost-dashboard',
    labelKey: 'nav.costDashboard',
    icon: 'Coin',
    path: '/cost',
    requiresAdmin: true
    // D1: not requiresTeam — personal admin can open /cost
  }
}

export function abComparisonNav(): NavItem {
  return {
    id: 'ab-comparison',
    labelKey: 'nav.abComparison',
    icon: 'DataAnalysis',
    path: '/ab-comparison',
    requiresAdmin: true,
    requiresTeam: true
  }
}

export function logsNav(): NavItem {
  return { id: 'logs', labelKey: 'nav.logs', icon: 'Document', path: '/logs' }
}

export function cacheNav(opts?: LeafOpts): NavItem {
  return withOpts({ id: 'cache', labelKey: 'nav.cache', icon: 'Coin', path: '/cache' }, opts)
}

export function evaluationNav(opts?: LeafOpts): NavItem {
  return withOpts({ id: 'evaluation', labelKey: 'nav.evaluation', icon: 'TrendCharts', path: '/evaluation' }, opts)
}

export function storageNav(opts?: LeafOpts): NavItem {
  return withOpts({ id: 'storage', labelKey: 'nav.storage', icon: 'FolderOpened', path: '/storage' }, opts)
}

export function dataStoresNav(opts?: LeafOpts): NavItem {
  return withOpts({ id: 'data-stores', labelKey: 'nav.dataStores', icon: 'Coin', path: '/data-stores' }, opts)
}

export function memoryNav(): NavItem {
  return { id: 'memory', labelKey: 'nav.memory', icon: 'Folder', path: '/memory' }
}

export function hostProxyNav(opts?: LeafOpts): NavItem {
  return withOpts({ id: 'host-proxy', labelKey: 'nav.hostProxy', icon: 'Link', path: '/host-proxy' }, opts)
}

export function systemProxyNav(opts?: LeafOpts): NavItem {
  return withOpts({ id: 'system-proxy', labelKey: 'nav.systemProxy', icon: 'Connection', path: '/system-proxy' }, opts)
}

export function clashRulesNav(opts?: LeafOpts): NavItem {
  return withOpts({ id: 'clash-rules', labelKey: 'nav.clashRules', icon: 'Document', path: '/clash-rules' }, opts)
}

export function configNav(opts?: LeafOpts): NavItem {
  return withOpts({ id: 'config-basic', labelKey: 'nav.config', icon: 'Setting', path: '/config' }, opts)
}

/** 通用分组（可嵌套） */
export function navGroup(
  id: string,
  labelKey: string,
  icon: string,
  children: NavItem[],
  path?: string
): NavItem {
  return {
    id,
    labelKey,
    icon,
    path: path ?? children[0]?.path,
    children
  }
}

/** 用量：会话记录 → 计量计费（Team 合并页）/ 用量统计（Personal） */
export function usageNavGroup(caps?: { role?: string }): NavItem {
  const isTeam = caps?.role === 'team_user' || caps?.role === 'team_admin'
  const children: NavItem[] = [conversationsNav()]
  if (isTeam) {
    children.push({
      id: 'metering-billing',
      labelKey: 'nav.meteringBilling',
      icon: 'Wallet',
      path: '/metering-billing',
      requiresTeam: true
    })
  } else {
    children.push(tokenUsageNav('nav.tokenUsage'))
  }
  return navGroup('usage', 'nav.usage', 'TrendCharts', children, children[0]?.path)
}

/** 接入：系统代理 + Agent 配置 */
export function accessNavGroup(): NavItem {
  return navGroup(
    'access',
    'nav.access',
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
  return navGroup('storage-config', 'nav.cacheManagement', 'FolderOpened', children, children[0]?.path)
}

/** 缓存管理：缓存 + 存储 + 数据存储 + 缓存评估（team admin 专用） */
export function cacheManagementNavGroup(): NavItem {
  const children: NavItem[] = [
    cacheNav(),
    storageNav(),
    dataStoresNav(),
    evaluationNav()
  ]
  return navGroup('cache-management', 'nav.cacheManagement', 'FolderOpened', children, children[0]?.path)
}

/** 用户与组管理：用户/组 + 额度模板 + 成本看板 + 计费规则（team admin 专用） */
export function userTenantGroup(): NavItem {
  return navGroup(
    'user-tenant',
    'nav.userGroupManagement',
    'UserFilled',
    [
      {
        id: 'system-users',
        labelKey: 'nav.users',
        icon: 'UserFilled',
        path: '/system/users',
        requiresAdmin: true,
        requiresTeam: true
      },
      {
        id: 'plan-templates',
        labelKey: 'nav.planTemplates',
        icon: 'Ticket',
        path: '/billing/plan-templates',
        requiresAdmin: true,
        requiresTeam: true
      },
      costDashboardNav(),
      {
        id: 'billing-statements',
        labelKey: 'nav.billingStatements',
        icon: 'Document',
        path: '/billing/statements',
        requiresAdmin: true,
        requiresTeam: true
      },
      {
        id: 'billing-rules',
        labelKey: 'nav.billingRules',
        icon: 'Coin',
        path: '/billing',
        requiresAdmin: true,
        requiresTeam: true
      }
    ],
    '/system/users'
  )
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
      navGroup('more-system', 'nav.system', 'Setting', systemChildren, systemChildren[0]?.path)
    )
  }

  return moreChildren
}

/**
 * Personal / Team User 同源导航（由 capabilities 裁剪节点）。
 * 无独立「对话」；无侧栏后端/策略列表（主入口在首页）。
 */
export function buildWorkerNav(caps: Capabilities): NavItem[] {
  const items: NavItem[] = [dashboardNav('nav.dashboard')]

  if (caps.usageBilling) {
    items.push(usageNavGroup(caps))
  }
  if (caps.localProxy || caps.agentSetup) {
    items.push(accessNavGroup())
  }

  if (caps.navMoreMenu) {
    const moreChildren = buildMoreNavChildren(caps)
    if (moreChildren.length) {
      items.push(navGroup('more', 'nav.more', 'MoreFilled', moreChildren, moreChildren[0]?.path))
    }
  }

  return items
}

/** @deprecated 使用 buildWorkerNav(getCapabilities(...)) */
export function personalConfigGroup(): NavItem {
  return navGroup(
    'personal-config',
    'nav.proxyStrategy',
    'Connection',
    [backendsNav({ labelKey: 'nav.backends' }), pipelinesNav({ labelKey: 'nav.pipelines' })],
    '/backends'
  )
}

/** @deprecated */
export function personalAppGroup(): NavItem {
  return navGroup(
    'personal-app',
    'nav.application',
    'Grid',
    [conversationsNav(), tokenUsageNav('nav.tokenUsage'), logsNav()],
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
    userAdmin: false,
    liteHome: true
  })
  return navGroup('more', 'nav.more', 'MoreFilled', children, children[0]?.path)
}

/** 代理策略：后端 + 流水线；团队版含节点插件且仅管理员可见 */
export function proxyStrategyGroup(options?: { includeNodePlugins?: boolean; adminOnly?: boolean }): NavItem {
  const admin = options?.adminOnly ? ({ requiresAdmin: true } as LeafOpts) : undefined
  const children: NavItem[] = [
    backendsNav({ ...admin, labelKey: 'nav.backends' }),
    pipelinesNav({ ...admin, labelKey: 'nav.pipelines' }),
    agentSetupNav(admin),
    agentProvidersNav(admin)
  ]
  if (options?.includeNodePlugins) {
    children.push(nodePluginsNav())
  }
  return {
    id: 'proxy-strategy',
    labelKey: 'nav.proxyStrategy',
    icon: 'Connection',
    path: '/backends',
    ...(options?.adminOnly ? { requiresAdmin: true } : {}),
    children
  }
}

export function appGroup(options?: { tokenUsageLabel?: string }): NavItem {
  return {
    id: 'app',
    labelKey: 'nav.application',
    icon: 'Grid',
    path: '/token-usage',
    children: [
      conversationsNav(),
      tokenUsageNav(options?.tokenUsageLabel ?? 'nav.tokenUsage'),
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
    labelKey: 'nav.cacheAndMemory',
    icon: 'Coin',
    path: '/memory',
    children: [cacheNav(admin), evaluationNav(admin), storageNav(admin), dataStoresNav(admin), memoryNav()]
  }
}

export function accessGroup(options?: { adminOnly?: boolean }): NavItem {
  const admin = options?.adminOnly ? ({ requiresAdmin: true } as LeafOpts) : undefined
  return {
    id: 'access',
    labelKey: 'nav.access',
    icon: 'Link',
    path: '/host-proxy',
    ...(options?.adminOnly ? { requiresAdmin: true } : {}),
    children: [hostProxyNav(admin), systemProxyNav(admin), clashRulesNav(admin)]
  }
}

/** 系统管理：团队 Web 含用户管理/更新；桌面版用户菜单为子集（配置项全员可见） */
export function systemAdminGroup(options?: { teamExtras?: boolean; relaxedAccess?: boolean }): NavItem {
  const admin = !(options?.relaxedAccess ?? false)
  const leafAdmin = admin ? ({ requiresAdmin: true } as LeafOpts) : undefined
  const children: NavItem[] = [
    configNav(leafAdmin),
    {
      id: 'system-users',
      labelKey: 'nav.users',
      icon: 'UserFilled',
      path: '/system/users',
      requiresAdmin: true,
      requiresTeam: true
    }
  ]
  if (options?.teamExtras) {
    children.push(
      {
        id: 'system-update',
        labelKey: 'nav.systemUpdate',
        icon: 'Upload',
        path: '/system/update',
        requiresAdmin: true,
        requiresTeam: true
      }
    )
  }
  return {
    id: 'system-admin',
    labelKey: 'nav.systemAdmin',
    icon: 'Setting',
    path: '/config',
    ...(admin ? { requiresAdmin: true } : {}),
    children
  }
}
