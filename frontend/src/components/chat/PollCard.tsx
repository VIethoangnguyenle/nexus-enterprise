import { BarChart3, Check } from 'lucide-react'
import type { PollView } from '../../api/chat-cards'
import { useClosePoll, usePoll, useUnvotePoll, useVotePoll } from '../../hooks/usePolls'
import { usePeople } from '../../hooks/usePeople'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { explain, statusOf } from '../../lib/errors'
import { formatDateTime, toMillis } from '../../lib/format'
import { displayName } from '../../lib/people'
import { useAuthStore } from '../../stores/auth.store'
import { Avatar, Button, Pressable, toast } from '../primitives'
import { CardSkeleton, ChatCard, UnavailableCard } from './ChatCard'

const MAX_FACES = 3

const isClosed = (p: PollView, now = Date.now()) => {
  const ends = toMillis(p.ends_at)
  return !!p.closed || (ends > 0 && ends <= now)
}

/**
 * A poll in the conversation (DESIGN.md "Bình chọn"). What the viewer sees comes
 * from the server and is never worked out here: until they vote, and while the
 * poll is open, there are no counts or voters; the choices are the whole card.
 * After a vote, or once closed, bars, counts and (unless anonymous) the first
 * voters appear. The creator can close it; the server decides who really may.
 */
export function PollCard({ pollId }: { pollId: string }) {
  const poll = usePoll(pollId)
  const vote = useVotePoll(pollId)
  const unvote = useUnvotePoll(pollId)
  const close = useClosePoll(pollId)
  const meId = useAuthStore((s) => s.user?.id)
  const { workspaceId } = useActiveWorkspace()
  const people = usePeople(workspaceId)

  if (poll.isLoading) return <CardSkeleton />
  const p = poll.data
  if (!p) {
    const gone = [403, 404].includes(statusOf(poll.error) ?? 0)
    return <UnavailableCard tone="accent" icon={<BarChart3 size={16} strokeWidth={1.75} />} title={gone ? 'Bình chọn không xem được' : 'Chưa tải được bình chọn'} />
  }

  const closed = isClosed(p)
  const mine = new Set(p.my_option_ids ?? [])
  const showResults = closed || !!p.results_visible
  const total = p.total_votes ?? 0
  const top = Math.max(0, ...p.options.map((o) => o.vote_count))
  const busy = vote.isPending || unvote.isPending || close.isPending

  const choose = (optionId: string) => {
    if (busy || closed) return
    // A second tap on a choice of a multiple-choice poll takes it back; in a
    // single-choice poll the same choice stays (withdrawing is "Rút phiếu").
    if (mine.has(optionId)) {
      if (p.is_multi) unvote.mutate([optionId], { onError: (e) => toast.error(explain(e, 'rút phiếu')) })
      return
    }
    vote.mutate(optionId, { onError: (e) => toast.error(explain(e, 'bỏ phiếu')) })
  }

  const closeIt = () =>
    close.mutate(undefined, {
      onError: (e) => toast.error(statusOf(e) === 403 ? 'Bạn không đóng được bình chọn này' : explain(e, 'đóng bình chọn')),
    })

  const kind = p.is_multi ? 'Chọn nhiều' : 'Chọn một'
  const when = closed
    ? p.closed_at ? `Đã đóng lúc ${formatDateTime(p.closed_at)}` : 'Đã đóng'
    : p.ends_at ? `Đóng lúc ${formatDateTime(p.ends_at)}` : undefined
  const role = p.is_multi ? 'checkbox' : 'radio'

  return (
    <ChatCard
      tone="accent"
      icon={<BarChart3 size={16} strokeWidth={1.75} aria-hidden="true" />}
      title={p.question}
      sub={['Bình chọn', kind, when].filter(Boolean).join(' · ')}
      label={p.question}
    >
      <div role={closed ? 'list' : p.is_multi ? 'group' : 'radiogroup'} aria-label={p.question} className="grid gap-1.5">
        {[...p.options].sort((a, b) => a.position - b.position).map((o) => {
          const chosen = mine.has(o.id)
          const pct = showResults && total > 0 ? Math.round((o.vote_count / total) * 100) : 0
          const voters = showResults && !p.is_anonymous ? (o.voter_ids ?? []).slice(0, MAX_FACES) : []
          const body = (
            <>
              {showResults && (
                <span
                  aria-hidden="true"
                  className="absolute inset-y-0 left-0 bg-accent-wash transition-[width] duration-layout ease-out-expo"
                  style={{ width: `${pct}%` }}
                />
              )}
              <span className="relative flex items-center gap-2 min-w-0 w-full">
                {!closed && (
                  <span
                    aria-hidden="true"
                    className={`grid place-items-center shrink-0 w-4.5 h-4.5 border ${p.is_multi ? 'rounded-sm' : 'rounded-full'}
                      ${chosen ? 'bg-accent border-accent text-on-accent' : 'border-line bg-raised'}`}
                  >
                    {chosen && <Check size={12} strokeWidth={2.5} />}
                  </span>
                )}
                <span className="min-w-0 flex-1 text-left break-words text-ink">{o.text}</span>
                {closed && showResults && top > 0 && o.vote_count === top && (
                  <span className="shrink-0 px-2 h-5.5 inline-flex items-center rounded-full bg-success-wash text-success text-xs font-semibold">Nhiều nhất</span>
                )}
                {voters.length > 0 && (
                  <span data-testid="poll-voters" className="flex shrink-0 -space-x-1">
                    {voters.map((id) => {
                      const name = displayName(people, id)
                      const person = people.byUserId.get(id)
                      return <Avatar key={id} name={name} hueKey={id} src={person?.avatarUrl} size={20} />
                    })}
                  </span>
                )}
                {showResults && <span data-testid="poll-count" className="shrink-0 min-w-4 text-right text-sm tnum">{o.vote_count}</span>}
              </span>
            </>
          )
          const base = 'relative overflow-hidden flex items-center min-h-11 px-3 rounded-md bg-sunk'
          return closed ? (
            <div key={o.id} role="listitem" className={base}>{body}</div>
          ) : (
            <Pressable
              key={o.id}
              role={role}
              aria-checked={chosen}
              disabled={busy}
              onClick={() => choose(o.id)}
              className={`${base} press disabled:cursor-default disabled:opacity-100`}
            >
              {body}
            </Pressable>
          )
        })}
      </div>
      <div className="flex items-center gap-2 min-h-8 text-xs text-ink-muted">
        {showResults && <span>{total} người đã bình chọn</span>}
        <span className="ml-auto flex items-center gap-1">
          {!closed && mine.size > 0 && (
            <Button variant="ghost" size="sm" disabled={busy} onClick={() => unvote.mutate([...mine], { onError: (e) => toast.error(explain(e, 'rút phiếu')) })}>
              Rút phiếu
            </Button>
          )}
          {!closed && meId && p.created_by === meId && (
            <Button variant="ghost" size="sm" disabled={busy} onClick={closeIt}>Đóng bình chọn</Button>
          )}
        </span>
      </div>
    </ChatCard>
  )
}
