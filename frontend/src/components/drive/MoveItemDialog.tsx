import { useState, useCallback } from 'react'
import { ArrowRight } from 'lucide-react'
import type { DriveItem } from '../../api/drive'
import { FolderTreeSelect } from './FolderTreeSelect'
import { Modal } from '../composites'
import { Button } from '../primitives'

interface MoveItemDialogProps {
  /** The item to move — null hides the dialog. */
  item: DriveItem | null
  /** Workspace ID for folder tree loading. */
  workspaceId: string
  /** Whether the move mutation is in progress. */
  isMoving: boolean
  /** Called when the user confirms the move. */
  onConfirm: (item: DriveItem, destinationFolderId: string) => void
  /** Called when the user cancels or dismisses the dialog. */
  onClose: () => void
}

/**
 * Drive-specific move dialog — composes Modal + FolderTreeSelect.
 *
 * Design source: Nexus Drive - Move File/Folder (99dc16ab)
 */
export function MoveItemDialog({ item, workspaceId, isMoving, onConfirm, onClose }: MoveItemDialogProps) {
  const [selectedFolderId, setSelectedFolderId] = useState<string | null>(null)

  const handleConfirm = useCallback(() => {
    if (item && selectedFolderId) onConfirm(item, selectedFolderId)
  }, [item, selectedFolderId, onConfirm])

  if (!item) return null

  return (
    <Modal onClose={onClose} size="lg">
      <Modal.Header onClose={onClose}>Move to…</Modal.Header>

      {/* Info bar — shows what's being moved */}
      <div className="px-6 py-3 bg-surface-container-low border-b border-outline-variant">
        <p className="text-sm text-on-surface-variant">
          Moving <span className="font-semibold text-on-surface">{item.name}</span>
        </p>
      </div>

      {/* Tree body — scrollable */}
      <div className="flex-1 overflow-y-auto px-3 py-3">
        <FolderTreeSelect
          workspaceId={workspaceId}
          selectedId={selectedFolderId}
          onSelect={setSelectedFolderId}
        />
      </div>

      {/* Footer actions */}
      <div className="border-t border-outline-variant px-6 py-4">
        <Modal.Actions className="mt-0">
          <Button variant="outline" onClick={onClose} disabled={isMoving}>
            Cancel
          </Button>
          <Button
            variant="primary"
            onClick={handleConfirm}
            disabled={!selectedFolderId}
            loading={isMoving}
          >
            <ArrowRight size={16} />
            Move Here
          </Button>
        </Modal.Actions>
      </div>
    </Modal>
  )
}
