import { describe, expect, it } from 'vitest'
import { folderSearch, sharedSearch, validateDriveSearch } from './drive-search'

const F = '66666666-aaaa-4bbb-8ccc-000000000001'

describe('validateDriveSearch', () => {
  it('keeps the workspace, the folder and the shared view', () => {
    expect(validateDriveSearch({ ws: 'w1', folder: F })).toEqual({ ws: 'w1', folder: F })
    expect(validateDriveSearch({ view: 'shared' })).toEqual({ view: 'shared' })
  })

  it('drops anything that is not a folder id or a known view, so a bad link opens the root', () => {
    expect(validateDriveSearch({ folder: 42, view: 'trash' })).toEqual({})
    expect(validateDriveSearch({ folder: '' })).toEqual({})
    expect(validateDriveSearch({})).toEqual({})
  })
})

describe('search builders', () => {
  it('opening a folder replaces the previous folder and leaves the shared view', () => {
    expect(folderSearch({ ws: 'w1', folder: 'old', view: 'shared' }, F)).toEqual({ ws: 'w1', folder: F })
  })

  it('opening the root clears the folder but keeps the workspace', () => {
    expect(folderSearch({ ws: 'w1', folder: F })).toEqual({ ws: 'w1' })
  })

  it('the shared view clears the folder but keeps the workspace', () => {
    expect(sharedSearch({ ws: 'w1', folder: F })).toEqual({ ws: 'w1', view: 'shared' })
  })
})
