<template>
  <el-dropdown trigger="click" @command="(cmd: string) => emit('command', cmd)" @visible-change="onVisibleChange">
    <slot>
      <el-button size="small" type="primary" plain>
        {{ t('pipelineModes.table.actions') }}
        <el-icon class="el-icon--right"><ArrowDown /></el-icon>
      </el-button>
    </slot>
    <template #dropdown>
      <el-dropdown-menu>
        <el-dropdown-item
          command="setDefault"
          :icon="row.id === defaultPipelineId ? Check : StarFilled"
          :disabled="row.id === defaultPipelineId"
        >
          {{ row.id === defaultPipelineId ? t('pipelineModes.table.currentDefault') : t('pipelineModes.table.setDefault') }}
        </el-dropdown-item>
        <el-dropdown-item command="test" :icon="ChatDotRound">
          {{ t('pipelineModes.table.test') }}
        </el-dropdown-item>
        <el-dropdown-item v-if="canConfigureCapabilitySlots(row)" command="configure" :icon="Connection">
          {{ t('pipelineModes.table.configureModel') }}
        </el-dropdown-item>
        <el-dropdown-item command="clone" :icon="CopyDocument">
          {{ isSystemPipeline(row) ? t('pipelineModes.table.createCopy') : t('pipelineModes.table.clone') }}
        </el-dropdown-item>
        <div
          ref="submenuRef"
          class="pipeline-submenu"
          @mouseenter="openSubmenu"
          @mouseleave="closeSubmenu"
        >
          <div class="el-dropdown-menu__item pipeline-submenu__trigger" @click.stop>
            <span>{{ t('pipelineModes.table.more') }}</span>
            <el-icon class="el-icon--right"><ArrowRight /></el-icon>
          </div>
          <Teleport to="body">
            <div
              ref="submenuPopRef"
              v-show="submenuOpen"
              class="pipeline-submenu__pop"
              :style="popStyle"
              @mouseenter="cancelHide"
              @mouseleave="closeSubmenu"
            >
            <div
              class="el-dropdown-menu__item"
              :class="{ 'is-disabled': editDisabled }"
              @click="!editDisabled && emit('command', 'edit')"
            >
              <el-icon><Edit /></el-icon>
              <span>{{ t('pipelineModes.table.edit') }}</span>
            </div>
            <div
              class="el-dropdown-menu__item"
              :class="{ 'is-disabled': historyDisabled }"
              @click="!historyDisabled && emit('command', 'history')"
            >
              <el-icon><Timer /></el-icon>
              <span>{{ t('pipelineModes.table.history') }}</span>
            </div>
            <div
              class="el-dropdown-menu__item"
              :class="{ 'is-disabled': exportDisabled }"
              @click="!exportDisabled && emit('command', 'export')"
            >
              <el-icon><Download /></el-icon>
              <span>{{ t('pipelineModes.table.export') }}</span>
            </div>
            </div>
          </Teleport>
        </div>
        <PipelineFeatureGuard
          feature="pipelineDelete"
          :pipeline="row"
          :is-admin="isAdmin"
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
import { ref, computed, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  ArrowDown, ArrowRight, Check, CopyDocument, ChatDotRound, Connection, Delete, Download, Edit, StarFilled, Timer
} from '@element-plus/icons-vue'
import PipelineFeatureGuard from './PipelineFeatureGuard.vue'
import { isSystemPipeline, resolvePipelineFeatureSupport } from '@/utils/pipeline/features'
import { canConfigureCapabilitySlots } from '@/utils/capabilitySlots'
import type { Pipeline } from '@/api/pipeline'

const props = defineProps<{
  row: Pipeline
  isAdmin: boolean
  defaultPipelineId: string
}>()

const emit = defineEmits<{ (e: 'command', command: string): void }>()

const { t } = useI18n()

const editDisabled = computed(() => !resolvePipelineFeatureSupport('pipelineEdit', props.row, props.isAdmin))
const historyDisabled = computed(() => !resolvePipelineFeatureSupport('executionHistory', props.row, props.isAdmin))
const exportDisabled = computed(() => !resolvePipelineFeatureSupport('pipelineExport', props.row, props.isAdmin))

const submenuOpen = ref(false)
const submenuRef = ref<HTMLElement | null>(null)
const submenuPopRef = ref<HTMLElement | null>(null)
const submenuPosition = ref({ left: '0px', top: '0px' })
const popStyle = computed(() => ({
  left: submenuPosition.value.left,
  top: submenuPosition.value.top,
}))
let hideTimer: ReturnType<typeof setTimeout> | undefined
const openSubmenu = () => {
  window.clearTimeout(hideTimer)
  submenuOpen.value = true
  nextTick(() => {
    const el = submenuRef.value
    const pop = submenuPopRef.value
    if (!el || !pop) return
    const rect = el.getBoundingClientRect()
    const gap = 8
    const margin = 8
    let left = rect.right + gap
    if (left + pop.offsetWidth > window.innerWidth - margin) {
      left = Math.max(margin, rect.left - pop.offsetWidth - gap)
    }
    const top = Math.min(Math.max(0, rect.top - 6), Math.max(0, window.innerHeight - pop.offsetHeight - 4))
    submenuPosition.value = { left: `${left}px`, top: `${top}px` }
  })
}
const cancelHide = () => {
  window.clearTimeout(hideTimer)
}
const closeSubmenu = (e?: MouseEvent) => {
  const pop = submenuPopRef.value
  if (e && pop && e.relatedTarget instanceof Node && pop.contains(e.relatedTarget)) return
  window.clearTimeout(hideTimer)
  hideTimer = window.setTimeout(() => {
    submenuOpen.value = false
  }, 120)
}
const onVisibleChange = (visible: boolean) => {
  if (!visible) {
    window.clearTimeout(hideTimer)
    submenuOpen.value = false
  }
}
</script>

<style scoped>
.pipeline-submenu {
  position: relative;
}

.pipeline-submenu__trigger {
  justify-content: space-between;
  min-width: 8em;
}

.pipeline-submenu__pop {
  position: fixed;
  padding: 5px 0;
  z-index: 3000;
  list-style: none;
  border: 1px solid var(--el-border-color-light);
  box-shadow: var(--el-box-shadow-light);
  border-radius: var(--el-border-radius-base);
  background-color: var(--el-bg-color);
  white-space: nowrap;
}

.pipeline-submenu__pop :deep(.el-dropdown-menu__item) {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 0 16px;
  height: 32px;
  font-size: 13px;
  color: var(--el-text-color-regular);
  cursor: pointer;
  outline: none;
}

.pipeline-submenu__pop :deep(.el-dropdown-menu__item:hover) {
  background-color: var(--el-fill-color-light);
  color: var(--el-text-color-primary);
}

.pipeline-submenu__pop :deep(.el-dropdown-menu__item.is-disabled) {
  color: var(--el-text-color-placeholder);
  cursor: not-allowed;
}
</style>
