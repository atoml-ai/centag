/**
 * i18n barrel completeness & locale config checks.
 * Run: npm run test:i18n  (from web/)
 */
import * as fs from 'fs'
import * as path from 'path'
import { fileURLToPath } from 'url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

function assert(cond: unknown, msg: string) {
  if (!cond) throw new Error(msg)
}

const LOCALES = ['en', 'zh-CN', 'ja', 'ko', 'ru', 'es'] as const
const LOCALE_DIR = path.resolve(__dirname, '../locales')

function readBarrelImports(locale: string): string[] {
  const barrel = path.join(LOCALE_DIR, locale, 'index.ts')
  const content = fs.readFileSync(barrel, 'utf-8')
  const imports: string[] = []
  for (const m of content.matchAll(/^import (\w+) from '\.\/(.+\.json)'/gm)) {
    imports.push(m[2])
  }
  return imports
}

function run() {
  // 1. All locale directories exist
  for (const loc of LOCALES) {
    const dir = path.join(LOCALE_DIR, loc)
    assert(fs.existsSync(dir), `locale dir exists: ${loc}`)
    const barrel = path.join(dir, 'index.ts')
    assert(fs.existsSync(barrel), `barrel exists: ${loc}/index.ts`)
  }

  // 2. Barrel files export the same set of JSON modules
  const moduleSets = LOCALES.map((loc) => ({
    loc,
    modules: new Set(readBarrelImports(loc)),
  }))

  const enModules = moduleSets[0].modules
  for (const { loc, modules } of moduleSets.slice(1)) {
    const missing = [...enModules].filter((m) => !modules.has(m))
    const extra = [...modules].filter((m) => !enModules.has(m))
    assert(missing.length === 0, `${loc} missing modules: ${missing.join(', ')}`)
    assert(extra.length === 0, `${loc} extra modules: ${extra.join(', ')}`)
  }

  // 3. Every JSON file referenced by barrel exists and is valid JSON
  for (const loc of LOCALES) {
    const modules = readBarrelImports(loc)
    for (const mod of modules) {
      const jsonPath = path.join(LOCALE_DIR, loc, mod)
      assert(fs.existsSync(jsonPath), `${loc}/${mod} exists`)
      const raw = fs.readFileSync(jsonPath, 'utf-8')
      try {
        JSON.parse(raw)
      } catch {
        assert(false, `${loc}/${mod} is valid JSON`)
      }
    }
  }

  // 4. Critical namespaces exist in en barrel
  const criticalNamespaces = [
    'common.json', 'nav.json', 'route.json', 'dashboard.json', 'settings.json',
    'config.json', 'login.json', 'chat.json', 'backends.json',
  ]
  for (const ns of criticalNamespaces) {
    assert(enModules.has(ns), `en barrel has critical namespace: ${ns}`)
  }

  // 5. supportedLocales & localeLabels consistency (inline check)
  const supportedLocales = ['en', 'zh-CN', 'ja', 'ko', 'ru', 'es']
  const localeLabels: Record<string, string> = {
    'en': 'English',
    'zh-CN': '简体中文',
    'ja': '日本語',
    'ko': '한국어',
    'ru': 'Русский',
    'es': 'Español',
  }
  assert(supportedLocales.length === 6, 'supportedLocales has 6 entries')
  for (const loc of supportedLocales) {
    assert(loc in localeLabels, `localeLabels has entry for ${loc}`)
  }

  console.log('i18n.selftest: OK')
}

run()
