import { apiFetch, apiUpload } from './client'

export interface Document {
  id: string
  title: string
  filename: string
  mime_type: string
  status: string
  owner_id?: string
  owner_name?: string
  ngac_node_id?: string
  workspace_id?: string
  created_at?: string
}

interface UploadURLResponse {
  upload_url: string
  doc_id: string
  object_key: string
}

interface DownloadURLResponse {
  download_url: string
}

export const documentApi = {
  /** Lists documents (proxied to Drive ListFolder — returns items not documents). */
  list: async (wsId: string): Promise<{ documents: Document[] }> => {
    const resp = await apiFetch<{ items?: any[] }>(`/workspaces/${wsId}/documents`)
    const items = resp?.items || []
    return {
      documents: items
        .filter(i => i.item_type === 'file' && i.status === 'active')
        .map(i => ({
          id: i.id,
          title: i.name?.replace(/\.[^/.]+$/, '') || i.name,
          filename: i.name,
          mime_type: i.mime_type || '',
          status: i.status || 'active',
          owner_id: i.owner_id,
          owner_name: i.owner_name,
          ngac_node_id: i.ngac_node_id,
          workspace_id: i.workspace_id,
          created_at: i.created_at?.seconds
            ? new Date(Number(i.created_at.seconds) * 1000).toISOString()
            : i.created_at,
        } as Document)),
    }
  },

  get: (id: string) =>
    apiFetch<Document>(`/documents/${id}`),

  /** Step 1: Get a presigned PUT URL for direct-to-MinIO upload. */
  getUploadUrl: (wsId: string, metadata: { title: string; filename: string; mime_type: string }) =>
    apiFetch<UploadURLResponse>(`/workspaces/${wsId}/documents/upload-url`, {
      method: 'POST',
      body: JSON.stringify(metadata),
    }),

  /** Step 2: Upload file directly to MinIO via presigned PUT URL. */
  uploadToMinIO: async (uploadUrl: string, file: File): Promise<void> => {
    const res = await fetch(uploadUrl, {
      method: 'PUT',
      body: file,
      headers: { 'Content-Type': file.type || 'application/octet-stream' },
    })
    if (!res.ok) {
      throw new Error(`MinIO upload failed: ${res.status} ${res.statusText}`)
    }
  },

  /** Step 3: Confirm the upload completed and finalize the document record. */
  confirmUpload: (docId: string) =>
    apiFetch<Document>(`/documents/${docId}/confirm`, { method: 'POST' }),

  /** Orchestrated three-step presigned upload flow. */
  create: async (wsId: string, file: File, title: string): Promise<Document> => {
    // Step 1: Get presigned URL
    const { upload_url, doc_id } = await documentApi.getUploadUrl(wsId, {
      title,
      filename: file.name,
      mime_type: file.type || 'application/octet-stream',
    })

    // Step 2: Upload to MinIO directly
    await documentApi.uploadToMinIO(upload_url, file)

    // Step 3: Confirm
    return documentApi.confirmUpload(doc_id)
  },

  /** Legacy upload via multipart (backward compat). */
  legacyCreate: (wsId: string, data: FormData) =>
    apiUpload<Document>(`/workspaces/${wsId}/documents`, data),

  /** Get a presigned download URL (access-checked). */
  getDownloadUrl: (docId: string) =>
    apiFetch<DownloadURLResponse>(`/documents/${docId}/download-url`),

  delete: (id: string) =>
    apiFetch(`/documents/${id}`, { method: 'DELETE' }),

  approve: (id: string) =>
    apiFetch(`/documents/${id}/approve`, { method: 'POST' }),

  share: (id: string, data: object) =>
    apiFetch(`/documents/${id}/share`, { method: 'POST', body: JSON.stringify(data) }),
}
