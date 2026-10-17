import { useCallback, useMemo } from 'react'
import type { Channel } from '../api/messaging'
import {
  buildTree, givenNameOf, privateGroups, rolesOf, type DirectoryTree, type RoleGroup,
} from '../components/contacts/contacts-model'
import { displayName } from '../lib/people'
import { useDepartments } from './useAdmin'
import { useContacts } from './useContacts'
import { useChannelMembers, useChannels } from './useMessaging'
import { usePeople } from './usePeople'

/*
 * The data behind the categories of Danh bạ. The directory itself (`useContacts`) is loaded for
 * the whole app; each category adds only what it needs, and only once it is open.
 */

/**
 * Phòng ban: the department tree with its people. `enabled` is false until the category opens,
 * and the departments are not asked for before then.
 */
export function useDepartmentTree(wsId: string, enabled: boolean) {
  const departments = useDepartments(wsId, enabled)
  const contacts = useContacts(wsId)
  const flat = departments.data?.flat
  const people = contacts.data?.contacts
  const tree = useMemo<DirectoryTree | undefined>(
    () => (flat && people ? buildTree(flat, people) : undefined),
    [flat, people],
  )
  const refetchDepartments = departments.refetch
  const refetchContacts = contacts.refetch
  // Stable, so what depends on it does not recompute with every render.
  const refetch = useCallback(() => { void refetchDepartments(); void refetchContacts() }, [refetchDepartments, refetchContacts])
  return {
    tree,
    isLoading: departments.isLoading || contacts.isLoading,
    isError: departments.isError || contacts.isError,
    refetch,
  }
}

/** Vai trò: the roles of the people already loaded. No request of its own. */
export function useRoleGroups(wsId: string): { groups: RoleGroup[]; isLoading: boolean; isError: boolean } {
  const { data, isLoading, isError } = useContacts(wsId)
  const groups = useMemo(() => rolesOf(data?.contacts ?? []), [data])
  return { groups, isLoading, isError }
}

/** Nhóm của tôi: the private groups the signed-in user can read; not #general, not a direct message. */
export function useMyGroups(wsId: string, enabled: boolean): {
  groups: Channel[]; isLoading: boolean; isError: boolean; refetch: () => void
  /** The list has been asked for (here or by the chat list): an empty `groups` then means no groups, not "not known yet". */
  known: boolean
} {
  const q = useChannels(wsId, enabled)
  const groups = useMemo(() => privateGroups(q.data?.channels), [q.data])
  const queryRefetch = q.refetch
  const refetch = useCallback(() => { void queryRefetch() }, [queryRefetch])
  return { groups, isLoading: q.isLoading, isError: q.isError, known: !!q.data, refetch }
}

export interface GroupMember {
  userId: string
  nodeId: string
  name: string
  avatarUrl: string
  /**
   * False for a member the directory hides from the signed-in user: shown by name, no profile to open.
   * Undefined while the directory is not ready: not known yet is not hidden.
   */
  visible: boolean | undefined
}

/**
 * The people of one group, by given name. Loaded only while a group is open (`channelId` null
 * loads nothing). A member the directory hides is named by the resolver; the group's own
 * membership is what the caller was allowed to read, so naming them reveals nothing new.
 */
export function useGroupMembers(wsId: string, channelId: string | null): {
  members: GroupMember[]; isLoading: boolean; isError: boolean; refetch: () => void
} {
  const q = useChannelMembers(channelId ?? '', !!channelId)
  const people = usePeople(wsId)
  const raw = q.data?.members
  const members = useMemo(() => {
    const out = (raw ?? []).map((m): GroupMember => ({
      userId: m.user_id,
      nodeId: m.ngac_node_id,
      // Not the login name: until the resolver answers, a hidden member is just "Thành viên".
      name: displayName(people, m.user_id),
      avatarUrl: people.byUserId.get(m.user_id)?.avatarUrl ?? '',
      visible: people.ready ? people.isVisible(m.user_id) : undefined,
    }))
    return out.sort((a, b) => givenNameOf(a.name).localeCompare(givenNameOf(b.name), 'vi', { sensitivity: 'base' })
      || a.name.localeCompare(b.name, 'vi', { sensitivity: 'base' }))
  }, [raw, people])
  const queryRefetch = q.refetch
  const refetch = useCallback(() => { void queryRefetch() }, [queryRefetch])
  return { members, isLoading: !!channelId && q.isLoading, isError: q.isError, refetch }
}
