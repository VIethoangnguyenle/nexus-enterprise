import { beforeEach, describe, expect, it } from 'vitest'
import { useDriveStore } from './drive.store'

beforeEach(() => useDriveStore.setState({ selectedItemId: null, expandedFolders: new Set() }))

describe('drive store', () => {
  it('keeps no location: the open folder is the URL, not state', () => {
    const keys = Object.keys(useDriveStore.getState())
    for (const gone of ['currentFolderId', 'folderStack', 'activePath', 'navigateToFolder', 'viewMode', 'selectedItemIds']) {
      expect(keys).not.toContain(gone)
    }
  })

  it('selects and clears the item shown in the detail panel', () => {
    useDriveStore.getState().selectItem('a')
    expect(useDriveStore.getState().selectedItemId).toBe('a')
    useDriveStore.getState().selectItem(null)
    expect(useDriveStore.getState().selectedItemId).toBeNull()
  })

  it('toggles a folder open and closed', () => {
    const { toggleFolder } = useDriveStore.getState()
    toggleFolder('f1')
    expect(useDriveStore.getState().expandedFolders.has('f1')).toBe(true)
    toggleFolder('f1')
    expect(useDriveStore.getState().expandedFolders.has('f1')).toBe(false)
  })

  it('opens several folders at once and leaves the state alone when they are already open', () => {
    const { expandFolders } = useDriveStore.getState()
    expandFolders(['a', 'b'])
    const first = useDriveStore.getState().expandedFolders
    expect([...first].sort()).toEqual(['a', 'b'])
    expandFolders(['a'])
    expect(useDriveStore.getState().expandedFolders).toBe(first)
  })
})
