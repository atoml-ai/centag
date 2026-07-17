/**
 * Table-driven unit checks for capabilitySlots (no vitest in repo).
 * Run: npm run test:capability-slots  (from web/)
 */
import type { AgentPatternPipeline } from '../api/pipeline'
import {
  applyAddCategory,
  applyCapabilitySlotBindings,
  buildCapabilitySlotRows,
  canConfigureCapabilitySlots,
  discoverCapabilitySlots,
  recommendCapabilitySlotRows,
  resolveCapabilitySlotsWithSource,
  slugifyNodeId,
  type CapabilitySlotRow
} from './capabilitySlots'

type Pipeline = AgentPatternPipeline

function assert(cond: unknown, msg: string) {
  if (!cond) throw new Error(msg)
}

function assertThrows(fn: () => void, includes: string) {
  let threw = false
  try {
    fn()
  } catch (e: any) {
    threw = true
    assert(String(e?.message || e).includes(includes), `error should include "${includes}", got: ${e}`)
  }
  assert(threw, `expected throw including "${includes}"`)
}

function emptyGlobal() {
  return {
    timeout: 60,
    max_retries: 1,
    bypass_on_error: true,
    stream_mode: false,
    parallel_limit: 1
  }
}

function baseRouterPipeline(): Pipeline {
  return {
    id: 'custom',
    name: 'custom',
    description: '',
    version: '1.0',
    nodes: [
      {
        id: 'r1',
        type: 'router',
        name: 'router',
        config: {
          custom_config: {
            default_route: 'gen-a',
            routes: { hello: 'gen-a', world: 'gen-b' }
          }
        }
      },
      {
        id: 'gen-a',
        type: 'generator',
        name: 'A',
        backend: '{{system.default_backend}}',
        model: '{{system.default_model}}',
        config: { backend: '', model: '' },
        route_config: { router_node_id: 'r1', route_value: 'hello' }
      },
      {
        id: 'gen-b',
        type: 'generator',
        name: 'B',
        backend: 'x',
        model: 'y',
        config: { backend: 'x', model: 'y' },
        route_config: { router_node_id: 'r1', route_value: 'world' }
      },
      {
        id: 'orphan-gen',
        type: 'generator',
        name: 'Orphan',
        config: { backend: '', model: '' }
      }
    ],
    global_config: emptyGlobal()
  }
}

function row(partial: Partial<CapabilitySlotRow> & Pick<CapabilitySlotRow, 'nodeId' | 'label'>): CapabilitySlotRow {
  return {
    slotId: partial.slotId || partial.nodeId,
    nodeId: partial.nodeId,
    label: partial.label,
    hint: partial.hint || '',
    tags: partial.tags || ['default'],
    kind: partial.kind || 'route',
    followSystem: partial.followSystem ?? false,
    backend: partial.backend || '',
    model: partial.model || ''
  }
}

function main() {
  // ── resolve / discover ─────────────────────────────────────────────
  {
    const cases: Array<{ name: string; run: () => void }> = [
      {
        name: 'Discover_WhitelistExcludesOrphan',
        run: () => {
          const p = baseRouterPipeline()
          const ids = discoverCapabilitySlots(p).map((s) => s.node_id)
          assert(ids.includes('gen-a') && ids.includes('gen-b'), 'route targets')
          assert(!ids.includes('orphan-gen'), 'R01 orphan excluded')
          assert(canConfigureCapabilitySlots(p), 'discover ≥2')
        }
      },
      {
        name: 'Discover_SingleSlot_HidesEntry',
        run: () => {
          const p = baseRouterPipeline()
          p.nodes = p.nodes.filter((n) => n.id !== 'gen-b' && n.id !== 'orphan-gen')
          const r1 = p.nodes.find((n) => n.id === 'r1')!
          r1.config.custom_config!.routes = { hello: 'gen-a' }
          assert(discoverCapabilitySlots(p).length === 1, 'one slot')
          assert(!canConfigureCapabilitySlots(p), 'pure discover <2 hides entry')
        }
      },
      {
        name: 'Resolve_CapabilitySlotsPriority',
        run: () => {
          const p = baseRouterPipeline()
          p.metadata = {
            capability_slots: [{ slot_id: 'only', node_id: 'gen-a', label: 'Only A', tags: ['code'] }],
            route_model_targets: [{ node_id: 'gen-b', label: 'ignored' }]
          }
          const { slots, source } = resolveCapabilitySlotsWithSource(p)
          assert(source === 'capability_slots', 'source')
          assert(slots.length === 1 && slots[0].node_id === 'gen-a', 'declared wins')
          assert(canConfigureCapabilitySlots(p), 'declared ≥1')
        }
      },
      {
        name: 'Resolve_RouteModelTargetsMap',
        run: () => {
          const p = baseRouterPipeline()
          p.metadata = { route_model_targets: [{ node_id: 'gen-b', label: 'B branch', hint: 'h' }] }
          const { slots, source } = resolveCapabilitySlotsWithSource(p)
          assert(source === 'route_model_targets', 'source')
          assert(slots.length === 1 && slots[0].slot_id === 'gen-b', 'mapped')
        }
      },
      {
        name: 'Resolve_RouterDefaults',
        run: () => {
          const p = baseRouterPipeline()
          p.id = 'router-mode'
          p.metadata = { aligned_proxy_mode: 'router-mode' }
          const { source, slots } = resolveCapabilitySlotsWithSource(p)
          assert(source === 'router_defaults', 'legacy defaults')
          assert(slots.length >= 4, 'four default targets')
        }
      }
    ]
    for (const c of cases) c.run()
  }

  // ── apply bindings ─────────────────────────────────────────────────
  {
    const p = baseRouterPipeline()
    const next = applyCapabilitySlotBindings(p, [
      row({ nodeId: 'gen-b', label: 'B', followSystem: true })
    ])
    const node = next.nodes.find((n) => n.id === 'gen-b')!
    assert(node.backend === '{{system.default_backend}}', 'follow backend')
    assert(node.model === '{{system.default_model}}', 'follow model')

    assertThrows(
      () =>
        applyCapabilitySlotBindings(p, [
          row({ nodeId: 'missing', label: 'X', followSystem: true })
        ]),
      '找不到节点'
    )

    assertThrows(
      () =>
        applyCapabilitySlotBindings(p, [
          row({ nodeId: 'gen-b', label: 'B', followSystem: false, backend: '', model: 'm' })
        ]),
      '后端'
    )

    assertThrows(
      () =>
        applyCapabilitySlotBindings(p, [
          row({ nodeId: 'gen-b', label: 'B', followSystem: false, backend: 'b', model: '' })
        ]),
      '模型'
    )

    const specified = applyCapabilitySlotBindings(p, [
      row({ nodeId: 'gen-a', label: 'A', followSystem: false, backend: 'openai', model: 'gpt-4o' })
    ])
    const a = specified.nodes.find((n) => n.id === 'gen-a')!
    assert(a.backend === 'openai' && a.config.backend === 'openai', 'config sync')
    assert(a.model === 'gpt-4o' && a.config.model === 'gpt-4o', 'model sync')
  }

  // ── recommend (draft only) ─────────────────────────────────────────
  {
    const rows = [
      row({ nodeId: 'c', label: 'Code', tags: ['code'], followSystem: true }),
      row({ nodeId: 'd', label: 'Default', tags: ['default'], followSystem: false, backend: 'x', model: 'y' })
    ]
    const empty = recommendCapabilitySlotRows(rows, [])
    assert(!!empty.warned, 'warn without backends')
    assert(empty.rows[0].followSystem === true, 'unchanged when no backends')

    const noModels = recommendCapabilitySlotRows(rows, [{ id: 'b1', enabled: true, supported_models: [] }])
    assert(!!noModels.warned, 'warn without models')

    const backends = [
      {
        id: 'cheap-be',
        enabled: true,
        supported_models: ['flash-lite', 'mini-model']
      },
      {
        id: 'code-be',
        enabled: true,
        supported_models: ['deepseek-coder', 'gpt-4o']
      },
      {
        id: 'reason-be',
        enabled: true,
        supported_models: ['o1-preview', 'o3-mini']
      },
      {
        id: 'off',
        enabled: false,
        supported_models: ['should-ignore']
      }
    ]
    const rec = recommendCapabilitySlotRows(rows, backends)
    assert(!rec.warned, 'no warn')
    assert(rec.rows[0].followSystem === false, 'code not follow')
    assert(rec.rows[0].model.toLowerCase().includes('coder') || rec.rows[0].backend === 'code-be', 'code preference')
    assert(rec.rows[1].followSystem === true, 'default tag → follow system')
    // Original rows not mutated
    assert(rows[0].followSystem === true, 'recommend must not mutate input rows')

    const tagRows = [
      row({ nodeId: 'cheap', label: 'Cheap', tags: ['cheap'], followSystem: false, backend: 'x', model: 'y' }),
      row({ nodeId: 'reason', label: 'Reason', tags: ['reasoning'], followSystem: false, backend: 'x', model: 'y' })
    ]
    // Isolate fixtures so gpt-4o (also scores high on reasoning) does not mask o1
    const tagBackends = [
      { id: 'cheap-be', enabled: true, supported_models: ['flash-lite', 'mini-model'] },
      { id: 'reason-be', enabled: true, supported_models: ['o1-preview', 'o3-mini'] },
      { id: 'plain-be', enabled: true, supported_models: ['generic-chat'] }
    ]
    const tagRec = recommendCapabilitySlotRows(tagRows, tagBackends)
    assert(
      tagRec.rows[0].backend === 'cheap-be' || /lite|mini|flash/i.test(tagRec.rows[0].model),
      'cheap tag preference'
    )
    assert(
      tagRec.rows[1].backend === 'reason-be' || /o1|o3/i.test(tagRec.rows[1].model),
      'reasoning tag preference'
    )
  }

  // ── add category ───────────────────────────────────────────────────
  {
    assertThrows(() => applyAddCategory(baseRouterPipeline(), { label: '', keywords: ['a'] }), '分类名称')
    assertThrows(() => applyAddCategory(baseRouterPipeline(), { label: 'X', keywords: [] }), '关键词')
    assertThrows(
      () =>
        applyAddCategory(
          {
            id: 'n',
            name: 'n',
            description: '',
            version: '1',
            nodes: [{ id: 'g', type: 'generator', name: 'g', config: { backend: '', model: '' } }],
            global_config: emptyGlobal()
          },
          { label: 'X', keywords: ['k'] }
        ),
      '路由节点'
    )

    const p = baseRouterPipeline()
    assertThrows(
      () => applyAddCategory(p, { label: 'Dup', keywords: ['d'], nodeId: 'gen-a' }),
      '已存在'
    )

    const beforeRoutes = { ...(p.nodes.find((n) => n.id === 'r1')!.config.custom_config!.routes as Record<string, string>) }
    const next = applyAddCategory(p, {
      label: '翻译专线',
      keywords: ['翻译', 'translate'],
      tags: ['multilingual'],
      isDefault: true
    })
    // Pure function: source pipeline routes unchanged
    const srcRoutes = p.nodes.find((n) => n.id === 'r1')!.config.custom_config!.routes as Record<string, string>
    assert(!srcRoutes['翻译'] && srcRoutes['hello'] === beforeRoutes['hello'], 'addCategory must not mutate source')

    const gen = next.nodes.find((n) => n.name === '翻译专线')!
    assert(!!gen, 'generator')
    const router = next.nodes.find((n) => n.id === 'r1')!
    const routes = router.config.custom_config!.routes as Record<string, string>
    assert(routes['翻译'] === gen.id && routes['translate'] === gen.id, 'routes')
    assert(router.config.custom_config!.default_route === gen.id, 'default_route')
    assert(gen.route_config?.router_node_id === 'r1', 'route_config')
    assert(gen.route_config?.route_value === '翻译', 'route_value')
    assert(gen.route_config?.is_default === true, 'is_default')
    assert((next.metadata?.capability_slots || []).some((s: any) => s.node_id === gen.id), 'slot')

    // After add, configure entry still works
    assert(canConfigureCapabilitySlots(next), 'still configurable')
    const built = buildCapabilitySlotRows(next)
    assert(built.some((r) => r.nodeId === gen.id), 'rows include new category')

    // Keyword collision on existing route key
    assertThrows(
      () => applyAddCategory(p, { label: 'Clash', keywords: ['hello'] }),
      '已存在'
    )
  }

  // ── slugify ────────────────────────────────────────────────────────
  {
    assert(slugifyNodeId('Hello World') === 'hello-world', 'ascii slug')
    assert(!!slugifyNodeId(''), 'empty fallback')
  }

  console.log('capabilitySlots.selftest: OK')
}

main()
