import type { ReactNode } from 'react'
import type { WorkspaceLocks } from '../../api/workspaces'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { workspaceDisplayName } from '../../lib/workspace'
import { SettingsRow, type Choice } from './SettingsGroup'

type Rule = WorkspaceLocks['group_creation']

export const GROUP_CREATION_CHOICES: Choice<Rule>[] = [
  { value: 'everyone', label: 'Mọi thành viên' },
  {
    value: 'admins',
    label: 'Chỉ quản trị viên',
    hint: 'Thành viên và vai trò tự tạo mất quyền tạo nhóm; chủ sở hữu vẫn tạo được. Mở lại không tự trả quyền cho vai trò tự tạo.',
  },
]

/** "Mọi thành viên" / "Chỉ quản trị viên". */
export const groupCreationLabel = (rule: Rule | undefined) => GROUP_CREATION_CHOICES.find((c) => c.value === rule)?.label

/**
 * "Ai được tạo nhóm" (DESIGN.md §6, the one organization lock of this round). For an administrator it opens the
 * choice (`onPress`); for anyone else it is a locked row, dimmed, with the value and "Quản trị viên <workspace> đã
 * đặt": no person's name, because the graph records no one who set it. The server refuses a change regardless.
 */
export function GroupCreationRow({ rule, onPress, icon }: { rule: Rule | undefined; onPress?: () => void; icon?: ReactNode }) {
  const { workspaceName } = useActiveWorkspace()
  // Said only of a rule that is known; a rule that could not be read is not claimed to be set.
  const locked = onPress || !rule ? undefined : `Quản trị viên ${workspaceDisplayName(workspaceName)} đã đặt`
  return <SettingsRow icon={icon} label="Ai được tạo nhóm" value={groupCreationLabel(rule)} locked={locked} onPress={onPress} />
}
