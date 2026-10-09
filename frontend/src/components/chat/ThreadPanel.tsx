import { useEffect, useMemo, useRef } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { AnimatePresence, motion } from 'motion/react'
import { CircleAlert } from 'lucide-react'
import { useThread, useSendReply } from '../../hooks/useMessaging'
import { useArrivals } from '../../hooks/useArrivals'
import { useMotionPresets } from '../../lib/motion'
import { personStyle } from '../../lib/person-hue'
import { explain } from '../../lib/errors'
import type { PeopleDirectory } from '../../lib/people'
import type { Message } from '../../api/messaging'
import { Button, toast } from '../primitives'
import { ChatEditor } from './ChatEditor'
import { SidePanel } from '../spaces/SidePanel'
import { MessageBlock } from '../spaces/MessageBlock'
import { EmptyState } from '../spaces/EmptyState'

type Msg = Message & { _optimistic?: boolean }

interface ThreadPanelProps {
  channelId: string
  /** The topic (top-level message) whose replies to show. */
  messageId: string
  /** Space or DM title, shown under "Chủ đề". */
  spaceName: string
  people: PeopleDirectory
  me: string | undefined
  onClose: () => void
}

/**
 * A topic opened at the side (design/mockups/spaces.html §2): the opening
 * message, a divider with the reply count, the replies, and a reply box.
 * Replies from others arrive in their hue with "vừa trả lời"; yours only slide in.
 * "Đang theo dõi" from the mockup is not shown: there is no follow API yet.
 */
export function ThreadPanel({ channelId, messageId, spaceName, people, me, onClose }: ThreadPanelProps) {
  const m = useMotionPresets()
  const qc = useQueryClient()
  const { data, isLoading, error, refetch } = useThread(messageId)
  const send = useSendReply(channelId, messageId)
  const bottomRef = useRef<HTMLDivElement>(null)

  const all = (data?.messages ?? []) as Msg[]
  const fromList = qc
    .getQueryData<{ messages: Message[] }>(['messages', channelId])
    ?.messages.find((x) => x.id === messageId)
  const root = all.find((x) => x.id === messageId) ?? fromList
  const replies = useMemo(() => all.filter((x) => x.id !== messageId), [all, messageId])

  const { fresh } = useArrivals(replies, {
    keyOf: (r) => r.id,
    authorOf: (r) => r.sender_id,
    me,
    ready: !isLoading,
    scope: messageId,
  })

  useEffect(() => {
    bottomRef.current?.scrollIntoView?.({ block: 'end', behavior: m.reduced ? 'auto' : 'smooth' })
  }, [replies.length, m.reduced])

  return (
    <SidePanel
      label="Chủ đề"
      title="Chủ đề"
      sub={spaceName}
      closeLabel="chủ đề"
      onClose={onClose}
      footer={
        <ChatEditor
          channelId={channelId}
          people={people}
          tone="base"
          placeholder="Trả lời trong chủ đề"
          isPending={send.isPending}
          onSend={(html) =>
            send.mutate(html, { onError: (err) => toast.error(explain(err, 'gửi câu trả lời')) })
          }
        />
      }
    >
      {error && !root ? (
        <EmptyState
          compact
          icon={<CircleAlert size={24} strokeWidth={1.75} />}
          text="Chưa tải được chủ đề này."
          action={<Button variant="soft" size="sm" onClick={() => refetch()}>Thử lại</Button>}
        />
      ) : (
        <div className="grid content-start gap-0.5">
          {root && <MessageBlock message={root} people={people} idBase={`thread-root-${messageId}`} me={me} className="px-3" />}
          <div className="flex items-center gap-2.5 px-3 py-1.5 text-xs font-semibold text-ink-muted after:content-[''] after:h-px after:flex-1 after:bg-line">
            {isLoading ? 'Đang tải câu trả lời' : replies.length ? `${replies.length} trả lời` : 'Chưa có câu trả lời'}
          </div>
          <AnimatePresence initial={false}>
            {replies.map((r) => {
              const a = fresh.get(r.id)
              const byOther = a?.source === 'other'
              return (
                <motion.div key={r.id} {...m.row}>
                  <div
                    data-realtime={a?.source}
                    style={byOther ? personStyle(a!.author ?? r.sender_id) : undefined}
                    className={`rounded-surface ${byOther ? 'rt-wash' : ''}`}
                  >
                    <MessageBlock
                      message={r}
                      people={people}
                      idBase={`reply-${r.id}`}
                      me={me}
                      halo={byOther}
                      tag={byOther ? 'vừa trả lời' : undefined}
                      className="px-3"
                    />
                  </div>
                </motion.div>
              )
            })}
          </AnimatePresence>
          <div ref={bottomRef} />
        </div>
      )}
    </SidePanel>
  )
}
