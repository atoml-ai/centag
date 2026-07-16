import type { NavItem } from './types'

/**
 * 统一导航目录（团队 Web 顶栏 + 桌面版侧栏/用户菜单共用）
 *
 * 一级分类：
 * - 首页/概览
 * - 代理策略（后端、流水线、节点插件）
 * - 应用（对话、用量、日志）
 * - 缓存与记忆
 * - 接入（主机/系统代理、Clash）
 * - 系统管理（配置、用户、租户、更新）
 */

export function dashboardNav(label: '首页' | '概览'): NavItem {
  return { id: 'dashboard', label, icon: 'Home', path: '/dashboard' }
}

/** 代理策略：后端 + 流水线；团队版含节点插件且仅管理员可见 */
export function proxyStrategyGroup(options?: { includeNodePlugins?: boolean; adminOnly?: boolean }): NavItem {
  const admin = options?.adminOnly ?? false
  const children: NavItem[] = [
    {
      id: 'backends',
      label: '后端管理',
      icon: 'Connection',
      path: '/backends',
      ...(admin ? { requiresAdmin: true } : {})
    },
    {
      id: 'pipelines',
      label: '策略管理',
      icon: 'Share',
      path: '/pipelines',
      ...(admin ? { requiresAdmin: true } : {})
    },
    {
      id: 'agent-setup',
      label: 'Agent 接入',
      icon: 'Link',
      path: '/agent-setup',
      ...(admin ? { requiresAdmin: true } : {})
    },
    {
      id: 'agent-providers',
      label: 'Agent 供应商',
      icon: 'Connection',
      path: '/agent-providers',
      ...(admin ? { requiresAdmin: true } : {})
    }
  ]
  if (options?.includeNodePlugins) {
    children.push({
      id: 'node-plugins',
      label: '节点插件',
      icon: 'Connection',
      path: '/pipeline/node-plugins',
      requiresAdmin: true
    })
  }
  return {
    id: 'proxy-strategy',
    label: '代理策略',
    icon: 'Connection',
    path: '/backends',
    ...(admin ? { requiresAdmin: true } : {}),
    children
  }
}

export function appGroup(options?: { tokenUsageLabel?: string }): NavItem {
  const tokenUsageLabel = options?.tokenUsageLabel ?? '用量统计'
  return {
    id: 'app',
    label: '应用',
    icon: 'Grid',
    path: '/chat',
    children: [
      { id: 'chat', label: '对话', icon: 'ChatDotRound', path: '/chat' },
      { id: 'token-usage', label: tokenUsageLabel, icon: 'TrendCharts', path: '/token-usage' },
      {
        id: 'cost-dashboard',
        label: '成本看板',
        icon: 'Coin',
        path: '/cost',
        requiresAdmin: true,
        requiresTeam: true
      },
      {
        id: 'ab-comparison',
        label: 'A/B 对比',
        icon: 'DataAnalysis',
        path: '/ab-comparison',
        requiresAdmin: true,
        requiresTeam: true
      },
      { id: 'logs', label: '日志', icon: 'Document', path: '/logs' }
    ]
  }
}

/** 缓存与记忆：云记忆全员可见；缓存/存储类仅管理员（团队 Web） */
export function cacheMemoryGroup(options?: { adminTools?: boolean }): NavItem {
  const admin = options?.adminTools ?? false
  return {
    id: 'cache-memory',
    label: '缓存与记忆',
    icon: 'Coin',
    path: '/memory',
    children: [
      {
        id: 'cache',
        label: '缓存监控',
        icon: 'Coin',
        path: '/cache',
        ...(admin ? { requiresAdmin: true } : {})
      },
      {
        id: 'evaluation',
        label: '缓存评估',
        icon: 'TrendCharts',
        path: '/evaluation',
        ...(admin ? { requiresAdmin: true } : {})
      },
      {
        id: 'storage',
        label: '存储管理',
        icon: 'FolderOpened',
        path: '/storage',
        ...(admin ? { requiresAdmin: true } : {})
      },
      {
        id: 'data-stores',
        label: '数据存储管理',
        icon: 'Coin',
        path: '/data-stores',
        ...(admin ? { requiresAdmin: true } : {})
      },
      { id: 'memory', label: '云记忆', icon: 'Folder', path: '/memory' }
    ]
  }
}

export function accessGroup(options?: { adminOnly?: boolean }): NavItem {
  const admin = options?.adminOnly ?? false
  return {
    id: 'access',
    label: '接入',
    icon: 'Link',
    path: '/host-proxy',
    ...(admin ? { requiresAdmin: true } : {}),
    children: [
      {
        id: 'host-proxy',
        label: '主机代理',
        icon: 'Link',
        path: '/host-proxy',
        ...(admin ? { requiresAdmin: true } : {})
      },
      {
        id: 'system-proxy',
        label: '系统代理',
        icon: 'Connection',
        path: '/system-proxy',
        ...(admin ? { requiresAdmin: true } : {})
      },
      {
        id: 'clash-rules',
        label: 'Clash 规则',
        icon: 'Document',
        path: '/clash-rules',
        ...(admin ? { requiresAdmin: true } : {})
      }
    ]
  }
}

/** 系统管理：团队 Web 含租户/更新；桌面版用户菜单为子集（配置项全员可见） */
export function systemAdminGroup(options?: { teamExtras?: boolean; relaxedAccess?: boolean }): NavItem {
  const admin = !(options?.relaxedAccess ?? false)
  const children: NavItem[] = [
    {
      id: 'my-tenant',
      label: '我的租户',
      icon: 'OfficeBuilding',
      path: '/my-tenant',
      requiresTeam: true
    },
    {
      id: 'config-basic',
      label: '系统配置',
      icon: 'Setting',
      path: '/config',
      ...(admin ? { requiresAdmin: true } : {})
    },
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