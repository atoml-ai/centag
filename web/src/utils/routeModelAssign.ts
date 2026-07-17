/**
 * Compatibility shim for v0.2.2 「分配模型」.
 * All logic lives in capabilitySlots.ts — do not add business logic here.
 */
export {
  SYSTEM_DEFAULT_BACKEND,
  SYSTEM_DEFAULT_MODEL,
  DEFAULT_ROUTE_MODEL_TARGETS,
  isRouterModePipeline,
  isFollowSystemBinding,
  getRouteModelTargets,
  buildRouteModelRows,
  applyRouteModelAssignments,
  canAssignRouteModels,
  // Preferred names
  resolveCapabilitySlots,
  canConfigureCapabilitySlots,
  buildCapabilitySlotRows,
  applyCapabilitySlotBindings,
  type RouteModelTarget,
  type RouteModelRow,
  type CapabilitySlot,
  type CapabilitySlotRow
} from './capabilitySlots'
