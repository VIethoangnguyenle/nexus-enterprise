import { useState } from 'react'
import { Users } from 'lucide-react'
import { matchesPerson, type Person } from '../../../lib/people'
import { Avatar, Pressable, SearchField } from '../../primitives'
import { EmptyState } from '../EmptyState'

/**
 * Picks one person among a group's members (Thêm quản trị nhóm, Chuyển quyền
 * chủ nhóm): a search box over names and titles and the list itself. People
 * are only ever chosen from the list, never typed by id.
 */
export function GroupMemberPicker({ label, candidates, emptyText, onPick }: {
  /** Names the search box and the list, e.g. "Chọn quản trị nhóm". */
  label: string
  candidates: Person[]
  /** Shown when nobody can be picked at all. */
  emptyText: string
  onPick: (person: Person) => void
}) {
  const [query, setQuery] = useState('')
  const visible = candidates.filter((p) => matchesPerson(p, query))

  if (candidates.length === 0) {
    return <EmptyState compact icon={<Users size={24} strokeWidth={1.75} />} text={emptyText} />
  }

  return (
    <div className="grid gap-2">
      <SearchField label={`Tìm trong ${label.toLowerCase()}`} value={query} onChange={(e) => setQuery(e.target.value)} />
      {visible.length === 0 ? (
        <EmptyState compact icon={<Users size={24} strokeWidth={1.75} />} text={`Không có thành viên nào khớp “${query}”.`} />
      ) : (
        <ul aria-label={label} className="grid gap-0.5 list-none m-0 p-0">
          {visible.map((p) => (
            <li key={p.userId}>
              <Pressable
                onClick={() => onPick(p)}
                className="flex items-center gap-2.5 w-full min-h-13 px-3 py-2 rounded-surface hover:bg-hover text-left"
              >
                <Avatar name={p.name} hueKey={p.userId} src={p.avatarUrl || undefined} size={32} />
                <span className="grid min-w-0">
                  <span className="font-medium text-ink truncate">{p.name}</span>
                  {p.role && <span className="text-xs text-ink-muted truncate">{p.role}</span>}
                </span>
              </Pressable>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
