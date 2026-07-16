import type {
  ProviderDef,
  ProviderModel,
} from '@shared/provider-registry.js'
import type { BackendEntry } from '@shared/config-builder.js'
import type { ProviderFormModel } from '@shared/provider-form.js'

import {
  PROVIDER_REGISTRY,
  getProviderList,
  getProviderById,
  getDefaultModels,
} from '@shared/provider-registry.js'

import { YamlWriter } from '@shared/yaml-writer.js'

import { ConfigBuilder } from '@shared/config-builder.js'

import {
  initProviderFormDeps,
  createEmptyProviderForm,
  applyProviderPreset,
  filterProviders,
  addModelToForm,
  removeModelFromForm,
  slugifyProviderId,
  validateProviderForm,
  toBackendEntry,
  toApiBackendPayload,
  fromApiBackend,
  fromBackendEntry,
} from '@shared/provider-form.js'

// Vite ESM 下 provider-form 不能裸引用全局函数，必须注入依赖
initProviderFormDeps({ getProviderById, ConfigBuilder })

export type { ProviderDef, ProviderModel, BackendEntry, ProviderFormModel }

export {
  PROVIDER_REGISTRY,
  getProviderList,
  getProviderById,
  getDefaultModels,
  YamlWriter,
  ConfigBuilder,
  initProviderFormDeps,
  createEmptyProviderForm,
  applyProviderPreset,
  filterProviders,
  addModelToForm,
  removeModelFromForm,
  slugifyProviderId,
  validateProviderForm,
  toBackendEntry,
  toApiBackendPayload,
  fromApiBackend,
  fromBackendEntry,
}
