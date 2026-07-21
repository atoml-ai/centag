import JSZip from 'jszip'
import * as yaml from 'js-yaml'
import { exportPipeline, createPipeline, type AgentPatternPipeline, type Pipeline } from '@/api/pipeline'
import { saveBlobAsFile } from '@/utils/downloadFile'

/** Safe download basename: keep CJK/letters, collapse other runs to `-`. */
export function sanitizePipelineFilename(name: string, id: string): string {
  const base = (name || id || 'pipeline')
    .replace(/[\\/:*?"<>|]+/g, '-')
    .replace(/\s+/g, '-')
    .replace(/-+/g, '-')
    .replace(/^-|-$/g, '')
  const safe = base || 'pipeline'
  const sid = (id || 'id').replace(/[\\/:*?"<>|]+/g, '-')
  return `${safe}-${sid}.yaml`
}

export function yamlContentFromExportResponse(response: unknown): string {
  if (typeof response === 'string') return response
  if (response && typeof response === 'object') {
    const data = (response as { data?: unknown }).data
    if (typeof data === 'string') return data
  }
  return ''
}

export async function fetchPipelineYaml(id: string): Promise<string> {
  const response = await exportPipeline(id)
  const content = yamlContentFromExportResponse(response)
  if (!content.trim()) {
    throw new Error('导出内容为空')
  }
  return content
}

export async function downloadPipelineYaml(id: string, name: string): Promise<void> {
  const content = await fetchPipelineYaml(id)
  const blob = new Blob([content], { type: 'text/yaml;charset=utf-8' })
  await saveBlobAsFile(blob, sanitizePipelineFilename(name, id))
}

export async function downloadPipelinesAsZip(
  items: Array<{ id: string; name: string }>
): Promise<void> {
  if (!items.length) {
    throw new Error('未选择流水线')
  }
  const zip = new JSZip()
  const usedNames = new Set<string>()
  for (const item of items) {
    const content = await fetchPipelineYaml(item.id)
    let filename = sanitizePipelineFilename(item.name, item.id)
    if (usedNames.has(filename)) {
      filename = `${item.id}.yaml`
    }
    usedNames.add(filename)
    zip.file(filename, content)
  }
  const blob = await zip.generateAsync({ type: 'blob' })
  const stamp = new Date().toISOString().slice(0, 10)
  await saveBlobAsFile(blob, `centag-pipelines-${stamp}.zip`)
}

export type ParsedPipelineTemplate = {
  id: string
  name: string
  description?: string
  version?: string
  shortcut_code?: string
  nodes?: AgentPatternPipeline['nodes']
  global_config?: AgentPatternPipeline['global_config']
  metadata?: AgentPatternPipeline['metadata']
  [key: string]: unknown
}

export async function parsePipelineYamlFiles(
  files: FileList | File[]
): Promise<{ templates: ParsedPipelineTemplate[]; failedFiles: string[] }> {
  const list = Array.from(files)
  const templates: ParsedPipelineTemplate[] = []
  const failedFiles: string[] = []
  for (const file of list) {
    try {
      const text = await file.text()
      const data = yaml.load(text)
      if (!data || typeof data !== 'object') {
        failedFiles.push(file.name)
        continue
      }
      const obj = data as Record<string, unknown>
      if (typeof obj.id !== 'string' || !obj.id || typeof obj.name !== 'string' || !obj.name) {
        failedFiles.push(file.name)
        continue
      }
      templates.push(obj as ParsedPipelineTemplate)
    } catch {
      failedFiles.push(file.name)
    }
  }
  return { templates, failedFiles }
}

export function pipelinePayloadFromTemplate(data: ParsedPipelineTemplate): Pipeline {
  return {
    id: data.id,
    name: data.name,
    description: (data.description as string) || '',
    version: (data.version as string) || '1.0',
    shortcut_code: (data.shortcut_code as string) || '',
    nodes: (data.nodes as AgentPatternPipeline['nodes']) || [],
    global_config: data.global_config || {
      timeout: 120,
      max_retries: 3,
      bypass_on_error: true,
      stream_mode: false,
      parallel_limit: 4,
      fallback_groups: [],
      storage: undefined,
      hooks: undefined
    },
    metadata: (data.metadata as AgentPatternPipeline['metadata']) || {}
  }
}

export async function importPipelineTemplates(
  templates: ParsedPipelineTemplate[],
  opts: {
    existingIds: Set<string>
    /** overwrite duplicates | skip duplicates */
    onDuplicate: 'overwrite' | 'skip'
  }
): Promise<{ successCount: number; failCount: number; skippedCount: number }> {
  let successCount = 0
  let failCount = 0
  let skippedCount = 0
  for (const data of templates) {
    const isDup = opts.existingIds.has(data.id)
    if (isDup && opts.onDuplicate === 'skip') {
      skippedCount++
      continue
    }
    try {
      const payload = pipelinePayloadFromTemplate(data)
      await createPipeline(payload, opts.onDuplicate === 'overwrite' && isDup)
      successCount++
    } catch {
      failCount++
    }
  }
  return { successCount, failCount, skippedCount }
}
