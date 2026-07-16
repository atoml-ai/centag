<template>
  <div class="pipeline-node" :class="nodeTypeClass">
    <div class="node-header">
      <span class="node-type-badge">{{ nodeTypeLabel }}</span>
      <div class="node-actions">
        <el-button
          size="small"
          text
          type="danger"
          @click.stop="deleteNode"
        >
          ✕
        </el-button>
      </div>
    </div>
    
    <div class="node-content">
      <div class="node-name">{{ (node?.data?.name || node?.name || node?.id || '未命名节点') }}</div>
      <div class="node-model" v-if="node?.data?.model || node?.model">
        {{ (node?.data?.backend || node?.backend || '未配置') }} • {{ (node?.data?.model || node?.model) }}
      </div>
      <div class="node-condition" v-if="node?.data?.condition || node?.condition">
        <small>条件: {{ (node?.data?.condition || node?.condition) }}</small>
      </div>
    </div>

    <!-- 输入输出把手 -->
    <Handle 
      type="target" 
      :position="Position.Left" 
      class="custom-handle"
    />
    <Handle 
      type="source" 
      :position="Position.Right" 
      class="custom-handle"
    />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { Handle, Position, useNode } from '@vue-flow/core'
import { ElMessageBox } from 'element-plus'

const { node } = useNode()

const emit = defineEmits(['delete'])

const nodeTypeClass = computed(() => {
  const t = node.value?.data?.type || node.value?.type || 'generator'
  return `type-${t}`
})

const nodeTypeLabel = computed(() => {
  const type = node.value?.data?.type || node.value?.type || 'generator'
  const map: Record<string, string> = {
    generator: '生成',
    processor: '处理',
    reviewer: '审核',
    router: '路由',
    aggregator: '聚合',
    parallel: '并行',
    cache: '缓存',
    token_usage: '计量',
    transparent_forward: '转发',
    tool_call_injector: '注入'
  }
  return map[type] || type
})

const deleteNode = () => {
  ElMessageBox.confirm('确定删除此节点吗？', '提示', {
    type: 'warning'
  }).then(() => {
    emit('delete', node.value?.id)
  }).catch(() => {})
}
</script>

<style scoped>
.pipeline-node {
  width: 176px;
  min-height: 76px;
  max-height: 96px;
  background: white;
  border: 2px solid #409eff;
  border-radius: 10px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
  cursor: pointer;
  transition: all 0.2s;
  position: relative;
  overflow: hidden;
  box-sizing: border-box;
}

.pipeline-node:hover {
  box-shadow: 0 8px 20px rgba(64, 158, 255, 0.2);
  transform: translateY(-2px);
}

.type-generator { border-color: #3b82f6; }
.type-generator .node-type-badge { background: #3b82f6; }
.type-generator .custom-handle { background: #3b82f6; box-shadow: 0 0 0 2px #3b82f6; }

.type-processor { border-color: #10b981; }
.type-processor .node-type-badge { background: #10b981; }
.type-processor .custom-handle { background: #10b981; box-shadow: 0 0 0 2px #10b981; }

.type-reviewer { border-color: #f59e0b; }
.type-reviewer .node-type-badge { background: #f59e0b; }
.type-reviewer .custom-handle { background: #f59e0b; box-shadow: 0 0 0 2px #f59e0b; }

.type-router { border-color: #ef4444; }
.type-router .node-type-badge { background: #ef4444; }
.type-router .custom-handle { background: #ef4444; box-shadow: 0 0 0 2px #ef4444; }

.type-aggregator { border-color: #8b5cf6; }
.type-aggregator .node-type-badge { background: #8b5cf6; }
.type-aggregator .custom-handle { background: #8b5cf6; box-shadow: 0 0 0 2px #8b5cf6; }

.type-parallel { border-color: #06b6d4; }
.type-parallel .node-type-badge { background: #06b6d4; }
.type-parallel .custom-handle { background: #06b6d4; box-shadow: 0 0 0 2px #06b6d4; }

.type-cache { border-color: #0ea5e9; }
.type-cache .node-type-badge { background: #0ea5e9; }
.type-cache .custom-handle { background: #0ea5e9; box-shadow: 0 0 0 2px #0ea5e9; }

.type-token_usage { border-color: #a855f7; }
.type-token_usage .node-type-badge { background: #a855f7; }
.type-token_usage .custom-handle { background: #a855f7; box-shadow: 0 0 0 2px #a855f7; }

.type-transparent_forward { border-color: #14b8a6; }
.type-transparent_forward .node-type-badge { background: #14b8a6; }
.type-transparent_forward .custom-handle { background: #14b8a6; box-shadow: 0 0 0 2px #14b8a6; }

.type-tool_call_injector { border-color: #f97316; }
.type-tool_call_injector .node-type-badge { background: #f97316; }
.type-tool_call_injector .custom-handle { background: #f97316; box-shadow: 0 0 0 2px #f97316; }

.node-header {
  padding: 6px 10px;
  background: #f8f9fa;
  border-bottom: 1px solid #eee;
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 11px;
}

.node-actions {
  display: flex;
  gap: 4px;
}

.node-type-badge {
  padding: 3px 10px;
  border-radius: 10px;
  color: white;
  font-size: 11px;
  font-weight: 600;
}

.node-content {
  padding: 8px 10px;
}

.node-name {
  font-weight: 600;
  font-size: 13px;
  margin-bottom: 4px;
  color: #303133;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.node-model {
  font-size: 11px;
  color: #606266;
  margin-bottom: 2px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.node-condition {
  font-size: 10px;
  color: #e6a23c;
  font-style: italic;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.custom-handle {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  border: 3px solid white;
  transition: transform 0.2s;
}

.custom-handle:hover {
  transform: scale(1.4);
}
</style>
