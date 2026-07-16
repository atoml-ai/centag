import { defineStore } from 'pinia'

export const useAppStore = defineStore('app', {
  state: () => ({
    // 当前主导航
    currentNav: 'dashboard',
    // 全局 loading
    loading: false,
    // 版本信息
    version: '',
    // 主题（light/dark）
    theme: 'light'
  }),

  getters: {
    isDarkMode: (state) => state.theme === 'dark'
  },

  actions: {
    // 设置当前导航
    setCurrentNav(navId: string) {
      this.currentNav = navId
    },

    // 设置 loading
    setLoading(loading: boolean) {
      this.loading = loading
    },

    // 设置版本信息
    setVersion(version: string) {
      this.version = version
    },

    // 切换主题
    toggleTheme() {
      this.theme = this.theme === 'light' ? 'dark' : 'light'
      localStorage.setItem('theme', this.theme)
      document.documentElement.setAttribute('data-theme', this.theme)
    },

    // 初始化主题
    initTheme() {
      const savedTheme = localStorage.getItem('theme') || 'light'
      this.theme = savedTheme
      document.documentElement.setAttribute('data-theme', savedTheme)
    }
  }
})
