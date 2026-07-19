import api from '@/api'

export interface ProxySetupStatus {
  mode: 'local' | 'lan' | string
  mitm_enabled: boolean
  listen_addr: string
  listen_is_loopback: boolean
  allow_lan_clients: boolean
  advertise_host: string
  pac_enabled: boolean
  pac_url: string
  ca_download_url: string
  ca_fingerprint_sha256: string
  global_proxy_mode: boolean
  mitm_proxy: string
}

export function getProxySetupStatus(): Promise<ProxySetupStatus> {
  return api.get('/api/v1/proxy/setup/status') as Promise<ProxySetupStatus>
}
