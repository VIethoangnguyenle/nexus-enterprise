import type { Message } from '../../api/messaging'
import { ApprovalCard } from './ApprovalCard'
import { DocCard } from './DocCard'
import { PollCard } from './PollCard'
import { TaskCard } from './TaskCard'

export type CardKind = 'poll' | 'task' | 'document' | 'approval'

/** The card a message carries, if any: the object it links to is shown in place of (or besides) its text. */
export function cardKindOf(m: Pick<Message, 'linked_entity_type' | 'linked_entity_id'>): CardKind | undefined {
  if (!m.linked_entity_id) return undefined
  const t = m.linked_entity_type
  return t === 'poll' || t === 'task' || t === 'document' || t === 'approval' ? t : undefined
}

/**
 * The card of a message (DESIGN.md "Thẻ"): a poll, a task, a document or an
 * approval request, read live from its own service under the viewer's access.
 * The task card gets no "open in Công việc" link: that area has no route yet.
 */
export function MessageCard({ message: m }: { message: Message }) {
  const kind = cardKindOf(m)
  const id = m.linked_entity_id ?? ''
  if (kind === 'poll') return <PollCard pollId={id} />
  if (kind === 'task') return <TaskCard taskId={id} channelId={m.channel_id} />
  if (kind === 'document') return <DocCard docId={id} />
  if (kind === 'approval') return <ApprovalCard requestId={id} />
  return null
}
