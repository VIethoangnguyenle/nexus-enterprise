import { act, renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { useToastStore } from '../components/primitives'
import { addRecent, readRecent } from '../lib/recent-searches'
import { useAuthStore } from '../stores/auth.store'
import { ME } from '../test/chat-fixtures'
import { useRecentSearches, type RecentScope } from './useRecentSearches'

beforeEach(() => {
  localStorage.clear()
  useToastStore.getState().clear()
  useAuthStore.setState({ user: ME, accessToken: 't', bootstrapping: false })
})
afterEach(() => useToastStore.getState().clear())

const seed = (...words: string[]) => words.forEach((w) => addRecent('chat', ME.id, w))

describe('useRecentSearches', () => {
  it('forgets one search for good', () => {
    seed('một', 'hai')
    const { result } = renderHook(() => useRecentSearches('chat'))
    act(() => result.current.group(() => {}).items[0]!.onRemove!())
    expect(result.current.list).toEqual(['một'])
    expect(readRecent('chat', ME.id)).toEqual(['một'])
  })

  it('offers Xoá tất cả from two searches on, and tells how many went with a Hoàn tác toast', () => {
    seed('một')
    const one = renderHook(() => useRecentSearches('chat'))
    expect(one.result.current.group(() => {}).onClearAll).toBeUndefined()

    seed('hai', 'ba')
    const { result } = renderHook(() => useRecentSearches('chat'))
    act(() => result.current.group(() => {}).onClearAll!())
    expect(result.current.list).toEqual([])
    expect(readRecent('chat', ME.id)).toEqual([])
    const [toast] = useToastStore.getState().toasts
    expect(toast).toMatchObject({ message: 'Đã xoá 3 lần tìm gần đây', duration: 5000, action: { label: 'Hoàn tác' } })
  })

  it('brings every search back, in order, on Hoàn tác', () => {
    seed('một', 'hai', 'ba')
    const { result } = renderHook(() => useRecentSearches('chat'))
    act(() => result.current.group(() => {}).onClearAll!())
    act(() => useToastStore.getState().toasts[0]!.action!.onClick())
    expect(result.current.list).toEqual(['ba', 'hai', 'một'])
    expect(readRecent('chat', ME.id)).toEqual(['ba', 'hai', 'một'])
  })

  it('keeps a list of its own for every scope of the search, Tất cả and Lịch included, and follows a change of scope', () => {
    seed('chat cũ')
    const { result, rerender } = renderHook(({ scope }: { scope: RecentScope }) => useRecentSearches(scope), {
      initialProps: { scope: 'all' as RecentScope },
    })
    expect(result.current.list).toEqual([])
    act(() => result.current.add('đối soát'))
    rerender({ scope: 'calendar' })
    expect(result.current.list).toEqual([])
    act(() => result.current.add('họp quý'))
    rerender({ scope: 'chat' })
    expect(result.current.list).toEqual(['chat cũ'])
    rerender({ scope: 'all' })
    expect(result.current.list).toEqual(['đối soát'])
    rerender({ scope: 'calendar' })
    expect(result.current.list).toEqual(['họp quý'])
  })
})
