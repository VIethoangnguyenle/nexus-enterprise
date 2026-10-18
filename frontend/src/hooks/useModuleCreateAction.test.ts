import '@testing-library/jest-dom/vitest'
import { cleanup, renderHook } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { useModuleCreateAction, useRegisterCreateAction, clearCreateActions, type CreateAction } from './useModuleCreateAction'

vi.mock('@tanstack/react-router', () => ({
  useLocation: ({ select }: { select?: (l: { pathname: string }) => unknown } = {}) => {
    const loc = { pathname: '/drive' }
    return select ? select(loc) : loc
  },
}))

vi.mock('../lib/nav-destinations', async (orig) => ({
  ...(await orig<typeof import('../lib/nav-destinations')>()),
  moduleOfPath: (p: string) => {
    if (p.includes('/drive')) return 'drive'
    if (p.includes('/approval')) return 'approval'
    if (p.includes('/channels')) return 'messaging'
    if (p.includes('/contacts')) return 'contacts'
    return undefined
  },
}))

afterEach(() => {
  cleanup()
  clearCreateActions()
  vi.restoreAllMocks()
})

describe('useModuleCreateAction', () => {
  it('returns null when no action is registered for the active module', () => {
    const { result } = renderHook(() => useModuleCreateAction())
    expect(result.current).toBeNull()
  })

  it('returns the registered action for the active module', () => {
    const handler = vi.fn()
    const { unmount } = renderHook(() =>
      useRegisterCreateAction('drive', { label: 'Tạo tài liệu', handler }),
    )
    const { result } = renderHook(() => useModuleCreateAction())
    expect(result.current).toEqual({ label: 'Tạo tài liệu', handler })
    unmount()
  })

  it('unregisters the action on unmount', () => {
    const handler = vi.fn()
    const { unmount } = renderHook(() =>
      useRegisterCreateAction('drive', { label: 'Tạo tài liệu', handler }),
    )
    unmount()
    const { result } = renderHook(() => useModuleCreateAction())
    expect(result.current).toBeNull()
  })

  it('returns null for a module that has no registered action', () => {
    const handler = vi.fn()
    renderHook(() =>
      useRegisterCreateAction('approval', { label: 'Tạo đề nghị', handler }),
    )
    // drive is active (from the mock), approval is registered → null for drive
    const { result } = renderHook(() => useModuleCreateAction())
    expect(result.current).toBeNull()
  })

  it('supports multiple modules registering simultaneously', () => {
    const driveHandler = vi.fn()
    const approvalHandler = vi.fn()
    renderHook(() =>
      useRegisterCreateAction('drive', { label: 'Tạo tài liệu', handler: driveHandler }),
    )
    renderHook(() =>
      useRegisterCreateAction('approval', { label: 'Tạo đề nghị', handler: approvalHandler }),
    )
    const { result } = renderHook(() => useModuleCreateAction())
    // drive is the active module
    expect(result.current).toEqual({ label: 'Tạo tài liệu', handler: driveHandler })
  })

  it('updates when the registered action changes', () => {
    const handler1 = vi.fn()
    const handler2 = vi.fn()
    const { rerender } = renderHook(
      ({ action }) => useRegisterCreateAction('drive', action),
      { initialProps: { action: { label: 'Label 1', handler: handler1 } } },
    )
    const { result } = renderHook(() => useModuleCreateAction())
    expect(result.current?.label).toBe('Label 1')
    rerender({ action: { label: 'Label 2', handler: handler2 } })
    expect(result.current?.label).toBe('Label 2')
  })

  it('unregisters when null is passed (conditional registration)', () => {
    const handler = vi.fn()
    const { rerender } = renderHook(
      ({ action }) => useRegisterCreateAction('drive', action),
      { initialProps: { action: { label: 'Thêm mới', handler } as CreateAction | null } },
    )
    const { result } = renderHook(() => useModuleCreateAction())
    expect(result.current).toEqual({ label: 'Thêm mới', handler })
    rerender({ action: null })
    expect(result.current).toBeNull()
  })
})
