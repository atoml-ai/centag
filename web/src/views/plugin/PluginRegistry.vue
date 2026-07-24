<template>
  <div class="plugin-registry">
    <el-card>
      <template #header>
        <div class="card-header">
          <span>{{ t('pluginRegistry.pluginMarket') }}</span>
          <el-button type="primary" @click="handleUpload">
            <el-icon><Upload /></el-icon>
            {{ t('pluginRegistry.uploadPlugin') }}
          </el-button>
        </div>
      </template>

      <!-- 搜索栏 -->
      <el-form :model="searchForm" inline class="search-form">
        <el-form-item>
          <el-input
            v-model="searchForm.search"
            :placeholder="t('pluginRegistry.searchPlaceholder')"
            clearable
            @keyup.enter="handleSearch"
          >
            <template #prefix>
              <el-icon><Search /></el-icon>
            </template>
          </el-input>
        </el-form-item>
        <el-form-item>
          <el-select v-model="searchForm.category" :placeholder="t('pluginRegistry.category')" clearable>
            <el-option :label="t('pluginRegistry.generator')" value="generator" />
            <el-option :label="t('pluginRegistry.processor')" value="processor" />
            <el-option :label="t('pluginRegistry.reviewer')" value="reviewer" />
            <el-option :label="t('pluginRegistry.router')" value="router" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-select v-model="searchForm.sort_by" :placeholder="t('pluginRegistry.sortBy')">
            <el-option :label="t('pluginRegistry.downloadCount')" value="download_count" />
            <el-option :label="t('pluginRegistry.rating')" value="rating" />
            <el-option :label="t('pluginRegistry.name')" value="name" />
            <el-option :label="t('pluginRegistry.createdAt')" value="created_at" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="handleSearch">
            <el-icon><Search /></el-icon>
            {{ t('pluginRegistry.search') }}
          </el-button>
          <el-button @click="handleReset">{{ t('pluginRegistry.reset') }}</el-button>
        </el-form-item>
      </el-form>

      <!-- 插件列表 -->
      <el-row :gutter="20" v-loading="loading">
        <el-col
          v-for="plugin in plugins"
          :key="plugin.id"
          :xs="24"
          :sm="12"
          :md="8"
          :lg="6"
          class="plugin-col"
        >
          <el-card shadow="hover" class="plugin-card" @click="handleView(plugin)">
            <div class="plugin-header">
              <h3 class="plugin-name">{{ plugin.name }}</h3>
              <el-tag size="small" type="info">{{ plugin.version }}</el-tag>
            </div>
            <p class="plugin-description">{{ plugin.description || t('pluginRegistry.noDescription') }}</p>
            <div class="plugin-meta">
              <span class="meta-item">
                <el-icon><User /></el-icon>
                {{ plugin.author || t('pluginRegistry.unknownAuthor') }}
              </span>
              <span class="meta-item">
                <el-icon><Download /></el-icon>
                {{ plugin.download_count || 0 }}
              </span>
              <span class="meta-item">
                <el-icon><Star /></el-icon>
                {{ plugin.rating?.toFixed(1) || '0.0' }}
              </span>
            </div>
            <div class="plugin-tags" v-if="plugin.tags?.length">
              <el-tag
                v-for="tag in plugin.tags.slice(0, 3)"
                :key="tag"
                size="small"
                effect="plain"
              >
                {{ tag }}
              </el-tag>
            </div>
          </el-card>
        </el-col>
      </el-row>

      <el-empty v-if="!loading && plugins.length === 0" :description="t('pluginRegistry.noPlugins')" />

      <!-- 分页 -->
      <el-pagination
        v-if="total > 0"
        v-model:current-page="searchForm.page"
        v-model:page-size="searchForm.page_size"
        :total="total"
        :page-sizes="[12, 24, 36, 48]"
        layout="total, sizes, prev, pager, next"
        @size-change="handleSizeChange"
        @current-change="handlePageChange"
        class="pagination"
      />
    </el-card>

    <!-- 上传对话框 -->
    <el-dialog v-model="uploadVisible" :title="t('pluginRegistry.uploadDialogTitle')" width="500px">
      <el-upload
        drag
        action="/api/v1/registry/plugins/upload"
        :on-success="handleUploadSuccess"
        :on-error="handleUploadError"
        accept=".zip"
      >
        <el-icon class="el-icon--upload"><upload-filled /></el-icon>
        <div class="el-upload__text" v-html="t('pluginRegistry.dragOrClick')"></div>
        <template #tip>
          <div class="el-upload__tip">
            {{ t('pluginRegistry.uploadTip') }}
          </div>
        </template>
      </el-upload>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Plus, Upload, Search, User, Download, Star, UploadFilled } from '@element-plus/icons-vue'
import { listPlugins, type PluginMetadata } from '@/api/plugin'

const { t } = useI18n()
const router = useRouter()

const loading = ref(false)
const plugins = ref<PluginMetadata[]>([])
const total = ref(0)

const searchForm = ref({
  search: '',
  category: '',
  sort_by: 'download_count',
  sort_order: 'desc',
  page: 1,
  page_size: 12
})

const uploadVisible = ref(false)

// 加载插件列表
const loadPlugins = async () => {
  loading.value = true
  try {
    const res = await listPlugins(searchForm.value)
    plugins.value = res.data.data?.plugins || []
    total.value = res.data.data?.total || 0
  } catch (error) {
    ElMessage.error(t('pluginRegistry.loadPluginsFailed'))
    console.error(error)
  } finally {
    loading.value = false
  }
}

// 搜索
const handleSearch = () => {
  searchForm.value.page = 1
  loadPlugins()
}

// 重置
const handleReset = () => {
  searchForm.value = {
    search: '',
    category: '',
    sort_by: 'download_count',
    sort_order: 'desc',
    page: 1,
    page_size: 12
  }
  loadPlugins()
}

// 分页
const handleSizeChange = (size: number) => {
  searchForm.value.page_size = size
  loadPlugins()
}

const handlePageChange = (page: number) => {
  searchForm.value.page = page
  loadPlugins()
}

// 查看详情
const handleView = (plugin: PluginMetadata) => {
  router.push(`/plugins/${plugin.id}`)
}

// 上传
const handleUpload = () => {
  uploadVisible.value = true
}

const handleUploadSuccess = () => {
  ElMessage.success(t('pluginRegistry.uploadSuccess'))
  uploadVisible.value = false
  loadPlugins()
}

const handleUploadError = () => {
  ElMessage.error(t('pluginRegistry.uploadFailed'))
}

onMounted(() => {
  loadPlugins()
})
</script>

<style scoped>
.plugin-registry {
  padding: 20px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.search-form {
  margin-bottom: 20px;
}

.plugin-col {
  margin-bottom: 20px;
}

.plugin-card {
  cursor: pointer;
  transition: all 0.3s;
}

.plugin-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
}

.plugin-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 10px;
}

.plugin-name {
  margin: 0;
  font-size: 16px;
  font-weight: 500;
}

.plugin-description {
  color: #666;
  font-size: 14px;
  line-height: 1.5;
  margin: 10px 0;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.plugin-meta {
  display: flex;
  gap: 15px;
  margin: 10px 0;
  font-size: 13px;
  color: #999;
}

.meta-item {
  display: flex;
  align-items: center;
  gap: 4px;
}

.plugin-tags {
  margin-top: 10px;
  display: flex;
  gap: 5px;
}

.pagination {
  margin-top: 20px;
  display: flex;
  justify-content: flex-end;
}
</style>
