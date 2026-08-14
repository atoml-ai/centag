<template>
  <div class="empty-state">
    <div class="empty-icon">
      <i class="icon-robot"></i>
    </div>
    <h2>欢迎使用Agent助手</h2>
    <p>选择一个Skill开始对话，或者创建一个新会话</p>
    <div class="quick-actions">
      <button class="action-btn" @click="$emit('create-session', 'status-check')">
        <i class="icon-stethoscope"></i>
        <span>检查状态</span>
      </button>
      <button class="action-btn" @click="$emit('create-session', 'config-analysis')">
        <i class="icon-cog"></i>
        <span>分析配置</span>
      </button>
      <button class="action-btn" @click="$emit('create-session', 'error-diagnosis')">
        <i class="icon-exclamation-triangle"></i>
        <span>错误诊断</span>
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
defineEmits<{
  'create-session': [skill?: string]
}>()
</script>

<style scoped>
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  padding: 32px;
  text-align: center;
}

.empty-icon {
  width: 80px;
  height: 80px;
  border-radius: 50%;
  background: #e3f2fd;
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 24px;
}

.empty-icon i {
  font-size: 32px;
  color: #1976d2;
}

h2 {
  margin: 0 0 8px 0;
  font-size: 24px;
  color: #333;
}

p {
  margin: 0 0 32px 0;
  color: #666;
}

.quick-actions {
  display: flex;
  gap: 16px;
}

.action-btn {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 16px 24px;
  background: white;
  border: 1px solid #e0e0e0;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s;
}

.action-btn:hover {
  background: #f8f8f8;
  border-color: #1976d2;
}

.action-btn i {
  font-size: 24px;
  margin-bottom: 8px;
  color: #1976d2;
}

.action-btn span {
  font-size: 14px;
  color: #333;
}
</style>