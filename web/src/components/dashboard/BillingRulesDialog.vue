<template>
  <el-drawer
    v-model="visible"
    :title="t('billingRulesDialog.dialogTitle')"
    direction="rtl"
    size="72%"
    destroy-on-close
    class="billing-rules-drawer"
  >
    <div class="drawer-hint">
      <el-button type="primary" link @click="goFullPage">{{ t('billingRules.openFullPage') }}</el-button>
    </div>
    <PricingRulesPanel v-if="visible" embedded @saved="emit('saved')" />
  </el-drawer>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import PricingRulesPanel from '@/components/billing/PricingRulesPanel.vue'

const visible = defineModel<boolean>({ default: false })
const emit = defineEmits<{ saved: [] }>()

const { t } = useI18n()
const router = useRouter()

function goFullPage() {
  visible.value = false
  router.push('/billing')
}
</script>

<style scoped>
.drawer-hint {
  margin-bottom: 8px;
}
</style>

<style>
.billing-rules-drawer .el-drawer__body {
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
</style>
