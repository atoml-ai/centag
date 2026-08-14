import api from '@/api'

export interface Session {
  id: string
  user_id: number
  tenant_id: string
  title: string
  skill: string
  backend_id: string
  model: string
  status: string
  created_at: string
  updated_at: string
}

export interface Message {
  id: string
  session_id: string
  role: string
  content: string
  skill: string
  tool_name?: string
  created_at: string
}

export interface Skill {
  name: string
  description: string
  version: string
  category: string
  tools: string[]
  steps: string[]
  enabled: boolean
  internal: boolean
}

export const agentApi = {
  // 健康检查
  async health() {
    return await api.get('/api/v1/builtin-agent/health')
  },

  // 创建会话
  async createSession(data: { skill?: string; backend_id?: string; model?: string }) {
    return (await api.post('/api/v1/builtin-agent/sessions', data)) as Session
  },

  // 获取会话列表
  async listSessions() {
    const data = (await api.get('/api/v1/builtin-agent/sessions')) as { sessions: Session[] }
    return data.sessions as Session[]
  },

  // 获取会话详情
  async getSession(sessionId: string) {
    return (await api.get(`/api/v1/builtin-agent/sessions/${sessionId}`)) as Session
  },

  // 删除会话
  async deleteSession(sessionId: string) {
    return await api.delete(`/api/v1/builtin-agent/sessions/${sessionId}`)
  },

  // 发送消息
  async sendMessage(
    sessionId: string,
    data: { content: string; skill?: string; backend_id?: string; model?: string }
  ) {
    // agent 多轮推理耗时长，单独放宽超时
    return (await api.post(`/api/v1/builtin-agent/sessions/${sessionId}/messages`, data, {
      timeout: 300000
    })) as Message
  },

  // 获取消息历史
  async listMessages(sessionId: string) {
    const data = (await api.get(`/api/v1/builtin-agent/sessions/${sessionId}/messages`)) as {
      messages: Message[]
    }
    return data.messages as Message[]
  },

  // 获取可用Skills
  async listSkills() {
    const data = (await api.get('/api/v1/builtin-agent/skills')) as { skills: Skill[] }
    return data.skills as Skill[]
  },

  // 确认工具执行
  async confirmTool(sessionId: string, data: { confirm: boolean; tool_id: string }) {
    return await api.post(`/api/v1/builtin-agent/sessions/${sessionId}/confirm`, data)
  },

  // 取消执行
  async cancelExecution(sessionId: string) {
    return await api.post(`/api/v1/builtin-agent/sessions/${sessionId}/cancel`)
  }
}
