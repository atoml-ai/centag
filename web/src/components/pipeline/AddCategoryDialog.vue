<template>
  <el-dialog
    v-model="visible"
    title="新增分类"
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
      title="将同时创建执行节点、写入路由关键词，并可选加入能力槽。"
    />
    <el-form label-width="110px" @submit.prevent>
      <el-form-item label="分类名称" required>
        <el-input v-model="form.label" placeholder="如：法务咨询" @blur="onLabelBlur" />
      </el-form-item>
      <el-form-item label="节点 ID">
        <el-input v-model="form.nodeId" placeholder="默认由名称生成" />
      </el-form-item>
      <el-form-item label="触发关键词" required>
        <el-select
          v-model="form.keywords"
          multiple
          filterable
          allow-create
          default-first-option
          placeholder="输入后回车添加，可多个"
          style="width: 100%"
        />
      </el-form-item>
      <el-form-item label="路由节点" required>
        <el-select v-model="form.routerNodeId" style="width: 100%" placeholder="选择 router">
          <el-option
            v-for="r in routers"
            :key="r.id"
            :label="`${r.name || r.id} (${r.id})`"
            :value="r.id"
          />
        </el-select>
      </el-form-item>
      <el-form-item label="默认分支">
        <el-switch v-model="form.isDefault" />
      </el-form-item>
      <el-form-item label="推荐标签">
        <el-select v-model="form.tags" multiple style="width: 100%" placeholder="可选">
          <el-option v-for="t in tagOptions" :key="t" :label="t" :value="t" />
        </el-select>
      </el-form-item>
      <el-form-item label="初始 Prompt">
        <el-input
          v-model="form.systemPrompt"
          type="textarea"
          :rows="3"
          placeholder="可选，写入节点 system_prompt"
        />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" @click="submit">添加到画布</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import { ElMessage } from 'element-plus'
import type { AgentPatternPipeline } from '@/api/pipeline'
import { applyAddCategory, listRouterNodes, slugifyNodeId } from '@/utils/capabilitySlots'

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
    ElMessage.warning('流水线未加载')
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
    ElMessage.success('已添加分类，请保存流水线后可在「配置模型」中绑定')
    visible.value = false
  } catch (err: any) {
    ElMessage.error(err?.message || '添加失败')
  }
}
</script>
