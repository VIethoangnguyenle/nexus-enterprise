import { useState } from 'react'
import type { DriveItem } from '../../api/drive'
import { Button } from '../primitives'
import { Dialog } from '../composites/Dialog'
import { TreeView, type TreeNode } from '../composites/TreeView'
import { useFolderNodes } from './DriveTree'

interface MoveItemDialogProps {
  /** The item to move; null keeps the dialog closed. */
  item: DriveItem | null
  workspaceId: string
  pending: boolean
  onConfirm: (item: DriveItem, targetFolderId: string) => void
  onClose: () => void
}

/**
 * "Di chuyển tới": pick a folder in the shared tree. A folder cannot be
 * moved into itself, and the folder an item already sits in is not offered.
 */
export function MoveItemDialog({ item, workspaceId, pending, onConfirm, onClose }: MoveItemDialogProps) {
  const [target, setTarget] = useState<string | null>(null)
  const [open, setOpen] = useState<ReadonlySet<string>>(new Set())

  // Each item starts clean: a pick made for one item must not carry over to the
  // next, or "Di chuyển" could send a folder into itself. Adjusting state while
  // rendering (rather than in an effect) means no render ever sees the stale pick.
  const [forId, setForId] = useState(item?.id)
  if (forId !== item?.id) {
    setForId(item?.id)
    setTarget(null)
    setOpen(new Set())
  }

  // The moved item is left out at every level, which also hides everything
  // inside it: a folder cannot be moved into itself or its own subfolders.
  const without = (nodes: TreeNode[] | undefined) => nodes?.filter((n) => n.id !== item?.id)
  const top = useFolderNodes(workspaceId)
  const useChildren = (parentId: string) => {
    const r = useFolderNodes(workspaceId, parentId)
    return { ...r, nodes: without(r.nodes) }
  }
  const roots = without(top.nodes) ?? []

  const toggle = (id: string) =>
    setOpen((o) => {
      const next = new Set(o)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })

  const close = () => {
    if (pending) return
    onClose()
  }

  // The tree hides the item and everything under it, so the pick can only be
  // one of those through stale state; refuse it anyway, and a no-op move.
  const canMove = !!item && !!target && target !== item.id && target !== item.parent_id

  return (
    <Dialog
      open={!!item}
      onClose={close}
      title="Di chuyển tới"
      footer={
        <>
          <Button type="button" variant="soft" onClick={close} disabled={pending}>Huỷ</Button>
          <Button
            type="button"
            loading={pending}
            disabled={!canMove}
            onClick={() => canMove && onConfirm(item, target)}
          >
            Di chuyển vào đây
          </Button>
        </>
      }
    >
      <p className="m-0 text-sm text-ink-muted">
        Chọn thư mục để chuyển <b className="font-semibold text-ink">{item?.name}</b> tới.
      </p>
      <div className="max-h-80 overflow-y-auto -mx-1.5 px-1.5">
        {top.isLoading ? (
          <div className="grid gap-1.5" aria-busy="true">
            {[0, 1, 2].map((i) => <div key={i} className="skeleton h-7 rounded-md" />)}
          </div>
        ) : top.isError ? (
          <p className="m-0 text-small text-danger">Không tải được danh sách thư mục.</p>
        ) : roots.length === 0 ? (
          <p className="m-0 text-small text-ink-muted">Chưa có thư mục nào để chuyển tới.</p>
        ) : (
          <TreeView
            label="Chọn thư mục"
            roots={roots}
            useChildren={useChildren}
            expanded={open}
            onToggle={toggle}
            selectedId={target}
            onSelect={setTarget}
          />
        )}
      </div>
    </Dialog>
  )
}
