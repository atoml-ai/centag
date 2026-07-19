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
  egress_api_key_configured?: boolean
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
