import { describe, it, expect, vi, beforeEach } from 'vitest'

const apiFetch = vi.fn()
vi.mock('./client', () => ({ apiFetch: (...args: unknown[]) => apiFetch(...args) }))

import { driveApi } from './drive'

beforeEach(() => apiFetch.mockReset())

describe('driveApi.listFolder', () => {
  it('names the workspace being browsed so the drive refuses a folder of another one', () => {
    driveApi.listFolder('ws-1', 'folder-9')
    expect(apiFetch).toHaveBeenCalledWith('/drive/folders/folder-9?ws=ws-1')
  })
})

// An upload URL may be used once: the PUT asks storage to refuse when an object
// already sits at the key, so a leaked URL cannot overwrite a confirmed file.
describe('uploads to storage are write-once', () => {
  it('sends If-None-Match: * with fetch', async () => {
    const fetchMock = vi.fn().mockResolvedValue({ ok: true, status: 200 })
    vi.stubGlobal('fetch', fetchMock)
    try {
      await driveApi.uploadToStorage('https://storage.test/u', new File(['abc'], 'a.txt', { type: 'text/plain' }))
      const init = fetchMock.mock.calls[0]?.[1] as RequestInit
      expect(init.method).toBe('PUT')
      expect(init.headers).toMatchObject({ 'Content-Type': 'text/plain', 'If-None-Match': '*' })
    } finally {
      vi.unstubAllGlobals()
    }
  })

  it('sends If-None-Match: * with the progress upload', async () => {
    const headers: Record<string, string> = {}
    class FakeXHR {
      upload = { onprogress: null }
      status = 200
      onload: (() => void) | null = null
      onerror = null
      onabort = null
      open() {}
      setRequestHeader(k: string, v: string) { headers[k] = v }
      send() { this.onload?.() }
      abort() {}
    }
    vi.stubGlobal('XMLHttpRequest', FakeXHR)
    try {
      await driveApi.putWithProgress('https://storage.test/u', new Blob(['abc']), () => {})
      expect(headers['If-None-Match']).toBe('*')
    } finally {
      vi.unstubAllGlobals()
    }
  })
})
