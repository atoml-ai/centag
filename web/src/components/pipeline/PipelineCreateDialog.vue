<template>
  <el-dialog
    :model-value="modelValue"
    title="创建流水线"
    width="560px"
    :close-on-click-modal="false"
    @update:model-value="emit('update:modelValue', $event)"
    @closed="resetForm"
  >
    <el-alert type="info" :closable="false" show-icon style="margin-bottom: 16px">
      填写流水线基础信息后即可进入可视化画布配置节点。ID 创建后不可修改。
    </el-alert>

    <el-form
      ref="formRef"
      :model="form"
      :rules="rules"
      label-width="90px"
      @submit.prevent
    >
      <el-form-item label="ID" prop="id">
        <el-input
          v-model="form.id"
          placeholder="例如 my-pipeline"
          autocomplete="off"
        />
        <div class="form-tip">仅支持字母、数字、下划线和连字符</div>
      </el-form-item>
      <el-form-item label="名称" prop="name">
        <el-input v-model="form.name" placeholder="请输入流水线名称" />
      </el-form-item>
      <el-form-item label="描述">
        <el-input
          v-model="form.description"
          type="textarea"
          :rows="3"
          placeholder="简要描述该流水线的用途（可选）"
        />
      </el-form-item>
      <el-form-item label="版本" prop="version">
        <el-input v-model="form.version" placeholder="1.0" />
      </el-form-item>
      <el-form-item label="快捷码" prop="shortcut_code">
        <el-input v-model="form.shortcut_code" placeholder="#myflow（可选）" />
        <div class="form-tip">用于聊天中快速切换，需以 # 开头</div>
      </el-form-item>
    </el-form>

    <template #footer>
      <el-button @click="emit('update:modelValue', false)">取消</el-button>
      <el-button type="primary" :loading="submitting" @click="handleConfirm">
        下一步：配置节点
      </el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, reactive, watch } from 'vue'
import type { FormInstance, FormRules } from 'element-plus'

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
    { required: true, message: '请输入流水线 ID', trigger: 'blur' },
    {
      validator: (_rule, value, callback) => {
        const id = (value || '').trim()
        if (!idPattern.test(id)) {
          callback(new Error('ID 仅支持字母、数字、下划线和连字符'))
          return
        }
        if (props.existingIds?.includes(id)) {
          callback(new Error('该 ID 已存在，请更换'))
          return
        }
        callback()
      },
      trigger: 'blur'
    }
  ],
  name: [{ required: true, message: '请输入流水线名称', trigger: 'blur' }],
  version: [{ required: true, message: '请输入版本号', trigger: 'blur' }],
  shortcut_code: [
    {
      validator: (_rule, value, callback) => {
        const code = (value || '').trim()
        if (code && !code.startsWith('#')) {
          callback(new Error('快捷码必须以 # 开头'))
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