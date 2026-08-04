<template>
  <el-table :data="rows" stripe size="small" class="rules-table" :empty-text="t('billingRulesDialog.emptyText')">
    <el-table-column prop="model" :label="t('billingRulesDialog.table.model')" min-width="150" show-overflow-tooltip />
    <el-table-column prop="name" :label="t('billingRulesDialog.table.name')" min-width="120" show-overflow-tooltip />
    <el-table-column prop="price_type" :label="t('billingRulesDialog.table.priceType')" width="90">
      <template #default="{ row }">
        <el-tag :type="row.price_type === 'revenue' ? 'warning' : 'info'" size="small">
          {{ row.price_type || 'cost' }}
        </el-tag>
      </template>
    </el-table-column>
    <el-table-column :label="t('billingRulesDialog.table.inputPrice', { unit: priceUnit })" width="150">
      <template #default="{ row }">
        <el-input-number
          v-model="draft[keyOf(row)].input"
          size="small"
          :min="0"
          :step="0.01"
          :precision="4"
          controls-position="right"
          class="price-input"
          @change="() => onPriceChange(row)"
        />
      </template>
    </el-table-column>
    <el-table-column :label="t('billingRulesDialog.table.outputPrice', { unit: priceUnit })" width="150">
      <template #default="{ row }">
        <el-input-number
          v-model="draft[keyOf(row)].output"
          size="small"
          :min="0"
          :step="0.01"
          :precision="4"
          controls-position="right"
          class="price-input"
          @change="() => onPriceChange(row)"
        />
      </template>
    </el-table-column>
    <el-table-column prop="priority" :label="t('billingRulesDialog.table.priority')" width="70" />
    <el-table-column :label="t('billingRulesDialog.table.enabled')" width="80">
      <template #default="{ row }">
        <el-switch
          :model-value="row.enabled"
          size="small"
          :loading="row.id != null && savingIds.has(row.id)"
          @change="(v: string | number | boolean) => emit('toggle-enabled', row, !!v)"
        />
      </template>
    </el-table-column>
    <el-table-column :label="t('billingRulesDialog.table.actions')" width="160" fixed="right">
      <template #default="{ row }">
        <el-button
          link
          type="primary"
          :loading="row.id != null && savingIds.has(row.id)"
          @click="saveRow(row)"
        >
          {{ t('billingRules.savePrices') }}
        </el-button>
        <el-button link type="primary" @click="emit('edit', row)">{{ t('billingRulesDialog.table.edit') }}</el-button>
        <el-button link type="danger" @click="emit('remove', row)">{{ t('billingRulesDialog.table.delete') }}</el-button>
      </template>
    </el-table-column>
  </el-table>
</template>

<script setup lang="ts">
import { reactive, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { PricingRule } from '@/api/billing'
import type { DisplayCurrency } from '@/utils/billing-currency'

const props = defineProps<{
  rows: PricingRule[]
  priceUnit: string
  displayCurrency: DisplayCurrency
  usdToCny: number
  savingIds: Set<number>
}>()

const emit = defineEmits<{
  edit: [row: PricingRule]
  remove: [row: PricingRule]
  'save-prices': [payload: { row: PricingRule; inputDisplay: number; outputDisplay: number }]
  'toggle-enabled': [row: PricingRule, enabled: boolean]
}>()

const { t } = useI18n()

const draft = reactive<Record<string, { input: number; output: number }>>({})

function keyOf(row: PricingRule): string {
  return String(row.id ?? `${row.backend_id}:${row.model}:${row.price_type}`)
}

function toDisplay(usd: number | undefined | null): number {
  const n = Number(usd) || 0
  const fx = props.usdToCny > 0 ? props.usdToCny : 7.2
  return props.displayCurrency === 'CNY' ? n * fx : n
}

function syncDraft() {
  for (const row of props.rows) {
    const k = keyOf(row)
    draft[k] = {
      input: toDisplay(row.input_price_per_m),
      output: toDisplay(row.output_price_per_m)
    }
  }
}

watch(
  () => [props.rows, props.displayCurrency, props.usdToCny] as const,
  () => syncDraft(),
  { immediate: true, deep: true }
)

function saveRow(row: PricingRule) {
  const d = draft[keyOf(row)]
  if (!d) return
  emit('save-prices', { row, inputDisplay: d.input, outputDisplay: d.output })
}

/** Auto-save on number change after a short idle would be noisy; keep explicit save + change marks dirty only. */
function onPriceChange(_row: PricingRule) {
  /* draft already updated via v-model */
}
</script>

<style scoped>
.price-input {
  width: 128px;
}
.rules-table {
  width: 100%;
}
</style>
