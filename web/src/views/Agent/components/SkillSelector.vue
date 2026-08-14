<template>
  <div class="skill-selector">
    <div class="skill-header">
      <h3>可用Skills</h3>
    </div>
    <div class="skill-items">
      <div
        v-for="skill in skills"
        :key="skill.name"
        class="skill-item"
        :class="{ active: skill.name === currentSkill }"
        @click="$emit('select-skill', skill.name)"
      >
        <div class="skill-icon" :class="getSkillIconClass(skill.category)">
          <i :class="getSkillIcon(skill.category)"></i>
        </div>
        <div class="skill-info">
          <div class="skill-name">{{ skill.name }}</div>
          <div class="skill-description">{{ skill.description }}</div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
interface Skill {
  name: string
  description: string
  category: string
}

defineProps<{
  skills: Skill[]
  currentSkill: string | null
}>()

defineEmits<{
  'select-skill': [skill: string]
}>()

const getSkillIconClass = (category: string) => {
  switch (category) {
    case '运维诊断':
      return 'icon-diagnosis'
    case '配置管理':
      return 'icon-config'
    case '策略优化':
      return 'icon-strategy'
    default:
      return 'icon-default'
  }
}

const getSkillIcon = (category: string) => {
  switch (category) {
    case '运维诊断':
      return 'icon-stethoscope'
    case '配置管理':
      return 'icon-cog'
    case '策略优化':
      return 'icon-chart'
    default:
      return 'icon-default'
  }
}
</script>

<style scoped>
.skill-selector {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.skill-header {
  padding: 16px;
  border-bottom: 1px solid #e0e0e0;
}

.skill-header h3 {
  margin: 0;
  font-size: 16px;
}

.skill-items {
  flex: 1;
  overflow-y: auto;
}

.skill-item {
  display: flex;
  align-items: center;
  padding: 12px 16px;
  border-bottom: 1px solid #f0f0f0;
  cursor: pointer;
}

.skill-item:hover {
  background: #f8f8f8;
}

.skill-item.active {
  background: #e8f4fd;
}

.skill-icon {
  width: 32px;
  height: 32px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  margin-right: 12px;
}

.icon-diagnosis {
  background: #e3f2fd;
  color: #1976d2;
}

.icon-config {
  background: #f3e5f5;
  color: #7b1fa2;
}

.icon-strategy {
  background: #e8f5e9;
  color: #388e3c;
}

.icon-default {
  background: #f5f5f5;
  color: #757575;
}

.skill-info {
  flex: 1;
}

.skill-name {
  font-size: 14px;
  font-weight: 500;
}

.skill-description {
  font-size: 12px;
  color: #666;
  margin-top: 4px;
}
</style>