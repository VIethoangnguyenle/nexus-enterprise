import { FileText } from 'lucide-react'
import { Link } from '@tanstack/react-router'
import { useTextDocument } from '../../hooks/useDocuments'
import { statusOf } from '../../lib/errors'
import { formatTime } from '../../lib/format'
import { CardSkeleton, ChatCard, UnavailableCard } from './ChatCard'

const icon = <FileText size={16} strokeWidth={1.75} aria-hidden="true" />

/**
 * A document in the conversation (DESIGN.md "Tài liệu mới"). The details come
 * from the documents service under the viewer's own access: without it the card
 * says only that the document cannot be viewed, and shows nothing of it.
 */
export function DocCard({ docId }: { docId: string }) {
  const doc = useTextDocument(docId)
  if (doc.isLoading) return <CardSkeleton />
  const d = doc.data
  if (!d) {
    const denied = [403, 404].includes(statusOf(doc.error) ?? 0)
    return <UnavailableCard tone="info" icon={icon} title={denied ? 'Tài liệu không xem được' : 'Chưa tải được tài liệu'} />
  }
  return (
    <ChatCard
      tone="info"
      icon={icon}
      title={d.title || 'Tài liệu chưa đặt tên'}
      sub={`Tài liệu · Sửa lần cuối ${formatTime(d.updated_at)}`}
    >
      <div className="flex items-center gap-2 min-h-8 text-xs text-ink-muted">
        <span className="min-w-0 truncate">{d.owner_name}</span>
        <Link
          to="/documents/$docId"
          params={{ docId: d.id }}
          className="press ml-auto inline-flex items-center justify-center min-h-11 md:min-h-8 px-3 rounded-md bg-hover
            text-sm font-semibold text-ink no-underline focus-ring hover:bg-line"
        >
          Mở
        </Link>
      </div>
    </ChatCard>
  )
}
