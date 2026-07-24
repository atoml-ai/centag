<template>
  <div class="pipeline-list">
    <div class="page-header">
      <h2>{{ t('pipelineList.title') }}</h2>
      <div class="toolbar">
        <el-input
          v-model="searchText"
          :placeholder="t('pipelineList.searchPlaceholder')"
          clearable
          style="width: 220px"
        >
          <template #prefix>
            <el-icon><Search /></el-icon>
          </template>
        </el-input>
        <span class="search-count" v-if="searchText">
          {{ t('pipelineList.searchCount', { n: filteredPipelines.length }) }}
        </span>
        <el-tooltip v-if="selectedPipelines.length > 0" :content="batchDeleteTooltip" placement="top" :disabled="canBatchDeleteSelected">
          <span>
            <el-button type="danger" :disabled="!canBatchDeleteSelected" @click="handleBatchDelete">
              <el-icon><Delete /></el-icon>
              {{ t('pipelineList.batchDelete', { n: selectedPipelines.length }) }}
            </el-button>
          </span>
        </el-tooltip>
        <el-button :loading="loading" @click="loadPipelines">
          <el-icon><Refresh /></el-icon>
          {{ t('pipelineList.refresh') }}
        </el-button>
        <el-button v-if="canAddOwnPipelines" type="primary" @click="handleCreate">
          <el-icon><Plus /></el-icon>
          {{ t('pipelineList.createPipeline') }}
        </el-button>
      </div>
    </div>

    <el-card>
      <el-table
        v-loading="loading"
        :data="filteredPipelines"
        stripe
        style="width: 100%"
        @selection-change="handleSelectionChange"
      >
        <el-table-column type="selection" width="50" />
        <el-table-column prop="id" :label="t('pipelineList.id')" width="200" />
        <el-table-column prop="name" :label="t('pipelineList.name')" />
        <el-table-column prop="version" :label="t('pipelineList.version')" width="100" />
        <el-table-column prop="description" :label="t('pipelineList.description')" show-overflow-tooltip />
        <el-table-column :label="t('pipelineList.nodeCount')" width="100">
          <template #default="{ row }">
            {{ row.nodes?.length || 0 }}
          </template>
        </el-table-column>
        <el-table-column :label="t('pipelineList.actions')" width="250" fixed="right">
          <template #default="{ row }">
            <div class="action-btns">
              <PipelineFeatureGuard
                feature="pipelineEdit"
                :pipeline="row"
                :is-admin="authStore.isAdmin"
                :action-label="t('pipelineList.edit')"
              >
                <template #default="{ disabled }">
                  <el-button size="default" :disabled="disabled" @click="handleEdit(row)">
                    <el-icon><Edit /></el-icon>
                    {{ t('pipelineList.edit') }}
                  </el-button>
                </template>
              </PipelineFeatureGuard>

              <PipelineFeatureGuard
                feature="pipelineExport"
                :pipeline="row"
                :is-admin="authStore.isAdmin"
                :action-label="t('pipelineList.export')"
              >
                <template #default="{ disabled }">
                  <el-button size="default" :disabled="disabled" @click="handleExport(row)">
                    <el-icon><Download /></el-icon>
                    {{ t('pipelineList.export') }}
                  </el-button>
                </template>
              </PipelineFeatureGuard>

              <PipelineFeatureGuard
                feature="pipelineDelete"
                :pipeline="row"
                :is-admin="authStore.isAdmin"
                :action-label="t('pipelineList.delete')"
              >
                <template #default="{ disabled }">
                  <el-button size="default" type="danger" :disabled="disabled" @click="handleDelete(row)">
                    <el-icon><Delete /></el-icon>
                    {{ t('pipelineList.delete') }}
                  </el-button>
                </template>
              </PipelineFeatureGuard>
            </div>
          </template>
        </el-table-column>
      </el-table>

      <el-empty v-if="!loading && filteredPipelines.length === 0" :description="t('pipelineList.noPipelines')" />
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Search, Plus, Edit, Delete, Download } from '@element-plus/icons-vue'
import { getPipelines, deletePipeline, exportPipeline, parsePipelinesResponse, type Pipeline } from '@/api/pipeline'
import { resolvePipelineFeatureSupport, type PipelineFeatureKey } from '@/utils/pipeline/features'
import { useAuthStore } from '@/stores/auth'
import { useUserResourceAccess } from '@/composables/useUserResourceAccess'
import PipelineFeatureGuard from '@/components/pipeline/PipelineFeatureGuard.vue'

const { t } = useI18n()
const router = useRouter()
const authStore = useAuthStore()
const { canAddOwnPipelines } = useUserResourceAccess()
const loading = ref(false)
const pipelines = ref<Pipeline[]>([])
const searchText = ref('')
const selectedPipelines = ref<Pipeline[]>([])

const filteredPipelines = computed(() => {
  if (!searchText.value.trim()) return pipelines.value
  const q = searchText.value.trim().toLowerCase()
  return pipelines.value.filter(p =>
    p.id?.toLowerCase().includes(q) ||
    p.name?.toLowerCase().includes(q)
  )
})

const handleSelectionChange = (rows: Pipeline[]) => {
  selectedPipelines.value = rows
}

const getPipelineFeatureSupport = (feature: PipelineFeatureKey, row: Pipeline) => {
  return resolvePipelineFeatureSupport(feature, row, { isAdmin: authStore.isAdmin })
}

const canBatchDeleteSelected = computed(() => {
  if (selectedPipelines.value.length === 0) return false
  return selectedPipelines.value.every((row) => getPipelineFeatureSupport('pipelineBatchDelete', row).enabled)
})

const batchDeleteTooltip = computed(() => {
  if (canBatchDeleteSelected.value) return t('pipelineList.batchDeleteTooltip')
  const unsupported = selectedPipelines.value.find((row) => !getPipelineFeatureSupport('pipelineBatchDelete', row).enabled)
  if (!unsupported) return t('pipelineList.batchDeleteTooltip')
  return getPipelineFeatureSupport('pipelineBatchDelete', unsupported).reason || t('pipelineList.batchDeleteUnsupported')
})

const handleBatchDelete = async () => {
  if (selectedPipelines.value.length === 0) return
  try {
    await ElMessageBox.confirm(
      t('pipelineList.batchDeleteConfirm', { n: selectedPipelines.value.length }),
      t('pipelineList.batchDeleteTitle'),
      { type: 'warning' }
    )
    for (const p of selectedPipelines.value) {
      await deletePipeline(p.id)
    }
    ElMessage.success(t('pipelineList.batchDeleteSuccess', { n: selectedPipelines.value.length }))
    selectedPipelines.value = []
    loadPipelines()
  } catch (error: any) {
    if (error !== 'cancel') {
      ElMessage.error(t('pipelineList.batchDeleteFailed') + '：' + error.message)
    }
  }
}

// 加载流水线列表
const loadPipelines = async () => {
  loading.value = true
  try {
    const res = await getPipelines()
    // 响应拦截器已处理：如果响应有 {success, data} 结构，返回 data；否则返回原始响应
    // 所以这里 res 可能是数组（流水线列表）或对象（需要进一步解析）
    pipelines.value = parsePipelinesResponse(res)
  } catch (error) {
    ElMessage.error(t('pipelineList.loadPipelinesFailed'))
    console.error(error)
  } finally {
    loading.value = false
  }
}

// 创建流水线
const handleCreate = () => {
  router.push('/pipelines/create')
}

// 编辑流水线
const handleEdit = (row: Pipeline) => {
  // 跳转到 PipelineModes 页面，通过路由参数打开编辑对话框
  router.push(`/pipelines/${row.id}`)
}

// 删除流水线
const handleDelete = async (row: Pipeline) => {
  try {
    await ElMessageBox.confirm(t('pipelineList.deleteConfirm'), t('pipelineList.deleteConfirmTitle'), {
      confirmButtonText: t('pipelineList.confirmButtonText'),
      cancelButtonText: t('pipelineList.cancelButtonText'),
      type: 'warning'
    })
    
    await deletePipeline(row.id)
    ElMessage.success(t('pipelineList.deleteSuccess'))
    loadPipelines()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error(t('pipelineList.deleteFailed'))
      console.error(error)
    }
  }
}

// 导出流水线
const handleExport = (row: Pipeline) => {
  const filename = `${row.name || 'pipeline'}-${row.id}.yaml`
  exportPipeline(row.id).then((response: any) => {
    const content = typeof response === 'string' ? response : response?.data || ''
    const blob = new Blob([content], { type: 'text/yaml' })
    const url = window.URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = filename
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    window.URL.revokeObjectURL(url)
    ElMessage.success(t('pipelineList.exportSuccess'))
  }).catch((error: any) => {
    ElMessage.error(t('pipelineList.exportFailed') + '：' + error.message)
  })
}

onMounted(() => {
  loadPipelines()
})
</script>

<style scoped>
.pipeline-list {
  padding: 20px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.page-header h2 {
  margin: 0;
  font-size: 22px;
  font-weight: 600;
  color: #1f2937;
}

.toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.search-count {
  font-size: 13px;
  color: #64748b;
  white-space: nowrap;
}

.action-btns {
  display: flex;
  align-items: center;
  gap: 8px;
}
</style>
