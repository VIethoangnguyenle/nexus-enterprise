import { Avatar } from '../primitives'
import type { ChainItem } from '../../lib/approval-model'
import { formatDateTime, formatListTime } from '../../lib/format'
import { StatusPill } from './StatusPill'

/**
 * The approval chain (DESIGN.md §6 ApprovalChain): one line per approver with
 * their avatar, name, status as a word, and under it the step and the time.
 * No approver or role code ever appears.
 */
export function ApprovalChain({ items, label = 'Chuỗi phê duyệt' }: { items: ChainItem[]; label?: string }) {
  if (items.length === 0) {
    return <p className="m-0 text-sm text-ink-muted">Đề nghị này chưa có bước duyệt nào.</p>
  }
  return (
    <ol aria-label={label} className="grid gap-3 m-0 p-0 list-none">
      {items.map((item) => (
        <li key={item.key} className="grid grid-cols-[24px_minmax(0,1fr)_auto] items-center gap-x-2.5 gap-y-0.5">
          <Avatar name={item.name} hueKey={item.hueKey} src={item.avatarUrl} size={24} className="row-span-2" />
          <span className="min-w-0 truncate text-sm">
            <b className="font-semibold">{item.name}</b>
            {item.isMe && <span className="text-ink-muted"> (bạn)</span>}
          </span>
          <StatusPill pill={item.pill} />
          <small className="col-start-2 col-span-2 text-xs text-ink-muted tnum">
            {item.stepName}
            {item.actedAt && (
              <>
                {' · '}
                <time title={formatDateTime(item.actedAt)}>{formatListTime(item.actedAt)}</time>
              </>
            )}
            {!item.actedAt && item.since && (
              <>
                {' · từ '}
                <time title={formatDateTime(item.since)}>{formatListTime(item.since)}</time>
              </>
            )}
          </small>
          {item.comment && (
            <q className="col-start-2 col-span-2 text-sm text-ink-muted italic">{item.comment}</q>
          )}
        </li>
      ))}
    </ol>
  )
}
