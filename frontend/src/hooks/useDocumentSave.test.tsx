import { act, renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '../api/client'
import type { TextDocument, TextSave } from '../api/documents'
import { useDocumentSave } from './useDocumentSave'

const doc = (over: Partial<TextDocument> = {}): TextDocument => ({
  id: 'd1', workspace_id: 'w1', title: 'Quy trình', content: '<p>a</p>', version: 1, status: 'draft',
  owner_id: 'u1', owner_name: 'Hoa', created_at: '2026-10-10T01:00:00Z', updated_at: '2026-10-10T01:00:00Z', ...over,
})

const initial = { version: 1, title: 'Quy trình', content: '<p>a</p>', status: 'draft' as const }

let save: ReturnType<typeof vi.fn<(body: TextSave) => Promise<TextDocument>>>

function setup(delay = 1000) {
  return renderHook(() => useDocumentSave({ initial, save, delay }))
}

beforeEach(() => {
  vi.useFakeTimers()
  save = vi.fn(async (body: TextSave) => doc({ version: body.base_version + 1, content: body.content ?? '<p>a</p>' }))
})
afterEach(() => vi.useRealTimers())

const tick = (ms: number) => act(async () => { await vi.advanceTimersByTimeAsync(ms) })

describe('useDocumentSave: autosave', () => {
  it('starts saved and sends nothing until something changes', async () => {
    const { result } = setup()
    expect(result.current.state).toBe('saved')
    await tick(5000)
    expect(save).not.toHaveBeenCalled()
  })

  it('waits for a pause in typing, then saves once with the version it was based on', async () => {
    const { result } = setup()
    act(() => result.current.setContent('<p>ab</p>'))
    expect(result.current.state).toBe('dirty')
    await tick(600)
    act(() => result.current.setContent('<p>abc</p>'))
    await tick(600)
    expect(save).not.toHaveBeenCalled() // typing again restarted the wait

    await tick(500)
    expect(save).toHaveBeenCalledTimes(1)
    expect(save).toHaveBeenCalledWith({ base_version: 1, content: '<p>abc</p>' })
    expect(result.current.state).toBe('saved')
    expect(result.current.savedAt).toBeInstanceOf(Date)
  })

  it('sends only what changed, and the next save builds on the new version', async () => {
    const { result } = setup()
    act(() => result.current.setTitle('Quy trình mới'))
    await tick(1000)
    expect(save).toHaveBeenLastCalledWith({ base_version: 1, title: 'Quy trình mới' })

    act(() => result.current.setContent('<p>x</p>'))
    await tick(1000)
    expect(save).toHaveBeenLastCalledWith({ base_version: 2, content: '<p>x</p>' })
  })

  it('does not save a change that was undone', async () => {
    const { result } = setup()
    act(() => result.current.setContent('<p>zzz</p>'))
    act(() => result.current.setContent('<p>a</p>'))
    expect(result.current.state).toBe('saved')
    await tick(3000)
    expect(save).not.toHaveBeenCalled()
  })

  it('keeps typing during a save and saves that too, one at a time', async () => {
    let release!: (d: TextDocument) => void
    save.mockImplementationOnce(() => new Promise<TextDocument>((r) => { release = r }))
    const { result } = setup()
    act(() => result.current.setContent('<p>one</p>'))
    await tick(1000)
    expect(result.current.state).toBe('saving')

    act(() => result.current.setContent('<p>two</p>'))
    await tick(5000)
    expect(save).toHaveBeenCalledTimes(1) // the second waits for the first

    await act(async () => release(doc({ version: 2, content: '<p>one</p>' })))
    await tick(1000)
    expect(save).toHaveBeenCalledTimes(2)
    expect(save).toHaveBeenLastCalledWith({ base_version: 2, content: '<p>two</p>' })
    expect(result.current.state).toBe('saved')
  })

  it('saves a status change at once', async () => {
    const { result } = setup()
    act(() => result.current.setStatus('active'))
    await tick(0)
    expect(save).toHaveBeenCalledWith({ base_version: 1, status: 'active' })
    expect(result.current.status).toBe('active')
  })

  it('flush saves now instead of waiting', async () => {
    const { result } = setup()
    act(() => result.current.setContent('<p>now</p>'))
    await act(async () => { await result.current.flush() })
    expect(save).toHaveBeenCalledTimes(1)
  })
})

describe('useDocumentSave: when saving cannot happen', () => {
  it('says it is waiting for the network and retries, without losing the text', async () => {
    save.mockRejectedValueOnce(new TypeError('Failed to fetch'))
    const { result } = setup()
    act(() => result.current.setContent('<p>offline</p>'))
    await tick(1000)
    expect(result.current.state).toBe('offline')

    await tick(5000)
    expect(save).toHaveBeenCalledTimes(2)
    expect(save).toHaveBeenLastCalledWith({ base_version: 1, content: '<p>offline</p>' })
    expect(result.current.state).toBe('saved')
  })

  it('retries as soon as the browser is back online', async () => {
    save.mockRejectedValueOnce(new TypeError('Failed to fetch'))
    const { result } = setup()
    act(() => result.current.setContent('<p>x</p>'))
    await tick(1000)
    expect(result.current.state).toBe('offline')

    await act(async () => { window.dispatchEvent(new Event('online')) })
    await tick(0)
    expect(save).toHaveBeenCalledTimes(2)
    expect(result.current.state).toBe('saved')
  })

  it('reports a refusal as an error, keeps the text, and tries again on the next change', async () => {
    save.mockRejectedValueOnce(new ApiError('nope', 403))
    const { result } = setup()
    act(() => result.current.setContent('<p>x</p>'))
    await tick(1000)
    expect(result.current.state).toBe('error')
    expect(result.current.errorStatus).toBe(403)

    act(() => result.current.setContent('<p>xy</p>'))
    await tick(1000)
    expect(save).toHaveBeenCalledTimes(2)
    expect(result.current.state).toBe('saved')
  })
})

describe('useDocumentSave: a version conflict', () => {
  const theirs = doc({ version: 5, title: 'Của người khác', content: '<p>their words</p>' })
  const conflict = () => new ApiError('changed', 409, { reason: 'version_conflict', current: theirs })

  it('stops, keeps both versions, and never retries by itself', async () => {
    save.mockRejectedValueOnce(conflict())
    const { result } = setup()
    act(() => result.current.setContent('<p>mine</p>'))
    await tick(1000)

    expect(result.current.state).toBe('conflict')
    expect(result.current.conflict?.content).toBe('<p>their words</p>')
    await tick(30000)
    expect(save).toHaveBeenCalledTimes(1)

    act(() => result.current.setContent('<p>mine more</p>'))
    await tick(5000)
    expect(save).toHaveBeenCalledTimes(1) // typing on does not overwrite anything
    expect(result.current.state).toBe('conflict')
  })

  it('"take theirs" adopts their version and builds on it', async () => {
    save.mockRejectedValueOnce(conflict())
    const { result } = setup()
    act(() => result.current.setContent('<p>mine</p>'))
    await tick(1000)

    let taken: TextDocument | undefined
    act(() => { taken = result.current.takeTheirs() })
    expect(taken?.content).toBe('<p>their words</p>')
    expect(result.current.state).toBe('saved')
    expect(result.current.conflict).toBeNull()
    expect(save).toHaveBeenCalledTimes(1) // nothing of mine was written

    act(() => result.current.setContent('<p>their words, edited</p>'))
    await tick(1000)
    expect(save).toHaveBeenLastCalledWith({ base_version: 5, content: '<p>their words, edited</p>' })
  })

  it('"keep mine" saves my text over theirs only because it was asked for, on their version', async () => {
    save.mockRejectedValueOnce(conflict())
    const { result } = setup()
    act(() => result.current.setContent('<p>mine</p>'))
    act(() => result.current.setTitle('Tiêu đề của tôi'))
    await tick(1000)
    expect(result.current.state).toBe('conflict')

    await act(async () => { result.current.keepMine() })
    await tick(0)
    expect(save).toHaveBeenLastCalledWith({ base_version: 5, title: 'Tiêu đề của tôi', content: '<p>mine</p>' })
    expect(result.current.state).toBe('saved')
    expect(result.current.conflict).toBeNull()
  })

  it('a second conflict while keeping mine returns to the conflict state', async () => {
    save.mockRejectedValueOnce(conflict())
    save.mockRejectedValueOnce(new ApiError('changed', 409, { reason: 'version_conflict', current: doc({ version: 6, content: '<p>newer</p>' }) }))
    const { result } = setup()
    act(() => result.current.setContent('<p>mine</p>'))
    await tick(1000)
    await act(async () => { result.current.keepMine() })
    await tick(0)
    expect(result.current.state).toBe('conflict')
    expect(result.current.conflict?.version).toBe(6)
  })
})

describe('useDocumentSave: leaving', () => {
  it('saves unsaved text when the editor goes away', async () => {
    const { result, unmount } = setup()
    act(() => result.current.setContent('<p>last words</p>'))
    unmount()
    await tick(0)
    expect(save).toHaveBeenCalledWith({ base_version: 1, content: '<p>last words</p>' })
  })

  it('does not save over a conflict when the editor goes away', async () => {
    save.mockRejectedValueOnce(new ApiError('changed', 409, { reason: 'version_conflict', current: doc({ version: 5 }) }))
    const { result, unmount } = setup()
    act(() => result.current.setContent('<p>mine</p>'))
    await tick(1000)
    unmount()
    await tick(0)
    expect(save).toHaveBeenCalledTimes(1)
  })

  it('sends nothing more once the document is discarded', async () => {
    const { result, unmount } = setup()
    act(() => result.current.setContent('<p>x</p>'))
    act(() => result.current.discard())
    await tick(5000)
    unmount()
    await tick(0)
    expect(save).not.toHaveBeenCalled()
  })

  it('reports what is on the page, saved or not', () => {
    const { result } = setup()
    act(() => result.current.setContent('<p>typed</p>'))
    act(() => result.current.setTitle('Tên mới'))
    expect(result.current.getLocal()).toMatchObject({ content: '<p>typed</p>', title: 'Tên mới' })
  })

  it('warns before the page unloads while something is unsaved', async () => {
    const { result } = setup()
    const event = new Event('beforeunload', { cancelable: true })
    window.dispatchEvent(event)
    expect(event.defaultPrevented).toBe(false)

    act(() => result.current.setContent('<p>x</p>'))
    const dirty = new Event('beforeunload', { cancelable: true })
    window.dispatchEvent(dirty)
    expect(dirty.defaultPrevented).toBe(true)
  })
})

describe('useDocumentSave: after the editor is gone', () => {
  it('stops retrying: an offline failure at close schedules nothing and says so', async () => {
    save.mockRejectedValue(new TypeError('Failed to fetch'))
    const onUnsaved = vi.fn()
    const { result, unmount } = renderHook(() => useDocumentSave({ initial, save, delay: 1000, onUnsaved }))
    act(() => result.current.setContent('<p>x</p>'))
    unmount()
    await tick(0)
    expect(save).toHaveBeenCalledTimes(1)
    expect(onUnsaved).toHaveBeenCalledWith('offline')

    await tick(60000)
    expect(save).toHaveBeenCalledTimes(1) // no timer was left running
  })

  it('stops retrying a retry that was already waiting when the editor goes', async () => {
    save.mockRejectedValue(new TypeError('Failed to fetch'))
    const { result, unmount } = renderHook(() => useDocumentSave({ initial, save, delay: 1000 }))
    act(() => result.current.setContent('<p>x</p>'))
    await tick(1000)
    expect(result.current.state).toBe('offline')
    unmount()
    await tick(60000)
    expect(save).toHaveBeenCalledTimes(2) // the close-time attempt, then nothing
  })

  it('tells the person when the close-time save is refused as a conflict', async () => {
    save.mockRejectedValueOnce(new ApiError('changed', 409, { reason: 'version_conflict', current: doc({ version: 7 }) }))
    const onUnsaved = vi.fn()
    const { result, unmount } = renderHook(() => useDocumentSave({ initial, save, delay: 1000, onUnsaved }))
    act(() => result.current.setContent('<p>mine</p>'))
    unmount()
    await tick(0)
    expect(onUnsaved).toHaveBeenCalledWith('conflict')
    expect(save).toHaveBeenCalledTimes(1)
  })

  it('tells the person when the close-time save fails for any other reason', async () => {
    save.mockRejectedValueOnce(new ApiError('boom', 500))
    const onUnsaved = vi.fn()
    const { result, unmount } = renderHook(() => useDocumentSave({ initial, save, delay: 1000, onUnsaved }))
    act(() => result.current.setContent('<p>x</p>'))
    unmount()
    await tick(0)
    expect(onUnsaved).toHaveBeenCalledWith('error')
  })

  it('does not send a close-time save while offline: it says so instead', async () => {
    const onLine = vi.spyOn(navigator, 'onLine', 'get').mockReturnValue(false)
    const onUnsaved = vi.fn()
    const { result, unmount } = renderHook(() => useDocumentSave({ initial, save, delay: 1000, onUnsaved }))
    act(() => result.current.setContent('<p>x</p>'))
    unmount()
    await tick(0)
    expect(save).not.toHaveBeenCalled()
    expect(onUnsaved).toHaveBeenCalledWith('offline')
    onLine.mockRestore()
  })

  it('a save that finishes after the editor went does not schedule another', async () => {
    let release!: (d: TextDocument) => void
    save.mockImplementationOnce(() => new Promise<TextDocument>((r) => { release = r }))
    const { result, unmount } = renderHook(() => useDocumentSave({ initial, save, delay: 1000 }))
    act(() => result.current.setContent('<p>one</p>'))
    await tick(1000)
    act(() => result.current.setContent('<p>two</p>')) // typed during the save
    unmount()
    await act(async () => release(doc({ version: 2, content: '<p>one</p>' })))
    await tick(60000)
    expect(save).toHaveBeenCalledTimes(1)
  })
})

describe('useDocumentSave: pause and resume', () => {
  it('sends nothing while paused, and carries on after resume', async () => {
    const { result } = setup()
    act(() => result.current.setContent('<p>x</p>'))
    act(() => result.current.pause())
    await tick(10000)
    expect(save).not.toHaveBeenCalled()

    act(() => result.current.resume())
    await tick(1000)
    expect(save).toHaveBeenCalledTimes(1)
  })

  it('does not save on close while paused', async () => {
    const { result, unmount } = setup()
    act(() => result.current.setContent('<p>x</p>'))
    act(() => result.current.pause())
    unmount()
    await tick(0)
    expect(save).not.toHaveBeenCalled()
  })
})
