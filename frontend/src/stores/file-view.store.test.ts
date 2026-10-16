import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useFileViewStore } from './file-view.store'

const KEY = 'ngac-file-view'
const reset = () => useFileViewStore.setState({ modes: { drive: 'grid', 'space-files': 'grid' } })

describe('file view store', () => {
  beforeEach(() => {
    localStorage.clear()
    reset()
  })
  afterEach(() => vi.restoreAllMocks())

  it('opens in grid on both screens', () => {
    expect(useFileViewStore.getState().modes).toEqual({ drive: 'grid', 'space-files': 'grid' })
  })

  it('remembers each screen separately and across a reload', async () => {
    useFileViewStore.getState().setMode('drive', 'list')
    expect(useFileViewStore.getState().modes['space-files']).toBe('grid')
    expect(JSON.parse(localStorage.getItem(KEY)!).state.modes).toEqual({ drive: 'list', 'space-files': 'grid' })

    // A reload: memory starts from the defaults and storage is what the last session left.
    const stored = localStorage.getItem(KEY)!
    reset()
    localStorage.setItem(KEY, stored)
    await useFileViewStore.persist.rehydrate()
    expect(useFileViewStore.getState().modes).toEqual({ drive: 'list', 'space-files': 'grid' })
  })

  it('ignores stored values it does not know', async () => {
    localStorage.setItem(KEY, JSON.stringify({ state: { modes: { drive: 'tiles', 'space-files': 'list', other: 'list' } }, version: 0 }))
    await useFileViewStore.persist.rehydrate()
    expect(useFileViewStore.getState().modes).toEqual({ drive: 'grid', 'space-files': 'list' })
  })

  it('falls back to grid when storage cannot be read', async () => {
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => { throw new Error('blocked') })
    await expect(useFileViewStore.persist.rehydrate()).resolves.not.toThrow()
    expect(useFileViewStore.getState().modes.drive).toBe('grid')
  })

  it('still switches layout for the session when storage cannot be written', () => {
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new Error('quota') })
    expect(() => useFileViewStore.getState().setMode('drive', 'list')).not.toThrow()
    expect(useFileViewStore.getState().modes.drive).toBe('list')
  })
})
