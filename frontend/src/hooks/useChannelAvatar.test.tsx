import { renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { QueryClientProvider } from '@tanstack/react-query'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { messagingApi, type Channel } from '../api/messaging'
import { ApiError } from '../api/client'
import { queryClient } from '../lib/query-client'
import { keys } from './keys'
import { useRemoveChannelAvatar, useUploadChannelAvatar } from './useChannelAvatar'

vi.mock('../api/messaging', async (orig) => {
  const real = await orig<typeof import('../api/messaging')>()
  return {
    ...real,
    messagingApi: { ...real.messagingApi, uploadChannelAvatar: vi.fn(), removeChannelAvatar: vi.fn() },
  }
})

const wrapper = ({ children }: { children: ReactNode }) => (
  <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
)
const PIC = '/api/channels/avatars/0123456789abcdef0123456789abcdef'
const ch = (id: string, over: Partial<Channel> = {}): Channel =>
  ({ id, name: id, channel_type: 'workspace', workspace_id: 'w', ...over })
const file = new File([new Uint8Array(10)], 'a.png', { type: 'image/png' })

beforeEach(() => {
  queryClient.clear()
  vi.clearAllMocks()
})

describe('useUploadChannelAvatar', () => {
  it('sends the file and shows the new picture in the channel lists and the channel record at once', async () => {
    vi.mocked(messagingApi.uploadChannelAvatar).mockResolvedValue({ avatar_url: PIC })
    queryClient.setQueryData(keys.messaging.channels('w'), { channels: [ch('c1'), ch('c2')] })
    queryClient.setQueryData(keys.messaging.channel('c1'), ch('c1'))
    const { result } = renderHook(() => useUploadChannelAvatar('c1'), { wrapper })

    result.current.mutate(file)
    await waitFor(() => expect(result.current.isSuccess).toBe(true))

    expect(messagingApi.uploadChannelAvatar).toHaveBeenCalledWith('c1', file)
    const list = queryClient.getQueryData<{ channels: Channel[] }>(keys.messaging.channels('w'))!
    expect(list.channels.map((c) => c.avatar_url)).toEqual([PIC, undefined])
    expect(queryClient.getQueryData<Channel>(keys.messaging.channel('c1'))!.avatar_url).toBe(PIC)
  })

  it('leaves the cache as it was and reports the failure', async () => {
    vi.mocked(messagingApi.uploadChannelAvatar).mockRejectedValue(new ApiError('x', 413, { code: 'avatar_too_large' }))
    queryClient.setQueryData(keys.messaging.channels('w'), { channels: [ch('c1')] })
    const { result } = renderHook(() => useUploadChannelAvatar('c1'), { wrapper })

    result.current.mutate(file)
    await waitFor(() => expect(result.current.isError).toBe(true))

    const list = queryClient.getQueryData<{ channels: Channel[] }>(keys.messaging.channels('w'))!
    expect(list.channels[0]!.avatar_url).toBeUndefined()
  })
})

describe('useRemoveChannelAvatar', () => {
  it('clears the picture in the lists, including the direct-message list shape', async () => {
    vi.mocked(messagingApi.removeChannelAvatar).mockResolvedValue({ avatar_url: '' })
    queryClient.setQueryData(keys.messaging.channels('w'), { channels: [ch('c1', { avatar_url: PIC })] })
    queryClient.setQueryData(keys.messaging.dms(), { channels: null })
    const { result } = renderHook(() => useRemoveChannelAvatar('c1'), { wrapper })

    result.current.mutate()
    await waitFor(() => expect(result.current.isSuccess).toBe(true))

    expect(messagingApi.removeChannelAvatar).toHaveBeenCalledWith('c1')
    const list = queryClient.getQueryData<{ channels: Channel[] }>(keys.messaging.channels('w'))!
    expect(list.channels[0]!.avatar_url).toBe('')
    expect(queryClient.getQueryData(keys.messaging.dms())).toEqual({ channels: null })
  })
})
