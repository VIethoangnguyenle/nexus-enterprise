import { useQuery, useMutation, queryOptions } from '@tanstack/react-query'
import { messagingApi } from '../api/messaging'
import { queryClient } from '../lib/query-client'

export const channelsQueryOptions = (wsId: string) =>
  queryOptions({ queryKey: ['channels', wsId], queryFn: () => messagingApi.listChannels(wsId), enabled: !!wsId })

export const messagesQueryOptions = (channelId: string) =>
  queryOptions({ queryKey: ['messages', channelId], queryFn: () => messagingApi.listMessages(channelId), enabled: !!channelId })

export const threadQueryOptions = (messageId: string) =>
  queryOptions({ queryKey: ['thread', messageId], queryFn: () => messagingApi.getThread(messageId), enabled: !!messageId })

export function useChannels(wsId: string) { return useQuery(channelsQueryOptions(wsId)) }
export function useMessages(channelId: string) { return useQuery(messagesQueryOptions(channelId)) }
export function useThread(messageId: string) { return useQuery(threadQueryOptions(messageId)) }

export function useSendMessage(channelId: string) {
  return useMutation({
    mutationFn: (content: string) => messagingApi.sendMessage(channelId, content),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['messages', channelId] }),
  })
}

export function useCreateChannel(wsId: string) {
  return useMutation({
    mutationFn: (data: any) => messagingApi.createChannel(wsId, data),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['channels', wsId] }),
  })
}
