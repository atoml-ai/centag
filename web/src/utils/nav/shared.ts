import type { NavItem } from './types'

/**
 * 导航叶子与分组工厂（personal / team / minimal 共用，避免重复 path/icon）。
 *
 * 一级分类（team）：
 * - 首页/概览
 * - 代理策略（后端、流水线、节点插件）
 * - 应用（对话、用量、日志）
 * - 缓存与记忆
 * - 接入（主机/系统代理、Clash）
 * - 系统管理
 *
 * personal 桌面：首页 / 配置 / 对话 / 应用 / 更多（更多下再分接入·缓存·Agent·系统）
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

export function nodePluginsNav(): NavItem {
  return {
    id: 'node-plugins',
    label: '节点插件',
    icon: 'Connection',
    path: '/pipeline/node-plugins',
    requiresAdmin: true
  }
}

export function chatNav(): NavItem {
  return { id: 'chat', label: '对话', icon: 'ChatDotRound', path: '/chat' }
}

export function conversationsNav(): NavItem {
  return { id: 'conversations', label: '会话记录', icon: 'ChatLineSquare', path: '/conversations' }
}

export function tokenUsageNav(label = '用量统计'): NavItem {
  return { id: 'token-usage', label, icon: 'TrendCharts', path: '/token-usage' }
}

export function billingRulesNav(): NavItem {
  return {
    id: 'billing-rules',
    label: '计费规则',
    icon: 'Coin',
    path: '/billing',
    requiresAdmin: true
  }
}

export function costDashboardNav(): NavItem {
  return {
    id: 'cost-dashboard',
    label: '成本看板',
    icon: 'Coin',
    path: '/cost',
    requiresAdmin: true,
    requiresTeam: true
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
  return { id: 'memory', label: '云记忆', icon: 'Folder', path: '/memory' }
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

/** 通用分组（可嵌套：更多 → 分类 → 叶子） */
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

/** 桌面 personal：后端 + 策略作为二级菜单 */
export function personalConfigGroup(): NavItem {
  return navGroup('personal-config', '配置', 'Connection', [
    backendsNav({ label: '后端管理' }),
    pipelinesNav({ label: '策略管理' })
  ], '/backends')
}

/** 桌面 personal：应用相关（会话 / 用量 / 计费 / 日志） */
export function personalAppGroup(): NavItem {
  return navGroup(
    'personal-app',
    '应用',
    'Grid',
    [conversationsNav(), tokenUsageNav('用量'), billingRulesNav(), logsNav()],
    '/conversations'
  )
}

/** 桌面 personal：进阶能力按分类挂在「更多」下 */
export function personalMoreGroup(): NavItem {
  return navGroup(
    'more',
    '更多',
    'MoreFilled',
    [
      navGroup(
        'more-access',
        '接入',
        'Link',
        [hostProxyNav(), systemProxyNav(), clashRulesNav()],
        '/host-proxy'
      ),
      navGroup(
        'more-cache',
        '缓存与记忆',
        'Coin',
        [cacheNav(), evaluationNav(), storageNav(), dataStoresNav(), memoryNav()],
        '/memory'
      ),
      navGroup(
        'more-agent',
        'Agent',
        'Cpu',
        [agentSetupNav(), agentProvidersNav(), nodePluginsNav()],
        '/agent-setup'
      ),
      navGroup('more-system', '系统', 'Setting', [configNav()], '/config')
    ],
    '/host-proxy'
  )
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
    path: '/chat',
    children: [
      chatNav(),
      conversationsNav(),
      tokenUsageNav(options?.tokenUsageLabel),
      billingRulesNav(),
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
    },
    {
      id: 'virtual-keys',
      label: '虚拟密钥',
      icon: 'Key',
      path: '/system/virtual-keys',
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
