import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { ElMessage } from 'element-plus'
import { agentApi, Session, Message, Skill } from '@/api/agent'

export const useAgentStore = defineStore('agent', () => {
  const sessions = ref<Session[]>([])
  const currentSessionId = ref<string | null>(null)
  const messages = ref<Message[]>([])
  const skills = ref<Skill[]>([])
  const currentSkill = ref<string | null>(null)
  const isLoading = ref(false)

  const currentSession = computed(() => {
    if (!currentSessionId.value) return null
    return (sessions.value || []).find(s => s.id === currentSessionId.value) || null
  })

  const loadSessions = async () => {
    try {
      isLoading.value = true
      const list = await agentApi.listSessions()
      sessions.value = Array.isArray(list) ? list : []
    } catch (error) {
      console.error('Failed to load sessions:', error)
    } finally {
      isLoading.value = false
    }
  }

  const loadSkills = async () => {
    try {
      const list = await agentApi.listSkills()
      skills.value = Array.isArray(list) ? list : []
    } catch (error) {
      console.error('Failed to load skills:', error)
    }
  }

  const setCurrentSession = async (sessionId: string) => {
    currentSessionId.value = sessionId
    await loadMessages(sessionId)
  }

  const loadMessages = async (sessionId: string) => {
    try {
      const list = await agentApi.listMessages(sessionId)
      messages.value = Array.isArray(list) ? list : []
    } catch (error) {
      console.error('Failed to load messages:', error)
    }
  }

  const createSession = async (skill?: string) => {
    try {
      const session = await agentApi.createSession({ skill })
      sessions.value.unshift(session)
      currentSessionId.value = session.id
      messages.value = []
      if (skill) {
        currentSkill.value = skill
      }
    } catch (error) {
      console.error('Failed to create session:', error)
    }
  }

  const deleteSession = async (sessionId: string) => {
    try {
      await agentApi.deleteSession(sessionId)
      sessions.value = sessions.value.filter(s => s.id !== sessionId)
      if (currentSessionId.value === sessionId) {
        currentSessionId.value = null
        messages.value = []
      }
    } catch (error) {
      console.error('Failed to delete session:', error)
    }
  }

  const setCurrentSkill = (skill: string) => {
    currentSkill.value = skill
  }

  // 任务 7/11：自定义 skill CRUD，成功后刷新列表
  const createSkill = async (form: { name: string; description?: string; category?: string; tools?: string[]; steps?: string[]; system_prompt?: string }) => {
    const result = await agentApi.createSkill(form)
    await loadSkills()
    return result
  }

  const updateSkill = async (name: string, form: { name: string; description?: string; category?: string; tools?: string[]; steps?: string[]; system_prompt?: string }) => {
    const result = await agentApi.updateSkill(name, form)
    await loadSkills()
    return result
  }

  const deleteSkill = async (name: string) => {
    await agentApi.deleteSkill(name)
    await loadSkills()
  }

  const cloneSkill = async (name: string, targetName?: string) => {
    const result = await agentApi.cloneSkill(name, targetName)
    await loadSkills()
    return result
  }

  const isResponding = ref(false)

  const sendMessage = async (
    content: string,
    skill?: string,
    backendId?: string,
    model?: string
  ) => {
    if (!currentSessionId.value || isResponding.value) return

    const sid = currentSessionId.value
    // 本地立即追加用户消息（乐观 UI）
    messages.value.push({
      id: `local-${Date.now()}`,
      session_id: sid,
      role: 'user',
      content,
      skill: skill || '',
      created_at: new Date().toISOString()
    })

    isResponding.value = true
    try {
      if (skill) currentSkill.value = skill
      await agentApi.sendMessage(sid, {
        content,
        skill: skill || undefined,
        backend_id: backendId || undefined,
        model: model || undefined
      })
      // 发送成功后从后端刷新完整历史（含 assistant 回复）
      const list = await agentApi.listMessages(sid)
      messages.value = Array.isArray(list) ? list : []
    } catch (error) {
      console.error('Failed to send message:', error)
      ElMessage.error('Agent 请求失败或超时，请稍后重试')
    } finally {
      isResponding.value = false
    }
  }

  const cancelExecution = async () => {
    if (!currentSessionId.value) return

    try {
      await agentApi.cancelExecution(currentSessionId.value)
    } catch (error) {
      console.error('Failed to cancel execution:', error)
    }
  }

  return {
    sessions,
    currentSessionId,
    messages,
    skills,
    currentSkill,
    isLoading,
    isResponding,
    currentSession,
    loadSessions,
    loadSkills,
    setCurrentSession,
    createSession,
    deleteSession,
    setCurrentSkill,
    createSkill,
    updateSkill,
    deleteSkill,
    cloneSkill,
    sendMessage,
    cancelExecution
  }
})