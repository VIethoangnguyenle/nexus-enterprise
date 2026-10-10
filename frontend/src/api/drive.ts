import { apiFetch } from './client'

/** A protobuf Timestamp as the REST layer serialises it, or an ISO string. `lib/format` reads both. */
export type DriveTimestamp = string | { seconds: number | string; nanos?: number }

/**
 * A file or folder as the drive REST returns it. The server omits empty
 * fields, so everything a folder or a root item lacks is optional.
 */
export interface DriveItem {
  id: string
  workspace_id: string
  drive_context: string
  drive_context_id?: string
  parent_id?: string
  item_type: 'file' | 'folder'
  name: string
  mime_type?: string
  size_bytes?: number
  object_key?: string
  ngac_node_id: string
  /** A user id on files, an NGAC node id on folders. Never shown; see `owner_name`. */
  owner_id: string
  /** Display name of the owner, when the owner is a person. */
  owner_name?: string
  status: string
  created_at: DriveTimestamp
  updated_at: DriveTimestamp
}

export interface DriveBreadcrumb {
  id: string
  name: string
}

/** A folder's contents. An empty folder comes back as `{}`. */
export interface DriveListing {
  items?: DriveItem[]
  /** Path from the top of the tree to this folder; present for folder listings, not the root. */
  breadcrumb?: DriveBreadcrumb[]
}

export interface DriveQuota {
  workspace_id: string
  max_bytes: number
  used_bytes: number
  max_files: number
  used_files: number
}

export interface DriveShare {
  id: string
  drive_item_id: string
  share_type: string
  target_ngac_id: string
  target_label: string
  operations: string[]
  created_by?: string
  created_at: DriveTimestamp
}

/** How much a share lets the grantee do. Mapped to NGAC operations server-side. */
export type SharePermission = 'read' | 'write'

interface CreateFileResponse {
  file_id: string
  upload_url: string
  object_key: string
}

interface DownloadURLResponse {
  download_url: string
}

export const driveApi = {
  // — Folders —
  listRoot: (wsId: string) =>
    apiFetch<DriveListing>(`/workspaces/${wsId}/drive`),

  /** wsId lets the drive refuse a folder that belongs to another workspace. */
  listFolder: (wsId: string, folderId: string) =>
    apiFetch<DriveListing>(`/drive/folders/${folderId}?ws=${encodeURIComponent(wsId)}`),

  createFolder: (wsId: string, name: string, parentId?: string) =>
    apiFetch<DriveItem>(`/workspaces/${wsId}/drive/folders`, {
      method: 'POST',
      body: JSON.stringify({ name, parent_id: parentId || '' }),
    }),

  // — Items —
  getItem: (itemId: string) =>
    apiFetch<DriveItem>(`/drive/items/${itemId}`),

  // The REST handler binds `target_folder_id` and `name`. Other spellings are
  // ignored without an error, and a rename then blanks the item's name.
  moveItem: (itemId: string, targetFolderId: string) =>
    apiFetch<DriveItem>(`/drive/items/${itemId}/move`, {
      method: 'POST',
      body: JSON.stringify({ target_folder_id: targetFolderId }),
    }),

  copyItem: (itemId: string, targetFolderId: string) =>
    apiFetch<DriveItem>(`/drive/items/${itemId}/copy`, {
      method: 'POST',
      body: JSON.stringify({ target_folder_id: targetFolderId }),
    }),

  renameItem: (itemId: string, name: string) =>
    apiFetch<DriveItem>(`/drive/items/${itemId}/rename`, {
      method: 'PUT',
      body: JSON.stringify({ name }),
    }),

  trashItem: (itemId: string) =>
    apiFetch(`/drive/items/${itemId}`, { method: 'DELETE' }),

  restoreItem: (itemId: string) =>
    apiFetch(`/drive/items/${itemId}/restore`, { method: 'POST' }),

  deleteItem: (itemId: string) =>
    apiFetch(`/drive/items/${itemId}/permanent`, { method: 'DELETE' }),

  // — Files —
  /** Step 1: Create file record and get presigned upload URL. */
  createFile: (wsId: string, filename: string, mimeType: string, sizeBytes: number, parentId?: string) =>
    apiFetch<CreateFileResponse>(`/workspaces/${wsId}/drive/files`, {
      method: 'POST',
      body: JSON.stringify({ name: filename, mime_type: mimeType, size_bytes: sizeBytes, parent_id: parentId || '' }),
    }),

  /** Step 2: Upload file directly to MinIO via presigned PUT URL. */
  uploadToStorage: async (uploadUrl: string, file: File): Promise<void> => {
    const res = await fetch(uploadUrl, {
      method: 'PUT',
      body: file,
      headers: { 'Content-Type': file.type || 'application/octet-stream' },
    })
    if (!res.ok) throw new Error(`Storage upload failed: ${res.status}`)
  },

  /** Step 3: Confirm upload. */
  confirmFile: (fileId: string) =>
    apiFetch(`/drive/files/${fileId}/confirm`, { method: 'POST' }),

  /** Orchestrated 3-step upload. */
  upload: async (wsId: string, file: File, parentId?: string): Promise<void> => {
    const { file_id, upload_url } = await driveApi.createFile(
      wsId, file.name, file.type || 'application/octet-stream', file.size, parentId,
    )
    await driveApi.uploadToStorage(upload_url, file)
    await driveApi.confirmFile(file_id)
  },

  getDownloadUrl: (fileId: string) =>
    apiFetch<DownloadURLResponse>(`/drive/files/${fileId}/download`),

  // — Sharing —
  createShare: (itemId: string, shareType: string, targetNodeId: string, permission: SharePermission) =>
    apiFetch<DriveShare>(`/drive/items/${itemId}/share`, {
      method: 'POST',
      body: JSON.stringify({ share_type: shareType, target_node_id: targetNodeId, permission }),
    }),

  revokeShare: (shareId: string) =>
    apiFetch(`/drive/shares/${shareId}`, { method: 'DELETE' }),

  listShares: (itemId: string) =>
    apiFetch<{ shares?: DriveShare[] }>(`/drive/items/${itemId}/shares`),

  sharedWithMe: () =>
    apiFetch<DriveListing>(`/drive/shared-with-me`),

  /** List drive items for a channel (uses Drive service ListRoot with context filter). */
  channelDrive: (wsId: string, channelId: string) =>
    apiFetch<DriveListing>(`/workspaces/${wsId}/drive?drive_context=channel&drive_context_id=${channelId}`),

  // — Quota —
  getQuota: (wsId: string) =>
    apiFetch<DriveQuota>(`/workspaces/${wsId}/drive/quota`),
}
