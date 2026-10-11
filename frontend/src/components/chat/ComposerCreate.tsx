import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { useSendCardMessage } from '../../hooks/useMessages'
import { explain } from '../../lib/errors'
import { CreateRequestDialog } from '../approval/CreateRequestDialog'
import { toast } from '../primitives'
import type { CreateKind } from './CreateMenuItems'
import { DocDialog } from './DocDialog'
import { PollDialog } from './PollDialog'
import { TaskDialog } from './TaskDialog'

interface ComposerCreateProps {
  kind: CreateKind
  channelId: string
  /** The composer of a thread: what is created is posted as a reply to this root. */
  parentMessageId?: string
  onClose: () => void
}

/**
 * The dialog behind a choice in the composer's "+" menu. Polls and tasks make
 * their own message on the server; a document and an approval request are made
 * first and then posted as a card message, in the thread when there is one.
 * Mounted only while a choice is open, so nothing here loads before it.
 */
export function ComposerCreate({ kind, channelId, parentMessageId, onClose }: ComposerCreateProps) {
  const { workspaceId } = useActiveWorkspace()
  const card = useSendCardMessage(channelId, parentMessageId)

  if (kind === 'poll') return <PollDialog open onClose={onClose} channelId={channelId} parentMessageId={parentMessageId} />
  if (kind === 'task') return <TaskDialog open onClose={onClose} channelId={channelId} parentMessageId={parentMessageId} />
  if (kind === 'doc') {
    return <DocDialog open onClose={onClose} workspaceId={workspaceId} channelId={channelId} parentMessageId={parentMessageId} />
  }

  const post = (requestId: string) => {
    card.send({ type: 'approval', id: requestId }).catch((err: unknown) => {
      toast.error(`Đã tạo đề nghị nhưng chưa gửi được vào cuộc trò chuyện. ${explain(err, 'gửi đề nghị vào cuộc trò chuyện')}`, {
        action: { label: 'Thử lại', onClick: () => post(requestId) },
      })
    })
  }
  return <CreateRequestDialog open onClose={onClose} onCreated={(request) => post(request.id)} />
}
