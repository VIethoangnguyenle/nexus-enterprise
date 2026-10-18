import type { GroupManagement } from '../../../api/group-management'
import type { Channel } from '../../../api/messaging'
import { keys } from '../../../hooks/keys'
import { groupManagementKeys } from '../../../hooks/keys/group-management'
import { queryClient } from '../../../lib/query-client'
import { WS_ID } from '../../../test/chat-fixtures'

/**
 * Test support: puts a conversation of the given kind, and for a group its
 * management record, in the app's query cache, as the server would answer.
 */
export function seedGroup(
  channelId: string,
  type: 'private' | 'workspace' | 'dm',
  management?: Partial<GroupManagement>,
) {
  const channel: Channel = { id: channelId, name: 'Đối soát giao dịch', channel_type: type, workspace_id: WS_ID }
  queryClient.setQueryData(keys.messaging.channel(channelId), channel)
  if (type === 'private') {
    queryClient.setQueryData<GroupManagement>(groupManagementKeys.management(channelId), {
      post_policy: 'everyone',
      invite_policy: 'everyone',
      mention_all_policy: 'everyone',
      owner_id: '',
      manager_ids: [],
      ...management,
    })
  }
}

/** The workspace-wide channel's administrator answer for the signed-in user. */
export function seedWorkspaceAdmin(canManage: boolean) {
  queryClient.setQueryData(keys.workspaces.details(WS_ID), { name: 'Khối Vận hành', description: '', can_manage: canManage })
}
