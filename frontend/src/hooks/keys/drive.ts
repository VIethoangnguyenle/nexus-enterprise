/**
 * Drive query keys. Everything listed per workspace sits under `all(ws)`, so
 * one prefix invalidates a workspace's folders, quota and channel drives.
 * Items and shares are addressed by their own id and live outside it.
 */
export const driveKeys = {
  /** Every drive query, any workspace. For resync after a reconnect. */
  everything: () => ['drive'] as const,
  all: (wsId: string) => ['drive', wsId] as const,
  /** Prefix of every cached folder listing, root included. */
  folders: (wsId: string) => ['drive', wsId, 'folder'] as const,
  /** A folder listing; no `folderId` is the workspace root. */
  folder: (wsId: string, folderId?: string) => ['drive', wsId, 'folder', folderId || 'root'] as const,
  quota: (wsId: string) => ['drive', wsId, 'quota'] as const,
  channel: (wsId: string, channelId: string) => ['drive', wsId, 'channel', channelId] as const,
  item: (itemId: string) => ['drive', 'item', itemId] as const,
  sharedWithMe: () => ['drive', 'shared-with-me'] as const,
  shares: (itemId: string) => ['drive', 'shares', itemId] as const,
  /** Presigned download URL of a file; short-lived, so cached for minutes, not the session. */
  downloadUrl: (fileId: string) => ['drive', 'download-url', fileId] as const,
  /** Items the user has starred in a workspace; re-checked for read on every fetch. */
  starred: (wsId: string) => ['drive', wsId, 'starred'] as const,
  /** Recently relevant items for the phone's Quick Access carousel. */
  quickAccess: (wsId: string) => ['drive', wsId, 'quick-access'] as const,
  /** The workspace's files the user can read, last edited first (the phone's Gần đây). */
  recent: (wsId: string) => ['drive', wsId, 'recent'] as const,
}
