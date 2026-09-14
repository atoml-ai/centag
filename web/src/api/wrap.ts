import api from '@/api'

export interface WrapPreset {
  id: string
  display_name: string
  description: string
  argv: string[]
}

export interface WrapApp {
  id: string
  display_name: string
  vendor?: string
  category?: string
  launch_mode: string
  argv?: string[]
  install_url?: string
  install_hint?: string
  note?: string
  model_config?: { agent_type?: string; env_keys?: string[]; requires?: boolean }
}

export interface WrapCheck {
  id: string
  ok: boolean
  message: string
  action?: string
}

export interface WrapPrepareResult {
  ok: boolean
  app_id: string
  display_name?: string
  launch_mode?: string
  argv?: string[]
  model: string
  pipeline_id?: string
  env?: Record<string, string>
  server?: string
  token?: string
  warnings?: string[]
  restart_required?: boolean
}

export interface WrapRunResult {
  ok: boolean
  command: string
  user_command?: string
  exec_command?: string
  argv: string[]
  server?: string
  opened?: boolean
  open_error?: string
  hint?: string
}

export function listWrapPresets(): Promise<{ presets: WrapPreset[] }> {
  return api.get('/api/v1/wrap/presets') as Promise<{ presets: WrapPreset[] }>
}

/** Proxy-launch app catalog (which local AI/agent apps Centag can proxy). */
export function listWrapApps(): Promise<{ apps: WrapApp[] }> {
  return api.get('/api/v1/wrap/apps') as Promise<{ apps: WrapApp[] }>
}

/** Resolve the model name for an app and optionally write its local config. */
export function prepareWrapApp(id: string, writeConfig = false): Promise<WrapPrepareResult> {
  return api.post(`/api/v1/wrap/apps/${id}/prepare`, {
    write_config: writeConfig,
  }) as Promise<WrapPrepareResult>
}

/** Proxy readiness checks (CA / MITM / egress key / LAN). */
export function wrapDoctor(): Promise<{ ok: boolean; checks: WrapCheck[] }> {
  return api.get('/api/v1/wrap/doctor') as Promise<{ ok: boolean; checks: WrapCheck[] }>
}

export function runWrapAgent(body: {
  preset_id?: string
  argv?: string[]
  command?: string
  open_terminal?: boolean
}): Promise<WrapRunResult> {
  return api.post('/api/v1/wrap/run', body) as Promise<WrapRunResult>
}

/** Client-side short command (no token); for System Proxy / Agent Run copy helpers. */
export function buildWrapRunCopyCommand(argv: string[], server = 'http://127.0.0.1:20060'): string {
  const parts = ['centag', 'wrap', 'run', '--server', server, '--', ...argv]
  return parts.join(' ')
}
