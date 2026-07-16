<template>
  <div class="schema-form">
    <el-form
      ref="formRef"
      :model="formData"
      :rules="formRules"
      label-position="top"
      @submit.prevent="handleSubmit"
    >
      <el-form-item
        v-for="field in schemaFields"
        :key="field.key"
        :label="field.label"
        :prop="field.key"
      >
        <component
          :is="getFieldComponent(field)"
          v-model="formData[field.key]"
          v-bind="getFieldProps(field)"
        >
          <template v-if="field.type === 'select' && field.options">
            <el-option
              v-for="opt in field.options"
              :key="opt.value"
              :label="opt.label"
              :value="opt.value"
            />
          </template>
        </component>
        <div v-if="field.description" class="field-description">
          {{ field.description }}
        </div>
      </el-form-item>
    </el-form>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import type { FormInstance, FormRules } from 'element-plus'

export interface SchemaField {
  key: string
  type: 'string' | 'number' | 'boolean' | 'array' | 'object' | 'select'
  label: string
  description?: string
  required?: boolean
  default?: any
  min?: number
  max?: number
  pattern?: string
  options?: { label: string; value: any }[]
  items?: SchemaField[]
  properties?: Record<string, SchemaField>
}

const props = defineProps<{
  schema: Record<string, any>
  modelValue?: Record<string, any>
}>()

const emit = defineEmits<{
  'update:modelValue': [value: Record<string, any>]
  submit: [value: Record<string, any>]
}>()

const formRef = ref<FormInstance>()
const formData = ref<Record<string, any>>({})

const schemaFields = computed<SchemaField[]>(() => {
  if (!props.schema?.properties) return []
  return Object.entries(props.schema.properties).map(([key, field]: [string, any]) => ({
    key,
    type: field.type || 'string',
    label: field.title || key,
    description: field.description,
    required: props.schema.required?.includes(key),
    default: field.default,
    min: field.minimum,
    max: field.maximum,
    pattern: field.pattern,
    options: field.enum?.map((v: any, i: number) => ({
      label: field.enumNames?.[i] || String(v),
      value: v
    })),
    items: field.items?.properties ? Object.entries(field.items.properties).map(([k, f]: [string, any]) => ({
      key: k,
      type: f.type || 'string',
      label: f.title || k
    })) : undefined,
    properties: field.properties ? Object.entries(field.properties).reduce((acc, [k, f]: [string, any]) => {
      acc[k] = {
        key: k,
        type: f.type || 'string',
        label: f.title || k
      }
      return acc
    }, {} as Record<string, SchemaField>) : undefined
  }))
})

const formRules = computed<FormRules>(() => {
  const rules: FormRules = {}
  schemaFields.value.forEach(field => {
    const fieldRules = []
    if (field.required) {
      fieldRules.push({ required: true, message: `${field.label} is required`, trigger: 'blur' })
    }
    if (field.type === 'number') {
      if (field.min !== undefined) {
        fieldRules.push({ min: field.min, type: 'number', message: `Minimum value is ${field.min}`, trigger: 'blur' })
      }
      if (field.max !== undefined) {
        fieldRules.push({ max: field.max, type: 'number', message: `Maximum value is ${field.max}`, trigger: 'blur' })
      }
    }
    if (field.pattern) {
      fieldRules.push({ pattern: new RegExp(field.pattern), message: 'Invalid format', trigger: 'blur' })
    }
    if (fieldRules.length > 0) {
      rules[field.key] = fieldRules
    }
  })
  return rules
})

watch(() => props.modelValue, (val) => {
  if (val) {
    formData.value = { ...val }
  }
}, { immediate: true })

watch(formData, (val) => {
  emit('update:modelValue', val)
}, { deep: true })

function getFieldComponent(field: SchemaField) {
  if (field.type === 'boolean') return 'el-switch'
  if (field.type === 'number') return 'el-input-number'
  if (field.type === 'array') return 'el-input'
  if (field.type === 'object') return 'el-input'
  if (field.type === 'select') return 'el-select'
  return 'el-input'
}

function getFieldProps(field: SchemaField) {
  const props: Record<string, any> = {}
  if (field.type === 'number') {
    if (field.min !== undefined) props.min = field.min
    if (field.max !== undefined) props.max = field.max
  }
  if (field.type === 'array' || field.type === 'object') {
    props.type = 'textarea'
    props.rows = 3
  }
  if (field.default !== undefined && !formData.value[field.key]) {
    formData.value[field.key] = field.default
  }
  return props
}

async function validate() {
  if (!formRef.value) return false
  return await formRef.value.validate().catch(() => false)
}

function handleSubmit() {
  emit('submit', formData.value)
}

defineExpose({ validate, submit: handleSubmit })
</script>

<style scoped>
.schema-form {
  padding: 16px 0;
}
.field-description {
  font-size: 12px;
  color: #909399;
  margin-top: 4px;
}
</style>