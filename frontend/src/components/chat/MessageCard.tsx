import type { Message } from '../../api/messaging'
import { cardKindOf } from '../../lib/message-model'
import { ApprovalCard } from './ApprovalCard'
import { DocCard } from './DocCard'
import { PollCard } from './PollCard'
import { TaskCard } from './TaskCard'
import { CallCard } from '../calls/CallCard'
import { EventChatCard } from '../calendar/EventChatCard'

/**
 * The card of a message (DESIGN.md "Thẻ"): a poll, a task, a document, an
 * approval request, a call, or an event of the group's calendar — read live from its own service under the
 * viewer's access. The task card gets no "open in Công việc" link: that area
 * has no route yet. Call cards have no thread affordance.
 */
export function MessageCard({ message: m }: { message: Message }) {
  const kind = cardKindOf(m)
  const id = m.linked_entity_id ?? ''
  if (kind === 'poll') return <PollCard pollId={id} />
  if (kind === 'task') return <TaskCard taskId={id} channelId={m.channel_id} />
  if (kind === 'document') return <DocCard docId={id} />
  if (kind === 'approval') return <ApprovalCard requestId={id} />
  if (kind === 'call') return <CallCard callId={id} channelId={m.channel_id} />
  if (kind === 'calendar_event') return <EventChatCard eventId={id} channelId={m.channel_id} text={m.content} />
  return null
}
