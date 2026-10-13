import { beforeEach, describe, expect, it, vi } from 'vitest'
import { MAX_RECENT, addRecent, readRecent, removeRecent } from './recent-searches'

beforeEach(() => localStorage.clear())

describe('recent searches', () => {
  it('starts empty', () => {
    expect(readRecent('u1')).toEqual([])
  })

  it('puts the newest first and does not repeat a search that differs only in case or accents', () => {
    addRecent('u1', 'tạm ứng')
    addRecent('u1', 'đối soát')
    addRecent('u1', 'Tam ung')
    expect(readRecent('u1')).toEqual(['Tam ung', 'đối soát'])
  })

  it('keeps the latest eight', () => {
    for (let i = 1; i <= MAX_RECENT + 3; i++) addRecent('u1', `từ khoá ${i}`)
    const all = readRecent('u1')
    expect(all).toHaveLength(8)
    expect(all[0]).toBe('từ khoá 11')
    expect(all[7]).toBe('từ khoá 4')
  })

  it('ignores a search that is blank or too short to be searched', () => {
    addRecent('u1', '   ')
    addRecent('u1', 'a')
    expect(readRecent('u1')).toEqual([])
  })

  it('removes one entry', () => {
    addRecent('u1', 'một')
    addRecent('u1', 'hai')
    expect(removeRecent('u1', 'một')).toEqual(['hai'])
    expect(readRecent('u1')).toEqual(['hai'])
  })

  it('is kept apart for each person on the device', () => {
    addRecent('u1', 'của tôi')
    expect(readRecent('u2')).toEqual([])
  })

  it('reads nothing from a stored value that is not a list of words', () => {
    localStorage.setItem('ngac-chat-recent-searches:u1', '{"a":1}')
    expect(readRecent('u1')).toEqual([])
    localStorage.setItem('ngac-chat-recent-searches:u1', '[1,"ok",null]')
    expect(readRecent('u1')).toEqual(['ok'])
    localStorage.setItem('ngac-chat-recent-searches:u1', 'not json')
    expect(readRecent('u1')).toEqual([])
  })

  it('still works for this visit when storage is blocked', () => {
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => { throw new Error('blocked') })
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new Error('blocked') })
    expect(readRecent('u1')).toEqual([])
    expect(addRecent('u1', 'đối soát')).toEqual(['đối soát'])
    vi.restoreAllMocks()
  })
})
