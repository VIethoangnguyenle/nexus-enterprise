import { afterEach, describe, expect, it, vi } from 'vitest'
import { loadCallPrefs, saveCallPrefs } from './call-prefs'

afterEach(() => { localStorage.clear(); vi.restoreAllMocks() })

describe('call prefs', () => {
  it('defaults to mic and camera on', () => {
    expect(loadCallPrefs('u1')).toEqual({ micOn: true, camOn: true })
  })

  it('remembers choices per user', () => {
    saveCallPrefs('u1', { micOn: false, camOn: true, micId: 'a' })
    expect(loadCallPrefs('u1')).toMatchObject({ micOn: false, micId: 'a' })
    expect(loadCallPrefs('u2').micOn).toBe(true)
  })

  it('survives garbage and a store that throws', () => {
    localStorage.setItem('ngac-call-prefs:u1', '{nope')
    expect(loadCallPrefs('u1')).toEqual({ micOn: true, camOn: true })
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new Error('full') })
    expect(() => saveCallPrefs('u1', { micOn: true, camOn: true })).not.toThrow()
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => { throw new Error('denied') })
    expect(loadCallPrefs('u1')).toEqual({ micOn: true, camOn: true })
  })

  it('ignores wrongly typed stored values', () => {
    localStorage.setItem('ngac-call-prefs:u1', JSON.stringify({ micOn: 'yes', camOn: false, micId: 7 }))
    expect(loadCallPrefs('u1')).toEqual({ micOn: true, camOn: false, micId: undefined, camId: undefined, speakerId: undefined })
  })
})
