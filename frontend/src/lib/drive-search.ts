import { validateWorkspaceSearch } from './workspace'

/**
 * What the Tài liệu URL carries: the workspace (`ws`), the open folder
 * (`folder`, absent at the root) and the "Được chia sẻ với tôi" view. Keeping
 * the folder here, not in a store, is what makes reload, back and forward land
 * where the user was, and what lets a folder be linked.
 */
export interface DriveSearch {
  ws?: string
  folder?: string
  /** `shared`: files shared with me. `texts`: the documents written in the app (Văn bản). */
  view?: 'shared' | 'texts'
  /** Which Văn bản, with `view=texts`: absent is all of them. */
  group?: 'drafts' | 'shared'
}

/**
 * `validateSearch` for the drive route. Unknown or malformed values are
 * dropped rather than rejected, so a hand-edited link opens the root instead
 * of an error page.
 */
export function validateDriveSearch(search: Record<string, unknown>): DriveSearch {
  const out: DriveSearch = { ...validateWorkspaceSearch(search) }
  if (typeof search.folder === 'string' && search.folder) out.folder = search.folder
  if (search.view === 'shared') out.view = 'shared'
  if (search.view === 'texts') {
    out.view = 'texts'
    if (search.group === 'drafts' || search.group === 'shared') out.group = search.group
  }
  return out
}

/** Search for opening a folder (or the root, with no id), keeping the workspace. */
export function folderSearch(prev: DriveSearch, folderId?: string): DriveSearch {
  const { folder: _f, view: _v, group: _g, ...rest } = prev
  return folderId ? { ...rest, folder: folderId } : rest
}

/** Search for the shared-with-me view, keeping the workspace. */
export function sharedSearch(prev: DriveSearch): DriveSearch {
  const { folder: _f, group: _g, ...rest } = prev
  return { ...rest, view: 'shared' }
}

/** Search for the Văn bản view (all of them, or the drafts / shared group), keeping the workspace. */
export function textsSearch(prev: DriveSearch, group?: 'drafts' | 'shared'): DriveSearch {
  const { folder: _f, group: _g, ...rest } = prev
  return group ? { ...rest, view: 'texts', group } : { ...rest, view: 'texts' }
}
