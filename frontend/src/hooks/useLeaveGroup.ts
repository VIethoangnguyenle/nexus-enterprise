import { useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { useMutation } from '@tanstack/react-query'
import { messagingApi } from '../api/messaging'
import { queryClient } from '../lib/query-client'
import { keys } from './keys'
import { toast } from '../components/primitives'

/**
 * Leave a group: DELETE /api/channels/:chId/members/:nodeId with the caller's
 * own NGAC node id. Not optimistic — the channel leaves the list only once the
 * server agrees. On success the mutation invalidates both the channel list and
 * the DM list so the navigator rebuilds.
 */
export function useLeaveGroup(channelId: string) {
  const navigate = useNavigate()
  const [error, setError] = useState<string | null>(null)

  const mutation = useMutation({
    meta: { silentError: true },
    mutationFn: (nodeId: string) => messagingApi.removeMember(channelId, nodeId),
    onSuccess: (_data, _nodeId, _ctx) => {
      // Invalidate every channel list so the navigator drops this conversation.
      queryClient.invalidateQueries({ queryKey: keys.messaging.channelsAll() })
      queryClient.invalidateQueries({ queryKey: keys.messaging.members(channelId) })
      queryClient.invalidateQueries({ queryKey: keys.messaging.unreadCounts() })
      queryClient.invalidateQueries({ queryKey: keys.messaging.conversationActivity() })
    },
    onError: () => {
      setError('Không rời được nhóm. Thử lại.')
    },
  })

  const leave = async (nodeId: string, groupName: string) => {
    setError(null)
    try {
      await mutation.mutateAsync(nodeId)
    } catch {
      // Error is already set in onError
      return false
    }
    toast(`Đã rời \u201C${groupName}\u201D`)
    void navigate({ to: '/channels' })
    return true
  }

  return {
    leave,
    isPending: mutation.isPending,
    error,
    clearError: () => setError(null),
  }
}
