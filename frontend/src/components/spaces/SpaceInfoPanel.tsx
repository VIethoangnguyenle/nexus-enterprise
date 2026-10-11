import { useState } from 'react'
import { ChevronRight, Pin } from 'lucide-react'
import { useUpdateChannel } from '../../hooks/useMessaging'
import { usePins } from '../../hooks/useMessages'
import { explain } from '../../lib/errors'
import { formatDateTime, formatListTime } from '../../lib/format'
import { displayName, type PeopleDirectory, type Person } from '../../lib/people'
import type { NotificationLevel } from '../../api/notification-levels'
import { optionFor } from '../../lib/notification-level-copy'
import { Avatar, Button, PersonChip, Pressable, Spinner, TextField, toast } from '../primitives'
import { LEVEL_ICON } from './ConversationTitleMenu'
import { NotificationLevelSheet } from './NotificationLevelSheet'
import { SidePanel } from './SidePanel'

function plain(html: string): string {
  return html.replace(/<[^>]+>/g, ' ').replace(/&nbsp;/g, ' ').replace(/\s+/g, ' ').trim()
}

/** What the phone's info screen needs to show and set the notification level. */
export interface InfoNotification {
  level: NotificationLevel
  /** A save is taking long enough to say so. */
  saving: boolean
  onChoose: (level: NotificationLevel) => void
}

/**
 * About a conversation: on a phone the Thông báo row first (it opens the level
 * sheet), then the name (renamable for spaces), and pinned messages.
 */
export function SpaceInfoPanel({ channelId, kind, title, memberCount, partner, people, notification, onOpenTopic, onClose }: {
  channelId: string
  kind: 'dm' | 'space'
  title: string
  memberCount: number
  partner?: Person
  people: PeopleDirectory
  /** Phone only: the desktop sets the level from the header menu. */
  notification?: InfoNotification
  onOpenTopic: (id: string) => void
  onClose: () => void
}) {
  const pins = usePins(channelId)
  const rename = useUpdateChannel(channelId)
  const [editing, setEditing] = useState(false)
  const [levelSheet, setLevelSheet] = useState(false)
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
        {notification && (
          <NotificationRow kind={kind} notification={notification} onOpen={() => setLevelSheet(true)} />
        )}
        {notification && levelSheet && (
          <NotificationLevelSheet
            conversation={{ kind, title }}
            level={notification.level}
            onChoose={notification.onChoose}
            onClose={() => setLevelSheet(false)}
          />
        )}
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

/** The first row of the phone's info screen: the level in force, and a tap to change it. */
function NotificationRow({ kind, notification, onOpen }: {
  kind: 'dm' | 'space'
  notification: InfoNotification
  onOpen: () => void
}) {
  const option = optionFor(kind, notification.level)
  const Icon = LEVEL_ICON[option.level]
  return (
    <Pressable
      onClick={onOpen}
      aria-haspopup="dialog"
      className="flex items-center gap-3 w-full min-h-14 px-2 rounded-surface text-left hover:bg-hover"
    >
      <span aria-hidden="true" className="grid place-items-center w-8 h-8 rounded-md bg-sunk text-ink-muted shrink-0">
        <Icon size={16} strokeWidth={1.75} />
      </span>
      <span className="grid min-w-0 flex-1">
        <span className="font-semibold text-ink">Thông báo</span>
        <span className="flex items-center gap-1.5 text-small text-ink-muted">
          {notification.saving ? (
            <>
              <Spinner size="sm" />
              Đang lưu…
            </>
          ) : (
            <span className="truncate">{option.title}</span>
          )}
        </span>
      </span>
      <ChevronRight size={18} strokeWidth={1.75} aria-hidden="true" className="text-ink-muted shrink-0" />
    </Pressable>
  )
}
