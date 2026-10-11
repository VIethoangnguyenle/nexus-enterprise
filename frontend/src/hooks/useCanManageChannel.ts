import { useQuery } from '@tanstack/react-query'
import { messagingApi } from '../api/messaging'
import { keys } from './keys'
import { useObjectPermissions } from './usePermissions'

/**
 * Whether the signed-in user manages this conversation, which is what lets
 * them delete other people's messages in it. Messages are not NGAC nodes: the
 * question is asked of the channel's content object. The server decides in
 * the end; this keeps "Xoá tin" from being offered where it would be refused.
 */
export function useCanManageChannel(channelId: string): boolean {
  const channel = useQuery({
    queryKey: keys.messaging.channel(channelId),
    queryFn: () => messagingApi.getChannel(channelId),
    enabled: !!channelId,
    staleTime: 5 * 60_000,
  })
  return useObjectPermissions(channel.data?.ngac_oa_id || undefined).manage
}
