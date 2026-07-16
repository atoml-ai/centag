<template>
  <div class="proxy-modes-page">
    <!-- 页面头部 -->
    <div class="header-with-toolbar">
      <div class="header-left">
        <h1 class="page-title">
          <el-icon><Connection /></el-icon>
          代理模式配置
        </h1>
        <p class="page-description">
          配置代理模式关键字，用户可通过请求头、请求体或内容前缀指定代理策略
          <br/>
          <el-tag type="info" size="small" style="margin-left: 8px">关键字格式：# + 单字母（如 #d, #s, #m）</el-tag>
        </p>
      </div>
      <div class="toolbar-actions">
        <el-button :loading="loading" @click="loadData">
          <el-icon><Refresh /></el-icon>
          刷新
        </el-button>
        <el-button type="primary" @click="openCreate">
          <el-icon><Plus /></el-icon>
          添加模式
        </el-button>
      </div>
    </div>

    <div class="proxy-modes-content">
      <!-- 模式关键字列表 -->
      <el-card class="table-card" v-loading="loading">
        <el-table :data="keywords" stripe size="large">
          <el-table-column prop="mode_key" label="关键字" width="100" align="center">
            <template #default="{ row }">
              <el-tag type="warning" size="large">{{ row.mode_key }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="mode_name" label="名称" min-width="120">
            <template #default="{ row }">
              <span style="font-weight: 500">{{ row.mode_name }}</span>
            </template>
          </el-table-column>
          <el-table-column prop="mode_type" label="类型" width="120" align="center">
            <template #default="{ row }">
              <el-tag :type="getTypeTag(row.mode_type)" size="small">
                {{ getTypeLabel(row.mode_type) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="description" label="描述" min-width="200" show-overflow-tooltip />
          <el-table-column prop="is_enabled" label="启用" width="80" align="center">
            <template #default="{ row }">
              <el-switch
                :model-value="row.is_enabled"
                @change="toggleStatus(row)"
                active-color="#10b981"
              />
            </template>
          </el-table-column>
          <el-table-column prop="sort_order" label="排序" width="80" align="center">
            <template #default="{ row }">
              {{ row.sort_order }}
            </template>
          </el-table-column>
          <el-table-column label="操作" width="180" align="center" fixed="right">
            <template #default="{ row }">
              <el-button type="primary" size="small" @click="openEdit(row)">
                <el-icon><Edit /></el-icon>
                编辑
              </el-button>
              <el-button type="danger" size="small" @click="handleDelete(row)">
                <el-icon><Delete /></el-icon>
                删除
              </el-button>
            </template>
          </el-table-column>
        </el-table>

        <el-empty v-if="!loading && keywords.length === 0" description="暂无代理模式配置" :image-size="120" />
      </el-card>

      <!-- 使用示例 -->
      <el-card class="example-card" style="margin-top: 20px">
        <template #header>
          <div class="card-header">
            <span>📖 使用示例</span>
          </div>
        </template>
        <el-descriptions :column="1" border>
          <el-descriptions-item label="请求头方式">
            <code>X-Centag-Mode: #d</code>
            <br/>
            <small>在请求头中添加自定义字段指定模式</small>
          </el-descriptions-item>
          <el-descriptions-item label="请求体扩展">
            <code>{"centag": {"mode": "#s"}}</code>
            <br/>
            <small>在 JSON 请求体中添加 centag 字段</small>
          </el-descriptions-item>
          <el-descriptions-item label="内容前缀方式">
            <code>#d 你好，请帮我...</code>
            <br/>
            <code>#m /backend:ollama 这个问题...</code>
            <br/>
            <small>在消息内容开头添加模式关键字，系统会自动解析并移除</small>
          </el-descriptions-item>
          <el-descriptions-item label="API 会话方式">
            <code>POST /api/v1/session/proxy-mode</code>
            <br/>
            <small>通过 API 设置会话级代理模式，有效期内的所有请求都使用该模式</small>
          </el-descriptions-item>
        </el-descriptions>
      </el-card>
    </div>

    <!-- 编辑/创建对话框 -->
    <el-dialog
      v-model="editing"
      :title="isCreate ? '添加代理模式' : '编辑代理模式'"
      width="600px"
      @close="resetForm"
    >
      <el-form :model="form" :rules="rules" ref="formRef" label-width="100px">
        <el-form-item label="关键字" prop="mode_key">
          <el-input 
            v-model="form.mode_key" 
            placeholder="#d"
            maxlength="10"
            :disabled="!isCreate"
          />
          <div class="form-tip">格式：# + 单字母（如 #d, #s, #m），创建后不可修改</div>
        </el-form-item>
        <el-form-item label="名称" prop="mode_name">
          <el-input v-model="form.mode_name" placeholder="如：指定后端" />
        </el-form-item>
        <el-form-item label="类型" prop="mode_type">
          <el-select v-model="form.mode_type" placeholder="选择模式类型" style="width: 100%">
            <el-option label="指定后端 (direct)" value="direct" />
            <el-option label="智能调度 (schedule)" value="schedule" />
            <el-option label="模型匹配 (match)" value="match" />
            <el-option label="意图分类 (classify)" value="classify" />
            <el-option label="透明模式 (transparent)" value="transparent" />
            <el-option label="降级容错 (fallback)" value="fallback" />
          </el-select>
        </el-form-item>
        <el-form-item label="描述" prop="description">
          <el-input 
            v-model="form.description" 
            type="textarea" 
            :rows="3"
            placeholder="描述该模式的用途和行为"
          />
        </el-form-item>
        <el-form-item label="启用状态">
          <el-switch v-model="form.is_enabled" active-color="#10b981" />
          <div class="form-tip">禁用后该关键字将不再生效</div>
        </el-form-item>
        <el-form-item label="排序">
          <el-input-number v-model="form.sort_order" :min="0" :max="999" />
          <div class="form-tip">数字越小越靠前</div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editing = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import api from '@/api'

interface ModeKeyword {
  id: number
  mode_key: string
  mode_name: string
  mode_type: string
  description: string
  is_enabled: boolean
  sort_order: number
  config?: any
}

const loading = ref(false)
const saving = ref(false)
const editing = ref(false)
const isCreate = ref(false)
const keywords = ref<ModeKeyword[]>([])
const formRef = ref()

const form = ref<ModeKeyword>({
  id: 0,
  mode_key: '',
  mode_name: '',
  mode_type: 'direct',
  description: '',
  is_enabled: true,
  sort_order: 0,
})

const rules = {
  mode_key: [
    { required: true, message: '请输入关键字', trigger: 'blur' },
    { pattern: /^#[a-zA-Z]$/, message: '格式：# + 单字母（如 #d, #s）', trigger: 'blur' }
  ],
  mode_name: [{ required: true, message: '请输入名称', trigger: 'blur' }],
  mode_type: [{ required: true, message: '请选择类型', trigger: 'change' }]
}

const loadData = async () => {
  loading.value = true
  try {
    const res = await api.get('/api/v1/proxy-modes')
    keywords.value = res || []
  } catch (error: any) {
    ElMessage.error('加载失败：' + error.message)
  } finally {
    loading.value = false
  }
}

const openCreate = () => {
  isCreate.value = true
  editing.value = true
  form.value = {
    id: 0,
    mode_key: '',
    mode_name: '',
    mode_type: 'direct',
    description: '',
    is_enabled: true,
    sort_order: keywords.value.length + 1,
  }
}

const openEdit = (row: ModeKeyword) => {
  isCreate.value = false
  editing.value = true
  form.value = { ...row }
}

const save = async () => {
  if (!formRef.value) return
  
  await formRef.value.validate(async (valid: boolean) => {
    if (!valid) return
    
    saving.value = true
    try {
      if (isCreate.value) {
        await api.post('/api/v1/proxy-modes', form.value)
        ElMessage.success('创建成功')
      } else {
        await api.put(`/api/v1/proxy-modes/${form.value.mode_key}`, form.value)
        ElMessage.success('更新成功')
      }
      editing.value = false
      loadData()
    } catch (error: any) {
      ElMessage.error('保存失败：' + error.message)
    } finally {
      saving.value = false
    }
  })
}

const handleDelete = async (row: ModeKeyword) => {
  try {
    await ElMessageBox.confirm(`确定要删除模式 "${row.mode_name}" 吗？`, '确认删除', {
      type: 'warning'
    })
    await api.delete(`/api/v1/proxy-modes/${row.mode_key}`)
    ElMessage.success('删除成功')
    loadData()
  } catch (error: any) {
    if (error !== 'cancel') {
      ElMessage.error('删除失败：' + error.message)
    }
  }
}

const toggleStatus = async (row: ModeKeyword) => {
  try {
    row.is_enabled = !row.is_enabled
    await api.put(`/api/v1/proxy-modes/${row.mode_key}`, row)
    ElMessage.success(row.is_enabled ? '已启用' : '已禁用')
  } catch (error: any) {
    row.is_enabled = !row.is_enabled
    ElMessage.error('操作失败：' + error.message)
  }
}

const resetForm = () => {
  if (formRef.value) {
    formRef.value.resetFields()
  }
}

const getTypeTag = (type: string) => {
  const map: Record<string, string> = {
    direct: 'success',
    schedule: 'primary',
    match: 'warning',
    classify: 'info',
    transparent: '',
    fallback: 'danger'
  }
  return map[type] || ''
}

const getTypeLabel = (type: string) => {
  const map: Record<string, string> = {
    direct: '直连后端',
    schedule: '智能调度',
    match: '模型匹配',
    classify: '意图分类',
    transparent: '透明模式',
    'transparent-fast': '透明模式（快）',
    'raw-forward': '原始HTTP转发',
    fallback: '降级容错'
  }
  return map[type] || type
}

onMounted(() => {
  loadData()
})
</script>

<style scoped>
.proxy-modes-page {
  padding: 20px;
}

.header-with-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 20px;
}

.header-left {
  flex: 1;
}

.page-title {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0 0 8px 0;
  font-size: 24px;
  font-weight: 600;
}

.page-description {
  margin: 0;
  color: #666;
  font-size: 14px;
  line-height: 1.6;
}

.toolbar-actions {
  display: flex;
  gap: 10px;
}

.proxy-modes-content {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.table-card {
  min-height: 400px;
}

.form-tip {
  margin-top: 4px;
  font-size: 12px;
  color: #999;
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-weight: 600;
}

code {
  background: #f5f7fa;
  padding: 2px 6px;
  border-radius: 4px;
  font-family: 'Courier New', monospace;
  font-size: 13px;
}
</style>
