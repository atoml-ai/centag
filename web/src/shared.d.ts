/**
 * Type declarations for shared/ JavaScript modules
 * These modules are plain JS (no build step) shared between webui shared modules and webui
 */

declare module '@shared/provider-registry.js' {
  export interface ProviderModel {
    name: string
    supports_tools: boolean
    supports_images: boolean
    supports_thinking: boolean
    max_context_tokens: number
  }

  export interface ProviderDef {
    id: string
    name: string
    type: string
    base_url: string
    env_key: string
    icon: string
    description: string
    default_models: ProviderModel[]
  }

  export const PROVIDER_REGISTRY: Record<string, ProviderDef>
  export function getProviderList(): ProviderDef[]
  export function getProviderById(id: string): ProviderDef | null
  export function getDefaultModels(providerId: string): ProviderModel[]
}

declare module '@shared/yaml-writer.js' {
  export class YamlWriter {
    static stringify(obj: unknown, indent?: number): string
    static escapeString(str: string): string
    static stringifyArray(arr: unknown[], indent: number): string
    static stringifyObject(obj: Record<string, unknown>, indent: number): string
    static escapeKey(key: string): string
  }
}

declare module '@shared/config-builder.js' {
  import type { ProviderDef } from '@shared/provider-registry.js'

  export interface BackendEntry {
    provider: ProviderDef
    apiKey: string
    defaultModel: string
    models: Array<{
      name: string
      actual_model?: string
      supports_tools: boolean
      supports_images: boolean
      supports_thinking: boolean
      max_context_tokens: number
    }>
    isPreset: boolean
    isDefault: boolean
    settings: {
      timeout: number
      maxRetries: number
      weight: number
      priority: number
    }
  }

  export class ConfigBuilder {
    static PIPELINE_TEMPLATES_DATA: Record<string, string>
    static getDefaultBackendInfo(backends: BackendEntry[]): { backendId: string; model: string }
    static buildBackendsYaml(backends: BackendEntry[]): string
    static buildSingleBackend(backend: BackendEntry): Record<string, unknown>
    static getMaxContextTokens(models: Array<{ max_context_tokens?: number }>): number
    static getFeatures(models: Array<{ supports_tools?: boolean; supports_images?: boolean }>): string[]
    static buildEnvContent(backends: BackendEntry[]): string
    static buildReadme(): string
    static buildPipelineTemplateYaml(templateId: string, backendId: string, model: string): string
    static buildAllYamlFiles(backends: BackendEntry[], selectedTemplateIds: string[]): Record<string, string>
    static exportAsArchive(backends: BackendEntry[], selectedTemplateIds: string[]): Promise<Blob>
  }
}

declare module '@shared/provider-form.js' {
  import type { BackendEntry } from '@shared/config-builder.js'

  export interface ProviderFormModel {
    providerId: string
    name: string
    type: string
    base_url: string
    api_key: string
    default_model: string
    models: Array<{
      name: string
      actual_model?: string
      supports_tools?: boolean
      supports_images?: boolean
      supports_thinking?: boolean
      max_context_tokens?: number
    }>
    isPreset: boolean
    id?: string
    has_api_key?: boolean
    description?: string
    timeout?: number
    max_retries?: number
    enabled?: boolean
    auto_fetch_models?: boolean
    probe_model?: string
  }

  export function createEmptyProviderForm(): ProviderFormModel
  export function initProviderFormDeps(deps: {
    getProviderById?: (id: string) => unknown
    ConfigBuilder?: { buildSingleBackend: (entry: BackendEntry) => Record<string, unknown> }
  }): void
  export function applyProviderPreset(form: ProviderFormModel, providerId: string): boolean
  export function filterProviders<T extends { name?: string; id?: string; description?: string }>(
    providers: T[],
    query: string
  ): T[]
  export function addModelToForm(form: ProviderFormModel, name: string): boolean
  export function removeModelFromForm(form: ProviderFormModel, index: number): void
  export function slugifyProviderId(name: string): string
  export function validateProviderForm(
    form: ProviderFormModel,
    options?: { isCreate?: boolean; requireApiKey?: boolean }
  ): { ok: boolean; errors: string[] }
  export function toBackendEntry(form: ProviderFormModel): BackendEntry
  export function toApiBackendPayload(
    form: ProviderFormModel,
    options?: { isCreate?: boolean }
  ): Record<string, unknown>
  export function fromApiBackend(row: Record<string, unknown>): ProviderFormModel
  export function fromBackendEntry(backend: BackendEntry): ProviderFormModel
}
