import { defineStore } from 'pinia'
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import {
  getModelVariables,
  updateModelVariables,
  deleteUserVariable,
  type ModelVariableItem
} from '@/api/model-config'

export const useModelConfigStore = defineStore('modelConfig', () => {
  const systemVariables = ref<ModelVariableItem[]>([])
  const userVariables = ref<ModelVariableItem[]>([])
  const loading = ref(false)
  const saving = ref(false)
  const skipWatch = ref(false)

  const loadConfig = async () => {
    loading.value = true
    skipWatch.value = true
    try {
      const res = await getModelVariables()
      if (res) {
        systemVariables.value = res.system_variables || []
        userVariables.value = res.user_variables || []
      }
    } catch (error) {
      console.error('Failed to load model variables:', error)
    } finally {
      loading.value = false
      skipWatch.value = false
    }
  }

  const saveConfig = async (variables: Record<string, string>) => {
    saving.value = true
    skipWatch.value = true
    try {
      await updateModelVariables(variables)
      await loadConfig()
      ElMessage.success('配置已保存')
    } catch (error) {
      ElMessage.error('保存失败')
      console.error('Failed to save model variables:', error)
    } finally {
      saving.value = false
      skipWatch.value = false
    }
  }

  const addVariable = (name: string, value: string) => {
    userVariables.value.push({
      name,
      value,
      category: 'user'
    })
  }

  const deleteVariable = async (name: string) => {
    try {
      await deleteUserVariable(name)
      userVariables.value = userVariables.value.filter(v => v.name !== name)
      ElMessage.success('变量已删除')
    } catch (error) {
      ElMessage.error('删除失败')
      console.error('Failed to delete variable:', error)
    }
  }

  return {
    systemVariables,
    userVariables,
    loading,
    saving,
    skipWatch,
    loadConfig,
    saveConfig,
    addVariable,
    deleteVariable
  }
})
