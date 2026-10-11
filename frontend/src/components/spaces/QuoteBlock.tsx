import { Quote } from 'lucide-react'
import type { QuotedMessage } from '../../api/messaging'
import { quoteSummary } from '../../lib/message-model'
import { displayName, type PeopleDirectory } from '../../lib/people'
import { Avatar, Pressable } from '../primitives'

interface QuoteBlockProps {
  quote: QuotedMessage
  people: PeopleDirectory
  me: string | undefined
  /** Scrolls to the quoted message; absent when it is not in the part of the conversation that is loaded. */
  onOpen?: () => void
}

/**
 * What a message quotes (DESIGN.md "Trích dẫn"): a raised block inside the
 * bubble with the quoted person's name and at most two lines of their text, or
 * that the message was deleted. Pressing it goes to the original.
 */
export function QuoteBlock({ quote, people, me, onOpen }: QuoteBlockProps) {
  const own = !!me && quote.sender_id === me
  const view = quoteSummary(quote, (id, fallback) => (own ? 'Bạn' : displayName(people, id, fallback)))
  const avatarUrl = people.byUserId.get(quote.sender_id)?.avatarUrl
  const body = (
    <>
      <span className="flex items-center gap-1.5 font-semibold text-ink">
        <Quote size={12} strokeWidth={1.75} aria-hidden="true" className="text-accent shrink-0" />
        <Avatar name={view.name} hueKey={quote.sender_id || view.name} src={avatarUrl} size={20} />
        <span className="truncate">{view.name}</span>
      </span>
      <span className={`line-clamp-2 wrap-anywhere text-ink-muted ${view.deleted ? 'italic' : ''}`}>{view.text}</span>
    </>
  )
  const frame = 'grid gap-0.5 w-full text-left -mx-1.5 mt-px mb-1.5 px-2.5 pt-1.75 pb-2 rounded-lg bg-raised text-sm leading-snug'
  if (!onOpen) return <div className={frame}>{body}</div>
  return (
    <Pressable onClick={onOpen} title="Đi tới tin gốc" className={`${frame} hover:bg-hover`}>
      {body}
    </Pressable>
  )
}
