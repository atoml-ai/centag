import api from '@/api'

export interface ModelVariableItem {
  name: string
  value: string
  description?: string
  category: 'system' | 'user'
}

export interface ModelVariablesResponse {
  system_variables: ModelVariableItem[]
  user_variables: ModelVariableItem[]
}

export interface AvailableVariablesResponse {
  system_variables: ModelVariableItem[]
  user_variables: ModelVariableItem[]
  node_variables: { name: string; description: string }[]
}

// 获取所有模型变量
export const getModelVariables = () => {
  return api.get<any, ModelVariablesResponse>('/api/v1/config/model-variables')
}

// 更新模型变量
export const updateModelVariables = (variables: Record<string, string>) => {
  return api.put('/api/v1/config/model-variables', { variables })
}

// 删除用户自定义变量
export const deleteUserVariable = (name: string) => {
  return api.delete(`/api/v1/config/model-variables/${encodeURIComponent(name)}`)
}

// 获取流水线可用变量
export const getAvailableVariables = (pipelineId: string) => {
  return api.get<any, { success: boolean; data: AvailableVariablesResponse }>(
    `/api/v1/pipelines/${pipelineId}/available-variables`
  )
}
