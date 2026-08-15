import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { existsSync } from 'fs'
import { homedir } from 'os'
import { resolve, dirname, join } from 'path'
import { fileURLToPath } from 'url'

const __dirname = dirname(fileURLToPath(import.meta.url))

/**
 * E3: Team pack — only Team builds (build-web-team.sh sets CENTAG_EDITION=team)
 * may embed the commercial pack. A stale web/.centag-team-pack left by a previous
 * team build must NOT leak into personal/minimal builds, otherwise the team-only
 * pages (计量计费/套餐与对账) silently appear in those editions.
 */
function resolveTeamPackDir(): string {
  const isTeamBuild = process.env.CENTAG_EDITION === 'team' || !!process.env.CENTAG_TEAM_PACK
  if (isTeamBuild) {
    const linked = resolve(__dirname, '.centag-team-pack')
    if (existsSync(linked)) return linked
    if (process.env.CENTAG_TEAM_PACK) return resolve(process.env.CENTAG_TEAM_PACK)
  }
  return resolve(__dirname, 'src/packs/team-stub')
}

/** Align with scripts/install.sh / scripts/lib/centag-layout.sh (default ~/.centag). */
function resolveStaticOutDir(): string {
  if (process.env.CENTAG_STATIC_DIR) {
    return resolve(process.env.CENTAG_STATIC_DIR)
  }
  const root = process.env.CENTAG_INSTALL_ROOT || join(homedir(), '.centag')
  const edition = process.env.CENTAG_EDITION || 'personal'
  return join(root, 'lib', edition, 'static')
}

const teamPackDir = resolveTeamPackDir()
const staticOutDir = resolveStaticOutDir()

export default defineConfig(({ mode }) => ({
  plugins: [vue()],
  // 生产模式使用 /static/ 路径，开发模式使用根路径
  base: mode === 'development' ? '/' : '/static/',
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src'),
      '@shared': resolve(__dirname, 'shared'),
      '@team-pack': teamPackDir,
    }
  },
  server: {
    host: '0.0.0.0',
    port: 5173,
    strictPort: true, // 端口被占用时报错,而不是自动选择下一个端口
    fs: {
      // Allow reading team pack outside web/ when CENTAG_TEAM_PACK is set.
      allow: ['.', teamPackDir, resolve(__dirname, '..')],
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
    outDir: staticOutDir,
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
