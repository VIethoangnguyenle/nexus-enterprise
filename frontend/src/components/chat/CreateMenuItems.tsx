import { BarChart3, FilePlus, ListChecks, Stamp } from 'lucide-react'
import { usePhone } from '../../hooks/usePhone'
import { useWorkspaceType } from '../../hooks/useWorkspaceType'
import { MenuItem, MenuSeparator } from '../primitives'

export type CreateKind = 'poll' | 'task' | 'doc' | 'approval'

/**
 * The second group of the composer's "+" menu (DESIGN.md "Nút +"): the things a
 * message can carry as a card. "Đề nghị phê duyệt" is left out of a personal
 * workspace, where the server would refuse it anyway. Mount it inside the
 * popover (desktop) or the bottom sheet (phone); rows are 44px on a phone.
 */
export function CreateMenuItems({ onSelect }: { onSelect: (kind: CreateKind) => void }) {
  const { isPersonal } = useWorkspaceType()
  const phone = usePhone()
  const row = phone ? 'h-11' : ''
  const icon = { size: 16, strokeWidth: 1.75, 'aria-hidden': true } as const
  return (
    <>
      <MenuSeparator />
      <MenuItem className={row} icon={<BarChart3 {...icon} />} onClick={() => onSelect('poll')}>Bình chọn</MenuItem>
      <MenuItem className={row} icon={<ListChecks {...icon} />} onClick={() => onSelect('task')}>Công việc</MenuItem>
      <MenuItem className={row} icon={<FilePlus {...icon} />} onClick={() => onSelect('doc')}>Tài liệu mới</MenuItem>
      {!isPersonal && (
        <MenuItem className={row} icon={<Stamp {...icon} />} onClick={() => onSelect('approval')}>Đề nghị phê duyệt</MenuItem>
      )}
    </>
  )
}
