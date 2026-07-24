<template>
  <el-drawer
    v-model="visible"
    :title="t('billingRulesDialog.dialogTitle')"
    direction="rtl"
    size="50%"
    destroy-on-close
    class="billing-rules-drawer"
    @opened="load"
  >
    <div class="billing-rules-body">
      <p class="sub">
        {{ t('billingRulesDialog.description') }}
      </p>
      <div class="actions">
        <el-button size="small" @click="load">{{ t('billingRulesDialog.refresh') }}</el-button>
        <el-button size="small" @click="openImport">{{ t('billingRulesDialog.importYaml') }}</el-button>
        <el-button size="small" @click="doExport">{{ t('billingRulesDialog.export') }}</el-button>
        <el-button size="small" type="primary" @click="openCreate">{{ t('billingRulesDialog.addRule') }}</el-button>
        <el-radio-group v-model="displayCurrency" size="small" @change="onDisplayCurrencyChange">
          <el-radio-button value="USD">{{ t('billingRulesDialog.usdLabel') }}</el-radio-button>
          <el-radio-button value="CNY">{{ t('billingRulesDialog.cnyLabel') }}</el-radio-button>
        </el-radio-group>
      </div>

      <el-table
        v-loading="loading"
        :data="rules"
        stripe
        size="small"
        :empty-text="t('billingRulesDialog.emptyText')"
        class="rules-table"
        height="100%"
      >
        <el-table-column prop="name" :label="t('billingRulesDialog.table.name')" min-width="120" />
        <el-table-column prop="backend_id" :label="t('billingRulesDialog.table.backend')" width="120" />
        <el-table-column prop="model" :label="t('billingRulesDialog.table.model')" width="140" />
        <el-table-column :label="t('billingRulesDialog.table.inputPrice', { unit: priceUnit })" width="110">
          <template #default="{ row }">{{ formatPrice(row.input_price_per_m) }}</template>
        </el-table-column>
        <el-table-column :label="t('billingRulesDialog.table.outputPrice', { unit: priceUnit })" width="110">
          <template #default="{ row }">{{ formatPrice(row.output_price_per_m) }}</template>
        </el-table-column>
        <el-table-column prop="priority" :label="t('billingRulesDialog.table.priority')" width="70" />
        <el-table-column :label="t('billingRulesDialog.table.enabled')" width="70">
          <template #default="{ row }">
            <el-tag :type="row.enabled ? 'success' : 'info'" size="small">
              {{ row.enabled ? t('billingRulesDialog.table.enabledYes') : t('billingRulesDialog.table.enabledNo') }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="t('billingRulesDialog.table.actions')" width="130" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openEdit(row)">{{ t('billingRulesDialog.table.edit') }}</el-button>
            <el-button link type="danger" @click="remove(row)">{{ t('billingRulesDialog.table.delete') }}</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <el-dialog v-model="formVisible" :title="editingId ? t('billingRulesDialog.form.editTitle') : t('billingRulesDialog.form.createTitle')" width="520px" append-to-body>
      <el-form :model="form" label-width="110px">
        <el-form-item :label="t('billingRulesDialog.form.name')"><el-input v-model="form.name" /></el-form-item>
        <el-form-item :label="t('billingRulesDialog.form.backendId')">
          <el-input v-model="form.backend_id" :placeholder="t('billingRulesDialog.form.backendPlaceholder')" />
        </el-form-item>
        <el-form-item :label="t('billingRulesDialog.form.model')">
          <el-input v-model="form.model" :placeholder="t('billingRulesDialog.form.modelPlaceholder')" />
        </el-form-item>
        <el-form-item :label="t('billingRulesDialog.form.inputPrice')">
          <el-input-number v-model="form.input_price_per_m" :min="0" :step="0.01" :precision="4" />
        </el-form-item>
        <el-form-item :label="t('billingRulesDialog.form.outputPrice')">
          <el-input-number v-model="form.output_price_per_m" :min="0" :step="0.01" :precision="4" />
        </el-form-item>
        <el-form-item :label="t('billingRulesDialog.form.priority')">
          <el-input-number v-model="form.priority" :step="1" />
        </el-form-item>
        <el-form-item :label="t('billingRulesDialog.form.enabled')"><el-switch v-model="form.enabled" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="formVisible = false">{{ t('billingRulesDialog.form.cancel') }}</el-button>
        <el-button type="primary" :loading="saving" @click="save">{{ t('billingRulesDialog.form.save') }}</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="importVisible" :title="t('billingRulesDialog.importDialog.title')" width="640px" append-to-body>
      <el-input v-model="importText" type="textarea" :rows="14" :placeholder="t('billingRulesDialog.importDialog.placeholder')" />
      <template #footer>
        <el-button @click="importVisible = false">{{ t('billingRulesDialog.importDialog.cancel') }}</el-button>
        <el-button type="primary" :loading="saving" @click="doImport">{{ t('billingRulesDialog.importDialog.importReplace') }}</el-button>
      </template>
    </el-dialog>
  </el-drawer>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
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

const { t } = useI18n()

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
    ElMessage.error(e instanceof Error ? e.message : t('billingRulesDialog.message.loadFailed'))
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
    ElMessage.warning(t('billingRulesDialog.message.fillRequired'))
    return
  }
  saving.value = true
  try {
    if (editingId.value != null) {
      await billingApi.updatePricingRule(editingId.value, { ...form })
    } else {
      await billingApi.createPricingRule({ ...form })
    }
    ElMessage.success(t('billingRulesDialog.message.saveSuccess'))
    formVisible.value = false
    await load()
    emit('saved')
  } catch (e: unknown) {
    ElMessage.error(e instanceof Error ? e.message : t('billingRulesDialog.message.saveFailed'))
  } finally {
    saving.value = false
  }
}

async function remove(row: PricingRule) {
  if (row.id == null) return
  try {
    await ElMessageBox.confirm(t('billingRulesDialog.confirmDelete', { name: row.name || row.id }), t('billingRulesDialog.confirmDeleteTitle'))
    await billingApi.deletePricingRule(row.id)
    ElMessage.success(t('billingRulesDialog.message.deleteSuccess'))
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
    ElMessage.warning(t('billingRulesDialog.message.importEmptyWarning'))
    return
  }
  saving.value = true
  try {
    const res = await billingApi.importPricingRules(importText.value)
    ElMessage.success(t('billingRulesDialog.message.importSuccess', { count: res?.imported ?? 0 }))
    importVisible.value = false
    await load()
    emit('saved')
  } catch (e: unknown) {
    ElMessage.error(e instanceof Error ? e.message : t('billingRulesDialog.message.importFailed'))
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
    ElMessage.error(e instanceof Error ? e.message : t('billingRulesDialog.message.exportFailed'))
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
