import { CalendarDays, ClipboardCheck, Folder, Package, type LucideIcon } from 'lucide-react'
import type { Asset } from '../../api/assets'
import type { ApprovalRequest } from '../../api/approval'
import type { Occurrence } from '../../api/calendar'
import type { DriveItem } from '../../api/drive'
import type { Message } from '../../api/messaging'
import { stateLabel } from '../../lib/asset-model'
import { requestTitle } from '../../lib/approval-model'
import { formatWhen, spanOf } from '../../lib/calendar-format'
import type { Conversation } from '../../lib/conversations'
import { fileKind } from '../../lib/file-kind'
import { formatListTime, formatMessageTime } from '../../lib/format'
import { messageText } from '../../lib/message-model'
import { displayName, type PeopleDirectory, type Person } from '../../lib/people'
import { snippet } from '../../lib/search-text'
import { conversationSub } from '../../lib/suggestions/chat'
import type { SuggestionItem } from '../../hooks/useSuggestionPanel'
import { Avatar } from '../primitives'
import { ConversationIcon } from '../spaces/ConversationIcon'
import type { SearchAction } from './search-actions'
import { driveKindOf, statusLabel } from './search-filters'

/** A message's opening, on one line. */
const SNIPPET = 90

/** The 36px tile of a result that is not a person (DESIGN.md §6: hàng 52, icon 36). */
export function Tile({ icon: Icon, tone = 'bg-sunk text-ink-muted' }: { icon: LucideIcon; tone?: string }) {
  return (
    <span aria-hidden="true" className={`grid place-items-center size-9 rounded-surface ${tone}`}>
      <Icon size={18} strokeWidth={1.75} />
    </span>
  )
}

const line = (...parts: (string | undefined | false)[]) => parts.filter(Boolean).join(' · ')

/** Builds the rows of each kind; `pick` keeps the words as a recent search and then runs `go`. */
export interface RowContext {
  term: string
  people: PeopleDirectory
  conversationsById: Map<string, Conversation>
  pick: (go: () => void) => () => void
}

export function personRow(p: Person, ctx: RowContext, open: (p: Person) => void): SuggestionItem {
  return {
    key: `person:${p.userId}`,
    title: p.name,
    mark: ctx.term,
    meta: p.role || undefined,
    label: p.role ? `${p.name}, ${p.role}` : p.name,
    icon: <Avatar name={p.name} hueKey={p.userId} src={p.avatarUrl} size={36} />,
    onSelect: ctx.pick(() => open(p)),
  }
}

export function conversationRow(c: Conversation, ctx: RowContext, open: (c: Conversation) => void): SuggestionItem {
  const sub = conversationSub(c)
  return {
    key: `conversation:${c.id}`,
    title: c.title,
    mark: ctx.term,
    meta: sub,
    label: `${c.title}, ${sub}`,
    icon: <ConversationIcon conversation={c} size={36} />,
    onSelect: ctx.pick(() => open(c)),
  }
}

export function messageRow(m: Message, ctx: RowContext, open: (m: Message) => void): SuggestionItem {
  const sender = displayName(ctx.people, m.sender_id, m.sender_name)
  const where = ctx.conversationsById.get(m.channel_id)?.title
  const text = snippet(messageText(m.content), ctx.term, SNIPPET)
  return {
    key: `message:${m.id}`,
    title: text,
    mark: ctx.term,
    meta: line(sender, where, formatMessageTime(m.created_at)),
    label: `Tin của ${sender}${where ? ` trong ${where}` : ''}: ${text}`,
    icon: <Avatar name={sender} hueKey={m.sender_id} src={ctx.people.byUserId.get(m.sender_id)?.avatarUrl} size={36} />,
    onSelect: ctx.pick(() => open(m)),
  }
}

export function documentRow(it: DriveItem, ctx: RowContext, open: (it: DriveItem) => void): SuggestionItem {
  const folder = it.item_type === 'folder'
  const kind = folder ? null : fileKind(it.name, it.mime_type)
  return {
    key: `document:${it.id}`,
    title: it.name,
    mark: ctx.term,
    meta: line(driveKindOf(it), it.owner_name, formatListTime(it.updated_at)),
    icon: folder ? <Tile icon={Folder} tone="bg-accent-wash text-accent" /> : <Tile icon={kind!.icon} tone={kind!.tile} />,
    onSelect: ctx.pick(() => open(it)),
  }
}

export function approvalRow(r: ApprovalRequest, ctx: RowContext, open: (r: ApprovalRequest) => void): SuggestionItem {
  const title = requestTitle(r)
  return {
    key: `approval:${r.id}`,
    title,
    mark: ctx.term,
    meta: line(statusLabel(r.status), r.created_by_name),
    icon: <Tile icon={ClipboardCheck} />,
    onSelect: ctx.pick(() => open(r)),
  }
}

export function eventRow(o: Occurrence, ctx: RowContext, open: (o: Occurrence) => void): SuggestionItem {
  const { start, end } = spanOf(o)
  return {
    key: `event:${o.eventId}:${o.occurrenceStart}`,
    title: o.title,
    mark: ctx.term,
    meta: line(formatWhen(start, end, o.allDay), o.location),
    icon: <Tile icon={CalendarDays} />,
    onSelect: ctx.pick(() => open(o)),
  }
}

export function assetRow(a: Asset, ctx: RowContext, open: (a: Asset) => void): SuggestionItem {
  const holder = a.assigned_to_user_id ? displayName(ctx.people, a.assigned_to_user_id, a.assigned_to_name) : undefined
  return {
    key: `asset:${a.id}`,
    title: a.name,
    mark: ctx.term,
    meta: line(a.type_name, stateLabel(a.state), holder),
    icon: <Tile icon={Package} />,
    onSelect: ctx.pick(() => open(a)),
  }
}

export function actionRow(a: SearchAction, run: (a: SearchAction) => void): SuggestionItem {
  return {
    key: `action:${a.id}`,
    title: a.label,
    icon: <Tile icon={a.icon} tone="bg-accent-wash text-accent" />,
    onSelect: () => run(a),
  }
}
