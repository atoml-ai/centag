<template>
  <el-dialog
    :model-value="modelValue"
    :title="t('skillManager.title')"
    width="720px"
    :close-on-click-modal="false"
    @update:model-value="onUpdateModelValue"
  >
    <div class="sm-body">
      <div class="sm-toolbar">
        <el-button type="primary" size="small" @click="openCreate">新建 Skill</el-button>
      </div>
      <div class="sm-list">
        <div v-for="skill in skills" :key="skill.name" class="sm-item">
          <div class="sm-item-main">
            <div class="sm-item-name">
              {{ skill.name }}
              <el-tag v-if="!skill.custom && skill.internal" size="small" type="info">内置</el-tag>
              <el-tag v-else size="small" type="success">自定义</el-tag>
            </div>
            <div class="sm-item-desc">{{ skill.description }}</div>
            <div class="sm-item-meta">
              <span v-if="skill.pipeline_id">pipeline: {{ skill.pipeline_id }}</span>
            </div>
          </div>
          <div class="sm-item-actions">
            <el-button size="small" @click="openClone(skill)">复制</el-button>
            <el-button size="small" @click="openEdit(skill)">编辑</el-button>
            <el-button size="small" type="danger" :disabled="skill.internal && !skill.custom" @click="handleDelete(skill)">
              删除
            </el-button>
          </div>
        </div>
        <el-empty v-if="skills.length === 0" description="暂无 Skill" />
      </div>
    </div>
    <template #footer>
      <el-button @click="onUpdateModelValue(false)">{{ t('common.close') }}</el-button>
    </template>

    <el-dialog
      v-model="dialogVisible"
      :title="dialogTitle"
      width="640px"
      :close-on-click-modal="false"
      append-to-body
    >
      <el-form :model="form" label-width="96px" class="sm-form">
        <el-form-item label="名称" required>
          <el-input v-model="form.name" :disabled="editing" placeholder="skill 注册名（小写字母/数字/-）" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="form.description" placeholder="skill 功能描述" />
        </el-form-item>
        <el-form-item label="分类">
          <el-input v-model="form.category" placeholder="如：运维诊断 / 配置管理 / 策略优化" />
        </el-form-item>
        <el-form-item label="工具">
          <el-select v-model="form.tools" multiple placeholder="选择可用工具" style="width: 100%">
            <el-option
              v-for="t in availableTools"
              :key="t"
              :label="t"
              :value="t"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="步骤">
          <el-select v-model="form.steps" multiple filterable allow-create default-first-option placeholder="输入步骤后回车" style="width: 100%">
            <el-option v-for="s in form.steps" :key="s" :label="s" :value="s" />
          </el-select>
        </el-form-item>
        <el-form-item label="系统提示词">
          <el-input
            v-model="form.system_prompt"
            type="textarea"
            :rows="6"
            placeholder="skill 专属系统提示词（将替换默认提示词注入）"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleSubmit">保存</el-button>
      </template>
    </el-dialog>
  </el-dialog>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useAgentStore } from '@/stores/agent'
import type { Skill } from '@/api/agent'

const { t } = useI18n()
const agentStore = useAgentStore()

const props = defineProps<{
  modelValue: boolean
  skills: Skill[]
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
}>()

const onUpdateModelValue = (value: boolean) => emit('update:modelValue', value)

const dialogVisible = ref(false)
const submitting = ref(false)
const editing = ref(false)
const dialogTitle = ref('新建 Skill')

// 可勾选工具：从 GET /skills 响应中的工具并集 + 默认白名单
const availableTools = [
  'read_config',
  'read_log',
  'read_database',
  'write_config',
  'analyze',
  'system_info',
  'centag_info'
]

const form = reactive<{
  name: string
  description: string
  category: string
  tools: string[]
  steps: string[]
  system_prompt: string
}>({
  name: '',
  description: '',
  category: '',
  tools: [],
  steps: [],
  system_prompt: ''
})

const resetForm = () => {
  form.name = ''
  form.description = ''
  form.category = ''
  form.tools = []
  form.steps = []
  form.system_prompt = ''
  editing.value = false
}

const openCreate = () => {
  resetForm()
  dialogTitle.value = '新建 Skill'
  dialogVisible.value = true
}

const openEdit = (skill: Skill) => {
  editing.value = true
  dialogTitle.value = `编辑 Skill: ${skill.name}`
  form.name = skill.name
  form.description = skill.description || ''
  form.category = skill.category || ''
  form.tools = skill.tools || []
  form.steps = skill.steps || []
  form.system_prompt = skill.system_prompt || ''
  dialogVisible.value = true
}

const openClone = (skill: Skill) => {
  resetForm()
  dialogTitle.value = `复制 ${skill.name}`
  form.name = `${skill.name}-copy`
  form.description = skill.description || ''
  form.category = skill.category || ''
  form.tools = skill.tools || []
  form.steps = skill.steps || []
  form.system_prompt = skill.system_prompt || ''
  dialogVisible.value = true
}

const handleSubmit = async () => {
  if (!form.name.trim()) {
    ElMessage.warning('名称不能为空')
    return
  }
  submitting.value = true
  try {
    if (editing.value) {
      await agentStore.updateSkill(form.name, { ...form })
      ElMessage.success('Skill 已更新')
    } else {
      const exists = props.skills.some(s => s.name === form.name)
      if (exists) {
        ElMessage.warning('同名 Skill 已存在')
        return
      }
      await agentStore.createSkill({ ...form })
      ElMessage.success('Skill 已创建')
    }
    dialogVisible.value = false
  } catch (error: any) {
    const msg = error?.response?.data?.error || '操作失败'
    ElMessage.error(msg)
  } finally {
    submitting.value = false
  }
}

const handleDelete = async (skill: Skill) => {
  try {
    await ElMessageBox.confirm(`确定删除 Skill「${skill.name}」？该操作不可恢复。`, '删除确认', {
      type: 'warning'
    })
    await agentStore.deleteSkill(skill.name)
    ElMessage.success('Skill 已删除')
  } catch (error: any) {
    if (error === 'cancel' || error?.toString().includes('cancel')) return
    const msg = error?.response?.data?.error || '删除失败'
    ElMessage.error(msg)
  }
}
</script>

<style scoped>
.sm-body {
  display: flex;
  flex-direction: column;
}

.sm-toolbar {
  display: flex;
  justify-content: flex-end;
  padding-bottom: 12px;
}

.sm-list {
  max-height: 55vh;
  overflow-y: auto;
}

.sm-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 12px;
  margin-bottom: 8px;
  border: 1px solid #eee;
  border-radius: 8px;
}

.sm-item:hover {
  background: #f8f8f8;
}

.sm-item-main {
  flex: 1;
  min-width: 0;
  margin-right: 12px;
}

.sm-item-name {
  font-size: 14px;
  font-weight: 500;
  display: flex;
  align-items: center;
  gap: 8px;
}

.sm-item-desc {
  font-size: 12px;
  color: #666;
  margin-top: 4px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sm-item-meta {
  font-size: 11px;
  color: #999;
  margin-top: 2px;
}

.sm-item-actions {
  display: flex;
  gap: 4px;
  flex-shrink: 0;
}
</style>
