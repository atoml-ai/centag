import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import * as ElementPlusIconsVue from '@element-plus/icons-vue'
import 'element-plus/dist/index.css'
import '@/assets/styles/variables.css'
import '@/assets/styles/main.css'
import '@/assets/styles/transitions.css'

// Vue Flow 样式
import '@vue-flow/core/dist/style.css'
import '@vue-flow/minimap/dist/style.css'
import '@vue-flow/controls/dist/style.css'
import '@vue-flow/node-resizer/dist/style.css'
import App from './App.vue'
import router from './router'
import i18n from './i18n'
import { setAuthStore } from './api/index'
import { useAuthStore } from '@/stores/auth'

const app = createApp(App)
const pinia = createPinia()

// Register Element Plus icons
for (const [key, component] of Object.entries(ElementPlusIconsVue)) {
  app.component(key, component)
}

app.use(pinia)
app.use(router)
app.use(ElementPlus)
app.use(i18n)

// After Pinia is active, wire the shared axios client to the auth store.
setAuthStore(useAuthStore())

app.mount('#app')
