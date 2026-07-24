<template>
  <el-dialog
    :model-value="modelValue"
    :title="t('pipelineCreateDialog.title')"
    width="560px"
    :close-on-click-modal="false"
    @update:model-value="emit('update:modelValue', $event)"
    @closed="resetForm"
  >
    <el-alert type="info" :closable="false" show-icon style="margin-bottom: 16px">
      {{ t('pipelineCreateDialog.alert') }}
    </el-alert>

    <el-form
      ref="formRef"
      :model="form"
      :rules="rules"
      label-width="90px"
      @submit.prevent
    >
      <el-form-item :label="t('pipelineCreateDialog.idLabel')" prop="id">
        <el-input
          v-model="form.id"
          :placeholder="t('pipelineCreateDialog.idPlaceholder')"
          autocomplete="off"
        />
        <div class="form-tip">{{ t('pipelineCreateDialog.idTip') }}</div>
      </el-form-item>
      <el-form-item :label="t('pipelineCreateDialog.nameLabel')" prop="name">
        <el-input v-model="form.name" :placeholder="t('pipelineCreateDialog.namePlaceholder')" />
      </el-form-item>
      <el-form-item :label="t('pipelineCreateDialog.descLabel')">
        <el-input
          v-model="form.description"
          type="textarea"
          :rows="3"
          :placeholder="t('pipelineCreateDialog.descPlaceholder')"
        />
      </el-form-item>
      <el-form-item :label="t('pipelineCreateDialog.versionLabel')" prop="version">
        <el-input v-model="form.version" :placeholder="t('pipelineCreateDialog.versionPlaceholder')" />
      </el-form-item>
      <el-form-item :label="t('pipelineCreateDialog.shortcutLabel')" prop="shortcut_code">
        <el-input v-model="form.shortcut_code" :placeholder="t('pipelineCreateDialog.shortcutPlaceholder')" />
        <div class="form-tip">{{ t('pipelineCreateDialog.shortcutTip') }}</div>
      </el-form-item>
    </el-form>

    <template #footer>
      <el-button @click="emit('update:modelValue', false)">{{ t('pipelineCreateDialog.cancel') }}</el-button>
      <el-button type="primary" :loading="submitting" @click="handleConfirm">
        {{ t('pipelineCreateDialog.next') }}
      </el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, reactive, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { FormInstance, FormRules } from 'element-plus'

const { t } = useI18n()

export interface PipelineCreateInfo {
  id: string
  name: string
  description: string
  version: string
  shortcut_code: string
}

const props = defineProps<{
  modelValue: boolean
  existingIds?: string[]
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  confirm: [info: PipelineCreateInfo]
}>()

const formRef = ref<FormInstance>()
const submitting = ref(false)

const defaultForm = (): PipelineCreateInfo => ({
  id: `pipeline-${Date.now()}`,
  name: '',
  description: '',
  version: '1.0',
  shortcut_code: ''
})

const form = reactive<PipelineCreateInfo>(defaultForm())

const idPattern = /^[a-zA-Z0-9_-]+$/

const rules: FormRules = {
  id: [
    { required: true, message: t('pipelineCreateDialog.idRequired'), trigger: 'blur' },
    {
      validator: (_rule, value, callback) => {
        const id = (value || '').trim()
        if (!idPattern.test(id)) {
          callback(new Error(t('pipelineCreateDialog.idPatternError')))
          return
        }
        if (props.existingIds?.includes(id)) {
          callback(new Error(t('pipelineCreateDialog.idExistsError')))
          return
        }
        callback()
      },
      trigger: 'blur'
    }
  ],
  name: [{ required: true, message: t('pipelineCreateDialog.nameRequired'), trigger: 'blur' }],
  version: [{ required: true, message: t('pipelineCreateDialog.versionRequired'), trigger: 'blur' }],
  shortcut_code: [
    {
      validator: (_rule, value, callback) => {
        const code = (value || '').trim()
        if (code && !code.startsWith('#')) {
          callback(new Error(t('pipelineCreateDialog.shortcutPatternError')))
          return
        }
        callback()
      },
      trigger: 'blur'
    }
  ]
}

watch(
  () => props.modelValue,
  (visible) => {
    if (visible) {
      Object.assign(form, defaultForm())
      formRef.value?.clearValidate()
    }
  }
)

function resetForm() {
  Object.assign(form, defaultForm())
  formRef.value?.clearValidate()
}

async function handleConfirm() {
  if (!formRef.value) return
  submitting.value = true
  try {
    await formRef.value.validate()
    emit('confirm', {
      id: form.id.trim(),
      name: form.name.trim(),
      description: form.description.trim(),
      version: form.version.trim() || '1.0',
      shortcut_code: form.shortcut_code.trim()
    })
    emit('update:modelValue', false)
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped>
.form-tip {
  font-size: 12px;
  color: #909399;
  margin-top: 4px;
  line-height: 1.4;
}
</style>
