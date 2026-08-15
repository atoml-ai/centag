<template>
  <div class="agent-page">
    <div class="agent-sidebar">
      <SessionList
        :sessions="sessions"
        :current-session-id="currentSessionId"
        @select-session="selectSession"
        @delete-session="deleteSession"
        @create-session="createSession"
      />
    </div>
    <div class="agent-main">
      <div class="agent-toolbar">
        <span class="agent-toolbar-title">内置 Agent</span>
        <el-button size="small" @click="openSkillManager">管理 Skills</el-button>
      </div>
      <ChatArea
        :session="currentSession"
        :messages="messages"
        :skills="skills"
        :current-skill="currentSkill"
        :is-responding="isResponding"
        @send-message="sendMessage"
        @cancel-execution="cancelExecution"
      />
      <SkillManager v-model="skillManagerVisible" :skills="skills" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useAgentStore } from '@/stores/agent'
import SessionList from './components/SessionList.vue'
import ChatArea from './components/ChatArea.vue'
import SkillManager from './components/SkillManager.vue'

const agentStore = useAgentStore()

const skillManagerVisible = ref(false)

const openSkillManager = async () => {
  await agentStore.loadSkills()
  skillManagerVisible.value = true
}

const sessions = computed(() => agentStore.sessions)
const skills = computed(() => agentStore.skills)
const currentSessionId = computed(() => agentStore.currentSessionId)
const currentSession = computed(() => agentStore.currentSession)
const messages = computed(() => agentStore.messages)
const currentSkill = computed(() => agentStore.currentSkill)
const isResponding = computed(() => agentStore.isResponding)

const selectSession = async (sessionId: string) => {
  await agentStore.setCurrentSession(sessionId)
}

const deleteSession = async (sessionId: string) => {
  await agentStore.deleteSession(sessionId)
}

const createSession = async () => {
  await agentStore.createSession()
}

// 发送：无会话先创建，再发送
const sendMessage = async (content: string, skill?: string, backendId?: string, model?: string) => {
  if (!currentSessionId.value) {
    await agentStore.createSession(skill)
  }
  await agentStore.sendMessage(content, skill, backendId, model)
}

const cancelExecution = async () => {
  await agentStore.cancelExecution()
}

onMounted(async () => {
  await agentStore.loadSessions()
  await agentStore.loadSkills()
})
</script>

<style scoped>
.agent-page {
  display: flex;
  height: 100vh;
  overflow: hidden;
}

.agent-sidebar {
  width: 280px;
  border-right: 1px solid var(--el-border-color-light);
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
  background: var(--el-fill-color-blank);
}

.agent-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
  background: var(--el-fill-color-light);
}

.agent-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 16px;
  border-bottom: 1px solid var(--el-border-color-light);
  background: var(--el-fill-color-blank);
}

.agent-toolbar-title {
  font-size: 14px;
  font-weight: 500;
}
</style>
