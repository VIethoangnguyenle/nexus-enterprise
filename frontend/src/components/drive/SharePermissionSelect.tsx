import { ChevronDown } from 'lucide-react'
import type { DriveShare, SharePermission } from '../../api/drive'
import { useChangeSharePermission } from '../../hooks/useDrive'
import { Select } from '../primitives'
import { sharePermissionOf } from './drive-model'

export const SHARE_PERMISSIONS: { value: SharePermission; label: string }[] = [
  { value: 'read', label: 'Có thể xem' },
  { value: 'write', label: 'Có thể sửa' },
]

/** A person's permission on an item as an inline dropdown (mockup §2, "Có thể sửa ▾"). */
export function SharePermissionSelect({ itemId, share, personName }: {
  itemId: string
  share: DriveShare
  personName: string
}) {
  const change = useChangeSharePermission(itemId)
  const current = sharePermissionOf(share.operations)
  return (
    <span className="relative shrink-0">
      <Select
        aria-label={`Quyền của ${personName}`}
        value={change.isPending ? (change.variables?.to ?? current) : current}
        disabled={change.isPending}
        onChange={(e) =>
          change.mutate({
            shareId: share.id, targetNodeId: share.target_ngac_id, from: current, to: e.target.value as SharePermission,
          })
        }
        className="w-auto py-1 pl-2 pr-6 border-transparent bg-transparent text-ink-muted hover:text-ink"
      >
        {SHARE_PERMISSIONS.map((p) => <option key={p.value} value={p.value}>{p.label}</option>)}
      </Select>
      <ChevronDown
        size={16}
        strokeWidth={1.75}
        aria-hidden="true"
        className="absolute right-1 top-1/2 -translate-y-1/2 text-ink-muted pointer-events-none"
      />
    </span>
  )
}
