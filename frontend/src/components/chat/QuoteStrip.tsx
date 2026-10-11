import { FileText, Image as ImageIcon, ListChecks, Quote, X } from 'lucide-react'
import { motion } from 'motion/react'
import { useMotionPresets } from '../../lib/motion'
import { displayName, type PeopleDirectory } from '../../lib/people'
import { DELETED_QUOTE } from '../../lib/message-model'
import type { PendingQuote, QuoteSummary } from '../../stores/drafts.store'
import { Avatar, IconButton, Pressable } from '../primitives'

interface QuoteStripProps {
  quote: PendingQuote
  people: PeopleDirectory
  me: string | undefined
  /** ✕ or Esc: the message goes without the quote. */
  onClear: () => void
  /** The strip was pressed: go to the message it quotes. Absent when it cannot be reached. */
  onGoTo?: () => void
}

const SUMMARY_ICON: Record<QuoteSummary['icon'], typeof ImageIcon> = { image: ImageIcon, file: FileText, card: ListChecks }

/**
 * The quote waiting in a composer (DESIGN.md "Trích dẫn đang chờ"): the first
 * row of the box, a sunk strip with the quoted person and one line of what they
 * said, or of what the message holds when it has no text. When the quoted
 * message was deleted the strip loses its fill, says so, and the message will
 * go without it. Pressing it goes to the original; ✕ takes the quote away.
 */
export function QuoteStrip({ quote, people, me, onClear, onGoTo }: QuoteStripProps) {
  const m = useMotionPresets()
  const own = !!me && quote.sender_id === me
  const name = own ? 'Bạn' : displayName(people, quote.sender_id, quote.sender_name)
  const gone = !!quote.deleted
  const avatarUrl = people.byUserId.get(quote.sender_id)?.avatarUrl
  const SummaryIcon = quote.summary ? SUMMARY_ICON[quote.summary.icon] : null
  const label = `Đang trích dẫn tin của ${own ? 'bạn' : name}${gone ? ', tin này đã bị xoá' : ''}`

  const head = (
    <span className="flex items-center gap-1.5 font-semibold min-w-0">
      <Quote size={12} strokeWidth={1.75} aria-hidden="true" className={`shrink-0 ${gone ? 'text-ink-muted' : 'text-accent'}`} />
      <Avatar name={name} hueKey={quote.sender_id || name} src={avatarUrl} size={20} />
      <span className="truncate">{name}</span>
    </span>
  )
  const line = gone ? (
    <span className="italic truncate">{DELETED_QUOTE}</span>
  ) : (
    <span className="flex items-center gap-1.25 min-w-0">
      {SummaryIcon && <SummaryIcon size={13} strokeWidth={1.75} aria-hidden="true" className="shrink-0" />}
      <span className="truncate">{quote.text || quote.summary?.label || ''}</span>
    </span>
  )
  const body = (
    <>
      {head}
      <span className="text-ink-muted min-w-0 block">{line}</span>
    </>
  )
  const bodyClass = 'grid gap-px min-w-0 min-h-11 py-1.25 text-left text-small leading-snug'

  return (
    <motion.div
      role="region"
      aria-label={label}
      initial={m.reduced ? { opacity: 0 } : { opacity: 0, y: 4 }}
      animate={{ opacity: 1, y: 0, transition: { duration: m.reduced ? 0.12 : 0.16 } }}
      className={`grid grid-cols-[minmax(0,1fr)_auto] items-center gap-1 pl-2.5 pr-0.5 rounded-surface ${
        gone ? 'ring-1 ring-inset ring-line' : 'bg-sunk'
      }`}
    >
      {onGoTo && !gone ? (
        <Pressable title="Đi tới tin gốc" onClick={onGoTo} className={`${bodyClass} text-ink rounded-md`}>
          {body}
        </Pressable>
      ) : (
        <div className={`${bodyClass} text-ink`}>{body}</div>
      )}
      <IconButton
        type="button"
        aria-label="Bỏ trích dẫn"
        title="Bỏ trích dẫn (Esc)"
        onClick={onClear}
        className="relative after:absolute after:-inset-1.5 after:content-[''] max-md:w-11 max-md:h-11"
      >
        <X size={16} strokeWidth={1.75} aria-hidden="true" />
      </IconButton>
      {gone && <p className="col-span-2 -mt-0.5 mb-1.75 text-xs text-ink-muted">Tin của bạn sẽ được gửi không kèm trích dẫn.</p>}
    </motion.div>
  )
}
