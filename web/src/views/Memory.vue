<template>
  <div class="memory">
    <div class="header">
      <h1 class="page-title">云记忆管理</h1>
      <p class="page-description">管理 Agent 记忆云端存储，支持语义搜索与版本控制</p>
    </div>

    <div class="content-wrapper full-width">
      <el-row :gutter="16">
        <!-- 左侧：统计信息 -->
        <el-col :xs="24" :sm="24" :md="5" :lg="4" :xl="3">
          <div class="left-panel">
            <!-- 记忆统计 -->
            <el-card class="stats-card">
              <template #header>
                <span class="card-title">记忆统计</span>
              </template>
              <div class="stats-content">
                <div class="stat-item-large">
                  <span class="stat-number">{{ memoryStats.vector_count || 0 }}</span>
                  <span class="stat-label">向量条目</span>
                </div>
                <div class="stat-item-large">
                  <span class="stat-number">{{ memoryStats.indexed_files?.length || 0 }}</span>
                  <span class="stat-label">记忆文件</span>
                </div>
                <div v-if="memoryStats.index_queue_enabled" class="queue-stats">
                  <div class="queue-row">
                    <span class="queue-label">索引队列长度</span>
                    <el-tag size="small" type="info">{{ memoryStats.index_queue_length || 0 }}</el-tag>
                  </div>
                  <div class="queue-row">
                    <span class="queue-label">已处理</span>
                    <el-tag size="small" type="success">{{ memoryStats.index_tasks_processed || 0 }}</el-tag>
                  </div>
                  <div class="queue-row">
                    <span class="queue-label">失败</span>
                    <el-tag size="small" :type="(memoryStats.index_tasks_failed || 0) > 0 ? 'danger' : 'success'">
                      {{ memoryStats.index_tasks_failed || 0 }}
                    </el-tag>
                  </div>
                  <div class="queue-row">
                    <span class="queue-label">丢弃</span>
                    <el-tag size="small" :type="(memoryStats.index_tasks_dropped || 0) > 0 ? 'warning' : 'info'">
                      {{ memoryStats.index_tasks_dropped || 0 }}
                    </el-tag>
                  </div>
                  <div v-if="memoryStats.index_last_error" class="queue-error">
                    最近错误：{{ memoryStats.index_last_error }}
                  </div>
                </div>
              </div>
            </el-card>

            <!-- 当前 Agent -->
            <el-card class="info-card">
              <template #header>
                <span class="card-title">当前 Agent</span>
              </template>
              <el-select v-model="currentAgent" placeholder="选择 Agent" @change="handleAgentChange">
                <el-option label="main" value="main" />
                <el-option v-for="agent in agentList" :key="agent" :label="agent" :value="agent" />
              </el-select>
            </el-card>

            <!-- 写运维：仅 memoryFull（personal）；team_user 为查询模式 -->
            <el-card v-if="memoryFull" class="actions-card">
              <el-button :loading="syncing" @click="handleSync" style="width: 100%">
                <el-icon><Upload /></el-icon>
                同步到云端
              </el-button>
              <el-button :loading="pulling" @click="handlePull" style="width: 100%; margin-left: 0; margin-top: 8px">
                <el-icon><Download /></el-icon>
                从云端拉取
              </el-button>
              <el-button :loading="indexing" @click="handleBuildIndex" style="width: 100%; margin-left: 0; margin-top: 8px">
                <el-icon><Search /></el-icon>
                重建索引
              </el-button>
            </el-card>
          </div>
        </el-col>

        <!-- 右侧：主要内容 -->
        <el-col :xs="24" :sm="24" :md="19" :lg="20" :xl="21">
          <!-- 搜索栏 -->
          <el-card class="search-card">
            <el-row :gutter="12">
              <el-col :span="18">
                <el-input
                  v-model="searchQuery"
                  placeholder="搜索记忆内容..."
                  @keyup.enter="handleSearch"
                >
                  <template #prefix>
                    <el-icon><Search /></el-icon>
                  </template>
                </el-input>
              </el-col>
              <el-col :span="6">
                <el-button type="primary" :loading="searching" @click="handleSearch" style="width: 100%">
                  搜索
                </el-button>
              </el-col>
            </el-row>
          </el-card>

          <!-- 搜索结果 / 文件列表 -->
          <el-card class="list-card">
            <template #header>
              <div class="card-header">
                <span class="card-title">{{ searchQuery ? '搜索结果' : '记忆文件' }}</span>
                <el-button
                  v-if="memoryFull"
                  type="primary"
                  size="small"
                  @click="showAddDialog = true"
                >
                  <el-icon><Plus /></el-icon>
                  新建文件
                </el-button>
              </div>
            </template>

            <!-- 标签页 -->
            <el-tabs v-model="activeTab" @tab-change="handleTabChange">
              <el-tab-pane label="文件列表" name="files">
                <div v-if="loading" class="loading-wrapper">
                  <el-skeleton :rows="10" animated />
                </div>
                <el-table v-else :data="fileList" stripe style="width: 100%">
                  <el-table-column prop="name" label="文件名" min-width="200">
                    <template #default="{ row }">
                      <el-link type="primary" @click="handleViewFile(row)">{{ row.name }}</el-link>
                    </template>
                  </el-table-column>
                  <el-table-column prop="size" label="大小" width="100">
                    <template #default="{ row }">
                      {{ formatSize(row.size) }}
                    </template>
                  </el-table-column>
                  <el-table-column prop="modified" label="修改时间" width="180">
                    <template #default="{ row }">
                      {{ formatTime(row.modified) }}
                    </template>
                  </el-table-column>
                  <el-table-column label="操作" width="180" fixed="right">
                    <template #default="{ row }">
                      <el-button-group>
                        <el-button size="small" @click="handleViewFile(row)">查看</el-button>
                        <el-button size="small" type="danger" @click="handleDeleteFile(row)">删除</el-button>
                      </el-button-group>
                    </template>
                  </el-table-column>
                </el-table>
              </el-tab-pane>

              <el-tab-pane label="搜索结果" name="results" v-if="searchResults.length > 0">
                <div v-for="(result, index) in searchResults" :key="index" class="search-result-item">
                  <div class="result-header">
                    <span class="result-path">{{ result.path }}</span>
                    <el-tag size="small" type="success">相似度: {{ (result.score * 100).toFixed(1) }}%</el-tag>
                  </div>
                  <div class="result-content">{{ result.content }}</div>
                </div>
              </el-tab-pane>

              <el-tab-pane label="版本历史" name="versions">
                <el-select v-model="versionFilePath" placeholder="选择文件" @change="loadVersions" style="width: 300px; margin-bottom: 16px;">
                  <el-option v-for="file in fileList" :key="file.name" :label="file.name" :value="file.name" />
                </el-select>
                <el-table v-if="versionList.length > 0" :data="versionList" stripe>
                  <el-table-column prop="version_id" label="版本 ID" width="200" />
                  <el-table-column prop="created_at" label="创建时间" width="180">
                    <template #default="{ row }">
                      {{ formatTime(row.created_at) }}
                    </template>
                  </el-table-column>
                  <el-table-column prop="lines" label="行数" width="80" />
                  <el-table-column prop="size" label="大小" width="100">
                    <template #default="{ row }">
                      {{ formatSize(row.size) }}
                    </template>
                  </el-table-column>
                  <el-table-column label="操作" width="120">
                    <template #default="{ row }">
                      <el-button size="small" @click="handleRestoreVersion(row)">恢复</el-button>
                    </template>
                  </el-table-column>
                </el-table>
                <el-empty v-else-if="versionFilePath" description="暂无版本记录" />
                <el-empty v-else description="请先选择文件" />
              </el-tab-pane>
            </el-tabs>
          </el-card>
        </el-col>
      </el-row>
    </div>

    <!-- 新建/编辑文件对话框 -->
    <el-dialog v-model="showAddDialog" :title="editingFile ? '编辑文件' : '新建文件'" width="600px">
      <el-form :model="fileForm" label-width="80px">
        <el-form-item label="文件名">
          <el-input v-model="fileForm.path" placeholder="memory/xxx.md" :disabled="!!editingFile" />
        </el-form-item>
        <el-form-item label="内容">
          <el-input v-model="fileForm.content" type="textarea" :rows="15" placeholder="记忆内容..." />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showAddDialog = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="handleSaveFile">保存</el-button>
      </template>
    </el-dialog>

    <!-- 查看文件对话框 -->
    <el-dialog v-model="showViewDialog" :title="viewingFile?.name" width="800px">
      <pre class="file-content">{{ viewingFile?.content }}</pre>
      <template #footer>
        <el-button @click="showViewDialog = false">关闭</el-button>
        <el-button v-if="memoryFull" type="primary" @click="handleEditFile">编辑</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Search, Upload, Download, Plus } from '@element-plus/icons-vue'
import { useAuthStore } from '@/stores/auth'
import { useEdition } from '@/composables/useEdition'
import { getCapabilities } from '@/utils/capabilities'

const authStore = useAuthStore()
const { edition } = useEdition()
const memoryFull = computed(
  () => getCapabilities(edition.value, authStore.isAdmin).memoryFull
)

function memoryAuthHeaders(json = false) {
  const h = { Authorization: `Bearer ${authStore.accessToken}` }
  if (json) {
    h['Content-Type'] = 'application/json'
  }
  return h
}

// 状态变量
const loading = ref(false)
const searching = ref(false)
const syncing = ref(false)
const pulling = ref(false)
const indexing = ref(false)
const saving = ref(false)

const currentAgent = ref('main')
const agentList = ref([])
const searchQuery = ref('')
const activeTab = ref('files')
const fileList = ref([])
const searchResults = ref([])
const versionList = ref([])
const versionFilePath = ref('')
const memoryStats = ref({})

const showAddDialog = ref(false)
const showViewDialog = ref(false)
const editingFile = ref(null)
const viewingFile = ref(null)

const fileForm = ref({
  path: '',
  content: ''
})

// API 基础 URL
const apiBase = ref('')

// 初始化：必须先拿到 stats（含 indexed_files），再建文件列表。OpenClaw 常只写 memory/*.md，无 MEMORY.md 时旧逻辑会因 get 404 清空列表。
onMounted(async () => {
  const baseUrl = import.meta.env.VITE_API_BASE_URL || ''
  apiBase.value = baseUrl
  await loadFileList()
})

// 获取记忆统计
async function loadMemoryStats() {
  try {
    const response = await fetch(`${apiBase.value}/api/v1/memory/stats?agent_id=${currentAgent.value}`, {
      headers: memoryAuthHeaders()
    })
    const data = await response.json()
    memoryStats.value = data
  } catch (error) {
    console.error('Failed to load memory stats:', error)
  }
}

// 文件列表完全以 stats.indexed_files 为准（含 memory/ 下 .md），不依赖根目录 MEMORY.md 是否存在
async function loadFileList() {
  loading.value = true
  try {
    await loadMemoryStats()
    const files = memoryStats.value.indexed_files
    if (!Array.isArray(files) || files.length === 0) {
      fileList.value = []
      return
    }
    fileList.value = files.map((name) => ({
      name,
      size: 0,
      modified: new Date()
    }))
  } catch (error) {
    console.error('Failed to load file list:', error)
    fileList.value = []
  } finally {
    loading.value = false
  }
}

// 搜索记忆
async function handleSearch() {
  if (!searchQuery.value) return
  if (!authStore.accessToken) {
    ElMessage.warning('请先登录后再搜索')
    return
  }

  searching.value = true
  try {
    const response = await fetch(`${apiBase.value}/api/v1/memory/search`, {
      method: 'POST',
      headers: memoryAuthHeaders(true),
      body: JSON.stringify({
        query: searchQuery.value,
        limit: 10,
        agent_id: currentAgent.value
      })
    })
    const data = await response.json()
    if (!response.ok) {
      searchResults.value = []
      ElMessage.error(data.error || `搜索失败（HTTP ${response.status}）`)
      return
    }
    searchResults.value = data.results || []
    activeTab.value = 'results'
  } catch (error) {
    ElMessage.error('搜索失败')
  } finally {
    searching.value = false
  }
}

// 同步到云端
async function handleSync() {
  syncing.value = true
  try {
    const response = await fetch(`${apiBase.value}/api/v1/memory/sync`, {
      method: 'POST',
      headers: memoryAuthHeaders(true),
      body: JSON.stringify({
        agent_id: currentAgent.value
      })
    })
    const data = await response.json()
    if (data.success) {
      ElMessage.success(`同步成功，${data.files_synced} 个文件`)
      await loadFileList()
    } else {
      ElMessage.error(data.error || '同步失败')
    }
  } catch (error) {
    ElMessage.error('同步失败')
  } finally {
    syncing.value = false
  }
}

// 从云端拉取
async function handlePull() {
  pulling.value = true
  try {
    const response = await fetch(`${apiBase.value}/api/v1/memory/pull`, {
      method: 'POST',
      headers: memoryAuthHeaders(true),
      body: JSON.stringify({
        agent_id: currentAgent.value
      })
    })
    const data = await response.json()
    if (data.success) {
      ElMessage.success(`拉取成功，${data.files_pulled} 个文件`)
    } else {
      ElMessage.error(data.error || '拉取失败')
    }
  } catch (error) {
    ElMessage.error('拉取失败')
  } finally {
    pulling.value = false
  }
}

// 重建索引
async function handleBuildIndex() {
  indexing.value = true
  try {
    const response = await fetch(`${apiBase.value}/api/v1/memory/index`, {
      method: 'POST',
      headers: memoryAuthHeaders(true),
      body: JSON.stringify({
        agent_id: currentAgent.value
      })
    })
    const data = await response.json()
    if (data.success) {
      ElMessage.success(`索引重建成功，${data.vector_count} 个向量`)
      await loadFileList()
    } else {
      ElMessage.error(data.error || '索引重建失败')
    }
  } catch (error) {
    ElMessage.error('索引重建失败')
  } finally {
    indexing.value = false
  }
}

// 查看文件
async function handleViewFile(row) {
  try {
    const response = await fetch(`${apiBase.value}/api/v1/memory/get?agent_id=${currentAgent.value}&path=${row.name}`, {
      headers: memoryAuthHeaders()
    })
    const data = await response.json()
    if (data.success) {
      viewingFile.value = {
        name: row.name,
        content: data.content
      }
      showViewDialog.value = true
    }
  } catch (error) {
    ElMessage.error('读取文件失败')
  }
}

// 编辑文件
function handleEditFile() {
  if (viewingFile.value) {
    fileForm.value = {
      path: viewingFile.value.name,
      content: viewingFile.value.content
    }
    editingFile.value = viewingFile.value
    showViewDialog.value = false
    showAddDialog.value = true
  }
}

// 保存文件
async function handleSaveFile() {
  if (!fileForm.value.path) {
    ElMessage.warning('请输入文件名')
    return
  }

  saving.value = true
  try {
    const response = await fetch(`${apiBase.value}/api/v1/memory/put`, {
      method: 'PUT',
      headers: memoryAuthHeaders(true),
      body: JSON.stringify({
        path: fileForm.value.path,
        content: fileForm.value.content,
        agent_id: currentAgent.value
      })
    })
    const data = await response.json()
    if (data.success) {
      ElMessage.success('保存成功')
      showAddDialog.value = false
      fileForm.value = { path: '', content: '' }
      editingFile.value = null
      await loadFileList()
    } else {
      ElMessage.error(data.error || '保存失败')
    }
  } catch (error) {
    ElMessage.error('保存失败')
  } finally {
    saving.value = false
  }
}

// 删除文件
async function handleDeleteFile(row) {
  try {
    await ElMessageBox.confirm(`确定要删除 ${row.name} 吗？`, '警告', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })

    const response = await fetch(`${apiBase.value}/api/v1/memory/doc?agent_id=${currentAgent.value}&path=${row.name}`, {
      method: 'DELETE',
      headers: memoryAuthHeaders()
    })
    const data = await response.json()
    if (data.success) {
      ElMessage.success('删除成功')
      await loadFileList()
    } else {
      ElMessage.error(data.error || '删除失败')
    }
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('删除失败')
    }
  }
}

// 加载版本历史
async function loadVersions() {
  if (!versionFilePath.value) return

  try {
    const response = await fetch(`${apiBase.value}/api/v1/memory/versions?agent_id=${currentAgent.value}&path=${versionFilePath.value}`, {
      headers: memoryAuthHeaders()
    })
    const data = await response.json()
    versionList.value = data.versions || []
  } catch (error) {
    console.error('Failed to load versions:', error)
    versionList.value = []
  }
}

// 恢复版本
async function handleRestoreVersion(row) {
  try {
    await ElMessageBox.confirm(`确定要恢复到版本 ${row.version_id} 吗？`, '确认', {
      confirmButtonText: '确定',
      cancelButtonText: '取消'
    })

    const response = await fetch(`${apiBase.value}/api/v1/memory/restore`, {
      method: 'POST',
      headers: memoryAuthHeaders(true),
      body: JSON.stringify({
        path: versionFilePath.value,
        version_id: row.version_id,
        agent_id: currentAgent.value
      })
    })
    const data = await response.json()
    if (data.success) {
      ElMessage.success('恢复成功')
      await loadFileList()
    } else {
      ElMessage.error(data.error || '恢复失败')
    }
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('恢复失败')
    }
  }
}

// 切换 Agent
async function handleAgentChange() {
  searchQuery.value = ''
  searchResults.value = []
  versionList.value = []
  versionFilePath.value = ''
  await loadFileList()
}

// 切换标签页
function handleTabChange(tab) {
  if (tab === 'versions' && !versionList.value.length && fileList.value.length > 0) {
    versionFilePath.value = fileList.value[0].name
    loadVersions()
  }
}

// 工具函数
function formatSize(bytes) {
  if (!bytes) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

function formatTime(date) {
  if (!date) return '-'
  return new Date(date).toLocaleString('zh-CN')
}
</script>

<style scoped>
.memory {
  padding: 0;
}

.header {
  margin-bottom: 24px;
}

.page-title {
  font-size: 24px;
  font-weight: 600;
  margin: 0 0 8px 0;
  color: #303133;
}

.page-description {
  font-size: 14px;
  color: #909399;
  margin: 0;
}

.content-wrapper {
  margin-top: 16px;
}

.left-panel {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.stats-card .stats-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.stat-item-large {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 12px 0;
}

.stat-number {
  font-size: 32px;
  font-weight: 600;
  color: #409eff;
}

.stat-label {
  font-size: 14px;
  color: #909399;
  margin-top: 4px;
}

.queue-stats {
  margin-top: 8px;
  border-top: 1px solid #ebeef5;
  padding-top: 10px;
}

.queue-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}

.queue-label {
  font-size: 12px;
  color: #606266;
}

.queue-error {
  margin-top: 6px;
  font-size: 12px;
  color: #f56c6c;
  line-height: 1.4;
  word-break: break-word;
}

.search-card {
  margin-bottom: 16px;
}

.list-card {
  min-height: 400px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.card-title {
  font-weight: 600;
}

.loading-wrapper {
  padding: 20px;
}

.search-result-item {
  padding: 12px;
  border-bottom: 1px solid #ebeef5;
}

.search-result-item:last-child {
  border-bottom: none;
}

.result-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.result-path {
  font-weight: 500;
  color: #303133;
}

.result-content {
  font-size: 14px;
  color: #606266;
  line-height: 1.6;
  background: #f5f7fa;
  padding: 12px;
  border-radius: 4px;
  white-space: pre-wrap;
  word-break: break-all;
  max-height: 200px;
  overflow-y: auto;
}

.file-content {
  white-space: pre-wrap;
  word-break: break-all;
  max-height: 500px;
  overflow-y: auto;
  background: #f5f7fa;
  padding: 16px;
  border-radius: 4px;
  font-size: 14px;
  line-height: 1.6;
}

.status-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.status-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.status-label {
  color: #909399;
  font-size: 14px;
}
</style>