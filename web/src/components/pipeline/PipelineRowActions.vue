<template>
  <el-dropdown trigger="click" @command="(cmd: string) => emit('command', cmd)">
    <slot>
      <el-button size="small" circle plain>
        <el-icon><MoreFilled /></el-icon>
      </el-button>
    </slot>
    <template #dropdown>
      <el-dropdown-menu>
        <el-dropdown-item v-if="canConfigureCapabilitySlots(row)" command="configure" :icon="Connection">
          {{ t('pipelineModes.table.configureModel') }}
        </el-dropdown-item>
        <el-dropdown-item v-if="!unrestricted" command="clone" :icon="CopyDocument">
          {{ isSystemPipeline(row) ? t('pipelineModes.table.createCopy') : t('pipelineModes.table.clone') }}
        </el-dropdown-item>
        <el-dropdown-item command="edit" :icon="Edit" :disabled="editDisabled">
          {{ t('pipelineModes.table.edit') }}
        </el-dropdown-item>
        <el-dropdown-item command="history" :icon="Timer" :disabled="historyDisabled">
          {{ t('pipelineModes.table.history') }}
        </el-dropdown-item>
        <el-dropdown-item command="export" :icon="Download" :disabled="exportDisabled">
          {{ t('pipelineModes.table.export') }}
        </el-dropdown-item>
        <PipelineFeatureGuard
          feature="pipelineDelete"
          :pipeline="row"
          :unrestricted="unrestricted"
          :action-label="t('pipelineModes.table.delete')"
        >
          <template #default="{ disabled }">
            <el-dropdown-item command="delete" :icon="Delete" :disabled="disabled" divided>
              {{ t('pipelineModes.table.delete') }}
            </el-dropdown-item>
          </template>
        </PipelineFeatureGuard>
      </el-dropdown-menu>
    </template>
  </el-dropdown>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  MoreFilled, CopyDocument, Connection, Delete, Download, Edit, Timer
} from '@element-plus/icons-vue'
import PipelineFeatureGuard from './PipelineFeatureGuard.vue'
import { isSystemPipeline, resolvePipelineFeatureSupport } from '@/utils/pipeline/features'
import { canConfigureCapabilitySlots } from '@/utils/capabilitySlots'
import type { Pipeline } from '@/api/pipeline'

const props = defineProps<{
  row: Pipeline
  unrestricted: boolean
  defaultPipelineId: string
}>()

const emit = defineEmits<{ (e: 'command', command: string): void }>()

const { t } = useI18n()

const editDisabled = computed(() => !resolvePipelineFeatureSupport('pipelineEdit', props.row, { unrestricted: props.unrestricted }))
const historyDisabled = computed(() => !resolvePipelineFeatureSupport('executionHistory', props.row, { unrestricted: props.unrestricted }))
const exportDisabled = computed(() => !resolvePipelineFeatureSupport('pipelineExport', props.row, { unrestricted: props.unrestricted }))
</script>

<style scoped>
:deep(.el-button.is-circle) {
  width: 32px;
  height: 32px;
  padding: 8px;
}
</style>
