import { beforeEach, describe, expect, it } from 'vitest'
import { keys } from '../hooks/keys'
import { queryClient } from './query-client'
import { invalidateUnreadThreadsFor } from './unread-threads-cache'

const state = () => queryClient.getQueryState(keys.notificationLevels.unreadThreads())!

beforeEach(() => {
  queryClient.clear()
  queryClient.setQueryData(keys.notificationLevels.unreadThreads(), { messages: [] })
})

describe('invalidateUnreadThreadsFor', () => {
  it.each(['threadState', 'threadReply'])('refreshes Trang chủ threads on a %s frame', (kind) => {
    expect(invalidateUnreadThreadsFor(kind)).toBe(true)
    expect(state().isInvalidated).toBe(true)
  })

  it.each(['newMessage', 'unreadCount', undefined])('leaves them alone on a %s frame', (kind) => {
    expect(invalidateUnreadThreadsFor(kind)).toBe(false)
    expect(state().isInvalidated).toBe(false)
  })
})
