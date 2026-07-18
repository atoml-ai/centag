<template>
  <el-drawer
    v-model="visible"
    title="计费规则"
    direction="rtl"
    size="50%"
    destroy-on-close
    class="billing-rules-drawer"
    @opened="load"
  >
    <div class="billing-rules-body">
      <p class="sub">
        按后端 + 模型配置单价（美元 / 1M tokens）。导入导出 YAML 默认 USD；显示可选人民币（仅换算，不改正本）。
      </p>
      <div class="actions">
        <el-button size="small" @click="load">刷新</el-button>
        <el-button size="small" @click="openImport">导入 YAML</el-button>
        <el-button size="small" @click="doExport">导出</el-button>
        <el-button size="small" type="primary" @click="openCreate">新增规则</el-button>
        <el-radio-group v-model="displayCurrency" size="small" @change="onDisplayCurrencyChange">
          <el-radio-button value="USD">美元</el-radio-button>
          <el-radio-button value="CNY">人民币</el-radio-button>
        </el-radio-group>
      </div>

      <el-table
        v-loading="loading"
        :data="rules"
        stripe
        size="small"
        empty-text="暂无规则，请导入或新增"
        class="rules-table"
        height="100%"
      >
        <el-table-column prop="name" label="名称" min-width="120" />
        <el-table-column prop="backend_id" label="后端" width="120" />
        <el-table-column prop="model" label="模型" width="140" />
        <el-table-column :label="`输入价(${priceUnit})`" width="110">
          <template #default="{ row }">{{ formatPrice(row.input_price_per_m) }}</template>
        </el-table-column>
        <el-table-column :label="`输出价(${priceUnit})`" width="110">
          <template #default="{ row }">{{ formatPrice(row.output_price_per_m) }}</template>
        </el-table-column>
        <el-table-column prop="priority" label="优先级" width="70" />
        <el-table-column label="启用" width="70">
          <template #default="{ row }">
            <el-tag :type="row.enabled ? 'success' : 'info'" size="small">
              {{ row.enabled ? '是' : '否' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="130" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
            <el-button link type="danger" @click="remove(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <el-dialog v-model="formVisible" :title="editingId ? '编辑规则' : '新增规则'" width="520px" append-to-body>
      <el-form :model="form" label-width="110px">
        <el-form-item label="名称"><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="后端 ID">
          <el-input v-model="form.backend_id" placeholder="如 ppinfra，或 *" />
        </el-form-item>
        <el-form-item label="模型">
          <el-input v-model="form.model" placeholder="如 deepseek-v3.2，或 *" />
        </el-form-item>
        <el-form-item label="输入价 $/1M">
          <el-input-number v-model="form.input_price_per_m" :min="0" :step="0.01" :precision="4" />
        </el-form-item>
        <el-form-item label="输出价 $/1M">
          <el-input-number v-model="form.output_price_per_m" :min="0" :step="0.01" :precision="4" />
        </el-form-item>
        <el-form-item label="优先级">
          <el-input-number v-model="form.priority" :step="1" />
        </el-form-item>
        <el-form-item label="启用"><el-switch v-model="form.enabled" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="formVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="importVisible" title="导入 YAML" width="640px" append-to-body>
      <el-input v-model="importText" type="textarea" :rows="14" placeholder="粘贴 pricing YAML" />
      <template #footer>
        <el-button @click="importVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="doImport">导入（替换全部）</el-button>
      </template>
    </el-dialog>
  </el-drawer>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import * as billingApi from '@/api/billing'
import type { PricingRule } from '@/api/billing'
import * as costApi from '@/api/cost'
import {
  formatDisplayCost,
  getDisplayCurrency,
  setDisplayCurrency,
  type DisplayCurrency
} from '@/utils/billing-currency'

const visible = defineModel<boolean>({ default: false })
const emit = defineEmits<{ saved: [] }>()

const loading = ref(false)
const saving = ref(false)
const rules = ref<PricingRule[]>([])
const formVisible = ref(false)
const importVisible = ref(false)
const importText = ref('')
const editingId = ref<number | null>(null)
const displayCurrency = ref<DisplayCurrency>(getDisplayCurrency())
const usdToCny = ref(7.2)

const priceUnit = computed(() => (displayCurrency.value === 'CNY' ? '¥' : '$'))

const form = reactive<PricingRule>({
  name: '',
  backend_id: '',
  model: '',
  input_price_per_m: 0,
  output_price_per_m: 0,
  priority: 100,
  enabled: true,
  currency: 'USD'
})

function resetForm() {
  form.name = ''
  form.backend_id = ''
  form.model = ''
  form.input_price_per_m = 0
  form.output_price_per_m = 0
  form.priority = 100
  form.enabled = true
  form.currency = 'USD'
}

function formatPrice(usd: number | undefined | null): string {
  return formatDisplayCost(usd, displayCurrency.value, usdToCny.value)
}

function onDisplayCurrencyChange(v: DisplayCurrency | string | number | boolean | undefined) {
  const c = v === 'CNY' ? 'CNY' : 'USD'
  displayCurrency.value = c
  setDisplayCurrency(c)
}

async function loadFx() {
  try {
    const s = await costApi.getCostSummary({ group_by: 'model' })
    if (s?.usd_to_cny && s.usd_to_cny > 0) usdToCny.value = s.usd_to_cny
  } catch {
    /* keep default */
  }
}

async function load() {
  loading.value = true
  try {
    await loadFx()
    const data = await billingApi.listPricingRules()
    rules.value = Array.isArray(data) ? data : []
  } catch (e: unknown) {
    ElMessage.error(e instanceof Error ? e.message : '加载失败')
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editingId.value = null
  resetForm()
  formVisible.value = true
}

function openEdit(row: PricingRule) {
  editingId.value = row.id ?? null
  Object.assign(form, row)
  formVisible.value = true
}

async function save() {
  if (!form.backend_id || !form.model) {
    ElMessage.warning('请填写后端与模型')
    return
  }
  saving.value = true
  try {
    if (editingId.value != null) {
      await billingApi.updatePricingRule(editingId.value, { ...form })
    } else {
      await billingApi.createPricingRule({ ...form })
    }
    ElMessage.success('已保存')
    formVisible.value = false
    await load()
    emit('saved')
  } catch (e: unknown) {
    ElMessage.error(e instanceof Error ? e.message : '保存失败')
  } finally {
    saving.value = false
  }
}

async function remove(row: PricingRule) {
  if (row.id == null) return
  try {
    await ElMessageBox.confirm(`删除规则「${row.name || row.id}」？`, '确认')
    await billingApi.deletePricingRule(row.id)
    ElMessage.success('已删除')
    await load()
    emit('saved')
  } catch {
    /* cancel */
  }
}

function openImport() {
  importText.value = ''
  importVisible.value = true
}

async function doImport() {
  if (!importText.value.trim()) {
    ElMessage.warning('请粘贴 YAML')
    return
  }
  saving.value = true
  try {
    const res = await billingApi.importPricingRules(importText.value)
    ElMessage.success(`已导入 ${res?.imported ?? 0} 条`)
    importVisible.value = false
    await load()
    emit('saved')
  } catch (e: unknown) {
    ElMessage.error(e instanceof Error ? e.message : '导入失败')
  } finally {
    saving.value = false
  }
}

async function doExport() {
  try {
    const text = await billingApi.exportPricingRules()
    const blob = new Blob([typeof text === 'string' ? text : String(text)], { type: 'application/x-yaml' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = 'pricing-rules.yaml'
    a.click()
    URL.revokeObjectURL(url)
  } catch (e: unknown) {
    ElMessage.error(e instanceof Error ? e.message : '导出失败')
  }
}
</script>

<style scoped>
.billing-rules-body {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
}
.sub {
  margin: 0 0 12px;
  color: var(--el-text-color-secondary);
  font-size: 13px;
  flex-shrink: 0;
}
.actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 12px;
  flex-shrink: 0;
}
.rules-table {
  flex: 1;
  min-height: 0;
}
</style>

<style>
.billing-rules-drawer .el-drawer__body {
  display: flex;
  flex-direction: column;
  height: calc(100% - 55px);
  padding-top: 0;
  overflow: hidden;
}
</style>
