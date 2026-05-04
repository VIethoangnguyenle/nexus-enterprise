import { useMemo, useState, useEffect, useCallback } from 'react'
import { X, FileText, Info, Shield, Clock, Search, UserPlus, Check } from 'lucide-react'
import { useDriveStore } from '../../stores/drive.store'
import { useDriveItem, useDriveShares, useCreateShare, useRevokeShare } from '../../hooks/useDrive'
import { useContacts, type Contact } from '../../hooks/useContacts'
import { useWorkspaces } from '../../hooks/useWorkspaces'
import { useAuthStore } from '../../stores/auth.store'
import { Spinner, IconButton, Button } from '../primitives'
import { useObjectPermissions } from '../../hooks/usePermissions'
import type { DriveItem } from '../../api/drive'
import { getFileIcon } from '../../lib/fileIcons'
import { isImageFile } from '../patterns/ImagePreviewCard'
import { driveApi } from '../../api/drive'

type ContextTab = 'preview' | 'metadata' | 'permissions' | 'activity'

const TABS: { id: ContextTab; label: string; icon: typeof FileText; requiresPerm?: keyof ReturnType<typeof useObjectPermissions> }[] = [
  { id: 'preview', label: 'Preview', icon: FileText },
  { id: 'metadata', label: 'Details', icon: Info },
  { id: 'permissions', label: 'Sharing', icon: Shield, requiresPerm: 'share' },
  { id: 'activity', label: 'Activity', icon: Clock },
]

function formatBytes(bytes: number): string {
  if (!bytes || bytes <= 0) return '—'
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1048576) return `${(bytes / 1024).toFixed(1)} KB`
  if (bytes < 1073741824) return `${(bytes / 1048576).toFixed(1)} MB`
  return `${(bytes / 1073741824).toFixed(2)} GB`
}

/** Slide-in context panel for file preview, metadata, permissions, activity. */
export function DriveContextPanel() {
  const open = useDriveStore((s) => s.contextPanelOpen)
  const tab = useDriveStore((s) => s.contextPanelTab)
  const setTab = useDriveStore((s) => s.setContextPanelTab)
  const closePanel = useDriveStore((s) => s.closeContextPanel)
  const selectedId = useDriveStore((s) => s.selectedItemId)

  const { data: item } = useDriveItem(selectedId || '')
  const perms = useObjectPermissions(item?.ngac_node_id)

  // Filter tabs based on permissions
  const visibleTabs = useMemo(() =>
    TABS.filter((t) => !t.requiresPerm || perms[t.requiresPerm]),
    [perms],
  )

  if (!open || !selectedId || !item) return null

  return (
    <div
      className="w-[320px] border-l border-outline-variant bg-surface-container-lowest flex flex-col flex-shrink-0
        animate-slide-in-right"
    >
      {/* Header */}
      <div className="h-[52px] px-4 flex items-center justify-between border-b border-outline-variant flex-shrink-0">
        <span className="text-sm font-medium text-on-surface truncate">{item.name}</span>
        <IconButton
          onClick={closePanel}
          aria-label="Close panel"
          size="sm"
        >
          <X size={16} />
        </IconButton>
      </div>

      {/* Tab bar */}
      <div className="flex border-b border-outline-variant">
        {visibleTabs.map((t) => {
          const Icon = t.icon
          return (
            <button
              key={t.id}
              onClick={() => setTab(t.id)}
              className={`flex-1 flex items-center justify-center gap-1 py-2 text-xs font-medium
                border-none cursor-pointer transition-colors
                ${tab === t.id
                  ? 'text-primary border-b-2 border-primary bg-transparent'
                  : 'text-on-surface-variant hover:text-on-surface bg-transparent'
                }`}
            >
              <Icon size={14} />
              {t.label}
            </button>
          )
        })}
      </div>

      {/* Tab content */}
      <div className="flex-1 overflow-y-auto p-4">
        {tab === 'preview' && <PreviewTab item={item} />}
        {tab === 'metadata' && <MetadataTab item={item} />}
        {tab === 'permissions' && <PermissionsTab item={item} />}
        {tab === 'activity' && <ActivityTab />}
      </div>
    </div>
  )
}

function PreviewTab({ item }: { item: DriveItem }) {
  const isFolder = item.item_type === 'folder'
  const fi = !isFolder ? getFileIcon(item.name, item.mime_type) : null
  const canPreviewImage = !isFolder && isImageFile(item.name)

  const [imgUrl, setImgUrl] = useState<string | null>(null)
  const [imgLoading, setImgLoading] = useState(false)
  const [imgError, setImgError] = useState(false)

  useEffect(() => {
    if (!canPreviewImage) return
    let cancelled = false
    setImgLoading(true)
    setImgError(false)
    setImgUrl(null)

    driveApi.getDownloadUrl(item.id)
      .then(({ download_url }) => {
        if (!cancelled) setImgUrl(download_url)
      })
      .catch(() => {
        if (!cancelled) setImgError(true)
      })
      .finally(() => {
        if (!cancelled) setImgLoading(false)
      })

    return () => { cancelled = true }
  }, [item.id, canPreviewImage])

  return (
    <div className="flex flex-col items-center gap-4 pt-4">
      {/* Image preview */}
      {canPreviewImage && !imgError ? (
        <div className="w-full rounded-lg overflow-hidden border border-outline-variant bg-surface-container">
          {imgLoading ? (
            <div className="w-full h-[180px] flex items-center justify-center animate-pulse">
              <span className="text-xs text-on-surface-variant">Loading preview…</span>
            </div>
          ) : imgUrl ? (
            <img
              src={imgUrl}
              alt={item.name}
              className="w-full max-h-[280px] object-contain cursor-pointer"
              onClick={() => window.open(imgUrl, '_blank')}
              onError={() => setImgError(true)}
              loading="lazy"
            />
          ) : null}
        </div>
      ) : (
        /* Generic icon fallback */
        <div className="w-20 h-20 rounded-2xl bg-surface-container flex items-center justify-center">
          {fi ? <fi.icon size={40} color={fi.color} /> : (
            <span className="text-4xl">📁</span>
          )}
        </div>
      )}
      <div className="text-center">
        <p className="text-sm font-medium text-on-surface">{item.name}</p>
        <p className="text-xs text-on-surface-variant mt-1">
          {isFolder ? 'Folder' : item.mime_type || 'File'}
          {!isFolder && item.size_bytes > 0 && ` · ${formatBytes(item.size_bytes)}`}
        </p>
      </div>
    </div>
  )
}

/** Parse proto timestamp (may arrive as {seconds,nanos} object or ISO string). */
function parseProtoTimestamp(val: unknown): Date | null {
  if (!val) return null
  // Proto JSON: { seconds: "1234567890", nanos: 0 }
  if (typeof val === 'object' && val !== null && 'seconds' in val) {
    const secs = Number((val as any).seconds)
    if (!isNaN(secs)) return new Date(secs * 1000)
  }
  // ISO string fallback
  if (typeof val === 'string') {
    const d = new Date(val)
    if (!isNaN(d.getTime())) return d
  }
  return null
}

function formatTimestamp(val: unknown): string {
  const d = parseProtoTimestamp(val)
  if (!d) return '—'
  return d.toLocaleString()
}

function MetadataTab({ item }: { item: DriveItem }) {
  const currentUser = useAuthStore((s) => s.user)

  // Check if owner matches current user (owner_id may be NGAC node ID or user ID)
  const isOwner = currentUser && (
    item.owner_id === currentUser.id ||
    item.owner_id === currentUser.ngac_node_id
  )

  const ownerDisplay = isOwner ? currentUser.username || 'You' : (item.owner_id || '—')

  const fields = [
    { label: 'Type', value: item.item_type === 'folder' ? 'Folder' : (item.mime_type || 'File') },
    { label: 'Size', value: formatBytes(item.size_bytes) },
    { label: 'Created', value: formatTimestamp(item.created_at) },
    { label: 'Modified', value: formatTimestamp(item.updated_at) },
    { label: 'Status', value: item.status || 'active' },
  ]

  return (
    <div className="space-y-3">
      {fields.map((f) => (
        <div key={f.label}>
          <span className="text-[11px] font-bold uppercase tracking-widest text-on-surface-variant">{f.label}</span>
          <p className="text-sm text-on-surface mt-1">{f.value}</p>
        </div>
      ))}

      {/* Owner — separate section with avatar */}
      <div>
        <span className="text-[11px] font-bold uppercase tracking-widest text-on-surface-variant">Owner</span>
        <div className="flex items-center gap-2 mt-1">
          <div className="w-6 h-6 rounded-full bg-primary-container text-on-primary-container
            flex items-center justify-center text-[10px] font-semibold flex-shrink-0">
            {(ownerDisplay)[0]?.toUpperCase() || '?'}
          </div>
          <span className="text-sm text-on-surface">{ownerDisplay}{isOwner ? ' (You)' : ''}</span>
        </div>
      </div>
    </div>
  )
}

function PermissionsTab({ item }: { item: DriveItem }) {
  const { data: wsData } = useWorkspaces()
  const wsId = wsData?.workspaces?.[0]?.id || ''
  const { data: sharesData, isLoading: sharesLoading } = useDriveShares(item.id)
  const { data: contactsData, isLoading: contactsLoading } = useContacts(wsId)
  const createShare = useCreateShare(item.id)
  const revokeShare = useRevokeShare(item.id)
  const shares = sharesData?.shares ?? []
  const contacts = contactsData?.contacts ?? []

  const [search, setSearch] = useState('')
  const [permission, setPermission] = useState<'read' | 'write'>('read')
  const [showPicker, setShowPicker] = useState(false)

  // Set of already-shared NGAC node IDs
  const sharedNodeIds = useMemo(
    () => new Set(shares.map((s) => s.target_ngac_id)),
    [shares],
  )

  // Filter contacts by search and exclude current user (owner can't share with themselves)
  const currentUser = useAuthStore((s) => s.user)
  const filteredContacts = useMemo(() => {
    let result = contacts.filter((c) =>
      c.user_id !== currentUser?.id &&
      c.ngac_node_id !== currentUser?.ngac_node_id
    )
    if (search.trim()) {
      const q = search.toLowerCase()
      result = result.filter(
        (c) =>
          c.display_name.toLowerCase().includes(q) ||
          c.username.toLowerCase().includes(q) ||
          c.email.toLowerCase().includes(q),
      )
    }
    return result
  }, [contacts, search, currentUser])

  const handleShare = useCallback(
    (contact: Contact) => {
      createShare.mutate({
        shareType: 'user',
        targetNgacId: contact.ngac_node_id,
        operations: [permission],
      })
    },
    [createShare, permission],
  )

  return (
    <div className="space-y-3">
      {/* Current shares header */}
      <div className="flex items-center justify-between">
        <span className="text-xs font-semibold text-on-surface-variant uppercase">
          Shared with ({shares.length})
        </span>
        <Button
          variant="ghost"
          size="sm"
          onClick={() => setShowPicker(!showPicker)}
        >
          {showPicker ? 'Hide' : '+ Add'}
        </Button>
      </div>

      {/* Current shares list */}
      {sharesLoading ? (
        <div className="flex justify-center py-2"><Spinner size="sm" /></div>
      ) : shares.length === 0 ? (
        <p className="text-xs text-on-surface-variant">Not shared with anyone</p>
      ) : (
        <div className="space-y-1">
          {shares.map((share) => (
            <div key={share.id} className="flex items-center justify-between p-2 rounded bg-surface-container-low">
              <div className="min-w-0">
                <p className="text-sm text-on-surface truncate">{share.target_label || share.target_ngac_id}</p>
                <p className="text-xs text-on-surface-variant">{share.operations?.join(', ')}</p>
              </div>
              <IconButton
                onClick={() => revokeShare.mutate(share.id)}
                aria-label="Remove access"
                title="Remove access"
                size="sm"
                className="text-on-surface-variant hover:text-error flex-shrink-0"
              >
                <X size={14} />
              </IconButton>
            </div>
          ))}
        </div>
      )}

      {/* Member picker */}
      {showPicker && (
        <>
          <div className="border-t border-outline-variant pt-3" />

          {/* Permission selector */}
          <div className="flex items-center gap-2">
            <label className="text-xs text-on-surface-variant">Permission:</label>
            <select
              value={permission}
              onChange={(e) => setPermission(e.target.value as 'read' | 'write')}
              className="text-xs px-2 py-1 rounded border border-outline-variant bg-surface-container-lowest
                text-on-surface focus:outline-none focus-ring cursor-pointer"
            >
              <option value="read">Can view</option>
              <option value="write">Can edit</option>
            </select>
          </div>

          {/* Search */}
          <div className="relative">
            <Search
              size={14}
              className="absolute left-2 top-1/2 -translate-y-1/2 text-on-surface-variant pointer-events-none"
            />
            <input
              type="text"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Search members..."
              className="w-full pl-7 pr-2 py-1.5 text-xs bg-surface-container-lowest border border-outline-variant
                rounded text-on-surface placeholder:text-on-surface-variant focus:outline-none focus-ring"
              autoFocus
            />
          </div>

          {/* Contacts */}
          <div className="max-h-[200px] overflow-y-auto space-y-1">
            {contactsLoading ? (
              <div className="flex justify-center py-3"><Spinner size="sm" /></div>
            ) : filteredContacts.length === 0 ? (
              <p className="text-xs text-on-surface-variant text-center py-2">
                {search ? 'No members found' : 'No workspace members'}
              </p>
            ) : (
              filteredContacts.map((contact) => {
                const isShared = sharedNodeIds.has(contact.ngac_node_id)
                return (
                  <div
                    key={contact.user_id}
                    className="flex items-center gap-2 px-2 py-1.5 rounded hover:bg-surface-container-high transition-colors"
                  >
                    <div className="w-6 h-6 rounded-full bg-primary-container text-on-primary-container
                      flex items-center justify-center text-[10px] font-semibold flex-shrink-0">
                      {(contact.display_name || contact.username || '?')[0].toUpperCase()}
                    </div>
                    <div className="flex-1 min-w-0">
                      <p className="text-xs text-on-surface truncate">
                        {contact.display_name || contact.username}
                      </p>
                    </div>
                    {isShared ? (
                      <span className="flex items-center gap-0.5 text-[10px] text-on-surface-variant px-1.5 py-0.5
                        bg-surface-container rounded-full flex-shrink-0">
                        <Check size={10} />
                        Shared
                      </span>
                    ) : (
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => handleShare(contact)}
                        disabled={createShare.isPending}
                        className="flex-shrink-0"
                      >
                        <UserPlus size={12} />
                        Share
                      </Button>
                    )}
                  </div>
                )
              })
            )}
          </div>
        </>
      )}
    </div>
  )
}

function ActivityTab() {
  return (
    <div className="flex flex-col items-center justify-center h-32 text-on-surface-variant text-xs">
      Activity timeline coming soon
    </div>
  )
}
