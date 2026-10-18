import { useEffect } from 'react'
import { useLocation } from '@tanstack/react-router'
import { useSyncExternalStore } from 'react'
import { moduleOfPath, type ModuleId } from '../lib/nav-destinations'

export interface CreateAction {
  label: string
  handler: () => void
}

/**
 * A module-keyed registry of create actions. Each module registers its action
 * on mount (via `useRegisterCreateAction`) and the `+` button on ModuleBar
 * reads the one for the active module (via `useModuleCreateAction`).
 *
 * The store is a simple map with a subscription list, outside React state,
 * so any module's mount/unmount updates every subscriber without prop drilling.
 */
const registry = new Map<ModuleId, CreateAction>()
const listeners = new Set<() => void>()
let version = 0

function emit() {
  version++
  listeners.forEach((fn) => fn())
}

function subscribe(cb: () => void) {
  listeners.add(cb)
  return () => listeners.delete(cb)
}

function snapshot() {
  return version
}

/** Register a create action for a module. Call from inside the module's route component. Pass `null` to unregister. */
export function useRegisterCreateAction(moduleId: ModuleId, action: CreateAction | null): void {
  useEffect(() => {
    if (!action) {
      // If we had a previous registration, clean it up.
      if (registry.has(moduleId)) {
        registry.delete(moduleId)
        emit()
      }
      return
    }
    registry.set(moduleId, action)
    emit()
    return () => {
      // Only delete if we're still the one registered (a re-render may have
      // set a new action before our cleanup runs).
      if (registry.get(moduleId) === action) {
        registry.delete(moduleId)
        emit()
      }
    }
  }, [moduleId, action])
}

/** Read the create action for the currently active module. Returns `null` when the module has none. */
export function useModuleCreateAction(): CreateAction | null {
  const pathname = useLocation({ select: (l) => l.pathname })
  const moduleId = moduleOfPath(pathname)
  // Subscribe to registry changes so we re-render when a module registers/unregisters.
  useSyncExternalStore(subscribe, snapshot, () => 0)
  if (!moduleId) return null
  return registry.get(moduleId) ?? null
}

/** For tests: clear all registered actions. */
export function clearCreateActions(): void {
  registry.clear()
  emit()
}
