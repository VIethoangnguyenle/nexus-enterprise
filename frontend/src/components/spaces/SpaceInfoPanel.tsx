import { useState } from 'react'
import { Pin } from 'lucide-react'
import { usePins, useUpdateChannel } from '../../hooks/useMessaging'
import { explain } from '../../lib/errors'
import { formatDateTime, formatListTime } from '../../lib/format'
import { displayName, type PeopleDirectory, type Person } from '../../lib/people'
import { Avatar, Button, PersonChip, Pressable, TextField, toast } from '../primitives'
import { SidePanel } from './SidePanel'

function plain(html: string): string {
  return html.replace(/<[^>]+>/g, ' ').replace(/&nbsp;/g, ' ').replace(/\s+/g, ' ').trim()
}

/** About a conversation: the name (renamable for spaces), and pinned messages. */
export function SpaceInfoPanel({ channelId, kind, title, memberCount, partner, people, onOpenTopic, onClose }: {
  channelId: string
  kind: 'dm' | 'space'
  title: string
  memberCount: number
  partner?: Person
  people: PeopleDirectory
  onOpenTopic: (id: string) => void
  onClose: () => void
}) {
  const pins = usePins(channelId)
  const rename = useUpdateChannel(channelId)
  const [editing, setEditing] = useState(false)
  const [name, setName] = useState(title)
  const list = pins.data?.pins ?? []

  const save = () => {
    const n = name.trim()
    if (!n || n === title) {
      setEditing(false)
      return
    }
    rename.mutate(
      { name: n },
      {
        onSuccess: () => {
          setEditing(false)
          toast(`Đã đổi tên nhóm thành “${n}”`)
        },
        onError: (err) => toast.error(explain(err, 'đổi tên nhóm')),
      },
    )
  }

  return (
    <SidePanel
      label="Thông tin"
      title={kind === 'space' ? 'Thông tin nhóm' : 'Thông tin'}
      sub={title}
      closeLabel="thông tin"
      onClose={onClose}
    >
      <div className="grid gap-4 px-3 pb-3">
        {kind === 'space' ? (
          editing ? (
            <form
              className="grid gap-2"
              onSubmit={(e) => {
                e.preventDefault()
                save()
              }}
            >
              <TextField label="Tên nhóm" value={name} maxLength={128} counter autoFocus onChange={(e) => setName(e.target.value)} />
              <div className="flex justify-end gap-2">
                <Button type="button" variant="ghost" size="sm" onClick={() => { setName(title); setEditing(false) }}>Huỷ</Button>
                <Button type="submit" variant="primary" size="sm" loading={rename.isPending}>Lưu</Button>
              </div>
            </form>
          ) : (
            <div className="flex items-center justify-between gap-2">
              <div className="grid min-w-0">
                <span className="text-xs text-ink-muted">Tên nhóm</span>
                <span className="font-semibold text-ink truncate">{title}</span>
                <span className="text-xs text-ink-muted tnum">{memberCount} thành viên</span>
              </div>
              <Button variant="soft" size="sm" onClick={() => setEditing(true)}>Đổi tên</Button>
            </div>
          )
        ) : partner ? (
          <div className="grid gap-1">
            <span className="text-xs text-ink-muted">Trò chuyện với</span>
            <PersonChip name={partner.name} hueKey={partner.userId} role={partner.role} avatarUrl={partner.avatarUrl} />
          </div>
        ) : null}

        <div className="grid gap-1">
          <span className="text-label text-ink-muted">Đã ghim</span>
          {pins.isLoading ? (
            <div className="skeleton h-12 rounded-surface" />
          ) : list.length === 0 ? (
            <span className="text-small text-ink-muted">Chưa ghim tin nào. Di chuột lên một tin rồi chọn Ghim.</span>
          ) : (
            <ul className="grid gap-0.5 list-none m-0 p-0">
              {list.map((p) => {
                const msg = p.message
                const name = displayName(people, msg.sender_id, msg.sender_name)
                return (
                  <li key={msg.id}>
                    <Pressable
                      onClick={() => onOpenTopic(msg.parent_message_id || msg.id)}
                      className="w-full grid grid-cols-[24px_minmax(0,1fr)] gap-2.5 px-2 py-2 rounded-surface hover:bg-hover"
                    >
                      <Avatar name={name} hueKey={msg.sender_id || name} size={24} />
                      <span className="grid gap-0.5 min-w-0">
                        <span className="flex items-baseline gap-2">
                          <span className="font-semibold text-ink truncate">{name}</span>
                          <time className="text-xs text-ink-muted tnum" title={formatDateTime(msg.created_at)}>
                            {formatListTime(msg.created_at)}
                          </time>
                          <Pin size={12} strokeWidth={1.75} className="text-ink-muted" aria-label="Đã ghim" />
                        </span>
                        <span className="text-small text-ink-muted line-clamp-2">{plain(msg.content)}</span>
                      </span>
                    </Pressable>
                  </li>
                )
              })}
            </ul>
          )}
        </div>
      </div>
    </SidePanel>
  )
}
