import { beforeEach, describe, expect, it, vi } from 'vitest'
import { MAX_RECENT, addRecent, clearAllRecent, readRecent, removeRecent } from './recent-searches'

beforeEach(() => localStorage.clear())

describe('recent searches', () => {
  it('starts empty', () => {
    expect(readRecent('chat', 'u1')).toEqual([])
  })

  it('puts the newest first and does not repeat a search that differs only in case or accents', () => {
    addRecent('chat', 'u1', 'tạm ứng')
    addRecent('chat', 'u1', 'đối soát')
    addRecent('chat', 'u1', 'Tam ung')
    expect(readRecent('chat', 'u1')).toEqual(['Tam ung', 'đối soát'])
  })

  it('keeps the latest eight', () => {
    for (let i = 1; i <= MAX_RECENT + 3; i++) addRecent('drive', 'u1', `từ khoá ${i}`)
    const all = readRecent('drive', 'u1')
    expect(all).toHaveLength(8)
    expect(all[0]).toBe('từ khoá 11')
    expect(all[7]).toBe('từ khoá 4')
  })

  it('ignores a search that is blank or too short to be searched', () => {
    addRecent('chat', 'u1', '   ')
    addRecent('chat', 'u1', 'a')
    expect(readRecent('chat', 'u1')).toEqual([])
  })

  it('removes one entry', () => {
    addRecent('assets', 'u1', 'một')
    addRecent('assets', 'u1', 'hai')
    expect(removeRecent('assets', 'u1', 'một')).toEqual(['hai'])
    expect(readRecent('assets', 'u1')).toEqual(['hai'])
  })

  it('is kept apart for each person on the device', () => {
    addRecent('chat', 'u1', 'của tôi')
    expect(readRecent('chat', 'u2')).toEqual([])
  })

  it('is kept apart for each module', () => {
    addRecent('drive', 'u1', 'hợp đồng')
    addRecent('approval', 'u1', 'công tác')
    expect(readRecent('drive', 'u1')).toEqual(['hợp đồng'])
    expect(readRecent('approval', 'u1')).toEqual(['công tác'])
    expect(readRecent('contacts', 'u1')).toEqual([])
  })

  it('reads nothing from a stored value that is not a list of words', () => {
    localStorage.setItem('ngac-recent-searches:drive:u1', '{"a":1}')
    expect(readRecent('drive', 'u1')).toEqual([])
    localStorage.setItem('ngac-recent-searches:drive:u1', '[1,"ok",null]')
    expect(readRecent('drive', 'u1')).toEqual(['ok'])
    localStorage.setItem('ngac-recent-searches:drive:u1', 'not json')
    expect(readRecent('drive', 'u1')).toEqual([])
  })

  it('still works for this visit when storage is blocked', () => {
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => { throw new Error('blocked') })
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new Error('blocked') })
    expect(readRecent('chat', 'u1')).toEqual([])
    expect(addRecent('chat', 'u1', 'đối soát')).toEqual(['đối soát'])
    vi.restoreAllMocks()
  })

  it('clears every person\'s searches in every module and nothing else', () => {
    addRecent('chat', 'u1', 'tạm ứng')
    addRecent('drive', 'u2', 'đối soát')
    localStorage.setItem('ngac-chat-recent-searches:u3', JSON.stringify(['cũ']))
    localStorage.setItem('other-key', 'keep')
    clearAllRecent()
    expect(readRecent('chat', 'u1')).toEqual([])
    expect(readRecent('drive', 'u2')).toEqual([])
    expect(readRecent('chat', 'u3')).toEqual([])
    expect(localStorage.length).toBe(1)
    expect(localStorage.getItem('other-key')).toBe('keep')
  })
})

describe('Chat searches kept before every module had its own', () => {
  const LEGACY = 'ngac-chat-recent-searches:u1'

  it('are read as Chat\'s and moved to the new place', () => {
    localStorage.setItem(LEGACY, JSON.stringify(['đối soát', 'tạm ứng']))
    expect(readRecent('chat', 'u1')).toEqual(['đối soát', 'tạm ứng'])
    expect(localStorage.getItem(LEGACY)).toBeNull()
    expect(JSON.parse(localStorage.getItem('ngac-recent-searches:chat:u1')!)).toEqual(['đối soát', 'tạm ứng'])
  })

  it('belong to Chat only', () => {
    localStorage.setItem(LEGACY, JSON.stringify(['đối soát']))
    expect(readRecent('drive', 'u1')).toEqual([])
    expect(localStorage.getItem(LEGACY)).not.toBeNull()
  })

  it('give way to what Chat already keeps in the new place', () => {
    localStorage.setItem(LEGACY, JSON.stringify(['cũ']))
    localStorage.setItem('ngac-recent-searches:chat:u1', JSON.stringify(['mới']))
    expect(readRecent('chat', 'u1')).toEqual(['mới'])
    expect(localStorage.getItem(LEGACY)).toBeNull()
  })

  it('are dropped when unreadable, without hiding what Chat keeps in the new place', () => {
    localStorage.setItem(LEGACY, '{corrupt')
    localStorage.setItem('ngac-recent-searches:chat:u1', JSON.stringify(['a1', 'b2']))
    expect(readRecent('chat', 'u1')).toEqual(['a1', 'b2'])
    expect(localStorage.getItem(LEGACY)).toBeNull()
    expect(addRecent('chat', 'u1', 'zz')).toEqual(['zz', 'a1', 'b2'])
    expect(readRecent('chat', 'u1')).toEqual(['zz', 'a1', 'b2'])
  })

  it('are dropped when unreadable and nothing is kept in the new place either', () => {
    localStorage.setItem(LEGACY, '{corrupt')
    expect(readRecent('chat', 'u1')).toEqual([])
    expect(localStorage.getItem(LEGACY)).toBeNull()
  })

  it('are not lost when the new place cannot be written', () => {
    localStorage.setItem(LEGACY, JSON.stringify(['đối soát']))
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new Error('full') })
    expect(readRecent('chat', 'u1')).toEqual(['đối soát'])
    vi.restoreAllMocks()
    expect(localStorage.getItem(LEGACY)).not.toBeNull()
  })
})
