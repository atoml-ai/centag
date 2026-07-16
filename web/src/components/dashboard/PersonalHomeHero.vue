<template>
  <el-card class="personal-hero">
    <ApiAccessPanel :base-url="baseUrl" />
    <div class="hero-actions">
      <el-button @click="$router.push('/backends')">配置模型</el-button>
      <el-button type="primary" plain @click="$router.push('/chat')">开始对话</el-button>
    </div>
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

.hero-actions {
  display: flex;
  gap: var(--spacing-sm);
  flex-wrap: wrap;
  margin-top: var(--spacing-md);
  padding-top: var(--spacing-md);
  border-top: 1px solid var(--shell-sidebar-border);
}
</style>