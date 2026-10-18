import { useCallback } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { toast } from '../components/primitives'
import type { Conversation } from '../lib/conversations'
import { explain } from '../lib/errors'
import type { Person } from '../lib/people'
import { useCreateDM } from './useMessaging'

/**
 * Opens the direct message with a person: the one that exists, or a new one created first. A failure is said
 * in a toast that names the person and what was being done; the caller decides nothing else.
 */
export function useOpenPerson(conversations: Conversation[]) {
  const navigate = useNavigate()
  const createDM = useCreateDM()
  const open = useCallback((p: Person) => {
    const existing = conversations.find((c) => c.kind === 'dm' && c.partner?.userId === p.userId)
    if (existing) {
      void navigate({ to: '/channels/$channelId', params: { channelId: existing.id } })
      return
    }
    createDM.mutate(
      { userId: p.userId, ngacNodeId: p.nodeId },
      {
        onSuccess: (ch) => void navigate({ to: '/channels/$channelId', params: { channelId: ch.id } }),
        onError: (err) => toast.error(explain(err, `mở cuộc trò chuyện với ${p.name}`)),
      },
    )
  }, [conversations, navigate, createDM])
  return { open, pending: createDM.isPending }
}
