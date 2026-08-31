<template>
  <div class="pricing-rules-panel" :class="{ embedded }">
    <p v-if="showDescription" class="sub">{{ t('billingRulesDialog.description') }}</p>

    <div class="toolbar">
      <el-input
        v-model="search"
        clearable
        size="small"
        class="search-input"
        :placeholder="t('billingRules.filters.searchPlaceholder')"
      />
      <el-select v-model="backendFilter" clearable size="small" class="filter-select" :placeholder="t('billingRules.filters.backend')">
        <el-option v-for="id in backendOptions" :key="id" :label="id" :value="id" />
      </el-select>
      <el-select v-model="priceTypeFilter" size="small" class="filter-select-sm">
        <el-option :label="t('billingRulesDialog.tabAll')" value="all" />
        <el-option :label="t('billingRulesDialog.tabCost')" value="cost" />
        <el-option :label="t('billingRulesDialog.tabRevenue')" value="revenue" />
      </el-select>
      <el-select v-model="freePaidFilter" size="small" class="filter-select-sm">
        <el-option :label="t('billingRules.filters.tierAll')" value="all" />
        <el-option :label="t('billingRules.filters.tierFree')" value="free" />
        <el-option :label="t('billingRules.filters.tierPaid')" value="paid" />
      </el-select>
      <el-button size="small" :loading="loading" @click="load">{{ t('billingRulesDialog.refresh') }}</el-button>
      <el-button size="small" :loading="syncing" @click="syncFromFeishu">{{ t('billingRules.syncOnline') }}</el-button>
      <el-button size="small" @click="openImport">{{ t('billingRulesDialog.importLocal') }}</el-button>
      <el-button size="small" @click="doExport">{{ t('billingRulesDialog.export') }}</el-button>
      <el-button size="small" type="primary" @click="openCreate">{{ t('billingRulesDialog.addRule') }}</el-button>
      <el-radio-group v-model="displayCurrency" size="small" @change="onDisplayCurrencyChange">
        <el-radio-button value="USD">{{ t('billingRulesDialog.usdLabel') }}</el-radio-button>
        <el-radio-button value="CNY">{{ t('billingRulesDialog.cnyLabel') }}</el-radio-button>
      </el-radio-group>
    </div>

    <div v-loading="loading" class="groups-wrap">
      <el-empty v-if="!loading && groups.length === 0" :description="t('billingRulesDialog.emptyText')" />
      <el-collapse v-else v-model="expandedBackends">
        <el-collapse-item v-for="g in groups" :key="g.backendId" :name="g.backendId">
          <template #title>
            <div class="group-title">
              <span class="backend-id">{{ g.backendId }}</span>
              <el-tag size="small" effect="plain">{{ t('billingRules.group.total', { count: g.rules.length }) }}</el-tag>
              <el-tag size="small" type="success" effect="plain">
                {{ t('billingRules.group.free', { count: g.freeCount }) }}
              </el-tag>
              <el-tag size="small" type="warning" effect="plain">
                {{ t('billingRules.group.paid', { count: g.paidCount }) }}
              </el-tag>
            </div>
          </template>

          <div v-if="g.freeRules.length" class="tier-block">
            <div class="tier-label">{{ t('billingRules.filters.tierFree') }}</div>
            <RulesTable
              :rows="g.freeRules"
              :price-unit="priceUnit"
              :display-currency="displayCurrency"
              :usd-to-cny="usdToCny"
              :saving-ids="savingIds"
              @edit="openEdit"
              @remove="remove"
              @save-prices="savePrices"
              @toggle-enabled="toggleEnabled"
            />
          </div>
          <div v-if="g.paidRules.length" class="tier-block">
            <div class="tier-label">{{ t('billingRules.filters.tierPaid') }}</div>
            <RulesTable
              :rows="g.paidRules"
              :price-unit="priceUnit"
              :display-currency="displayCurrency"
              :usd-to-cny="usdToCny"
              :saving-ids="savingIds"
              @edit="openEdit"
              @remove="remove"
              @save-prices="savePrices"
              @toggle-enabled="toggleEnabled"
            />
          </div>
        </el-collapse-item>
      </el-collapse>
    </div>

    <el-dialog
      v-model="formVisible"
      :title="editingId ? t('billingRulesDialog.form.editTitle') : t('billingRulesDialog.form.createTitle')"
      width="520px"
      append-to-body
    >
      <el-form :model="form" label-width="110px">
        <el-form-item :label="t('billingRulesDialog.form.name')"><el-input v-model="form.name" /></el-form-item>
        <el-form-item :label="t('billingRulesDialog.form.priceType')">
          <el-select v-model="form.price_type" :placeholder="t('billingRulesDialog.form.priceTypePlaceholder')">
            <el-option label="Cost" value="cost" />
            <el-option label="Revenue" value="revenue" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('billingRulesDialog.form.backendId')">
          <el-select
            v-model="form.backend_id"
            filterable
            allow-create
            default-first-option
            :placeholder="t('billingRulesDialog.form.backendPlaceholder')"
            style="width: 100%"
          >
            <el-option v-for="id in backendOptions" :key="id" :label="id" :value="id" />
          </el-select>
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
        <el-button type="primary" :loading="saving" @click="doImport">
          {{ t('billingRulesDialog.importDialog.importReplace') }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import * as billingApi from '@/api/billing'
import type { PricingRule } from '@/api/billing'
import { getBackends } from '@/api/backend'
import * as costApi from '@/api/cost'
import {
  collectBackendModelNames,
  filterPricingRules,
  filterRulesToConfigured,
  groupRulesByBackend,
  orphanBackendRules,
  uniqueBackendIds,
  type FreePaidFilter,
  type PriceTypeFilter
} from '@/utils/pricing-rule-groups'
import {
  getDisplayCurrency,
  setDisplayCurrency,
  type DisplayCurrency
} from '@/utils/billing-currency'
import RulesTable from './PricingRulesTable.vue'

const props = withDefaults(
  defineProps<{
    embedded?: boolean
    showDescription?: boolean
    autoLoad?: boolean
  }>(),
  {
    embedded: false,
    showDescription: true,
    autoLoad: true
  }
)

const emit = defineEmits<{ saved: [] }>()

const { t } = useI18n()

const loading = ref(false)
const saving = ref(false)
const syncing = ref(false)
const rules = ref<PricingRule[]>([])
const configuredBackendIds = ref<Set<string>>(new Set())
const modelsByBackend = ref<Map<string, Set<string>>>(new Map())
const formVisible = ref(false)
const importVisible = ref(false)
const importText = ref('')
const editingId = ref<number | null>(null)
const displayCurrency = ref<DisplayCurrency>(getDisplayCurrency())
const usdToCny = ref(7.2)
const search = ref('')
const backendFilter = ref('')
const priceTypeFilter = ref<PriceTypeFilter>('all')
const freePaidFilter = ref<FreePaidFilter>('all')
const expandedBackends = ref<string[]>([])
const savingIds = ref<Set<number>>(new Set())

const priceUnit = computed(() => (displayCurrency.value === 'CNY' ? '¥' : '$'))

const scopedRules = computed(() =>
  filterRulesToConfigured(rules.value, configuredBackendIds.value, modelsByBackend.value)
)

const filteredRules = computed(() =>
  filterPricingRules(scopedRules.value, {
    search: search.value,
    priceType: priceTypeFilter.value,
    freePaid: freePaidFilter.value,
    backendId: backendFilter.value
  })
)

const groups = computed(() => groupRulesByBackend(filteredRules.value))
const backendOptions = computed(() => {
  const fromConfig = [...configuredBackendIds.value]
  const fromRules = uniqueBackendIds(scopedRules.value)
  return [...new Set([...fromConfig, ...fromRules])].sort((a, b) => a.localeCompare(b))
})

watch(
  groups,
  (gs) => {
    const ids = gs.map((g) => g.backendId)
    if (expandedBackends.value.length === 0 && ids.length) {
      expandedBackends.value = ids.slice(0, 8)
    }
  },
  { immediate: true }
)

const form = reactive<PricingRule>({
  name: '',
  backend_id: '',
  model: '',
  input_price_per_m: 0,
  output_price_per_m: 0,
  priority: 100,
  enabled: true,
  currency: 'USD',
  price_type: 'cost'
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
  form.price_type = 'cost'
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

async function loadConfiguredBackends() {
  try {
    const data = await getBackends()
    const list = Array.isArray(data) ? data : (data as { backends?: unknown[] })?.backends || []
    const ids = new Set<string>()
    const models = new Map<string, Set<string>>()
    for (const raw of list as Array<Record<string, unknown>>) {
      const id = String(raw.id || raw.backend_id || '').trim()
      if (!id) continue
      ids.add(id)
      models.set(id, collectBackendModelNames(raw.supported_models))
    }
    configuredBackendIds.value = ids
    modelsByBackend.value = models
  } catch {
    configuredBackendIds.value = new Set()
    modelsByBackend.value = new Map()
  }
}

/** Delete seed/orphan rules for backends that are not configured (e.g. ollama-local, bigmodel). */
async function pruneOrphanBackendRules(allRules: PricingRule[]) {
  if (configuredBackendIds.value.size === 0) return
  const orphans = orphanBackendRules(allRules, configuredBackendIds.value)
  if (!orphans.length) return
  let deleted = 0
  for (const r of orphans) {
    if (r.id == null) continue
    try {
      await billingApi.deletePricingRule(r.id)
      deleted++
    } catch {
      /* continue */
    }
  }
  if (deleted > 0) {
    ElMessage.success(t('billingRules.prunedOrphans', { count: deleted }))
  }
}

async function load() {
  loading.value = true
  try {
    await Promise.all([loadFx(), loadConfiguredBackends()])
    let data = await billingApi.listPricingRules()
    let list = Array.isArray(data) ? data : []
    await pruneOrphanBackendRules(list)
    if (configuredBackendIds.value.size > 0) {
      data = await billingApi.listPricingRules()
      list = Array.isArray(data) ? data : []
    }
    rules.value = list
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
    await ElMessageBox.confirm(
      t('billingRulesDialog.confirmDelete', { name: row.name || row.id }),
      t('billingRulesDialog.confirmDeleteTitle')
    )
    await billingApi.deletePricingRule(row.id)
    ElMessage.success(t('billingRulesDialog.message.deleteSuccess'))
    await load()
    emit('saved')
  } catch {
    /* cancel */
  }
}

async function savePrices(payload: {
  row: PricingRule
  inputDisplay: number
  outputDisplay: number
}) {
  const { row, inputDisplay, outputDisplay } = payload
  if (row.id == null) return
  const fx = usdToCny.value > 0 ? usdToCny.value : 7.2
  const toUsd = (v: number) => (displayCurrency.value === 'CNY' ? v / fx : v)
  const next: PricingRule = {
    ...row,
    input_price_per_m: toUsd(inputDisplay),
    output_price_per_m: toUsd(outputDisplay)
  }
  const same =
    Math.abs((row.input_price_per_m || 0) - (next.input_price_per_m || 0)) < 1e-9 &&
    Math.abs((row.output_price_per_m || 0) - (next.output_price_per_m || 0)) < 1e-9
  if (same) return

  const id = row.id
  const nextSet = new Set(savingIds.value)
  nextSet.add(id)
  savingIds.value = nextSet
  try {
    await billingApi.updatePricingRule(id, next)
    ElMessage.success(t('billingRulesDialog.message.saveSuccess'))
    await load()
    emit('saved')
  } catch (e: unknown) {
    ElMessage.error(e instanceof Error ? e.message : t('billingRulesDialog.message.saveFailed'))
  } finally {
    const cleared = new Set(savingIds.value)
    cleared.delete(id)
    savingIds.value = cleared
  }
}

async function toggleEnabled(row: PricingRule, enabled: boolean) {
  if (row.id == null) return
  const id = row.id
  const nextSet = new Set(savingIds.value)
  nextSet.add(id)
  savingIds.value = nextSet
  try {
    await billingApi.updatePricingRule(id, { ...row, enabled })
    row.enabled = enabled
    emit('saved')
  } catch (e: unknown) {
    ElMessage.error(e instanceof Error ? e.message : t('billingRulesDialog.message.saveFailed'))
  } finally {
    const cleared = new Set(savingIds.value)
    cleared.delete(id)
    savingIds.value = cleared
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

async function syncFromFeishu() {
  try {
    await ElMessageBox.confirm(
      t('billingRules.confirmSyncFeishu'),
      t('billingRules.confirmSyncFeishuTitle')
    )
  } catch {
    return
  }
  syncing.value = true
  try {
    await billingApi.triggerConfigSyncNow()
    ElMessage.success(t('billingRules.syncFeishuSuccess'))
    await load()
    emit('saved')
  } catch (e: unknown) {
    ElMessage.error(e instanceof Error ? e.message : t('billingRules.syncFeishuFailed'))
  } finally {
    syncing.value = false
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

onMounted(() => {
  if (props.autoLoad) load()
})

defineExpose({ load, reload: load })
</script>

<style scoped>
.pricing-rules-panel {
  display: flex;
  flex-direction: column;
  gap: 12px;
  /* 交给外层 .main 滚动，避免 height:100% + overflow 锁死 */
  height: auto;
  min-height: 0;
}
.sub {
  margin: 0;
  color: var(--el-text-color-secondary);
  font-size: 13px;
}
.toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
}
.search-input {
  width: 220px;
}
.filter-select {
  width: 160px;
}
.filter-select-sm {
  width: 120px;
}
.groups-wrap {
  min-height: 0;
  overflow: visible;
}
.group-title {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.backend-id {
  font-weight: 600;
  margin-right: 4px;
}
.tier-block + .tier-block {
  margin-top: 14px;
}
.tier-label {
  font-size: 13px;
  font-weight: 600;
  color: var(--el-text-color-secondary);
  margin-bottom: 8px;
}
</style>
