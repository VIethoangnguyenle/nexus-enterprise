import { describe, it, expect, beforeEach } from 'vitest'
import { MutationObserver } from '@tanstack/react-query'
import { queryClient } from './query-client'
import { ApiError } from '../api/client'
import { useToastStore } from '../components/primitives/Toast'

function run(error: unknown, meta?: { silentError?: boolean; action?: string }) {
  return new MutationObserver(queryClient, {
    mutationFn: async () => {
      throw error
    },
    meta,
  })
    .mutate()
    .catch(() => {})
}

const toasts = () => useToastStore.getState().toasts

beforeEach(() => {
  useToastStore.getState().clear()
})

describe('mutation errors', () => {
  it('tells the user they lack permission on 403', async () => {
    await run(new ApiError('forbidden', 403))
    expect(toasts()).toHaveLength(1)
    expect(toasts()[0]!.tone).toBe('error')
    expect(toasts()[0]!.message).toMatch(/quyền/)
  })

  it('asks for a retry later on a server error, without echoing the server text', async () => {
    await run(new ApiError('pq: relation 42 does not exist', 500))
    expect(toasts()).toHaveLength(1)
    expect(toasts()[0]!.message).toMatch(/Máy chủ/)
    expect(toasts()[0]!.message).not.toMatch(/pq:/)
  })

  it('shows the network message when the request never got an answer', async () => {
    await run(new TypeError('Failed to fetch'))
    expect(toasts()).toHaveLength(1)
    expect(toasts()[0]!.message).toMatch(/kết nối mạng/)
  })

  it('names the action when the mutation declares one', async () => {
    await run(new ApiError('forbidden', 403), { action: 'xoá tệp' })
    expect(toasts()[0]!.message).toMatch(/xoá tệp/)
  })

  it('stays quiet when the caller reports the error itself', async () => {
    await run(new ApiError('boom', 500), { silentError: true })
    expect(toasts()).toHaveLength(0)
  })

  it('leaves 401 to the client, which refreshes or signs the user out', async () => {
    await run(new ApiError('Unauthorized', 401))
    expect(toasts()).toHaveLength(0)
  })
})
