<template>
  <el-card class="personal-hero">
    <ApiAccessPanel :base-url="baseUrl" />
  </el-card>
</template>

<script setup lang="ts">
import ApiAccessPanel from '@/components/dashboard/ApiAccessPanel.vue'

defineProps<{
  baseUrl: string
}>()
</script>

<style scoped>
.personal-hero {
  margin-bottom: var(--spacing-lg);
  border: 1px solid var(--shell-sidebar-border);
  background: linear-gradient(135deg, rgba(102, 126, 234, 0.06) 0%, rgba(255, 255, 255, 1) 55%);
}
</style>
