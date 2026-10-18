import { useCallback, useMemo } from 'react'
import { useMutation, useQuery } from '@tanstack/react-query'
import { groupManagementApi, type GroupManagement, type GroupSettings } from '../api/group-management'
import { messagingApi } from '../api/messaging'
import { explain, isForbidden } from '../lib/errors'
import { queryClient } from '../lib/query-client'
import { useAuthStore } from '../stores/auth.store'
import { keys } from './keys'
import { groupManagementKeys } from './keys/group-management'
import { useWorkspaceDetails } from './useWorkspaces'

/** What an action the server refused because the caller's role changed underneath them reads as (DESIGN.md "Quản lý nhóm"). */
export const PERMISSION_LOST = 'Bạn không còn quyền làm việc này'

/** A person's role in a group: exactly one owner, any number of managers, everyone else a member. */
export type GroupRole = 'owner' | 'manager' | 'member'

export const GROUP_ROLE_LABEL: Record<GroupRole, string> = {
  owner: 'Chủ nhóm',
  manager: 'Quản trị nhóm',
  member: 'Thành viên',
}

/** The kinds of conversation the server knows; anything else is treated as unknown (no extra rights). */
type ConversationType = 'dm' | 'workspace' | 'private'
const typeOf = (raw: string | undefined): ConversationType | undefined =>
  raw === 'dm' || raw === 'workspace' || raw === 'private' ? raw : undefined

/** The sentence for a failed group action: a refusal means the caller's rights changed, anything else is explained. */
export function groupActionError(err: unknown, action: string): string {
  return isForbidden(err) ? PERMISSION_LOST : explain(err, action)
}

/** Everything that follows from someone's group role: the management record, the permission answers, the roster. */
function refreshGroup(channelId: string) {
  void queryClient.invalidateQueries({ queryKey: groupManagementKeys.management(channelId) })
  void queryClient.invalidateQueries({ queryKey: keys.permissions.all() })
  void queryClient.invalidateQueries({ queryKey: keys.messaging.members(channelId) })
}

/** The channel's own record, shared with `useCanManageChannel` (same key, same fetch). */
function useChannelRecord(channelId: string) {
  return useQuery({
    queryKey: keys.messaging.channel(channelId),
    queryFn: () => messagingApi.getChannel(channelId),
    enabled: !!channelId,
    staleTime: 5 * 60_000,
  })
}

/** A private group's settings, owner and managers. Only private groups have them. */
export function useGroupManagement(channelId: string, enabled = true) {
  return useQuery({
    queryKey: groupManagementKeys.management(channelId),
    queryFn: () => groupManagementApi.get(channelId),
    enabled: !!channelId && enabled,
    staleTime: 30_000,
  })
}

export interface GroupRoleInfo {
  /** The management record of a private group; undefined elsewhere and until it loads. */
  management: GroupManagement | undefined
  /** A person's role, or undefined when the conversation has no roles (or they have not loaded). */
  roleOf: (userId: string) => GroupRole | undefined
  /** The signed-in user's role. */
  myRole: GroupRole | undefined
  isOwner: boolean
  /** Owner or manager: may open Quản lý nhóm. */
  isManager: boolean
  /** @Tất cả may be offered: never in a DM, administrators in the workspace-wide channel, the group's rule in a group. */
  canMentionAll: boolean
  /**
   * A message with no parent may be sent. False only for a plain member of a group whose posting is
   * restricted; true while unknown, the server deciding in the end.
   */
  canPostTopLevel: boolean
}

/**
 * The signed-in user's standing in a conversation, as the UI needs it to offer
 * or hide group actions. The server decides every action; this only keeps the
 * UI from offering what would be refused. A `management_changed` event
 * invalidates the channel key, and with it this answer.
 */
export function useGroupRole(channelId: string): GroupRoleInfo {
  const me = useAuthStore((s) => s.user?.id)
  const channel = useChannelRecord(channelId)
  const type = typeOf(channel.data?.channel_type)
  const management = useGroupManagement(channelId, type === 'private').data
  const wsAdmin = useWorkspaceDetails(type === 'workspace' ? (channel.data?.workspace_id ?? '') : '').data?.can_manage === true

  return useMemo(() => {
    const m = type === 'private' ? management : undefined
    const roleOf = (userId: string): GroupRole | undefined => {
      if (!m || !userId) return undefined
      if (m.owner_id === userId) return 'owner'
      return m.manager_ids.includes(userId) ? 'manager' : 'member'
    }
    const myRole = me ? roleOf(me) : undefined
    const isManager = myRole === 'owner' || myRole === 'manager'
    const canMentionAll =
      type === 'private' ? !!m && (m.mention_all_policy === 'everyone' || isManager)
        : type === 'workspace' ? wsAdmin
          : false
    const canPostTopLevel = !(m && m.post_policy === 'managers' && !isManager)
    return { management: m, roleOf, myRole, isOwner: myRole === 'owner', isManager, canMentionAll, canPostTopLevel }
  }, [type, management, me, wsAdmin])
}

/** Re-reads the caller's standing after the server refused something the UI offered. */
export function useRefreshGroupRole(channelId: string) {
  return useCallback(() => refreshGroup(channelId), [channelId])
}

/** Changes the three rules. Optimistic: the radio moves at once and returns to the old choice if the server refuses. */
export function useUpdateGroupSettings(channelId: string) {
  const key = groupManagementKeys.management(channelId)
  return useMutation({
    meta: { silentError: true },
    mutationFn: (settings: GroupSettings) => groupManagementApi.updateSettings(channelId, settings),
    onMutate: async (settings) => {
      await queryClient.cancelQueries({ queryKey: key })
      const previous = queryClient.getQueryData<GroupManagement>(key)
      if (previous) queryClient.setQueryData<GroupManagement>(key, { ...previous, ...settings })
      return { previous }
    },
    onError: (_err, _settings, ctx) => {
      if (ctx?.previous) queryClient.setQueryData(key, ctx.previous)
    },
    onSettled: () => refreshGroup(channelId),
  })
}

export function useAppointManager(channelId: string) {
  return useMutation({
    meta: { silentError: true },
    mutationFn: (userId: string) => groupManagementApi.appointManager(channelId, userId),
    onSettled: () => refreshGroup(channelId),
  })
}

export function useRemoveManager(channelId: string) {
  return useMutation({
    meta: { silentError: true },
    mutationFn: (userId: string) => groupManagementApi.removeManager(channelId, userId),
    onSettled: () => refreshGroup(channelId),
  })
}

export function useTransferOwnership(channelId: string) {
  return useMutation({
    meta: { silentError: true },
    mutationFn: (userId: string) => groupManagementApi.transferOwnership(channelId, userId),
    onSettled: () => refreshGroup(channelId),
  })
}

/** Ends the group. On success every list that showed it is refreshed and its own records are dropped. */
export function useDisbandGroup(channelId: string) {
  return useMutation({
    meta: { silentError: true },
    mutationFn: () => groupManagementApi.disband(channelId),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: keys.messaging.channelsAll() })
      void queryClient.invalidateQueries({ queryKey: keys.messaging.unreadCounts() })
      void queryClient.invalidateQueries({ queryKey: keys.messaging.conversationActivity() })
      queryClient.removeQueries({ queryKey: keys.messaging.channel(channelId) })
      queryClient.removeQueries({ queryKey: keys.messaging.members(channelId) })
    },
  })
}
