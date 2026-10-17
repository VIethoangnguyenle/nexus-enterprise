import { renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { adminApi } from '../api/admin'
import { contactsApi } from '../api/contacts'
import { messagingApi } from '../api/messaging'
import { clearResolvers } from '../lib/people-resolver'
import type { Contact } from './useContacts'
import { useDepartmentTree, useGroupMembers, useMyGroups, useRoleGroups } from './useContactCategories'

const id = (n: number | string) => `00000000-0000-4000-8000-${String(n).padStart(12, '0')}`
const state = { contacts: [] as Contact[], loading: false, failed: false }
vi.mock('./useContacts', () => ({
  useContacts: () => ({
    isLoading: state.loading, isError: state.failed,
    data: state.loading || state.failed ? undefined : { contacts: state.contacts, total: state.contacts.length, visibility: 'all' },
  }),
}))
vi.mock('./useWorkspaceType', () => ({
  useWorkspaceTypeOf: () => ({ type: 'organization', isPersonal: false, isLoading: false }),
}))

const person = (id: string, name: string, over: Partial<Contact> = {}): Contact => ({
  user_id: id, ngac_node_id: `node-${id}`, username: id, display_name: name, email: '', title: '', department: '',
  department_id: '', roles: [], location: '', avatar_url: '', is_online: false, ...over,
})

let client: QueryClient
const wrapper = ({ children }: { children: ReactNode }) => <QueryClientProvider client={client}>{children}</QueryClientProvider>
const idle = () => new Promise((r) => setTimeout(r, 30))

beforeEach(() => {
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  clearResolvers()
  vi.restoreAllMocks()
  state.contacts = []
  state.loading = false
  state.failed = false
})

describe('Phòng ban', () => {
  it('loads the departments only when the category is open, and builds the tree with the loaded people', async () => {
    state.contacts = [person('a', 'An', { department_id: 'd1' })]
    const list = vi.spyOn(adminApi, 'listDepartments').mockResolvedValue({
      departments: [{ id: 'd1', name: 'Đối soát', parent_id: '', member_count: 1 }],
    })
    const { result, rerender } = renderHook(({ on }) => useDepartmentTree('w1', on), { wrapper, initialProps: { on: false } })
    await idle()
    expect(list).not.toHaveBeenCalled()
    expect(result.current.tree).toBeUndefined()

    rerender({ on: true })
    await waitFor(() => expect(result.current.tree).toBeDefined())
    expect(list).toHaveBeenCalledTimes(1)
    expect(result.current.tree!.roots[0]!.peopleCount).toBe(1)
  })

  it('says so when the departments cannot be loaded', async () => {
    vi.spyOn(adminApi, 'listDepartments').mockRejectedValue(new Error('boom'))
    const { result } = renderHook(() => useDepartmentTree('w1', true), { wrapper })
    await waitFor(() => expect(result.current.isError).toBe(true))
    expect(result.current.tree).toBeUndefined()
  })
})

describe('Vai trò', () => {
  it('come from the people already loaded: no request of their own', () => {
    const list = vi.spyOn(adminApi, 'listRoles')
    state.contacts = [person('a', 'An', { roles: [{ id: 'r', name: 'Kế toán', kind: 'custom' }] })]
    const { result } = renderHook(() => useRoleGroups('w1'), { wrapper })
    expect(result.current.groups.map((g) => g.name)).toEqual(['Kế toán'])
    expect(list).not.toHaveBeenCalled()
  })
})

describe('Nhóm của tôi', () => {
  const channel = (id: string, channel_type: string) => ({ id, name: id, channel_type, workspace_id: 'w1' })

  it('loads only when open, and lists the private groups: not #general, not a direct message', async () => {
    const list = vi.spyOn(messagingApi, 'listChannels').mockResolvedValue({
      channels: [channel('general', 'workspace'), channel('nhom-a', 'private'), channel('dm', 'dm')],
    })
    const { result, rerender } = renderHook(({ on }) => useMyGroups('w1', on), { wrapper, initialProps: { on: false } })
    await idle()
    expect(list).not.toHaveBeenCalled()
    rerender({ on: true })
    await waitFor(() => expect(result.current.groups.map((g) => g.id)).toEqual(['nhom-a']))
  })
})

describe('members of a group', () => {
  it('are not loaded until a group is opened', async () => {
    const list = vi.spyOn(messagingApi, 'listMembers')
    renderHook(() => useGroupMembers('w1', null), { wrapper })
    await idle()
    expect(list).not.toHaveBeenCalled()
  })

  it('are named, hidden ones included, by given name; a hidden member is marked not visible', async () => {
    state.contacts = [person(id(1), 'Nguyễn Văn An'), person(id(2), 'Trần Thị Bình')]
    vi.spyOn(messagingApi, 'listMembers').mockResolvedValue({
      members: [
        { user_id: id(2), username: 'binh', ngac_node_id: 'node-u-binh' },
        { user_id: id(3), username: 'hidden', ngac_node_id: 'node-hidden' },
        { user_id: id(1), username: 'an', ngac_node_id: 'node-u-an' },
      ],
      can_remove_members: true,
    })
    vi.spyOn(contactsApi, 'resolve').mockResolvedValue({
      people: [{ user_id: id(3), ngac_node_id: 'node-hidden', display_name: 'Võ Xuân', avatar_url: '' }],
    })
    const { result } = renderHook(() => useGroupMembers('w1', 'g1'), { wrapper })
    await waitFor(() => expect(result.current.members.map((m) => m.name)).toEqual(['Nguyễn Văn An', 'Trần Thị Bình', 'Võ Xuân']))
    expect(result.current.members.map((m) => m.visible)).toEqual([true, true, false])
  })

  it.each([['are still loading', 'loading'], ['failed to load', 'failed']] as const)(
    'are never called hidden while the directory %s',
    async (_t, flag) => {
      state[flag] = true
      vi.spyOn(messagingApi, 'listMembers').mockResolvedValue({
        members: [{ user_id: id(1), username: 'an', ngac_node_id: 'node-an' }],
      })
      const resolve = vi.spyOn(contactsApi, 'resolve')
      const { result } = renderHook(() => useGroupMembers('w1', 'g1'), { wrapper })
      await waitFor(() => expect(result.current.members).toHaveLength(1))
      expect(result.current.members[0]!.visible).toBeUndefined()
      expect(resolve).not.toHaveBeenCalled()
    },
  )

  it('tolerates a group with no members', async () => {
    vi.spyOn(messagingApi, 'listMembers').mockResolvedValue({ members: null })
    const { result } = renderHook(() => useGroupMembers('w1', 'g1'), { wrapper })
    await waitFor(() => expect(result.current.isLoading).toBe(false))
    expect(result.current.members).toEqual([])
  })
})
