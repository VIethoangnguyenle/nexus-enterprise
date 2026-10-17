import { ChevronRight, Shield, Users } from 'lucide-react'
import type { Channel } from '../../api/messaging'
import type { GroupMember } from '../../hooks/useContactCategories'
import type { PeopleDirectory } from '../../lib/people'
import { Avatar, Pressable } from '../primitives'
import { PersonLink } from '../primitives/PersonLink'
import type { RoleGroup } from './contacts-model'

const ROW = `press flex items-center gap-3 w-full min-w-0 min-h-13 px-2 py-2 rounded-lg border-none bg-transparent text-left
  cursor-pointer hover:bg-hover focus-ring transition-colors duration-quick`

/** A role in the list of roles: a shield for the owners (Quản trị viên), a group mark for the others. */
export function RoleRow({ role, onOpen }: { role: RoleGroup; onOpen: (id: string) => void }) {
  const Icon = role.kind === 'owners' ? Shield : Users
  return (
    <Pressable onClick={() => onOpen(role.id)} className={ROW}>
      <span aria-hidden="true" className="grid place-items-center shrink-0 w-8 h-8 rounded-md bg-sunk text-ink-muted">
        <Icon size={16} strokeWidth={1.75} />
      </span>
      <span className="flex-1 min-w-0 truncate font-medium text-ink">{role.name}</span>
      <span className="shrink-0 text-small text-ink-muted tnum">{role.people.length} người</span>
      <ChevronRight size={16} strokeWidth={1.75} aria-hidden="true" className="shrink-0 text-ink-subtle" />
    </Pressable>
  )
}

/** One of the caller's groups: the group's square avatar (its picture or its first letter), name, how many members. */
export function GroupRow({ group, onOpen }: { group: Channel; onOpen: (id: string) => void }) {
  return (
    <Pressable onClick={() => onOpen(group.id)} className={ROW}>
      <Avatar name={group.name} hueKey={group.id} src={group.avatar_url || undefined} shape="square" size={32} />
      <span className="flex-1 min-w-0 truncate font-medium text-ink">{group.name}</span>
      <span className="shrink-0 text-small text-ink-muted tnum">{group.member_count ?? 0} thành viên</span>
      <ChevronRight size={16} strokeWidth={1.75} aria-hidden="true" className="shrink-0 text-ink-subtle" />
    </Pressable>
  )
}

/**
 * A member of a group the directory hides from the caller: their name and picture, as in the chat the group
 * belongs to, and a press that opens the small card ("Hồ sơ này không còn hiển thị với bạn.") rather than a profile.
 */
export function HiddenMemberRow({ member, people }: { member: GroupMember; people: PeopleDirectory }) {
  return (
    <PersonLink
      userId={member.userId}
      people={people}
      className="w-full min-h-13 px-2 py-2 gap-3 rounded-lg hover:bg-hover transition-colors duration-quick"
    >
      <Avatar name={member.name} hueKey={member.userId} src={member.avatarUrl || undefined} size={32} />
      <span className="flex-1 min-w-0 truncate font-medium text-ink">{member.name}</span>
    </PersonLink>
  )
}
