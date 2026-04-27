import { createFileRoute } from '@tanstack/react-router'
import { useState, useRef } from 'react'
import { useDocuments } from '../../hooks/useDocuments'
import { useWorkspaces } from '../../hooks/useWorkspaces'
import { documentApi } from '../../api/documents'
import { LoadingState } from '../../components/LoadingState'
import { ErrorState } from '../../components/ErrorState'
import { EmptyState } from '../../components/EmptyState'
import { Button, Badge, Heading, Spinner } from '../../components/primitives'
import { Card } from '../../components/composites'

export const Route = createFileRoute('/_workspace/documents')({ component: DocumentsPage })

function DocumentsPage() {
  const { data: wsData } = useWorkspaces()
  const wsId = wsData?.workspaces?.[0]?.id || ''
  const { data, isLoading, error, refetch } = useDocuments(wsId)
  const [uploading, setUploading] = useState(false)
  const [uploadStep, setUploadStep] = useState('')
  const fileInputRef = useRef<HTMLInputElement>(null)

  if (isLoading) return <LoadingState />
  if (error) return <ErrorState title="Failed to load documents" message={error.message} onRetry={() => refetch()} />

  const docs = data?.documents || []

  const handleUpload = async () => {
    const file = fileInputRef.current?.files?.[0]
    if (!file || !wsId) return

    setUploading(true)
    try {
      setUploadStep('Getting upload URL...')
      const title = file.name.replace(/\.[^/.]+$/, '')

      setUploadStep('Uploading to storage...')
      await documentApi.create(wsId, file, title)

      setUploadStep('Done!')
      refetch()
      if (fileInputRef.current) fileInputRef.current.value = ''
    } catch (err: any) {
      console.error('Upload failed:', err.message)
    } finally {
      setUploading(false)
      setUploadStep('')
    }
  }

  const handleDownload = async (docId: string, filename: string) => {
    try {
      const { download_url } = await documentApi.getDownloadUrl(docId)
      const a = document.createElement('a')
      a.href = download_url
      a.download = filename
      a.click()
    } catch (err: any) {
      console.error('Download failed:', err.message)
    }
  }

  return (
    <div className="animate-fade-in">
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <Heading as="h2">Documents</Heading>
        <div className="flex items-center gap-3">
          <input
            ref={fileInputRef}
            type="file"
            id="doc-file-input"
            className="text-sm text-text-secondary file:mr-3 file:py-1.5 file:px-3
              file:rounded-[var(--radius-sm)] file:border file:border-border
              file:bg-bg-glass file:text-text-primary file:text-sm file:cursor-pointer
              file:transition-colors file:hover:bg-bg-hover"
          />
          <Button
            id="upload-btn"
            onClick={handleUpload}
            disabled={uploading}
            size="md"
          >
            {uploading ? <><Spinner size="sm" /> {uploadStep}</> : '📤 Upload'}
          </Button>
        </div>
      </div>

      {/* Documents table */}
      {docs.length > 0 ? (
        <Card>
          <table className="w-full border-collapse">
            <thead>
              <tr>
                <th className="text-left px-4 py-2.5 text-[0.7rem] font-semibold text-text-muted uppercase tracking-wider border-b border-border">Title</th>
                <th className="text-left px-4 py-2.5 text-[0.7rem] font-semibold text-text-muted uppercase tracking-wider border-b border-border">File</th>
                <th className="text-left px-4 py-2.5 text-[0.7rem] font-semibold text-text-muted uppercase tracking-wider border-b border-border">Status</th>
                <th className="text-left px-4 py-2.5 text-[0.7rem] font-semibold text-text-muted uppercase tracking-wider border-b border-border">Created</th>
                <th className="text-left px-4 py-2.5 text-[0.7rem] font-semibold text-text-muted uppercase tracking-wider border-b border-border">Actions</th>
              </tr>
            </thead>
            <tbody>
              {docs.map(d => (
                <tr key={d.id} className="border-b border-border/50 hover:bg-bg-hover transition-colors">
                  <td className="px-4 py-3 text-sm font-medium text-text-primary">{d.title || d.filename}</td>
                  <td className="px-4 py-3 text-sm text-text-muted">{d.filename}</td>
                  <td className="px-4 py-3">
                    <Badge variant="primary">{d.status}</Badge>
                  </td>
                  <td className="px-4 py-3 text-sm text-text-muted">
                    {d.created_at ? new Date(d.created_at).toLocaleDateString() : ''}
                  </td>
                  <td className="px-4 py-3">
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => handleDownload(d.id, d.filename)}
                      title="Download"
                    >
                      📥
                    </Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </Card>
      ) : <EmptyState icon="📄" title="No documents" description="Upload your first document to get started." />}
    </div>
  )
}
