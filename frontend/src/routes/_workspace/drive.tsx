import { createFileRoute } from '@tanstack/react-router'
import { useState, useRef, useCallback } from 'react'
import { useWorkspaces } from '../../hooks/useWorkspaces'
import { useDriveFolder, useCreateFolder, useUploadFile, useTrashItem, useDriveQuota } from '../../hooks/useDrive'
import { driveApi, type DriveItem } from '../../api/drive'
import { LoadingState } from '../../components/LoadingState'
import { ErrorState } from '../../components/ErrorState'
import { EmptyState } from '../../components/EmptyState'
import { Button, Heading, Spinner, Text, Badge } from '../../components/primitives'
import { Card } from '../../components/composites'

export const Route = createFileRoute('/_workspace/drive')({ component: DrivePage })

function DrivePage() {
  const { data: wsData } = useWorkspaces()
  const wsId = wsData?.workspaces?.[0]?.id || ''
  const [folderStack, setFolderStack] = useState<{ id: string; name: string }[]>([])
  const currentFolderId = folderStack.length > 0 ? folderStack[folderStack.length - 1].id : undefined

  const { data, isLoading, error, refetch } = useDriveFolder(wsId, currentFolderId)
  const createFolder = useCreateFolder(wsId)
  const uploadFile = useUploadFile(wsId)
  const trashItem = useTrashItem(wsId)
  const { data: quota } = useDriveQuota(wsId)

  const fileInputRef = useRef<HTMLInputElement>(null)
  const [showNewFolder, setShowNewFolder] = useState(false)
  const [newFolderName, setNewFolderName] = useState('')

  if (isLoading) return <LoadingState />
  if (error) return <ErrorState title="Failed to load drive" message={error.message} onRetry={() => refetch()} />

  const items = data?.items || []
  const folders = items.filter(i => i.item_type === 'folder')
  const files = items.filter(i => i.item_type === 'file')

  const navigateToFolder = (folder: DriveItem) => {
    setFolderStack(prev => [...prev, { id: folder.id, name: folder.name }])
  }

  const navigateBack = (index: number) => {
    setFolderStack(prev => prev.slice(0, index))
  }

  const handleCreateFolder = async () => {
    if (!newFolderName.trim()) return
    await createFolder.mutateAsync({ name: newFolderName.trim(), parentId: currentFolderId })
    setNewFolderName('')
    setShowNewFolder(false)
  }

  const handleUpload = async () => {
    const file = fileInputRef.current?.files?.[0]
    if (!file) return
    await uploadFile.mutateAsync({ file, parentId: currentFolderId })
    if (fileInputRef.current) fileInputRef.current.value = ''
  }

  const handleDownload = async (item: DriveItem) => {
    try {
      const { download_url } = await driveApi.getDownloadUrl(item.id)
      const a = document.createElement('a')
      a.href = download_url
      a.download = item.name
      a.click()
    } catch (err: any) {
      console.error('Download failed:', err.message)
    }
  }

  const handleTrash = async (itemId: string) => {
    if (!confirm('Move to trash?')) return
    await trashItem.mutateAsync(itemId)
  }

  const formatSize = (bytes: number) => {
    if (!bytes || bytes <= 0) return '—'
    if (bytes < 1024) return `${bytes} B`
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
    if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
    return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`
  }

  const getIcon = (item: DriveItem) => {
    if (item.item_type === 'folder') return '📁'
    const ext = item.name.split('.').pop()?.toLowerCase()
    const iconMap: Record<string, string> = {
      pdf: '📕', doc: '📘', docx: '📘', xls: '📗', xlsx: '📗',
      ppt: '📙', pptx: '📙', zip: '🗜️', rar: '🗜️',
      jpg: '🖼️', jpeg: '🖼️', png: '🖼️', gif: '🖼️', svg: '🖼️',
      mp4: '🎬', mov: '🎬', mp3: '🎵', wav: '🎵',
      js: '📜', ts: '📜', go: '📜', py: '📜', rs: '📜',
      md: '📝', txt: '📝', json: '📋', csv: '📊',
    }
    return iconMap[ext || ''] || '📄'
  }

  return (
    <div className="animate-fade-in">
      {/* Header */}
      <div className="flex items-center justify-between mb-4">
        <Heading as="h2">Drive</Heading>
        <div className="flex items-center gap-2">
          <input
            ref={fileInputRef}
            type="file"
            id="drive-file-input"
            className="text-sm text-text-secondary file:mr-2 file:py-1.5 file:px-3
              file:rounded-[var(--radius-sm)] file:border file:border-border
              file:bg-bg-glass file:text-text-primary file:text-sm file:cursor-pointer
              file:transition-colors file:hover:bg-bg-hover"
          />
          <Button
            id="drive-upload-btn"
            onClick={handleUpload}
            disabled={uploadFile.isPending}
            size="md"
          >
            {uploadFile.isPending ? <><Spinner size="sm" /> Uploading...</> : '📤 Upload'}
          </Button>
          <Button
            id="drive-new-folder-btn"
            onClick={() => setShowNewFolder(true)}
            size="md"
            variant="outline"
          >
            📁 New Folder
          </Button>
        </div>
      </div>

      {/* Quota bar */}
      {quota && quota.max_bytes > 0 && (
        <div className="mb-4 flex items-center gap-3">
          <div className="flex-1 h-1.5 bg-border rounded-full overflow-hidden">
            <div
              className="h-full bg-accent rounded-full transition-all"
              style={{ width: `${Math.min((quota.used_bytes / quota.max_bytes) * 100, 100)}%` }}
            />
          </div>
          <Text variant="caption" muted>
            {formatSize(quota.used_bytes)} / {formatSize(quota.max_bytes)}
          </Text>
        </div>
      )}

      {/* Breadcrumb */}
      <div className="flex items-center gap-1 mb-4 text-sm">
        <button
          onClick={() => navigateBack(0)}
          className="text-accent hover:underline cursor-pointer bg-transparent border-none p-0"
        >
          Root
        </button>
        {folderStack.map((f, i) => (
          <span key={f.id} className="flex items-center gap-1">
            <span className="text-text-muted">/</span>
            <button
              onClick={() => navigateBack(i + 1)}
              className={`bg-transparent border-none p-0 cursor-pointer
                ${i === folderStack.length - 1 ? 'text-text-primary font-medium' : 'text-accent hover:underline'}`}
            >
              {f.name}
            </button>
          </span>
        ))}
      </div>

      {/* New folder inline form */}
      {showNewFolder && (
        <Card className="mb-4 p-3 flex items-center gap-2">
          <span>📁</span>
          <input
            autoFocus
            type="text"
            value={newFolderName}
            onChange={e => setNewFolderName(e.target.value)}
            onKeyDown={e => e.key === 'Enter' && handleCreateFolder()}
            placeholder="Folder name"
            className="flex-1 px-2 py-1 text-sm bg-bg-glass border border-border rounded-[var(--radius-sm)]
              text-text-primary focus:outline-none focus:ring-1 focus:ring-accent"
          />
          <Button size="sm" onClick={handleCreateFolder} disabled={createFolder.isPending}>
            {createFolder.isPending ? <Spinner size="sm" /> : 'Create'}
          </Button>
          <Button size="sm" variant="ghost" onClick={() => { setShowNewFolder(false); setNewFolderName('') }}>
            Cancel
          </Button>
        </Card>
      )}

      {/* Contents */}
      {items.length > 0 ? (
        <Card>
          <table className="w-full border-collapse">
            <thead>
              <tr>
                <th className="text-left px-4 py-2.5 text-[0.7rem] font-semibold text-text-muted uppercase tracking-wider border-b border-border">Name</th>
                <th className="text-left px-4 py-2.5 text-[0.7rem] font-semibold text-text-muted uppercase tracking-wider border-b border-border">Type</th>
                <th className="text-left px-4 py-2.5 text-[0.7rem] font-semibold text-text-muted uppercase tracking-wider border-b border-border">Size</th>
                <th className="text-left px-4 py-2.5 text-[0.7rem] font-semibold text-text-muted uppercase tracking-wider border-b border-border">Modified</th>
                <th className="text-left px-4 py-2.5 text-[0.7rem] font-semibold text-text-muted uppercase tracking-wider border-b border-border">Actions</th>
              </tr>
            </thead>
            <tbody>
              {/* Folders first */}
              {folders.map(item => (
                <tr
                  key={item.id}
                  className="border-b border-border/50 hover:bg-bg-hover transition-colors cursor-pointer"
                  onDoubleClick={() => navigateToFolder(item)}
                >
                  <td className="px-4 py-3 text-sm font-medium text-text-primary flex items-center gap-2">
                    <span>{getIcon(item)}</span>
                    <button
                      onClick={() => navigateToFolder(item)}
                      className="text-text-primary hover:text-accent bg-transparent border-none p-0 cursor-pointer text-sm font-medium text-left"
                    >
                      {item.name}
                    </button>
                  </td>
                  <td className="px-4 py-3"><Badge variant="muted">Folder</Badge></td>
                  <td className="px-4 py-3 text-sm text-text-muted">—</td>
                  <td className="px-4 py-3 text-sm text-text-muted">
                    {item.updated_at ? new Date(item.updated_at).toLocaleDateString() : ''}
                  </td>
                  <td className="px-4 py-3">
                    <Button variant="ghost" size="sm" onClick={() => handleTrash(item.id)} title="Trash">🗑️</Button>
                  </td>
                </tr>
              ))}
              {/* Then files */}
              {files.map(item => (
                <tr key={item.id} className="border-b border-border/50 hover:bg-bg-hover transition-colors">
                  <td className="px-4 py-3 text-sm font-medium text-text-primary flex items-center gap-2">
                    <span>{getIcon(item)}</span>
                    <span>{item.name}</span>
                  </td>
                  <td className="px-4 py-3"><Badge variant="primary">{item.mime_type || 'File'}</Badge></td>
                  <td className="px-4 py-3 text-sm text-text-muted">{formatSize(item.size_bytes)}</td>
                  <td className="px-4 py-3 text-sm text-text-muted">
                    {item.updated_at ? new Date(item.updated_at).toLocaleDateString() : ''}
                  </td>
                  <td className="px-4 py-3 flex items-center gap-1">
                    <Button variant="ghost" size="sm" onClick={() => handleDownload(item)} title="Download">📥</Button>
                    <Button variant="ghost" size="sm" onClick={() => handleTrash(item.id)} title="Trash">🗑️</Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </Card>
      ) : (
        <EmptyState
          icon="📂"
          title={folderStack.length > 0 ? 'Folder is empty' : 'Drive is empty'}
          description="Upload files or create folders to get started."
        />
      )}
    </div>
  )
}
