// 路由路径
export const ROUTES = {
  HOME: '/',
  LOGIN: '/login',
  DASHBOARD: '/dashboard',
  BACKENDS: '/backends',
  CACHE: '/cache',
  EVALUATION: '/evaluation',
  CHAT: '/chat',
  CONFIG: '/config',
  STORAGE: '/storage',
  HOST_PROXY: '/host-proxy',
  SYSTEM_PROXY: '/system-proxy',
  PROFILE: '/profile',
  USERS: '/system/users',
  SYSTEM_UPDATE: '/system/update',
  CLASH_RULES: '/clash-rules',
  LOGS: '/logs',
  MEMORY: '/memory',
  PIPELINE_MODES: '/pipeline-modes',
  PIPELINE_DEFAULTS: '/pipeline/defaults',
  NODE_PLUGINS: '/pipeline/node-plugins'
}

export type { NavItem } from '@/utils/nav/types'

// 后端类型
export const BACKEND_TYPES = {
  OPENAI: { value: 'openai', label: 'OpenAI' },
  OLLAMA: { value: 'ollama', label: 'Ollama' }
}

// 后端状态
export const BACKEND_STATUS = {
  ENABLED: { value: true, label: '启用', color: 'success' },
  DISABLED: { value: false, label: '禁用', color: 'info' }
}

// 代理模式
export const PROXY_MODES = {
  DIRECT: { value: 'direct', label: '直连' },
  CACHE: { value: 'cache', label: '缓存优先' },
  FALLBACK: { value: 'fallback', label: '回退' }
}
