import { useState, useMemo, useCallback } from 'react'
import { Share2, X, Search, UserPlus, Check } from 'lucide-react'
import { Modal } from '../composites'
import { Button, IconButton, Spinner } from '../primitives'
import { useContacts, type Contact } from '../../hooks/useContacts'
import { useDriveShares, useCreateShare, useRevokeShare } from '../../hooks/useDrive'
import { ConfirmDialog } from '../composites'
import type { DriveItem, DriveShare } from '../../api/drive'

interface ShareDialogProps {
  /** The drive item to share — null hides the dialog. */
  item: DriveItem | null
  /** Current workspace ID for fetching contacts. */
  workspaceId: string
  /** Called when the dialog should close. */
  onClose: () => void
}

/**
 * Share dialog — composes Modal + useContacts + useDriveShares.
 * Allows searching workspace members and sharing/revoking access.
 */
export function ShareDialog({ item, workspaceId, onClose }: ShareDialogProps) {
  const [search, setSearch] = useState('')
  const [permission, setPermission] = useState<'read' | 'write'>('read')
  const [revokeTarget, setRevokeTarget] = useState<DriveShare | null>(null)

  const { data: contactsData, isLoading: contactsLoading } = useContacts(workspaceId)
  const { data: sharesData, isLoading: sharesLoading } = useDriveShares(item?.id || '')
  const createShare = useCreateShare(item?.id || '')
  const revokeShare = useRevokeShare(item?.id || '')

  const contacts = contactsData?.contacts ?? []
  const shares = sharesData?.shares ?? []

  // Set of already-shared NGAC node IDs for quick lookup
  const sharedNodeIds = useMemo(
    () => new Set(shares.map((s) => s.target_ngac_id)),
    [shares],
  )

  // Filter contacts by search term
  const filteredContacts = useMemo(() => {
    if (!search.trim()) return contacts
    const q = search.toLowerCase()
    return contacts.filter(
      (c) =>
        c.display_name.toLowerCase().includes(q) ||
        c.username.toLowerCase().includes(q) ||
        c.email.toLowerCase().includes(q),
    )
  }, [contacts, search])

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

  const handleRevokeConfirm = useCallback(() => {
    if (revokeTarget) {
      revokeShare.mutate(revokeTarget.id)
      setRevokeTarget(null)
    }
  }, [revokeTarget, revokeShare])

  if (!item) return null

  return (
    <>
      <Modal open={!!item} onClose={onClose}>
        <Modal.Header>
          <div className="flex items-center gap-2">
            <Share2 size={18} className="text-primary" />
            <span>Share "{item.name}"</span>
          </div>
        </Modal.Header>
        <Modal.Body>
          {/* Current shares */}
          <div className="mb-4">
            <p className="text-xs font-semibold text-on-surface-variant uppercase tracking-wider mb-2">
              Shared with ({shares.length})
            </p>
            {sharesLoading ? (
              <div className="flex justify-center py-3">
                <Spinner size="sm" />
              </div>
            ) : shares.length === 0 ? (
              <p className="text-xs text-on-surface-variant py-2">
                Not shared with anyone yet
              </p>
            ) : (
              <div className="space-y-1 max-h-[120px] overflow-y-auto">
                {shares.map((share) => (
                  <div
                    key={share.id}
                    className="flex items-center justify-between px-3 py-2 rounded-md bg-surface-container-low"
                  >
                    <div className="min-w-0">
                      <p className="text-sm text-on-surface truncate">
                        {share.target_label || share.target_ngac_id}
                      </p>
                      <p className="text-xs text-on-surface-variant">
                        {share.operations?.join(', ')}
                      </p>
                    </div>
                    <IconButton
                      onClick={() => setRevokeTarget(share)}
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
          </div>

          {/* Divider */}
          <div className="border-t border-outline-variant mb-4" />

          {/* Permission selector */}
          <div className="flex items-center gap-2 mb-3">
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

          {/* Search input */}
          <div className="relative mb-3">
            <Search
              size={14}
              className="absolute left-3 top-1/2 -translate-y-1/2 text-on-surface-variant pointer-events-none"
            />
            <input
              type="text"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Search workspace members..."
              className="w-full pl-8 pr-3 py-2 text-sm bg-surface-container-lowest border border-outline-variant
                rounded-md text-on-surface placeholder:text-on-surface-variant
                focus:outline-none focus-ring"
              autoFocus
            />
          </div>

          {/* Contact list */}
          <div className="max-h-[240px] overflow-y-auto space-y-1">
            {contactsLoading ? (
              <div className="flex justify-center py-4">
                <Spinner size="sm" />
              </div>
            ) : filteredContacts.length === 0 ? (
              <p className="text-xs text-on-surface-variant text-center py-4">
                {search ? 'No members found' : 'No workspace members'}
              </p>
            ) : (
              filteredContacts.map((contact) => {
                const isShared = sharedNodeIds.has(contact.ngac_node_id)
                return (
                  <div
                    key={contact.user_id}
                    className="flex items-center gap-3 px-3 py-2 rounded-md hover:bg-surface-container-high transition-colors"
                  >
                    {/* Avatar */}
                    <div className="w-8 h-8 rounded-full bg-primary-container text-on-primary-container
                      flex items-center justify-center text-xs font-semibold flex-shrink-0">
                      {(contact.display_name || contact.username || '?')[0].toUpperCase()}
                    </div>

                    {/* Info */}
                    <div className="flex-1 min-w-0">
                      <p className="text-sm text-on-surface truncate">
                        {contact.display_name || contact.username}
                      </p>
                      <p className="text-xs text-on-surface-variant truncate">
                        {contact.email}
                      </p>
                    </div>

                    {/* Action */}
                    {isShared ? (
                      <span className="flex items-center gap-1 text-xs text-on-surface-variant px-2 py-1
                        bg-surface-container rounded-full flex-shrink-0">
                        <Check size={12} />
                        Shared
                      </span>
                    ) : (
                      <Button
                        size="sm"
                        variant="ghost"
                        onClick={() => handleShare(contact)}
                        disabled={createShare.isPending}
                      >
                        <UserPlus size={14} />
                        Share
                      </Button>
                    )}
                  </div>
                )
              })
            )}
          </div>
        </Modal.Body>
        <Modal.Actions>
          <Button variant="ghost" onClick={onClose}>
            Done
          </Button>
        </Modal.Actions>
      </Modal>

      {/* Revoke confirmation */}
      <ConfirmDialog
        open={!!revokeTarget}
        onClose={() => setRevokeTarget(null)}
        onConfirm={handleRevokeConfirm}
        title="Remove access"
        description={
          <>
            Remove access for{' '}
            <span className="font-semibold text-on-surface">
              {revokeTarget?.target_label || revokeTarget?.target_ngac_id}
            </span>
            ?
          </>
        }
        warning="This person will no longer be able to access this file."
        confirmLabel="Remove"
        confirmVariant="error"
        loading={revokeShare.isPending}
      />
    </>
  )
}
