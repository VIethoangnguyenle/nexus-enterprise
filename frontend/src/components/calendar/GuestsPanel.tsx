import { useMemo } from 'react'
import { Hash, X } from 'lucide-react'
import { useChannels } from '../../hooks/useMessaging'
import { usePeople } from '../../hooks/usePeople'
import type { EditorForm, GuestPick } from '../../lib/calendar-editor'
import { guestStatus, type Span } from '../../lib/calendar-findtime'
import { isDirect } from '../../lib/conversations'
import { personHue } from '../../lib/person-hue'
import type { Person } from '../../lib/people'
import { Avatar, Checkbox, ChoicePicker, Heading, IconButton, PeoplePicker, type Choice } from '../primitives'

export const guestFromPerson = (p: Person): GuestPick => ({
  nodeId: p.nodeId,
  displayName: p.name,
  avatarUrl: p.avatarUrl || undefined,
  hue: personHue(p.userId) - 1,
})

interface Props {
  wsId: string
  form: EditorForm
  onChange: (patch: Partial<EditorForm>) => void
  /** Whoever organizes the event: the creator of a new one, the organizer of an existing one. Not removable. */
  organizer?: GuestPick
  /** Busy spans of each person on the day, by node, for the status under a name. */
  busy: Map<string, Span[]>
  span: { start: Date; end: Date } | null
}

/**
 * Khách (DESIGN.md §6, Trình soạn đầy đủ): a picker of people (never an email or an id), a picker of group
 * chats to invite whole, the guests with whether they are free at the chosen time, and what guests may do.
 */
export function GuestsPanel({ wsId, form, onChange, organizer, busy, span }: Props) {
  const people = usePeople(wsId)
  const channels = useChannels(wsId)

  const exclude = useMemo(() => {
    const out = new Set<string>()
    for (const g of [...form.guests, ...(organizer ? [organizer] : [])]) {
      const p = people.byNodeId.get(g.nodeId)
      if (p) out.add(p.userId)
    }
    return out
  }, [form.guests, organizer, people.byNodeId])

  const groups = useMemo<Choice[]>(
    () => (channels.data?.channels ?? []).filter((c) => !isDirect(c)).map((c) => ({
      id: c.id, name: c.name, hint: c.member_count ? `Nhóm · ${c.member_count} thành viên` : 'Nhóm',
    })),
    [channels.data],
  )
  const picked: Choice | null = form.inviteChannelId ? { id: form.inviteChannelId, name: form.inviteChannelName ?? '' } : null

  const row = (g: GuestPick, sub: { text: string; warn?: boolean } | null, onRemove?: () => void) => (
    <li key={g.nodeId} className="grid grid-cols-[2rem_minmax(0,1fr)_auto] items-center gap-2.5 min-h-10">
      <Avatar name={g.displayName} hue={g.hue} src={g.avatarUrl} size={32} />
      <span className="grid min-w-0 leading-tight">
        <span className="truncate text-sm text-ink">{g.displayName}</span>
        {sub && <span className={`truncate text-xs ${sub.warn ? 'text-warning' : 'text-ink-muted'}`}>{sub.text}</span>}
      </span>
      {onRemove ? (
        <IconButton aria-label={`Bỏ ${g.displayName}`} onClick={onRemove}><X size={16} strokeWidth={1.75} /></IconButton>
      ) : <span />}
    </li>
  )

  return (
    <section aria-label="Khách" className="grid content-start gap-3">
      <Heading as="h3" className="text-lg! font-semibold! font-body!">Khách</Heading>
      <PeoplePicker
        label="Thêm khách"
        placeholder="Thêm khách"
        people={people.list}
        value={[]}
        exclude={exclude}
        onChange={(next) => {
          const p = next[0]
          if (p) onChange({ guests: [...form.guests, guestFromPerson(p)] })
        }}
      />
      <ChoicePicker
        label="Mời cả nhóm"
        placeholder="Mời cả nhóm chat"
        icon={<Hash size={14} strokeWidth={1.75} />}
        choices={groups}
        value={picked}
        loading={channels.isLoading}
        emptyText="Không có nhóm nào để mời."
        onChange={(c) => onChange({ inviteChannelId: c?.id ?? null, inviteChannelName: c?.name ?? null })}
      />
      <ul className="m-0 p-0 list-none grid gap-1">
        {organizer && row(organizer, { text: 'Người tổ chức' })}
        {form.guests.map((g) => {
          const spans = busy.get(g.nodeId)
          const status = spans && span ? guestStatus(spans, span.start, span.end) : null
          return row(
            g,
            status ? { text: status.text, warn: !status.free } : null,
            () => onChange({ guests: form.guests.filter((x) => x.nodeId !== g.nodeId) }),
          )
        })}
      </ul>
      <div className="grid gap-2 pt-3 border-t border-line">
        <span className="text-xs font-semibold text-ink-muted">Khách có thể</span>
        <label className="flex items-center gap-2.5 text-sm text-ink cursor-pointer">
          <Checkbox label="Mời người khác" checked={form.guestsCanInvite} onChange={(e) => onChange({ guestsCanInvite: e.target.checked })} />
          Mời người khác
        </label>
        <label className="flex items-center gap-2.5 text-sm text-ink cursor-pointer">
          <Checkbox label="Xem danh sách khách" checked={form.guestsCanSeeList} onChange={(e) => onChange({ guestsCanSeeList: e.target.checked })} />
          Xem danh sách khách
        </label>
      </div>
    </section>
  )
}
