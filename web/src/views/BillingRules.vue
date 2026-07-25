<template>
  <div class="billing-rules-page">
    <el-result
      icon="info"
      :title="$t('billingRules.title')"
      :sub-title="$t('billingRules.subtitle')"
    >
      <template #extra>
        <el-button type="primary" @click="open = true">{{ $t('billingRules.openDialog') }}</el-button>
      </template>
    </el-result>
    <BillingRulesDialog v-model="open" />
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import BillingRulesDialog from '@/components/dashboard/BillingRulesDialog.vue'

const route = useRoute()
const open = ref(false)

onMounted(() => {
  if (route.query.billing === '1' || route.query.billing === 'true' || route.query.open === '1') {
    open.value = true
  }
})
</script>

<style scoped>
.billing-rules-page {
  width: 100%;
  padding: 0 0 24px;
}
</style>
