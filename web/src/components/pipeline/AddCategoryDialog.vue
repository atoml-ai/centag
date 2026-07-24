<template>
  <el-dialog
    v-model="visible"
    :title="t('addCategoryDialog.title')"
    width="560px"
    destroy-on-close
    append-to-body
    @closed="resetForm"
  >
    <el-alert
      type="info"
      :closable="false"
      show-icon
      style="margin-bottom: 14px"
      :title="t('addCategoryDialog.alert')"
    />
    <el-form label-width="110px" @submit.prevent>
      <el-form-item :label="t('addCategoryDialog.labelField')" required>
        <el-input v-model="form.label" :placeholder="t('addCategoryDialog.labelPlaceholder')" @blur="onLabelBlur" />
      </el-form-item>
      <el-form-item :label="t('addCategoryDialog.nodeIdField')">
        <el-input v-model="form.nodeId" :placeholder="t('addCategoryDialog.nodeIdPlaceholder')" />
      </el-form-item>
      <el-form-item :label="t('addCategoryDialog.keywordField')" required>
        <el-select
          v-model="form.keywords"
          multiple
          filterable
          allow-create
          default-first-option
          :placeholder="t('addCategoryDialog.keywordPlaceholder')"
          style="width: 100%"
        />
      </el-form-item>
      <el-form-item :label="t('addCategoryDialog.routerField')" required>
        <el-select v-model="form.routerNodeId" style="width: 100%" :placeholder="t('addCategoryDialog.routerPlaceholder')">
          <el-option
            v-for="r in routers"
            :key="r.id"
            :label="`${r.name || r.id} (${r.id})`"
            :value="r.id"
          />
        </el-select>
      </el-form-item>
      <el-form-item :label="t('addCategoryDialog.defaultBranch')">
        <el-switch v-model="form.isDefault" />
      </el-form-item>
      <el-form-item :label="t('addCategoryDialog.recommendedTags')">
        <el-select v-model="form.tags" multiple style="width: 100%" :placeholder="t('addCategoryDialog.tagsPlaceholder')">
          <el-option v-for="tag in tagOptions" :key="tag" :label="tag" :value="tag" />
        </el-select>
      </el-form-item>
      <el-form-item :label="t('addCategoryDialog.initialPrompt')">
        <el-input
          v-model="form.systemPrompt"
          type="textarea"
          :rows="3"
          :placeholder="t('addCategoryDialog.promptPlaceholder')"
        />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="visible = false">{{ t('addCategoryDialog.cancel') }}</el-button>
      <el-button type="primary" @click="submit">{{ t('addCategoryDialog.submit') }}</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'
import type { AgentPatternPipeline } from '@/api/pipeline'
import { applyAddCategory, listRouterNodes, slugifyNodeId } from '@/utils/capabilitySlots'

const { t } = useI18n()

const visible = defineModel<boolean>({ default: false })

const props = defineProps<{
  pipeline: AgentPatternPipeline | null
}>()

const emit = defineEmits<{
  applied: [pipeline: AgentPatternPipeline]
}>()

const tagOptions = ['code', 'reasoning', 'review', 'cheap', 'fast', 'multilingual', 'explain', 'default']

const form = reactive({
  label: '',
  nodeId: '',
  keywords: [] as string[],
  routerNodeId: '',
  isDefault: false,
  tags: ['default'] as string[],
  systemPrompt: ''
})

const routers = computed(() => (props.pipeline ? listRouterNodes(props.pipeline) : []))

watch(visible, (open) => {
  if (!open) return
  const list = routers.value
  form.routerNodeId = list[0]?.id || ''
})

function onLabelBlur() {
  if (!form.nodeId.trim() && form.label.trim()) {
    form.nodeId = slugifyNodeId(form.label)
  }
}

function resetForm() {
  form.label = ''
  form.nodeId = ''
  form.keywords = []
  form.routerNodeId = ''
  form.isDefault = false
  form.tags = ['default']
  form.systemPrompt = ''
}

function submit() {
  if (!props.pipeline) {
    ElMessage.warning(t('addCategoryDialog.pipelineNotLoaded'))
    return
  }
  try {
    const next = applyAddCategory(props.pipeline, {
      label: form.label,
      keywords: form.keywords,
      nodeId: form.nodeId || undefined,
      routerNodeId: form.routerNodeId || undefined,
      isDefault: form.isDefault,
      tags: form.tags,
      systemPrompt: form.systemPrompt,
      appendSlot: true
    })
    emit('applied', next)
    ElMessage.success(t('addCategoryDialog.addSuccess'))
    visible.value = false
  } catch (err: any) {
    ElMessage.error(err?.message || t('addCategoryDialog.addFailed'))
  }
}
</script>
