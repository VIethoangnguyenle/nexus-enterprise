import { MessageSquareReply } from 'lucide-react'
import { Button } from '../../primitives'

/**
 * Stands where the main composer would be for a plain member of a group whose
 * owner keeps top-level posting for the owner and managers. Replying in a
 * thread stays open to everyone, so that is what it offers.
 */
export function RestrictedPostingNotice({ onReplyInThread }: { onReplyInThread?: () => void }) {
  return (
    <div role="note" className="flex items-center gap-3 mx-3 mb-3 px-3.5 py-3 rounded-2xl bg-raised text-sm text-ink-muted">
      <span className="flex-1 min-w-0">
        Chỉ chủ nhóm và quản trị nhóm được gửi tin mới trong nhóm này. Bạn vẫn trả lời được trong thread của một tin.
      </span>
      {onReplyInThread && (
        <Button variant="soft" size="sm" onClick={onReplyInThread}>
          <MessageSquareReply size={16} strokeWidth={1.75} aria-hidden="true" /> Trả lời trong thread
        </Button>
      )}
    </div>
  )
}
