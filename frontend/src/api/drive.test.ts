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
