<template>
  <div class="session-list">
    <div class="session-header">
      <div class="session-title-group">
        <el-icon :size="16"><ChatLineSquare /></el-icon>
        <span>会话</span>
      </div>
      <el-button size="small" type="primary" circle :icon="Plus" @click="$emit('create-session')" />
    </div>

    <div class="session-items">
      <div v-if="!sessions.length" class="session-empty">
        <el-icon :size="32"><ChatLineSquare /></el-icon>
        <span>暂无会话</span>
      </div>
      <div
        v-for="session in sessions"
        :key="session.id"
        class="session-item"
        :class="{ active: session.id === currentSessionId }"
        @click="$emit('select-session', session.id)"
      >
        <div class="session-main">
          <div class="session-title">{{ session.title || '新会话' }}</div>
          <div class="session-time">{{ formatTime(session.updated_at) }}</div>
        </div>
        <el-button
          class="delete-btn"
          size="small"
          text
          type="danger"
          :icon="Delete"
          @click.stop="$emit('delete-session', session.id)"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { Plus, Delete, ChatLineSquare } from '@element-plus/icons-vue'

interface Session {
  id: string
  title: string
  updated_at: string
}

defineProps<{
  sessions: Session[]
  currentSessionId: string | null
}>()

defineEmits<{
  'select-session': [sessionId: string]
  'delete-session': [sessionId: string]
  'create-session': []
}>()

const formatTime = (time: string) => {
  const date = new Date(time)
  if (isNaN(date.getTime())) return ''
  const now = new Date()
  const sameDay = date.toDateString() === now.toDateString()
  return date.toLocaleTimeString('zh-CN', {
    hour: '2-digit',
    minute: '2-digit',
    ...(sameDay ? {} : { month: '2-digit', day: '2-digit' })
  })
}
</script>

<style scoped>
.session-list {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
}

.session-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 14px 16px;
  border-bottom: 1px solid var(--el-border-color-light);
}

.session-title-group {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 15px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.session-items {
  flex: 1;
  overflow-y: auto;
  padding: 8px;
}

.session-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 40px 0;
  color: var(--el-text-color-placeholder);
  font-size: 13px;
}

.session-item {
  display: flex;
  align-items: center;
  padding: 10px 12px;
  border-radius: 8px;
  cursor: pointer;
  margin-bottom: 2px;
  transition: background 0.15s;
}

.session-item:hover {
  background: var(--el-fill-color-light);
}

.session-item.active {
  background: var(--el-color-primary-light-9);
}

.session-main {
  flex: 1;
  min-width: 0;
}

.session-title {
  font-size: 14px;
  color: var(--el-text-color-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.session-time {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-top: 2px;
}

.delete-btn {
  opacity: 0;
  transition: opacity 0.15s;
  flex-shrink: 0;
}

.session-item:hover .delete-btn {
  opacity: 1;
}
</style>
