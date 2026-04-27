import { apiFetch } from './client'

export interface DriveItem {
  id: string
  workspace_id: string
  drive_context: string
  drive_context_id: string
  parent_id: string
  item_type: 'file' | 'folder'
  name: string
  mime_type: string
  size_bytes: number
  object_key: string
  ngac_node_id: string
  owner_id: string
  status: string
  created_at: string
  updated_at: string
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
  created_by: string
  created_at: string
}

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
    apiFetch<{ items: DriveItem[] }>(`/workspaces/${wsId}/drive`),

  listFolder: (folderId: string) =>
    apiFetch<{ items: DriveItem[] }>(`/drive/folders/${folderId}`),

  createFolder: (wsId: string, name: string, parentId?: string) =>
    apiFetch<DriveItem>(`/workspaces/${wsId}/drive/folders`, {
      method: 'POST',
      body: JSON.stringify({ name, parent_id: parentId || '' }),
    }),

  // — Items —
  getItem: (itemId: string) =>
    apiFetch<DriveItem>(`/drive/items/${itemId}`),

  moveItem: (itemId: string, newParentId: string) =>
    apiFetch<DriveItem>(`/drive/items/${itemId}/move`, {
      method: 'POST',
      body: JSON.stringify({ new_parent_id: newParentId }),
    }),

  copyItem: (itemId: string, destParentId: string, destWorkspaceId?: string) =>
    apiFetch<DriveItem>(`/drive/items/${itemId}/copy`, {
      method: 'POST',
      body: JSON.stringify({ dest_parent_id: destParentId, dest_workspace_id: destWorkspaceId }),
    }),

  renameItem: (itemId: string, newName: string) =>
    apiFetch<DriveItem>(`/drive/items/${itemId}/rename`, {
      method: 'PUT',
      body: JSON.stringify({ new_name: newName }),
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
      body: JSON.stringify({ filename, mime_type: mimeType, size_bytes: sizeBytes, parent_id: parentId || '' }),
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
  createShare: (itemId: string, shareType: string, targetNgacId: string, operations: string[]) =>
    apiFetch<DriveShare>(`/drive/items/${itemId}/share`, {
      method: 'POST',
      body: JSON.stringify({ share_type: shareType, target_ngac_id: targetNgacId, operations }),
    }),

  revokeShare: (shareId: string) =>
    apiFetch(`/drive/shares/${shareId}`, { method: 'DELETE' }),

  listShares: (itemId: string) =>
    apiFetch<{ shares: DriveShare[] }>(`/drive/items/${itemId}/shares`),

  sharedWithMe: () =>
    apiFetch<{ items: DriveItem[] }>(`/drive/shared-with-me`),

  // — Channel drives —
  channelDrive: (channelId: string) =>
    apiFetch<{ items: DriveItem[] }>(`/channels/${channelId}/drive`),

  // — Quota —
  getQuota: (wsId: string) =>
    apiFetch<DriveQuota>(`/workspaces/${wsId}/drive/quota`),
}
