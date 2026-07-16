<template>
  <div class="storage">
    <div class="header-with-toolbar">
      <div class="header-left">
        <h1 class="page-title">存储管理</h1>
        <p class="page-description">配置和管理存储后端，用于缓存数据和持久化存储</p>
      </div>
      <div class="toolbar-actions">
        <el-input
          v-model="searchText"
          placeholder="搜索存储..."
          clearable
          style="width: 200px"
        >
          <template #prefix>
            <el-icon><Search /></el-icon>
          </template>
        </el-input>
        <el-select v-model="filterType" placeholder="类型筛选" clearable style="width: 120px">
          <el-option label="全部" value="" />
          <el-option label="Redis" value="redis" />
          <el-option label="Memcached" value="memcached" />
          <el-option label="本地文件" value="file" />
          <el-option label="Elasticsearch" value="elasticsearch" />
          <el-option label="ChromaDB" value="chroma" />
          <el-option label="PostgreSQL" value="postgresql" />
        </el-select>
        <el-button :loading="loading" @click="load">
          <el-icon><Refresh /></el-icon>
          刷新
        </el-button>
        <el-button :loading="loading" @click="load">
          <el-icon><CircleCheck /></el-icon>
          检查健康状态
        </el-button>
        <el-button type="primary" @click="openCreate">
          <el-icon><Plus /></el-icon>
          添加存储
        </el-button>
        <el-button @click="goToKVBrowser">
          <el-icon><View /></el-icon>
          KV 数据浏览
        </el-button>
      </div>
    </div>

    <div class="content-wrapper">

      <!-- 存储列表 -->
      <el-card class="table-card" v-loading="loading">
        <el-table :data="filteredStorages" stripe size="large">
          <el-table-column prop="name" label="名称" min-width="150">
            <template #default="{ row }">
              <div class="name-cell">
                <span class="name-title">{{ row.name }}</span>
                <span class="name-subtitle" v-if="row.description">{{ row.description }}</span>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="类型" width="200" align="center">
            <template #default="{ row }">
              <el-tag :type="getStorageTypeColor(row.type)" size="small" effect="plain">
                {{ getStorageTypeLabel(row.type) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="config" label="地址" min-width="200" show-overflow-tooltip>
            <template #default="{ row }">
              {{ getStorageAddress(row) }}
            </template>
          </el-table-column>
          <el-table-column label="启用状态" width="90" align="center">
            <template #default="{ row }">
              <el-switch
                :model-value="row.enabled"
                @change="toggleStatus(row)"
                active-color="#10b981"
              />
            </template>
          </el-table-column>
          <el-table-column label="默认" width="150" align="center">
            <template #default="{ row }">
              <el-tag v-if="row.is_default" type="success" size="small" effect="plain">
                是
              </el-tag>
              <span v-else class="text-muted">-</span>
            </template>
          </el-table-column>
          <el-table-column label="健康状态" width="200" align="center">
            <template #default="{ row }">
              <span v-if="row.healthy === undefined" class="health-badge health-badge--info">
                未检查
              </span>
              <span v-else-if="row.healthy" class="health-badge health-badge--success">
                <el-icon><SuccessFilled /></el-icon>正常
              </span>
              <el-tooltip v-else effect="dark" placement="top">
                <template #content>
                  <div class="error-tooltip">{{ row.error || '未知错误' }}</div>
                </template>
                <span class="health-badge health-badge--danger" style="cursor: help;">
                  <el-icon><CircleCloseFilled /></el-icon>异常
                </span>
              </el-tooltip>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="80" align="center" fixed="right">
            <template #default="{ row }">
              <el-dropdown trigger="click" @command="(command) => handleCommand(command, row)">
                <el-button type="primary" link>
                  <el-icon><MoreFilled /></el-icon>
                </el-button>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item :command="'toggle'">
                      <el-icon><Switch /></el-icon>
                      {{ row.enabled ? '禁用' : '启用' }}
                    </el-dropdown-item>
                    <el-dropdown-item v-if="!row.is_default" :command="'setDefault'">
                      <el-icon><Check /></el-icon>
                      设为默认
                    </el-dropdown-item>
                    <el-dropdown-item :command="'test'">
                      <el-icon><Link /></el-icon>
                      测试
                    </el-dropdown-item>
                    <el-dropdown-item :command="'edit'">
                      <el-icon><Edit /></el-icon>
                      编辑
                    </el-dropdown-item>
                    <el-dropdown-item :command="'delete'" divided>
                      <el-icon><Delete /></el-icon>
                      删除
                    </el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
            </template>
          </el-table-column>
        </el-table>

        <el-empty
          v-if="!loading && filteredStorages.length === 0"
          description="暂无存储配置"
          :image-size="120"
        />
      </el-card>

      <!-- 编辑/创建对话框 -->
      <el-dialog
        v-model="editing"
        :title="isCreate ? '添加存储' : '编辑存储'"
        width="600px"
        @close="resetForm"
      >
        <el-form label-width="100px" :model="form" :rules="rules" ref="formRef">
          <el-form-item label="名称" prop="name">
            <el-input v-model="form.name" placeholder="请输入存储名称" />
          </el-form-item>
          <el-form-item label="类型" prop="type">
            <el-select v-model="form.type" style="width: 100%" placeholder="请选择类型" @change="handleTypeChange">
              <el-option label="Redis" value="redis" />
              <el-option label="Memcached" value="memcached" />
              <el-option label="本地文件" value="file" />
              <el-option label="Elasticsearch" value="elasticsearch" />
              <el-option label="ChromaDB" value="chroma" />
              <el-option label="PostgreSQL" value="postgresql" />
            </el-select>
          </el-form-item>

          <!-- Redis/Memcached 配置 -->
          <template v-if="form.type === 'redis' || form.type === 'memcached'">
            <el-form-item label="主机" prop="host">
              <el-input v-model="form.host" placeholder="localhost" />
            </el-form-item>
            <el-form-item label="端口" prop="port">
              <el-input-number v-model="form.port" :min="1" :max="65535" style="width: 100%" />
            </el-form-item>
            <el-form-item label="密码" prop="password">
              <el-input
                v-model="form.password"
                type="password"
                show-password
                placeholder="可选"
              />
            </el-form-item>
            <el-form-item label="数据库" prop="database" v-if="form.type === 'redis'">
              <el-input-number v-model="form.database" :min="0" :max="15" style="width: 100%" />
            </el-form-item>
            <el-form-item label="超时(秒)" prop="timeout">
              <el-input-number v-model="form.timeout" :min="1" :max="300" style="width: 100%" />
            </el-form-item>
          </template>

          <!-- 本地文件配置 -->
          <template v-if="form.type === 'file'">
            <el-form-item label="路径" prop="path">
              <el-input v-model="form.path" placeholder="/path/to/storage" />
            </el-form-item>
            <el-form-item label="最大大小">
              <el-input-number v-model="form.max_size" :min="0" style="width: 200px" />
              <span class="unit">MB</span>
            </el-form-item>
          </template>

          <!-- Elasticsearch 配置 -->
          <template v-if="form.type === 'elasticsearch'">
            <el-form-item label="地址列表" prop="addresses">
              <el-input
                v-model="form.addressesText"
                type="textarea"
                :rows="2"
                placeholder="http://localhost:9200&#10;http://localhost:9201"
              />
              <div class="form-tip">每行一个地址，支持多个节点</div>
            </el-form-item>
            <el-form-item label="用户名" prop="username">
              <el-input v-model="form.username" placeholder="elastic" />
            </el-form-item>
            <el-form-item label="密码" prop="password">
              <el-input
                v-model="form.password"
                type="password"
                show-password
                placeholder="请输入密码"
              />
            </el-form-item>
            <el-form-item label="API Key" prop="api_key">
              <el-input
                v-model="form.api_key"
                type="password"
                show-password
                placeholder="可选，使用API Key认证"
              />
            </el-form-item>
            <el-form-item label="精确索引" prop="exact_index">
              <el-input v-model="form.exact_index" placeholder="cache_exact_index" />
            </el-form-item>
            <el-form-item label="语义索引" prop="semantic_index">
              <el-input v-model="form.semantic_index" placeholder="cache_semantic_index" />
            </el-form-item>
            <el-form-item label="向量维度" prop="vector_dimension">
              <el-input-number v-model="form.vector_dimension" :min="128" :max="2048" style="width: 100%" placeholder="1024" />
              <div class="form-tip">bge-m3模型使用1024维</div>
            </el-form-item>
            <el-form-item label="启用TLS">
              <el-switch v-model="form.enable_tls" />
            </el-form-item>
            <el-form-item label="请求超时">
              <el-input-number v-model="form.request_timeout" :min="5" :max="300" style="width: 200px" />
              <span class="unit">秒</span>
            </el-form-item>
          </template>

          <!-- ChromaDB 配置 -->
          <template v-if="form.type === 'chroma'">
            <el-form-item label="地址" prop="addr">
              <el-input v-model="form.addr" placeholder="localhost:8000" />
            </el-form-item>
            <el-form-item label="集合名称" prop="collection">
              <el-input v-model="form.collection" placeholder="llm_cache" />
            </el-form-item>
            <el-form-item label="超时(秒)" prop="timeout">
              <el-input-number v-model="form.timeout" :min="5" :max="300" style="width: 100%" />
            </el-form-item>
          </template>

          <!-- PostgreSQL 配置 -->
          <template v-if="form.type === 'postgresql'">
            <el-form-item label="主机" prop="host">
              <el-input v-model="form.host" placeholder="localhost" />
            </el-form-item>
            <el-form-item label="端口" prop="port">
              <el-input-number v-model="form.port" :min="1" :max="65535" style="width: 100%" />
            </el-form-item>
            <el-form-item label="用户名" prop="user">
              <el-input v-model="form.user" placeholder="postgres" />
            </el-form-item>
            <el-form-item label="密码" prop="password">
              <el-input
                v-model="form.password"
                type="password"
                show-password
                placeholder="请输入密码"
              />
            </el-form-item>
            <el-form-item label="数据库" prop="database">
              <el-input v-model="form.database" placeholder="centag" />
            </el-form-item>
            <el-form-item label="SSL模式" prop="ssl_mode">
              <el-select v-model="form.ssl_mode" style="width: 100%">
                <el-option label="禁用" value="disable" />
                <el-option label="允许" value="allow" />
                <el-option label="首选" value="prefer" />
                <el-option label="必须" value="require" />
                <el-option label="验证CA" value="verify-ca" />
                <el-option label="验证全链路" value="verify-full" />
              </el-select>
            </el-form-item>
            <el-form-item label="最大连接数" prop="max_conns">
              <el-input-number v-model="form.max_conns" :min="1" :max="100" style="width: 100%" />
            </el-form-item>
            <el-form-item label="最小连接数" prop="min_conns">
              <el-input-number v-model="form.min_conns" :min="1" :max="100" style="width: 100%" />
            </el-form-item>
            <el-form-item label="最大连接生存时间(秒)" prop="max_conn_lifetime">
              <el-input-number v-model="form.max_conn_lifetime" :min="60" :max="86400" style="width: 100%" />
              <div class="form-tip">0表示无限制</div>
            </el-form-item>
            <el-form-item label="最大空闲时间(秒)" prop="max_conn_idle_time">
              <el-input-number v-model="form.max_conn_idle_time" :min="60" :max="3600" style="width: 100%" />
              <div class="form-tip">0表示无限制</div>
            </el-form-item>
            <el-form-item label="KV表名" prop="kv_table">
              <el-input v-model="form.kv_table" placeholder="kv_cache" />
            </el-form-item>
            <el-form-item label="向量表名" prop="vector_table">
              <el-input v-model="form.vector_table" placeholder="vector_cache" />
            </el-form-item>
            <el-form-item label="问答表名" prop="qa_table">
              <el-input v-model="form.qa_table" placeholder="qa_pairs" />
            </el-form-item>
            <el-form-item label="向量维度" prop="vector_dimension">
              <el-input-number v-model="form.vector_dimension" :min="128" :max="2048" style="width: 100%" placeholder="1024" />
              <div class="form-tip">bge-m3模型使用1024维</div>
            </el-form-item>
            <el-form-item label="索引类型" prop="index_type">
              <el-select v-model="form.index_type" style="width: 100%">
                <el-option label="HNSW" value="hnsw" />
                <el-option label="IVFFlat" value="ivfflat" />
                <el-option label="GIN" value="gin" />
              </el-select>
            </el-form-item>
          </template>

          <el-form-item label="描述">
            <el-input
              v-model="form.description"
              type="textarea"
              :rows="2"
              placeholder="可选，描述此存储的用途"
            />
          </el-form-item>

          <el-form-item label="启用状态">
            <el-switch v-model="form.enabled" />
          </el-form-item>
          <el-form-item label="设为默认">
            <el-switch v-model="form.is_default" />
          </el-form-item>
        </el-form>
        <template #footer>
          <div style="display: flex; justify-content: space-between; align-items: center;">
            <el-button :loading="testing" @click="handleTestConnection">
              <el-icon><Link /></el-icon>
              测试连接
            </el-button>
            <div>
              <el-button @click="editing = false">取消</el-button>
              <el-button type="primary" :loading="saving" @click="save">
                保存
              </el-button>
            </div>
          </div>
        </template>
      </el-dialog>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Search,
  Refresh,
  Plus,
  Edit,
  Delete,
  Switch,
  Check,
  Link,
  MoreFilled,
  SuccessFilled,
  CircleCloseFilled,
  WarningFilled,
  View
} from '@element-plus/icons-vue'
import {
  getStorages,
  addStorage,
  updateStorage,
  deleteStorage,
  toggleStorage,
  setDefaultStorage,
  testStorage
} from '@/api'

const loading = ref(false)
const saving = ref(false)
const testing = ref(false)
const searchText = ref('')
const filterType = ref('')
const list = ref<any[]>([])

const editing = ref(false)
const isCreate = ref(false)
const form = ref<any>({})
const currentId = ref('')
const formRef = ref()
const router = useRouter()

const rules = {
  name: [{ required: true, message: '请输入存储名称', trigger: 'blur' }],
  type: [{ required: true, message: '请选择类型', trigger: 'change' }],
  host: [{ required: true, message: '请输入主机地址', trigger: 'blur' }],
  port: [{ required: true, message: '请输入端口', trigger: 'blur' }],
  path: [{ required: true, message: '请输入路径', trigger: 'blur' }]
}

const filteredStorages = computed(() => {
  let result = list.value

  if (searchText.value) {
    const text = searchText.value.toLowerCase()
    result = result.filter(
      (item) =>
        item.name?.toLowerCase().includes(text) ||
        getStorageAddress(item)?.toLowerCase().includes(text)
    )
  }

  if (filterType.value) {
    result = result.filter((item) => item.type === filterType.value)
  }

  return result
})

// 获取存储类型的显示标签
function getStorageTypeLabel(type: string) {
  const typeMap: Record<string, string> = {
    redis: 'Redis',
    memcached: 'Memcached',
    file: '本地文件',
    elasticsearch: 'Elasticsearch',
    chroma: 'ChromaDB',
    postgresql: 'PostgreSQL'
  }
  return typeMap[type] || type
}

// 获取存储类型的标签颜色
function getStorageTypeColor(type: string) {
  const colorMap: Record<string, string> = {
    redis: 'danger',
    memcached: 'primary',
    file: 'warning',
    elasticsearch: 'success',
    chroma: 'info',
    postgresql: 'primary'
  }
  return colorMap[type] || 'info'
}

// 获取存储的地址信息
function getStorageAddress(row: any) {
  if (!row.config) return '-'

  const config = row.config

  // Redis
  if (row.type === 'redis') {
    return config.addr || `${config.host || 'localhost'}:${config.port || 6379}`
  }

  // Memcached
  if (row.type === 'memcached') {
    return config.addr || `${config.host || 'localhost'}:${config.port || 11211}`
  }

  // Elasticsearch
  if (row.type === 'elasticsearch') {
    if (config.addresses && Array.isArray(config.addresses) && config.addresses.length > 0) {
      return config.addresses[0]
    }
    return config.url || '-'
  }

  // ChromaDB
  if (row.type === 'chroma') {
    return config.addr || config.url || '-'
  }

  // PostgreSQL（默认端口 5432，与后端一致）
  if (row.type === 'postgresql') {
    return `${config.host || 'localhost'}:${config.port ?? 5432}/${config.database || 'postgres'}`
  }

  // 本地文件
  if (row.type === 'file') {
    return config.path || '-'
  }

  return '-'
}

async function load() {
  loading.value = true
  try {
    const res = await getStorages()
    console.log('Storages response:', res)

    // 处理后端返回的数据结构: {default_kv: "redis", storages: [...]}
    let data = res?.data || res

    // 如果数据有 storages 字段,提取它
    if (data && typeof data === 'object' && 'storages' in data) {
      list.value = Array.isArray(data.storages) ? data.storages : []
    } else {
      // 否则尝试直接当作数组使用
      list.value = Array.isArray(data) ? data : []
    }

    console.log('Loaded storages:', list.value)
  } catch (error: any) {
    console.error('Failed to load storages:', error)
    ElMessage.error('加载失败: ' + (error.message || '未知错误'))
    list.value = []
  } finally {
    loading.value = false
  }
}

function openCreate() {
  isCreate.value = true
  form.value = {
    name: '',
    type: 'redis',
    // Redis/Memcached
    host: 'redis.atoml.net',
    port: 26379,
    password: '',
    database: 0,
    timeout: 30,
    // File
    path: '',
    max_size: 1024,
    // Elasticsearch
    addressesText: 'http://es.atoml.net:29200',
    username: 'elastic',
    api_key: '',
    exact_index: 'cache_exact_index',
    semantic_index: 'cache_semantic_index',
    vector_dimension: 1024,
    enable_tls: false,
    request_timeout: 30,
    // ChromaDB
    addr: 'chromadb.atoml.net:20008',
    collection: 'llm_cache',
    // PostgreSQL
    user: 'postgres',
    ssl_mode: 'disable',
    max_conns: 20,
    min_conns: 5,
    max_conn_lifetime: 3600,
    max_conn_idle_time: 600,
    kv_table: 'kv_cache',
    vector_table: 'vector_cache',
    qa_table: 'qa_pairs',
    index_type: 'hnsw',
    // Common
    description: '',
    enabled: true,
    is_default: false,
    config: {}
  }
  editing.value = true
}

function openEdit(row: any) {
  isCreate.value = false
  currentId.value = row.name

    // 深拷贝并处理配置（仅用 config/row，不用环境变量）
  const config = row.config || {}
  const defaultPort = (t: string) => (t === 'redis' ? 6379 : t === 'postgresql' ? 5432 : 11211)
  form.value = {
    ...row,
    // Redis/Memcached/PostgreSQL - 从 config 或扁平字段提取
    host: config.host ?? row.host ?? 'localhost',
    port: config.port ?? row.port ?? defaultPort(row.type || ''),
    password: config.password || row.password || '',
    database: config.db ?? config.database ?? row.database ?? 0,
    timeout: config.timeout || row.timeout || 30,
    // File
    path: config.path || row.path || '',
    max_size: config.max_size || row.max_size || 1024,
    // Elasticsearch - 从config提取
    addressesText: config.addresses ? config.addresses.join('\n') : 'http://localhost:9200',
    username: config.username || 'elastic',
    api_key: config.api_key || '',
    exact_index: config.exact_index || 'cache_exact_index',
    semantic_index: config.semantic_index || 'cache_semantic_index',
    vector_dimension: config.vector_dimension || 1024,
    enable_tls: config.enable_tls || false,
    request_timeout: config.request_timeout || 30,
    // ChromaDB
    addr: config.addr || row.addr || 'localhost:8000',
    collection: config.collection || row.collection || 'llm_cache',
    // PostgreSQL - 从config提取
    user: config.user || 'postgres',
    ssl_mode: config.ssl_mode || 'disable',
    max_conns: config.max_conns || 20,
    min_conns: config.min_conns || 5,
    max_conn_lifetime: config.max_conn_lifetime || 3600,
    max_conn_idle_time: config.max_conn_idle_time || 600,
    kv_table: config.kv_table || 'kv_cache',
    vector_table: config.vector_table || 'vector_cache',
    qa_table: config.qa_table || 'qa_pairs',
    index_type: config.index_type || 'hnsw',
    // 保留原始config
    config: config
  }
  editing.value = true
}

async function save() {
  try {
    await formRef.value?.validate()
  } catch {
    return
  }

  saving.value = true
  try {
    // 构建保存的payload
    const payload: any = {
      name: form.value.name,
      type: form.value.type,
      enabled: form.value.enabled,
      description: form.value.description || '',
      config: {}
    }

    // 根据类型构建config
    if (form.value.type === 'redis') {
      payload.config = {
        addr: `${form.value.host}:${form.value.port}`,
        password: form.value.password || '',
        db: form.value.database || 0,
        pool_size: 10
      }
    } else if (form.value.type === 'memcached') {
      payload.config = {
        addr: `${form.value.host}:${form.value.port}`,
        timeout: form.value.timeout || 30
      }
    } else if (form.value.type === 'file') {
      payload.config = {
        path: form.value.path,
        max_size: form.value.max_size || 1024
      }
    } else if (form.value.type === 'elasticsearch') {
      // 解析地址列表
      const addresses = form.value.addressesText
        .split('\n')
        .map((addr: string) => addr.trim())
        .filter((addr: string) => addr.length > 0)

      payload.config = {
        addresses: addresses,
        username: form.value.username || 'elastic',
        password: form.value.password || '',
        api_key: form.value.api_key || '',
        exact_index: form.value.exact_index || 'cache_exact_index',
        semantic_index: form.value.semantic_index || 'cache_semantic_index',
        vector_dimension: form.value.vector_dimension || 1024,
        enable_tls: form.value.enable_tls || false,
        request_timeout: form.value.request_timeout || 30
      }
    } else if (form.value.type === 'chroma') {
      payload.config = {
        addr: form.value.addr || 'localhost:8000',
        collection: form.value.collection || 'llm_cache',
        timeout: form.value.timeout || 30
      }
    } else if (form.value.type === 'postgresql') {
      payload.config = {
        host: form.value.host || 'localhost',
        port: form.value.port ?? 5432,
        user: form.value.user || 'postgres',
        password: form.value.password || '',
        database: form.value.database || 'centag',
        ssl_mode: form.value.ssl_mode || 'disable',
        max_conns: form.value.max_conns || 20,
        min_conns: form.value.min_conns || 5,
        max_conn_lifetime: form.value.max_conn_lifetime || 3600,
        max_conn_idle_time: form.value.max_conn_idle_time || 600,
        kv_table: form.value.kv_table || 'kv_cache',
        vector_table: form.value.vector_table || 'vector_cache',
        qa_table: form.value.qa_table || 'qa_pairs',
        vector_dimension: form.value.vector_dimension || 1024,
        index_type: form.value.index_type || 'hnsw'
      }
    }

    if (isCreate.value) {
      await addStorage(payload)
      ElMessage.success('添加成功')
    } else {
      await updateStorage(payload)
      ElMessage.success('更新成功')
    }
    
    // 如果设置为默认存储，单独调用设置默认存储接口
    if (form.value.is_default) {
      try {
        await setDefaultStorage(form.value.name)
        ElMessage.success('已设置为默认存储')
      } catch (error: any) {
        console.error('Failed to set default storage:', error)
        ElMessage.warning('保存成功但设置默认失败: ' + (error.message || '未知错误'))
      }
    }
    
    editing.value = false
    await load()
  } catch (error: any) {
    console.error('Failed to save storage:', error)
    ElMessage.error('保存失败: ' + (error.message || '未知错误'))
  } finally {
    saving.value = false
  }
}

function resetForm() {
  form.value = {}
  currentId.value = ''
  formRef.value?.resetFields()
}

function goToKVBrowser() {
  router.push('/storage/kv')
}

function handleTypeChange(type: string) {
  // 根据类型设置默认值
  if (type === 'redis') {
    form.value.port = 26379
    form.value.host = 'redis.atoml.net'
    form.value.database = 0
  } else if (type === 'memcached') {
    form.value.port = 11211
    form.value.host = 'redis.atoml.net'
  } else if (type === 'elasticsearch') {
    form.value.addressesText = 'http://es.atoml.net:29200'
    form.value.username = 'elastic'
    form.value.exact_index = 'cache_exact_index'
    form.value.semantic_index = 'cache_semantic_index'
    form.value.vector_dimension = 1024
    form.value.enable_tls = false
    form.value.request_timeout = 30
  } else if (type === 'chroma') {
    form.value.addr = 'chromadb.atoml.net:20008'
    form.value.collection = 'llm_cache'
    form.value.timeout = 30
  } else if (type === 'postgresql') {
    form.value.host = '127.0.0.1'
    form.value.port = 5432
    form.value.user = 'postgres'
    form.value.database = 'centag'
    form.value.ssl_mode = 'disable'
    form.value.max_conns = 20
    form.value.min_conns = 5
    form.value.max_conn_lifetime = 3600
    form.value.max_conn_idle_time = 600
    form.value.kv_table = 'kv_cache'
    form.value.vector_table = 'vector_cache'
    form.value.qa_table = 'qa_pairs'
    form.value.vector_dimension = 1024
    form.value.index_type = 'hnsw'
  } else if (type === 'file') {
    form.value.path = '/tmp/storage'
    form.value.max_size = 1024
  }
}

async function toggleStatus(row: any) {
  try {
    await toggleStorage(row.name, !row.enabled)
    ElMessage.success(row.enabled ? '已禁用' : '已启用')
    await load()
  } catch (error: any) {
    console.error('Failed to toggle status:', error)
    ElMessage.error('操作失败: ' + (error.message || '未知错误'))
  }
}

async function handleDelete(row: any) {
  try {
    await ElMessageBox.confirm(`确定要删除存储 "${row.name}" 吗?`, '确认删除', {
      type: 'warning'
    })
    await deleteStorage(row.name)
    ElMessage.success('删除成功')
    await load()
  } catch (error: any) {
    if (error !== 'cancel') {
      console.error('Failed to delete storage:', error)
      ElMessage.error('删除失败: ' + (error.message || '未知错误'))
    }
  }
}

async function handleSetDefault(row: any) {
  try {
    await setDefaultStorage(row.name)
    ElMessage.success(`已将 "${row.name}" 设置为默认存储`)
    await load()
  } catch (error: any) {
    console.error('Failed to set default storage:', error)
    ElMessage.error('设置默认存储失败: ' + (error.message || '未知错误'))
  }
}

// 处理下拉菜单命令
function handleCommand(command: string, row: any) {
  switch (command) {
    case 'toggle':
      toggleStatus(row)
      break
    case 'setDefault':
      handleSetDefault(row)
      break
    case 'test':
      testConnectionForRow(row)
      break
    case 'edit':
      openEdit(row)
      break
    case 'delete':
      handleDelete(row)
      break
  }
}

// 测试连接（列表中的存储）
async function testConnectionForRow(row: any) {
  testing.value = true
  try {
    // 构建测试请求的payload
    const payload = buildTestPayload(row)
    
    await testStorage(payload)
    ElMessage.success(`存储 "${row.name}" 连接测试成功`)
  } catch (error: any) {
    console.error('Failed to test storage:', error)
    ElMessage.error(`连接测试失败: ${error.message || '未知错误'}`)
  } finally {
    testing.value = false
  }
}

// 测试连接（对话框中）
async function handleTestConnection() {
  try {
    await formRef.value?.validate()
  } catch {
    ElMessage.warning('请先填写完整的配置信息')
    return
  }

  testing.value = true
  try {
    // 构建测试请求的payload
    const payload = buildTestPayload(form.value)
    
    await testStorage(payload)
    ElMessage.success('连接测试成功')
  } catch (error: any) {
    console.error('Failed to test storage:', error)
    ElMessage.error(`连接测试失败: ${error.message || '未知错误'}`)
  } finally {
    testing.value = false
  }
}

// 构建测试payload
function buildTestPayload(storageConfig: any) {
  const payload: any = {
    name: storageConfig.name,
    type: storageConfig.type,
    enabled: true
  }

  // 根据类型构建config
  if (storageConfig.type === 'redis') {
    const config = storageConfig.config || {}
    payload.config = {
      addr: config.addr || `${storageConfig.host || 'localhost'}:${storageConfig.port || 6379}`,
      password: config.password || storageConfig.password || '',
      db: config.db ?? storageConfig.database ?? 0,
      pool_size: config.pool_size || 10
    }
  } else if (storageConfig.type === 'memcached') {
    const config = storageConfig.config || {}
    payload.config = {
      addr: config.addr || `${storageConfig.host || 'localhost'}:${storageConfig.port || 11211}`,
      timeout: config.timeout || storageConfig.timeout || 30
    }
  } else if (storageConfig.type === 'file') {
    const config = storageConfig.config || {}
    payload.config = {
      path: config.path || storageConfig.path || '/tmp/storage',
      max_size: config.max_size || storageConfig.max_size || 1024
    }
  } else if (storageConfig.type === 'elasticsearch') {
    const config = storageConfig.config || {}
    
    // 处理地址列表
    let addresses = config.addresses || ['http://localhost:9200']
    if (storageConfig.addressesText) {
      addresses = storageConfig.addressesText
        .split('\n')
        .map((addr: string) => addr.trim())
        .filter((addr: string) => addr.length > 0)
    }
    
    payload.config = {
      addresses: addresses,
      username: config.username || storageConfig.username || 'elastic',
      password: config.password || storageConfig.password || '',
      api_key: config.api_key || storageConfig.api_key || '',
      exact_index: config.exact_index || storageConfig.exact_index || 'cache_exact_index',
      semantic_index: config.semantic_index || storageConfig.semantic_index || 'cache_semantic_index',
      vector_dimension: config.vector_dimension || storageConfig.vector_dimension || 1024,
      enable_tls: config.enable_tls || storageConfig.enable_tls || false,
      request_timeout: config.request_timeout || storageConfig.request_timeout || 30
    }
  } else if (storageConfig.type === 'chroma') {
    const config = storageConfig.config || {}
    payload.config = {
      addr: config.addr || storageConfig.addr || 'localhost:8000',
      collection: config.collection || storageConfig.collection || 'llm_cache',
      timeout: config.timeout || storageConfig.timeout || 30
    }
  } else if (storageConfig.type === 'postgresql') {
    const config = storageConfig.config || {}
    payload.config = {
      host: config.host || storageConfig.host || 'localhost',
      port: config.port ?? storageConfig.port ?? 5432,
      user: config.user || storageConfig.user || 'postgres',
      password: config.password || storageConfig.password || '',
      database: config.database || storageConfig.database || 'centag',
      ssl_mode: config.ssl_mode || storageConfig.ssl_mode || 'disable',
      max_conns: config.max_conns || storageConfig.max_conns || 20,
      min_conns: config.min_conns || storageConfig.min_conns || 5,
      max_conn_lifetime: config.max_conn_lifetime || storageConfig.max_conn_lifetime || 3600,
      max_conn_idle_time: config.max_conn_idle_time || storageConfig.max_conn_idle_time || 600,
      kv_table: config.kv_table || storageConfig.kv_table || 'kv_cache',
      vector_table: config.vector_table || storageConfig.vector_table || 'vector_cache',
      qa_table: config.qa_table || storageConfig.qa_table || 'qa_pairs',
      vector_dimension: config.vector_dimension || storageConfig.vector_dimension || 1024,
      index_type: config.index_type || storageConfig.index_type || 'hnsw'
    }
  }

  return payload
}

onMounted(() => {
  load()
})
</script>

<style scoped>
.header-with-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: var(--spacing-lg);
  gap: var(--spacing-lg);
}

.header-left {
  flex-shrink: 0;
}

.toolbar-actions {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
}

.content-wrapper {
  width: 100%;
}

.table-card {
  width: 100%;
}

/* 表格样式 */
.el-table {
  width: 100%;
}

/* 健康状态徽标 */
.health-badge {
  display: inline-flex;
  flex-direction: row;
  align-items: center;
  gap: 4px;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 12px;
  line-height: 20px;
  white-space: nowrap;
}

.health-badge--info {
  color: var(--el-color-info);
  background-color: var(--el-color-info-light-9);
  border: 1px solid var(--el-color-info-light-5);
}

.health-badge--success {
  color: var(--el-color-success);
  background-color: var(--el-color-success-light-9);
  border: 1px solid var(--el-color-success-light-5);
}

.health-badge--danger {
  color: var(--el-color-danger);
  background-color: var(--el-color-danger-light-9);
  border: 1px solid var(--el-color-danger-light-5);
}

.name-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.name-title {
  font-weight: 600;
  color: var(--color-gray-900);
  font-size: 0.9375rem;
}

.name-subtitle {
  font-size: 0.8125rem;
  color: var(--color-gray-500);
  line-height: 1.4;
}

.text-muted {
  color: var(--color-gray-400);
}

/* 错误提示tooltip */
.error-tooltip {
  max-width: 400px;
  word-wrap: break-word;
  line-height: 1.5;
}

.unit {
  margin-left: var(--spacing-sm);
  color: var(--color-gray-600);
}

/* 表单提示文字 */
.form-tip {
  font-size: 0.75rem;
  color: var(--color-gray-500);
  margin-top: 4px;
  line-height: 1.4;
}

/* 下拉菜单样式 */
:deep(.el-dropdown-menu__item) {
  display: flex;
  align-items: center;
  gap: 8px;
}


@media (max-width: 1200px) {
  .toolbar-actions :deep(.el-input) {
    width: 160px !important;
  }
}

@media (max-width: 1024px) {
  .header-with-toolbar {
    flex-direction: column;
    align-items: stretch;
  }

  .toolbar-actions {
    flex-wrap: wrap;
  }

  .toolbar-actions :deep(.el-input),
  .toolbar-actions :deep(.el-select) {
    width: 180px !important;
  }
}

@media (max-width: 768px) {
  .toolbar-actions {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: var(--spacing-sm);
  }

  .toolbar-actions :deep(.el-input),
  .toolbar-actions :deep(.el-select),
  .toolbar-actions :deep(.el-button) {
    width: 100% !important;
  }
}
</style>
