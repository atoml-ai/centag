import api from '@/api'

export interface WrapPreset {
  id: string
  display_name: string
  description: string
  argv: string[]
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
