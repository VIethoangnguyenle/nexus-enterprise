import { useMutation } from '@tanstack/react-query'
import { messagingApi, type Channel } from '../api/messaging'
import { queryClient } from '../lib/query-client'
import { keys } from './keys'

/**
 * Shows `avatarUrl` on a group in every cached channel list and in its own
 * record, so the picture changes where it is drawn without waiting for a
 * refetch. Other members get it from the channel event, which refreshes the
 * same lists.
 */
function showPicture(channelId: string, avatarUrl: string) {
  const patch = (ch: Channel) => (ch.id === channelId ? { ...ch, avatar_url: avatarUrl } : ch)
  queryClient.setQueriesData<{ channels: Channel[] | null }>({ queryKey: keys.messaging.channelsAll() }, (old) =>
    old?.channels ? { ...old, channels: old.channels.map(patch) } : old,
  )
  queryClient.setQueryData<Channel>(keys.messaging.channel(channelId), (old) => (old ? patch(old) : old))
}

/**
 * Sets a group's picture. Errors are left to the panel, which says what was
 * wrong with the picture (silentError).
 */
export function useUploadChannelAvatar(channelId: string) {
  return useMutation({
    meta: { silentError: true },
    mutationFn: (file: File) => messagingApi.uploadChannelAvatar(channelId, file),
    onSuccess: (res) => showPicture(channelId, res.avatar_url),
  })
}

/** Removes a group's picture; its letter shows again. */
export function useRemoveChannelAvatar(channelId: string) {
  return useMutation({
    meta: { silentError: true },
    mutationFn: () => messagingApi.removeChannelAvatar(channelId),
    onSuccess: (res) => showPicture(channelId, res.avatar_url),
  })
}
