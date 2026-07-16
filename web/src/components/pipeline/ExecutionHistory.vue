<template>
  <el-dialog
    v-model="visible"
    :title="`执行历史 — ${pipelineName}`"
    width="900px"
    top="5vh"
    :close-on-click-modal="false"
    @open="loadHistory"
  >
    <div v-loading="loading">
      <!-- 统计摘要 -->
      <el-row :gutter="12" style="margin-bottom: 16px">
        <el-col :span="6">
          <el-statistic title="总执行次数" :value="records.length" />
        </el-col>
        <el-col :span="6">
          <el-statistic title="成功" :value="successCount">
            <template #suffix>
              <el-tag type="success" size="small">次</el-tag>
            </template>
          </el-statistic>
        </el-col>
        <el-col :span="6">
          <el-statistic title="失败" :value="failCount">
            <template #suffix>
              <el-tag type="danger" size="small">次</el-tag>
            </template>
          </el-statistic>
        </el-col>
        <el-col :span="6">
          <el-statistic title="平均耗时" :value="avgDuration">
            <template #suffix>ms</template>
          </el-statistic>
        </el-col>
      </el-row>

      <!-- 执行记录列表 -->
      <el-table :data="records" stripe size="small" style="width: 100%" @expand-change="handleExpandChange">
        <el-table-column type="expand">
          <template #default="{ row }">
            <div style="padding: 8px 16px">
              <!-- 输入/输出 -->
              <el-descriptions :column="1" border size="small" style="margin-bottom: 12px">
                <el-descriptions-item label="输入内容">
                  <pre class="content-pre">{{ row.input_content }}</pre>
                </el-descriptions-item>
                <el-descriptions-item label="输出内容" v-if="row.output_content">
                  <pre class="content-pre">{{ row.output_content }}</pre>
                </el-descriptions-item>
                <el-descriptions-item label="错误信息" v-if="row.error_message">
                  <el-alert type="error" :title="row.error_message" :closable="false" show-icon />
                </el-descriptions-item>
              </el-descriptions>

              <!-- 节点审计日志 -->
              <div v-if="parseAuditLog(row.node_audit_log).length > 0">
                <div style="font-weight: 600; margin-bottom: 8px; font-size: 14px">
                  <el-icon><List /></el-icon>
                  节点审计摘要
                </div>
                <el-timeline>
                  <el-timeline-item
                    v-for="(node, idx) in parseAuditLog(row.node_audit_log)"
                    :key="idx"
                    :type="node.success ? 'success' : 'danger'"
                    :icon="node.success ? Check : Close"
                    :timestamp="`${node.duration_ms}ms`"
                  >
                    <div style="display: flex; align-items: center; gap: 8px; flex-wrap: wrap">
                      <el-tag size="small" type="info">{{ node.node_id }}</el-tag>
                      <el-tag size="small" type="primary" v-if="node.implementation">{{ node.implementation }}</el-tag>
                      <el-tag size="small" v-if="node.kind">{{ node.kind }}</el-tag>
                      <el-tag size="small" :type="node.success ? 'success' : 'danger'">
                        {{ node.success ? '成功' : '失败' }}
                      </el-tag>
                      <el-tag size="small" type="warning" v-if="node.error_code">{{ node.error_code }}</el-tag>
                    </div>
                  </el-timeline-item>
                </el-timeline>
              </div>

              <el-empty v-else description="暂无节点审计日志" :image-size="60" style="padding: 12px 0" />
            </div>
          </template>
        </el-table-column>

        <el-table-column label="ID" width="70" align="center">
          <template #default="{ row }">
            <el-tag type="info" size="small">{{ row.id }}</el-tag>
          </template>
        </el-table-column>

        <el-table-column label="状态" width="90" align="center">
          <template #default="{ row }">
            <el-tag :type="row.status === 'success' ? 'success' : 'danger'" size="small">
              {{ row.status === 'success' ? '成功' : '失败' }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column label="耗时" width="100" align="center">
          <template #default="{ row }">
            <span>{{ row.duration_ms }}ms</span>
          </template>
        </el-table-column>

        <el-table-column label="Tokens" width="90" align="center">
          <template #default="{ row }">
            <span>{{ row.total_tokens || '-' }}</span>
          </template>
        </el-table-column>

        <el-table-column label="输入摘要" min-width="200" show-overflow-tooltip>
          <template #default="{ row }">
            <span class="ellipsis">{{ row.input_content }}</span>
          </template>
        </el-table-column>

        <el-table-column label="执行时间" width="170" align="center">
          <template #default="{ row }">
            <span>{{ formatTime(row.created_at) }}</span>
          </template>
        </el-table-column>

        <el-table-column label="操作" width="100" align="center" fixed="right">
          <template #default="{ row }">
            <el-button size="small" type="primary" link @click="openDetail(row)">
              <el-icon><View /></el-icon>
              详情
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <el-empty v-if="!loading && records.length === 0" description="暂无执行记录" :image-size="100" style="margin-top: 24px" />
    </div>

    <!-- 详情对话框（全屏展示单条记录） -->
    <el-dialog
      v-model="detailVisible"
      title="执行详情"
      width="700px"
      append-to-body
      :close-on-click-modal="false"
    >
      <div v-if="currentRecord">
        <el-descriptions :column="2" border size="small">
          <el-descriptions-item label="执行ID">{{ currentRecord.id }}</el-descriptions-item>
          <el-descriptions-item label="流水线ID">{{ currentRecord.pipeline_id }}</el-descriptions-item>
          <el-descriptions-item label="状态">
            <el-tag :type="currentRecord.status === 'success' ? 'success' : 'danger'" size="small">
              {{ currentRecord.status === 'success' ? '成功' : '失败' }}
            </el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="耗时">{{ currentRecord.duration_ms }}ms</el-descriptions-item>
          <el-descriptions-item label="Total Tokens">{{ currentRecord.total_tokens || '-' }}</el-descriptions-item>
          <el-descriptions-item label="执行时间">{{ formatTime(currentRecord.created_at) }}</el-descriptions-item>
        </el-descriptions>

        <el-divider>输入内容</el-divider>
        <pre class="content-pre full">{{ currentRecord.input_content }}</pre>

        <template v-if="currentRecord.output_content">
          <el-divider>输出内容</el-divider>
          <pre class="content-pre full">{{ currentRecord.output_content }}</pre>
        </template>

        <template v-if="currentRecord.error_message">
          <el-divider>错误信息</el-divider>
          <el-alert type="error" :title="currentRecord.error_message" :closable="false" show-icon />
        </template>

        <template v-if="parseAuditLog(currentRecord.node_audit_log).length > 0">
          <el-divider>节点审计摘要</el-divider>
          <el-timeline>
            <el-timeline-item
              v-for="(node, idx) in parseAuditLog(currentRecord.node_audit_log)"
              :key="idx"
              :type="node.success ? 'success' : 'danger'"
              :icon="node.success ? Check : Close"
              :timestamp="`${node.duration_ms}ms`"
            >
              <div style="display: flex; align-items: center; gap: 8px; flex-wrap: wrap; margin-bottom: 4px">
                <el-tag size="small" type="info">{{ node.node_id }}</el-tag>
                <el-tag size="small" type="primary" v-if="node.implementation">{{ node.implementation }}</el-tag>
                <el-tag size="small" v-if="node.kind">{{ node.kind }}</el-tag>
                <el-tag size="small" :type="node.success ? 'success' : 'danger'">
                  {{ node.success ? '成功' : '失败' }}
                </el-tag>
              </div>
              <el-tag size="small" type="warning" v-if="node.error_code">错误码: {{ node.error_code }}</el-tag>
            </el-timeline-item>
          </el-timeline>
        </template>
      </div>
    </el-dialog>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { View, List, Check, Close } from '@element-plus/icons-vue'
import { getExecutionHistory, type ExecutionRecord, type NodeAuditSummary } from '@/api/pipeline'

const props = defineProps<{
  modelValue: boolean
  pipelineId: string
  pipelineName: string
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
}>()

const visible = computed({
  get: () => props.modelValue,
  set: (val) => emit('update:modelValue', val)
})

const loading = ref(false)
const records = ref<ExecutionRecord[]>([])
const detailVisible = ref(false)
const currentRecord = ref<ExecutionRecord | null>(null)

const successCount = computed(() => records.value.filter(r => r.status === 'success').length)
const failCount = computed(() => records.value.filter(r => r.status !== 'success').length)
const avgDuration = computed(() => {
  if (records.value.length === 0) return 0
  const total = records.value.reduce((sum, r) => sum + (r.duration_ms || 0), 0)
  return Math.round(total / records.value.length)
})

function parseAuditLog(log?: string): NodeAuditSummary[] {
  if (!log) return []
  try {
    const parsed = JSON.parse(log)
    return Array.isArray(parsed) ? parsed : []
  } catch {
    return []
  }
}

function formatTime(iso: string): string {
  if (!iso) return '-'
  const d = new Date(iso)
  return d.toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  })
}

async function loadHistory() {
  if (!props.pipelineId) return
  loading.value = true
  try {
    const res = await getExecutionHistory(props.pipelineId, 50)
    // API 拦截器已经提取了 data 字段，res 直接就是数组
    const data = Array.isArray(res) ? res : (res?.data ?? [])
    records.value = Array.isArray(data) ? data : []
  } catch (error: any) {
    console.error('Failed to load execution history:', error)
    ElMessage.error('加载执行历史失败: ' + (error.message || '未知错误'))
    records.value = []
  } finally {
    loading.value = false
  }
}

function openDetail(row: ExecutionRecord) {
  currentRecord.value = row
  detailVisible.value = true
}

function handleExpandChange(_row: ExecutionRecord, _expandedRows: ExecutionRecord[]) {
  // 展开行已自动处理
}

watch(() => props.pipelineId, (newId) => {
  if (newId && visible.value) {
    loadHistory()
  }
})
</script>

<style scoped>
.content-pre {
  background: #f5f7fa;
  padding: 10px 12px;
  border-radius: 6px;
  font-size: 13px;
  line-height: 1.5;
  max-height: 160px;
  overflow-y: auto;
  white-space: pre-wrap;
  word-break: break-word;
  margin: 0;
}
.content-pre.full {
  max-height: 320px;
}
.ellipsis {
  display: inline-block;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
