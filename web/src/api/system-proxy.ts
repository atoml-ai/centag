import api from '@/api'

export interface ProxySetupStatus {
  mode: 'local' | 'lan' | string
  mitm_enabled: boolean
  listen_addr: string
  listen_is_loopback: boolean
  allow_lan_clients: boolean
  advertise_host: string
  suggested_lan_hosts?: string[]
  in_container?: boolean
  pac_enabled: boolean
  pac_url: string
  ca_download_url: string
  ca_fingerprint_sha256: string
  global_proxy_mode: boolean
  mitm_proxy: string
  egress_api_key_configured?: boolean
  /** LAN 开启时 MITM 对非本机客户端强制 Proxy-Authorization */
  proxy_auth_required?: boolean
  /**
   * 是否允许「一键写入配置」。仅当 centag 与浏览器同机（local 模式 + 经由 loopback 访问）时为真；
   * 远程/LAN 部署下写入的是服务端文件系统，对用户本机 Agent 无效，应改用生成配置复制或 wrap run。
   */
  write_config_supported?: boolean
  /** 浏览器是否经由非 loopback 地址访问（即相对 centag 服务而言是远程访问） */
  accessed_remotely?: boolean
}

export interface EgressKeyEnsureResult {
  configured: boolean
  changed: boolean
  key_name: string
}

export function getProxySetupStatus(): Promise<ProxySetupStatus> {
  return api.get('/api/v1/proxy/setup/status') as Promise<ProxySetupStatus>
}

export function ensureEgressAPIKey(): Promise<EgressKeyEnsureResult> {
  return api.post('/api/v1/proxy/egress-key/ensure') as Promise<EgressKeyEnsureResult>
}

export function bindEgressAPIKey(apiKeyId: number): Promise<{ configured: boolean; api_key_id: number }> {
  return api.post('/api/v1/proxy/egress-key/bind', { api_key_id: apiKeyId }) as Promise<{
    configured: boolean
    api_key_id: number
  }>
}
