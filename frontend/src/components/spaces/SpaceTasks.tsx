import { useState } from 'react'
import { Check, CircleAlert, ListChecks } from 'lucide-react'
import { useCreateTask, useTasks, useUpdateTask } from '../../hooks/useChannelTasks'
import { explain } from '../../lib/errors'
import { formatDate } from '../../lib/format'
import { displayName, type PeopleDirectory } from '../../lib/people'
import { Button, PersonChip, Pressable, TextField, toast } from '../primitives'
import { EmptyState } from './EmptyState'

/** Readable task states (never the stored code). */
const STATUS: Record<string, { label: string; pill: string }> = {
  todo: { label: 'Cần làm', pill: 'bg-info-wash text-info' },
  open: { label: 'Cần làm', pill: 'bg-info-wash text-info' },
  in_progress: { label: 'Đang làm', pill: 'bg-warning-wash text-warning' },
  done: { label: 'Đã xong', pill: 'bg-success-wash text-success' },
}

/**
 * Tasks of a space (existing tasks API): add by title, tick to finish. Each
 * row names the assignee by display name and shows the due date.
 */
export function SpaceTasks({ channelId, people }: { channelId: string; people: PeopleDirectory }) {
  const { data, isLoading, isError, refetch } = useTasks(channelId)
  const create = useCreateTask(channelId)
  const update = useUpdateTask(channelId)
  const [title, setTitle] = useState('')
  const tasks = data?.tasks ?? []

  const submit = () => {
    const t = title.trim()
    if (!t) return
    create.mutate(
      { title: t },
      {
        onSuccess: () => setTitle(''),
        onError: (err) => toast.error(explain(err, 'thêm công việc')),
      },
    )
  }

  return (
    <div className="px-5 py-3 grid content-start gap-3">
      <form
        className="flex items-end gap-2"
        onSubmit={(e) => {
          e.preventDefault()
          submit()
        }}
      >
        <TextField
          label="Thêm công việc"
          className="flex-1"
          value={title}
          maxLength={200}
          placeholder="Ví dụ: Cập nhật biên bản đối soát"
          onChange={(e) => setTitle(e.target.value)}
        />
        <Button type="submit" variant="soft" disabled={!title.trim()} loading={create.isPending} className="h-10">
          Thêm
        </Button>
      </form>

      {isError ? (
        <EmptyState
          icon={<CircleAlert size={24} strokeWidth={1.75} />}
          text="Chưa tải được công việc của nhóm."
          action={<Button variant="soft" size="sm" onClick={() => refetch()}>Thử lại</Button>}
        />
      ) : !isLoading && tasks.length === 0 ? (
        <EmptyState
          compact
          icon={<ListChecks size={24} strokeWidth={1.75} />}
          text="Chưa có công việc nào. Thêm việc đầu tiên ở ô bên trên."
        />
      ) : (
        <ul aria-label="Công việc" className="list-none m-0 p-0 border-t border-line" aria-busy={isLoading || undefined}>
          {tasks.map((t) => {
            const done = t.status === 'done'
            const s = STATUS[t.status] ?? STATUS.todo!
            const assignee = t.assignee_id
              ? people.byUserId.get(t.assignee_id)?.name ?? displayName(people, t.assignee_id, t.assignee_name)
              : ''
            return (
              <li key={t.id} className="grid grid-cols-[28px_minmax(0,1fr)_auto] gap-3 items-center min-h-11 py-1.5 border-b border-line">
                <Pressable
                  role="checkbox"
                  aria-checked={done}
                  aria-label={done ? `Mở lại: ${t.title}` : `Đánh dấu xong: ${t.title}`}
                  onClick={() =>
                    update.mutate(
                      { taskId: t.id, status: done ? 'todo' : 'done' },
                      { onError: (err) => toast.error(explain(err, 'cập nhật công việc')) },
                    )
                  }
                  className={`press grid place-items-center w-5 h-5 rounded-sm
                    ${done ? 'bg-accent text-on-accent' : 'field-line bg-raised text-transparent hover:field-focus'}`}
                >
                  <Check size={14} strokeWidth={2.25} aria-hidden="true" />
                </Pressable>
                <span className="grid gap-0.5 min-w-0">
                  <span className={`truncate text-sm ${done ? 'line-through text-ink-muted' : 'text-ink'}`}>{t.title}</span>
                  <span className="flex flex-wrap items-center gap-x-3 gap-y-0.5 text-xs text-ink-muted">
                    {assignee && <PersonChip name={assignee} hueKey={t.assignee_id} />}
                    {t.due_date && <span className="tnum">Hạn {formatDate(t.due_date)}</span>}
                  </span>
                </span>
                <span className={`inline-flex items-center h-5.5 px-2 rounded-full text-xs font-semibold ${s.pill}`}>
                  {s.label}
                </span>
              </li>
            )
          })}
        </ul>
      )}
    </div>
  )
}
