import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'

export default defineConfig(({ mode }) => ({
  plugins: [vue()],
  // 生产模式使用 /static/ 路径，开发模式使用根路径
  base: mode === 'development' ? '/' : '/static/',
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src'),
      '@shared': resolve(__dirname, 'shared'),
    }
  },
  server: {
    host: '0.0.0.0',
    port: 5173,
    strictPort: true, // 端口被占用时报错,而不是自动选择下一个端口
    fs: {
      allow: ['.'],
    },
    proxy: {
      '/api': {
        // 本地开发默认连本机后端；生产/公网部署时在对应 env 中设置 VITE_API_BASE_URL
        target: process.env.VITE_API_BASE_URL || 'http://127.0.0.1:20060',
        changeOrigin: true
      },
      '/v1': {
        target: process.env.VITE_API_BASE_URL || 'http://127.0.0.1:20060',
        changeOrigin: true
      },
      '/clash': {
        target: process.env.VITE_API_BASE_URL || 'http://127.0.0.1:20060',
        changeOrigin: true
      }
    }
  },
  build: {
    outDir: '../bin/server/static',
    // 输出在 webui 之外，须显式允许清空，避免残留旧 hash 资源
    emptyOutDir: true,
    assetsDir: 'assets',
    sourcemap: false,
    chunkSizeWarningLimit: 1000,
    rolldownOptions: {
      // element-plus 内嵌 @vueuse/core 的 #__PURE__ 注解位置 Rolldown 无法识别，属无害第三方警告
      onLog(level, log, handler) {
        if (log.code === 'INVALID_ANNOTATION') return
        handler(level, log)
      },
      output: {
        manualChunks(id) {
          if (
            id.includes('node_modules/@vue-flow/core') ||
            id.includes('node_modules/@vue-flow/minimap') ||
            id.includes('node_modules/@vue-flow/controls')
          ) {
            return 'vue-flow'
          }
        },
      },
    },
  },
}))
