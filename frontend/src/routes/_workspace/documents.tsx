import { createFileRoute } from '@tanstack/react-router'
import { useState, useRef } from 'react'
import { useDocuments } from '../../hooks/useDocuments'
import { useWorkspaces } from '../../hooks/useWorkspaces'
import { documentApi } from '../../api/documents'
import { queryClient } from '../../lib/query-client'
import { LoadingState } from '../../components/LoadingState'
import { ErrorState } from '../../components/ErrorState'
import { EmptyState } from '../../components/EmptyState'
import { Button, Badge, Heading, Spinner } from '../../components/primitives'
import { Card } from '../../components/composites'
import { Upload, Download, FileText } from 'lucide-react'

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
      await queryClient.invalidateQueries({ queryKey: ['documents', wsId] })
      if (fileInputRef.current) fileInputRef.current.value = ''
    } catch (err: any) {
      console.error('Upload failed:', err.message)
      setUploadStep(`Error: ${err.message}`)
      setTimeout(() => setUploadStep(''), 3000)
    } finally {
      setUploading(false)
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
      <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between mb-4 md:mb-6">
        <Heading as="h2">Documents</Heading>
        <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:gap-3">
          <input
            ref={fileInputRef}
            type="file"
            id="doc-file-input"
            className="text-small text-on-surface-variant file:mr-3 file:py-2 file:px-3
              file:rounded-md file:border file:border-outline-variant
              file:bg-surface-container file:text-on-surface file:text-small file:cursor-pointer
              file:transition-colors file:hover:bg-surface-container-high w-full sm:w-auto"
          />
          <Button
            id="upload-btn"
            onClick={handleUpload}
            disabled={uploading}
            size="md"
          >
            {uploading ? <><Spinner size="sm" /> {uploadStep}</> : <><Upload size={14} className="inline mr-1" /> Upload</>}
          </Button>
        </div>
      </div>

      {/* Documents table */}
      {docs.length > 0 ? (
        <Card>
          <div className="overflow-x-auto">
            <table className="w-full border-collapse min-w-[400px]">
              <thead>
                <tr>
                  <th className="text-left px-3 py-2 text-caption-ui text-on-surface-variant uppercase tracking-wider border-b border-outline-variant bg-surface-container">Title</th>
                  <th className="text-left px-3 py-2 text-caption-ui text-on-surface-variant uppercase tracking-wider border-b border-outline-variant bg-surface-container hidden md:table-cell">File</th>
                  <th className="text-left px-3 py-2 text-caption-ui text-on-surface-variant uppercase tracking-wider border-b border-outline-variant bg-surface-container hidden sm:table-cell">Status</th>
                  <th className="text-left px-3 py-2 text-caption-ui text-on-surface-variant uppercase tracking-wider border-b border-outline-variant bg-surface-container hidden md:table-cell">Created</th>
                  <th className="text-left px-3 py-2 text-caption-ui text-on-surface-variant uppercase tracking-wider border-b border-outline-variant bg-surface-container">Actions</th>
                </tr>
              </thead>
              <tbody>
                {docs.map(d => (
                  <tr key={d.id} className="border-b border-outline-variant-subtle hover:bg-surface-container-high transition-colors duration-instant h-9">
                    <td className="px-3 py-0 text-small text-on-surface font-medium truncate max-w-[200px]">{d.title || d.filename}</td>
                    <td className="px-3 py-0 text-small text-on-surface-variant hidden md:table-cell">{d.filename}</td>
                    <td className="px-4 py-3 hidden sm:table-cell">
                      <Badge variant="primary">{d.status}</Badge>
                    </td>
                    <td className="px-3 py-0 text-small text-on-surface-variant hidden md:table-cell">
                      {d.created_at ? new Date(d.created_at).toLocaleDateString() : ''}
                    </td>
                    <td className="px-4 py-3">
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => handleDownload(d.id, d.filename)}
                        title="Download"
                      >
                        <Download size={14} />
                      </Button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </Card>
      ) : <EmptyState icon={<FileText size={40} color="#3b82f6" strokeWidth={1.5} />} title="No documents" description="Upload your first document to get started." />}
    </div>
  )
}
