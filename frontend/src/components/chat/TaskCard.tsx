import { Check, ListChecks } from 'lucide-react'
import { useTasks, useUpdateTask } from '../../hooks/useChannelTasks'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { usePeople } from '../../hooks/usePeople'
import { explain, statusOf } from '../../lib/errors'
import { formatDate } from '../../lib/format'
import { displayName } from '../../lib/people'
import { useAuthStore } from '../../stores/auth.store'
import { Avatar, Button, Pressable, toast } from '../primitives'
import { StatusPill } from '../approval/StatusPill'
import { CardSkeleton, ChatCard, UnavailableCard } from './ChatCard'

const STATUS: Record<string, { tone: 'idle' | 'wait' | 'ok'; label: string }> = {
  open: { tone: 'idle', label: 'Chưa làm' },
  in_progress: { tone: 'wait', label: 'Đang làm' },
  done: { tone: 'ok', label: 'Đã xong' },
}

interface TaskCardProps {
  taskId: string
  channelId: string
  /** Where "Mở trong Công việc" goes; without it the button is not offered. */
  onOpenTasks?: () => void
}

/**
 * A task in the conversation (DESIGN.md "Công việc"): a check box for its
 * assignee and creator, the assignee by name, the due day and the status in
 * words. The server decides who may change it; a refusal is told and the card
 * stays as it was.
 */
export function TaskCard({ taskId, channelId, onOpenTasks }: TaskCardProps) {
  const tasks = useTasks(channelId)
  const update = useUpdateTask(channelId)
  const meId = useAuthStore((s) => s.user?.id)
  const { workspaceId } = useActiveWorkspace()
  const people = usePeople(workspaceId)

  if (tasks.isLoading) return <CardSkeleton />
  const t = tasks.data?.tasks.find((x) => x.id === taskId)
  if (!t) {
    return <UnavailableCard tone="success" icon={<ListChecks size={16} strokeWidth={1.75} />} title="Công việc không xem được" />
  }

  const done = t.status === 'done'
  const canToggle = !!meId && (t.assignee_id === meId || t.created_by === meId)
  const assignee = t.assignee_id ? t.assignee_name || displayName(people, t.assignee_id) : ''
  const person = t.assignee_id ? people.byUserId.get(t.assignee_id) : undefined
  const status = STATUS[t.status] ?? STATUS.open!

  const toggle = () =>
    update.mutate(
      { taskId: t.id, status: done ? 'open' : 'done' },
      {
        onError: (e) =>
          toast.error(statusOf(e) === 403 ? 'Bạn không đổi được trạng thái công việc này' : explain(e, 'đổi trạng thái công việc')),
      },
    )

  const box = `grid place-items-center w-5 h-5 rounded-md border ${done ? 'bg-success border-success text-on-success' : 'border-line bg-raised'}`
  return (
    <ChatCard
      tone="success"
      done={done}
      icon={
        canToggle ? (
          <Pressable
            role="checkbox"
            aria-checked={done}
            aria-label={`${done ? 'Mở lại' : 'Đánh dấu xong'}: ${t.title}`}
            onClick={toggle}
            className="grid place-items-center w-full h-full min-h-11 min-w-11 -m-1.5 rounded-md"
          >
            <span className={box}>{done && <Check size={14} strokeWidth={2.5} aria-hidden="true" />}</span>
          </Pressable>
        ) : (
          <ListChecks size={16} strokeWidth={1.75} aria-hidden="true" />
        )
      }
      title={t.title}
      sub={['Công việc', t.due_date ? `Hạn ${formatDate(t.due_date).slice(0, 5)}` : undefined].filter(Boolean).join(' · ')}
    >
      <div className="flex flex-wrap items-center gap-2 min-h-8">
        {assignee ? (
          <span className="inline-flex items-center gap-2 min-w-0 text-sm">
            <Avatar name={assignee} hueKey={t.assignee_id} src={person?.avatarUrl} size={24} />
            <span className="truncate font-medium text-ink">{assignee}</span>
          </span>
        ) : (
          <span className="text-sm text-ink-muted">Chưa giao</span>
        )}
        <StatusPill pill={status} />
        {onOpenTasks && (
          <Button variant="ghost" size="sm" className="ml-auto" onClick={onOpenTasks}>Mở trong Công việc</Button>
        )}
      </div>
    </ChatCard>
  )
}
