<template>
  <div class="system-update-page">
    <div class="header-with-toolbar">
      <div class="header-left">
        <h1 class="page-title">系统更新</h1>
        <p class="page-description">上传更新包进行热更新，服务期间不会中断</p>
      </div>
      <div class="toolbar-actions">
        <el-button :loading="loadingHistory" @click="loadHistory">
          <el-icon><Refresh /></el-icon>刷新记录
        </el-button>
      </div>
    </div>

    <el-row :gutter="24">
      <!-- 上传更新包 -->
      <el-col :xs="24" :md="10">
        <el-card shadow="never" class="su-card">
          <template #header>
            <div class="su-section-hd">
              <span class="su-section-icon upload-color"><el-icon><Upload /></el-icon></span>
              <div>
                <div class="su-section-title">上传更新包</div>
                <div class="su-section-sub">支持 .tar.gz 格式的更新压缩包</div>
              </div>
            </div>
          </template>

          <el-upload
            ref="uploadRef"
            drag
            :auto-upload="false"
            :limit="1"
            accept=".tar.gz,.tgz"
            :on-change="handleFileChange"
            :on-exceed="handleExceed"
            :on-remove="() => (selectedFile = null)"
          >
            <el-icon style="font-size:48px;color:#667eea;opacity:.75"><UploadFilled /></el-icon>
            <div style="margin-top:12px;color:var(--el-text-color-regular)">
              拖拽文件至此，或 <em style="color:#667eea;font-style:normal;cursor:pointer">点击选择</em>
            </div>
            <div style="margin-top:6px;font-size:12px;color:var(--el-text-color-secondary)">
              仅支持 .tar.gz 格式（通过 ./start.sh pack 生成）
            </div>
          </el-upload>

          <el-button
            type="primary"
            size="large"
            :loading="updating"
            :disabled="!selectedFile"
            @click="doUpdate"
            style="width:100%;margin-top:16px"
          >
            {{ updating ? '更新中，请稍候…' : '开始更新' }}
          </el-button>

          <div v-if="updateLog" class="su-log">
            <div class="su-log-hd"><el-icon><Document /></el-icon> 更新输出</div>
            <pre class="su-log-body">{{ updateLog }}</pre>
          </div>
        </el-card>
      </el-col>

      <!-- 更新历史 -->
      <el-col :xs="24" :md="14">
        <el-card shadow="never" class="su-card">
          <template #header>
            <div class="su-section-hd">
              <span class="su-section-icon history-color"><el-icon><Clock /></el-icon></span>
              <div>
                <div class="su-section-title">更新历史</div>
                <div class="su-section-sub">历史更新记录与回滚操作</div>
              </div>
            </div>
          </template>

          <el-table
            :data="history"
            v-loading="loadingHistory"
            empty-text="暂无更新记录"
            stripe
            size="large"
          >
            <el-table-column label="版本" prop="version" min-width="100">
              <template #default="{ row }">
                <code class="ver-badge">{{ row.version || '—' }}</code>
              </template>
            </el-table-column>
            <el-table-column label="更新时间" prop="time" min-width="140" />
            <el-table-column label="状态" width="90" align="center">
              <template #default="{ row }">
                <el-tag :type="row.success ? 'success' : 'danger'" size="small" effect="light">
                  {{ row.success ? '成功' : '失败' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="80" align="center">
              <template #default="{ row }">
                <el-dropdown trigger="click">
                  <el-button type="primary" link>
                    <el-icon><MoreFilled /></el-icon>
                  </el-button>
                  <template #dropdown>
                    <el-dropdown-menu>
                      <el-dropdown-item v-if="row.can_rollback" @click="rollback(row)">
                        <el-icon><RefreshLeft /></el-icon>回滚到此版本
                      </el-dropdown-item>
                      <el-dropdown-item :divided="row.can_rollback" @click="deleteRecord(row)">
                        <el-icon><Delete /></el-icon>删除记录
                      </el-dropdown-item>
                    </el-dropdown-menu>
                  </template>
                </el-dropdown>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Upload, UploadFilled, Clock, Refresh, Document, RefreshLeft, Delete, MoreFilled } from '@element-plus/icons-vue'
import type { UploadInstance, UploadFile } from 'element-plus'
import api from '@/api/index'

const uploadRef = ref<UploadInstance>()
const selectedFile = ref<File | null>(null)
const updating = ref(false)
const updateLog = ref('')

interface HistoryItem {
  version: string
  time: string
  success: boolean
  can_rollback: boolean
  package?: string
  history_file?: string
}
const history = ref<HistoryItem[]>([])
const loadingHistory = ref(false)

onMounted(loadHistory)

function handleFileChange(file: UploadFile) { selectedFile.value = file.raw ?? null }
function handleExceed() { ElMessage.warning('只能上传一个文件，请先移除已选文件') }

async function doUpdate() {
  if (!selectedFile.value) return
  updating.value = true; updateLog.value = ''
  const fd = new FormData()
  fd.append('package', selectedFile.value)
  try {
    const resp = await api.post('/api/v1/system/update', fd, { headers: { 'Content-Type': 'multipart/form-data' } })
    updateLog.value = typeof resp === 'string' ? resp : JSON.stringify(resp, null, 2)
    ElMessage.success('系统更新成功')
    uploadRef.value?.clearFiles(); selectedFile.value = null
    loadHistory()
  } catch (e: any) {
    ElMessage.error(e.message || '更新失败'); updateLog.value = e.message || '更新失败'
  } finally { updating.value = false }
}

async function loadHistory() {
  loadingHistory.value = true
  try {
    const data = await api.get('/api/v1/system/update/history')
    const rawList = Array.isArray(data) ? data : (data?.history || [])
    history.value = rawList.map((row: any) => ({
      version: row.version || '—',
      time: formatHistoryTime(row.start_time || row.end_time || ''),
      success: !!row.success,
      can_rollback: !!row.success && !!row.history_file,
      package: row.package_name || row.package,
      history_file: row.history_file,
    }))
  } catch { history.value = [] }
  finally { loadingHistory.value = false }
}

async function rollback(item: HistoryItem) {
  if (!item.history_file) {
    ElMessage.error('缺少历史记录文件名，无法回滚')
    return
  }
  try {
    await ElMessageBox.confirm(`确定回滚到版本「${item.version}」？`, '回滚确认', { confirmButtonText: '确认回滚', cancelButtonText: '取消', type: 'warning' })
    const fd = new FormData()
    fd.append('history_file', item.history_file)
    await api.post('/api/v1/system/rollback', fd)
    ElMessage.success('回滚成功'); loadHistory()
  } catch (e: any) { if (e !== 'cancel' && e?.message) ElMessage.error(e.message) }
}

async function deleteRecord(item: HistoryItem) {
  if (!item.history_file) {
    ElMessage.error('缺少历史记录文件名，无法删除')
    return
  }
  try {
    await ElMessageBox.confirm('确定删除此更新记录？', '删除确认', { confirmButtonText: '删除', cancelButtonText: '取消', type: 'warning' })
    const fd = new FormData()
    fd.append('history_file', item.history_file)
    await api.post('/api/v1/system/delete-update', fd)
    ElMessage.success('记录已删除'); loadHistory()
  } catch (e: any) { if (e !== 'cancel' && e?.message) ElMessage.error(e.message) }
}

function formatHistoryTime(raw: string): string {
  if (!raw) return '—'
  const dt = new Date(raw)
  if (Number.isNaN(dt.getTime())) return raw
  return dt.toLocaleString()
}
</script>

<style scoped>
.header-with-toolbar {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  margin-bottom: var(--spacing-lg);
  flex-wrap: wrap;
  gap: var(--spacing-md);
}

.header-left { flex: 1; }
.toolbar-actions { display: flex; align-items: center; gap: var(--spacing-sm); }

.su-card { width: 100%; }

.su-section-hd {
  display: flex;
  align-items: center;
  gap: 12px;
}

.su-section-icon {
  width: 36px;
  height: 36px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 18px;
  flex-shrink: 0;
}

.upload-color {
  background: rgba(102,126,234,.12);
  color: #667eea;
}

.history-color {
  background: rgba(16,185,129,.12);
  color: #10b981;
}

.su-section-title {
  font-size: .9375rem;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.su-section-sub {
  font-size: .8125rem;
  color: var(--el-text-color-secondary);
  margin-top: 2px;
}

.su-log {
  margin-top: 16px;
  border: 1px solid var(--el-border-color-light);
  border-radius: 8px;
  overflow: hidden;
}

.su-log-hd {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 14px;
  background: var(--el-fill-color-light);
  font-size: .8125rem;
  font-weight: 500;
  color: var(--el-text-color-secondary);
  border-bottom: 1px solid var(--el-border-color-light);
}

.su-log-body {
  padding: 12px 14px;
  margin: 0;
  font-size: .8125rem;
  font-family: 'Menlo', 'Monaco', 'Consolas', monospace;
  white-space: pre-wrap;
  word-break: break-all;
  max-height: 200px;
  overflow-y: auto;
  color: var(--el-text-color-regular);
}

.ver-badge {
  font-family: monospace;
  font-size: .8125rem;
  background: var(--el-fill-color-light);
  padding: 2px 8px;
  border-radius: 4px;
}
</style>
