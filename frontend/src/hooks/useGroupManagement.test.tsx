import { renderHook, waitFor, act } from '@testing-library/react'
import { QueryClientProvider } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '../api/client'
import { groupManagementApi, parseGroupManagement } from '../api/group-management'
import { seedGroup, seedWorkspaceAdmin } from '../components/spaces/group-management/seed-group'
import { queryClient } from '../lib/query-client'
import { useAuthStore } from '../stores/auth.store'
import { CH, ME, U } from '../test/chat-fixtures'
import { groupManagementKeys } from './keys/group-management'
import { keys } from './keys'
import { groupActionError, PERMISSION_LOST, useGroupRole, useUpdateGroupSettings } from './useGroupManagement'

const wrapper = ({ children }: { children: ReactNode }) => (
  <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
)

beforeEach(() => {
  useAuthStore.setState({ user: ME, accessToken: 't', bootstrapping: false })
  queryClient.setDefaultOptions({ queries: { retry: false, staleTime: Infinity } })
})
afterEach(() => {
  queryClient.clear()
  vi.restoreAllMocks()
})

const role = () => renderHook(() => useGroupRole(CH.doisoat), { wrapper }).result

describe('useGroupRole in a group', () => {
  it('names the owner, managers and members from the management record', () => {
    seedGroup(CH.doisoat, 'private', { owner_id: U.duc.id, manager_ids: [U.duc.id, U.lan.id] })
    const r = role()
    expect(r.current.roleOf(U.duc.id)).toBe('owner')
    expect(r.current.roleOf(U.lan.id)).toBe('manager')
    expect(r.current.roleOf(U.yen.id)).toBe('member')
    expect(r.current.myRole).toBe('member')
    expect(r.current.isManager).toBe(false)
  })

  it('a plain member of a restricted group may neither post at the top level nor mention everyone', () => {
    seedGroup(CH.doisoat, 'private', { owner_id: U.duc.id, post_policy: 'managers', mention_all_policy: 'managers' })
    const r = role()
    expect(r.current.canPostTopLevel).toBe(false)
    expect(r.current.canMentionAll).toBe(false)
  })

  it('a manager of the same restricted group may do both', () => {
    seedGroup(CH.doisoat, 'private', { owner_id: U.duc.id, manager_ids: [ME.id], post_policy: 'managers', mention_all_policy: 'managers' })
    const r = role()
    expect(r.current.isManager).toBe(true)
    expect(r.current.isOwner).toBe(false)
    expect(r.current.canPostTopLevel).toBe(true)
    expect(r.current.canMentionAll).toBe(true)
  })

  it('an open group lets every member post and mention everyone', () => {
    seedGroup(CH.doisoat, 'private', { owner_id: U.duc.id })
    const r = role()
    expect(r.current.canPostTopLevel).toBe(true)
    expect(r.current.canMentionAll).toBe(true)
  })

  it('the owner is a manager', () => {
    seedGroup(CH.doisoat, 'private', { owner_id: ME.id })
    expect(role().current).toMatchObject({ isOwner: true, isManager: true, myRole: 'owner' })
  })
})

describe('useGroupRole outside groups', () => {
  it('a DM never offers @Tất cả and has no roles', () => {
    seedGroup(CH.doisoat, 'dm')
    const r = role()
    expect(r.current.canMentionAll).toBe(false)
    expect(r.current.isManager).toBe(false)
    expect(r.current.roleOf(ME.id)).toBeUndefined()
    expect(r.current.canPostTopLevel).toBe(true)
  })

  it('the workspace-wide channel offers @Tất cả to administrators only', async () => {
    seedGroup(CH.doisoat, 'workspace')
    seedWorkspaceAdmin(false)
    expect(role().current.canMentionAll).toBe(false)
    queryClient.clear()
    seedGroup(CH.doisoat, 'workspace')
    seedWorkspaceAdmin(true)
    const r = role()
    await waitFor(() => expect(r.current.canMentionAll).toBe(true))
    expect(r.current.isManager).toBe(false)
  })

  it('denies @Tất cả while the conversation is unknown', () => {
    const r = role()
    expect(r.current.canMentionAll).toBe(false)
    expect(r.current.isManager).toBe(false)
  })
})

describe('useGroupRole follows a role change', () => {
  it('loses manager standing when the record is refetched after an invalidation', async () => {
    seedGroup(CH.doisoat, 'private', { owner_id: U.duc.id, manager_ids: [ME.id] })
    vi.spyOn(groupManagementApi, 'get').mockResolvedValue(parseGroupManagement({ owner_id: U.duc.id, manager_ids: [] }))
    const r = role()
    expect(r.current.isManager).toBe(true)
    // A channel event invalidates the channel key; the management record sits under it.
    await act(() => queryClient.invalidateQueries({ queryKey: keys.messaging.channel(CH.doisoat) }))
    await waitFor(() => expect(r.current.isManager).toBe(false))
  })
})

describe('useUpdateGroupSettings', () => {
  it('returns the radio to the old choice when the server refuses', async () => {
    seedGroup(CH.doisoat, 'private', { owner_id: ME.id })
    vi.spyOn(groupManagementApi, 'updateSettings').mockRejectedValue(new ApiError('forbidden', 403))
    vi.spyOn(groupManagementApi, 'get').mockResolvedValue(parseGroupManagement({ owner_id: ME.id }))
    const { result } = renderHook(() => useUpdateGroupSettings(CH.doisoat), { wrapper })
    await act(async () => {
      await result.current
        .mutateAsync({ post_policy: 'managers', invite_policy: 'everyone', mention_all_policy: 'everyone' })
        .catch(() => {})
    })
    expect(queryClient.getQueryData<{ post_policy: string }>(groupManagementKeys.management(CH.doisoat))?.post_policy).toBe('everyone')
  })
})

describe('groupActionError', () => {
  it('reads a refusal as a lost right', () => {
    expect(groupActionError(new ApiError('x', 403), 'đổi cài đặt')).toBe(PERMISSION_LOST)
  })
  it('explains anything else', () => {
    expect(groupActionError(new ApiError('x', 500), 'đổi cài đặt')).toMatch(/Máy chủ/)
  })
})

describe('parseGroupManagement', () => {
  it('reads unknown rules as open and drops malformed managers', () => {
    expect(parseGroupManagement({ post_policy: 'x', manager_ids: ['a', 3, ''] })).toEqual({
      post_policy: 'everyone', invite_policy: 'everyone', mention_all_policy: 'everyone', owner_id: '', manager_ids: ['a'],
    })
  })
})
