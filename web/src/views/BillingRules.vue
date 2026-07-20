<template>
  <div class="billing-rules-page">
    <el-result
      icon="info"
      title="计费规则"
      sub-title="配置模型单价与导入导出（YAML，默认 USD）。仅管理员可管理。"
    >
      <template #extra>
        <el-button type="primary" @click="open = true">打开计费规则</el-button>
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
  padding: 24px;
}
</style>
