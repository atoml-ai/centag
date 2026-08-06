<template>
  <el-card v-loading="loading">
    <template #header>
      <div class="card-header">
        <span class="card-title">{{ t('cache.config.title') }}</span>
        <el-button type="primary" :loading="saving" @click="save">{{ t('cache.config.save') }}</el-button>
      </div>
    </template>

    <el-form label-width="180px" style="max-width: 720px">
      <el-form-item :label="t('cache.config.enabled')">
        <el-switch v-model="form.enabled" />
      </el-form-item>
      <el-form-item :label="t('cache.config.backend')">
        <el-select v-model="form.backend" style="width: 240px">
          <el-option :label="t('cache.filterOptions.exact')" value="exact" />
          <el-option :label="t('cache.filterOptions.semantic')" value="semantic" />
          <el-option :label="t('cache.filterOptions.external')" value="external" />
        </el-select>
        <div class="hint">{{ t('cache.config.backendHint') }}</div>
      </el-form-item>
      <el-form-item :label="t('cache.config.hitStrategies')">
        <el-select
          v-model="form.hit_strategies"
          multiple
          allow-create
          filterable
          default-first-option
          style="width: 360px"
        >
          <el-option label="normalize" value="normalize" />
          <el-option label="expand" value="expand" />
        </el-select>
        <div class="hint">{{ t('cache.config.hitStrategiesHint') }}</div>
      </el-form-item>
      <el-form-item :label="t('cache.config.stacking')">
        <el-switch v-model="form.allow_backend_stacking" />
        <div class="hint">{{ t('cache.config.stackingHint') }}</div>
      </el-form-item>
      <el-form-item :label="t('cache.config.readWrite')">
        <el-checkbox v-model="form.enable_cache_read">{{ t('cache.config.read') }}</el-checkbox>
        <el-checkbox v-model="form.enable_cache_write">{{ t('cache.config.write') }}</el-checkbox>
      </el-form-item>
      <el-form-item :label="t('cache.config.defaultTtl')">
        <el-input-number v-model="form.default_ttl" :min="0" :step="60" />
      </el-form-item>
      <el-form-item v-if="form.backend === 'semantic'" :label="t('cache.config.threshold')">
        <el-slider v-model="form.semantic.threshold" :min="0" :max="1" :step="0.01" show-input style="width: 360px" />
      </el-form-item>
      <el-alert
        v-if="form.backend === 'external'"
        type="info"
        :closable="false"
        :title="t('cache.config.externalHint')"
        show-icon
      />
      <el-alert
        v-if="form.backend === 'semantic'"
        type="info"
        :closable="false"
        :title="t('cache.empty.semantic')"
        show-icon
        style="margin-top: 8px"
      />
    </el-form>
  </el-card>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { getConfig, saveConfig } from '@/api'

const emit = defineEmits<{ saved: [backend: string] }>()
const { t } = useI18n()

const loading = ref(false)
const saving = ref(false)
const fullConfig = ref<any>({})

const form = reactive({
  enabled: true,
  backend: 'exact',
  hit_strategies: ['normalize', 'expand'] as string[],
  allow_backend_stacking: false,
  enable_cache_read: true,
  enable_cache_write: true,
  default_ttl: 3600,
  semantic: { threshold: 0.85, top_k: 5, enable_auto_embedding: true, distance_type: 'cosine' }
})

async function load() {
  loading.value = true
  try {
    const res = await getConfig()
    const data = res?.data || res
    fullConfig.value = data || {}
    const c = data?.cache || {}
    form.enabled = c.enabled !== false
    form.backend = c.backend || c.strategy || 'exact'
    form.hit_strategies = Array.isArray(c.hit_strategies) && c.hit_strategies.length
      ? [...c.hit_strategies]
      : ['normalize', 'expand']
    form.allow_backend_stacking = !!c.allow_backend_stacking
    form.enable_cache_read = c.enable_cache_read !== false
    form.enable_cache_write = c.enable_cache_write !== false
    form.default_ttl = c.default_ttl ?? 3600
    form.semantic = {
      threshold: c.semantic?.threshold ?? 0.85,
      top_k: c.semantic?.top_k ?? 5,
      enable_auto_embedding: c.semantic?.enable_auto_embedding !== false,
      distance_type: c.semantic?.distance_type || 'cosine'
    }
    emit('saved', form.backend)
  } catch (e: any) {
    ElMessage.error(t('cache.message.loadFailed') + ': ' + (e.message || t('cache.message.unknownError')))
  } finally {
    loading.value = false
  }
}

async function save() {
  saving.value = true
  try {
    // 只提交 cache 段，避免带回空 deployment 等字段导致本地无 DATA_DIR 时 SaveAllConfig 500
    const payload = {
      cache: {
        ...(fullConfig.value.cache || {}),
        enabled: form.enabled,
        backend: form.backend,
        strategy: form.backend,
        hit_strategies: form.hit_strategies,
        allow_backend_stacking: form.allow_backend_stacking,
        enable_cache_read: form.enable_cache_read,
        enable_cache_write: form.enable_cache_write,
        default_ttl: form.default_ttl,
        semantic: form.semantic
      }
    }
    await saveConfig(payload)
    // 刷新本地缓存配置快照，避免下次保存丢字段
    const res = await getConfig()
    fullConfig.value = res?.data || res || fullConfig.value
    ElMessage.success(t('cache.config.saved'))
    emit('saved', form.backend)
  } catch (e: any) {
    ElMessage.error(t('cache.config.saveFailed') + ': ' + (e.message || t('cache.message.unknownError')))
  } finally {
    saving.value = false
  }
}

onMounted(load)

defineExpose({ load })
</script>

<style scoped>
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.card-title {
  font-weight: 600;
}
.hint {
  margin-top: 4px;
  font-size: 12px;
  color: var(--color-gray-600);
  line-height: 1.4;
}
</style>
