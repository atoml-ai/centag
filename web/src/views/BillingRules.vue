<template>
  <div class="billing-rules-page">
    <el-result icon="info" title="计费规则"
      sub-title="minimal 版请在概览「用量与会话」中打开计费规则对话框。团队版可继续使用本页。">
      <template #extra>
        <el-button type="primary" @click="open = true">打开计费规则</el-button>
      </template>
    </el-result>
    <BillingRulesDialog v-model="open" />
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import BillingRulesDialog from '@/components/dashboard/BillingRulesDialog.vue'

const open = ref(true)
</script>

<style scoped>
.billing-rules-page {
  padding: 24px;
}
</style>
