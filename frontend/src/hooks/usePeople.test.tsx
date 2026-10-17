import { describe, expect, it, vi } from 'vitest'
import { renderHook } from '@testing-library/react'
import type { Contact } from './useContacts'

const state = { isPersonal: false }
vi.mock('./useWorkspaceType', () => ({
  useWorkspaceTypeOf: () => ({ type: state.isPersonal ? 'personal' : 'organization', isPersonal: state.isPersonal, isLoading: false }),
}))
vi.mock('./useContacts', () => ({
  useContacts: () => ({
    isLoading: false,
    data: {
      total: 1,
      contacts: [{ user_id: 'u1', ngac_node_id: 'n1', username: 'duc', display_name: 'Trần Minh Đức', title: 'Trưởng phòng', department: 'Đối soát' } as Contact],
    },
  }),
}))

const { usePeople } = await import('./usePeople')

describe('usePeople: departments', () => {
  it('names the department in a company workspace', () => {
    state.isPersonal = false
    const { result } = renderHook(() => usePeople('ws-1'))
    expect(result.current.list[0]!.role).toBe('Trưởng phòng · Đối soát')
  })

  it('says nothing about departments in a personal workspace', () => {
    state.isPersonal = true
    const { result } = renderHook(() => usePeople('ws-1'))
    expect(result.current.list[0]!.role).toBe('Trưởng phòng')
  })
})
