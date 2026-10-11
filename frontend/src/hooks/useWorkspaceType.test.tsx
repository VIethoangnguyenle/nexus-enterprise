import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook } from '@testing-library/react'

let active: { workspaceId: string; workspaces: { id: string; name: string; type?: string }[]; isLoading: boolean }
vi.mock('./useActiveWorkspace', () => ({ useActiveWorkspace: () => active }))

const { useWorkspaceType } = await import('./useWorkspaceType')

describe('useWorkspaceType', () => {
  beforeEach(() => {
    active = {
      workspaceId: 'ws-1',
      workspaces: [
        { id: 'ws-1', name: 'Lan', type: 'personal' },
        { id: 'ws-2', name: 'Kế toán', type: 'organization' },
      ],
      isLoading: false,
    }
  })

  it('reports the active workspace’s type', () => {
    expect(renderHook(() => useWorkspaceType()).result.current).toEqual({ type: 'personal', isPersonal: true, isLoading: false })
    active.workspaceId = 'ws-2'
    expect(renderHook(() => useWorkspaceType()).result.current.isPersonal).toBe(false)
  })

  it('claims nothing while the workspace is unknown', () => {
    active = { workspaceId: '', workspaces: [], isLoading: true }
    expect(renderHook(() => useWorkspaceType()).result.current).toEqual({ type: undefined, isPersonal: false, isLoading: true })
  })

  it('treats a type it does not know as unknown, never as personal', () => {
    active.workspaces = [{ id: 'ws-1', name: 'Lan', type: 'team' }]
    expect(renderHook(() => useWorkspaceType()).result.current.type).toBeUndefined()
  })
})
