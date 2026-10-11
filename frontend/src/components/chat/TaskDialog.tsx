import { useId, useMemo, useState } from 'react'
import { useCreateTask } from '../../hooks/useChannelTasks'
import { useChannelMembers } from '../../hooks/useMessaging'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { usePeople } from '../../hooks/usePeople'
import { keys } from '../../hooks/keys'
import { explain } from '../../lib/errors'
import { queryClient } from '../../lib/query-client'
import type { Person } from '../../lib/people'
import { Button, PeoplePicker, TextField } from '../primitives'
import { AlertBanner } from '../composites/AlertBanner'
import { Dialog } from '../composites/Dialog'

const TITLE_MAX = 200

interface TaskDialogProps {
  open: boolean
  onClose: () => void
  channelId: string
  /** Made inside a thread: the task is posted as a reply to this root. */
  parentMessageId?: string
}

/** The create request with the thread it belongs to; the server reads `parent_message_id`. */
type TaskRequest = Parameters<ReturnType<typeof useCreateTask>['mutateAsync']>[0] & { parent_message_id?: string }

/**
 * "Tạo công việc": what to do, who does it (picked from the people in this
 * conversation, never typed as an id) and by when. The task reaches the
 * conversation as a card once the server accepts it.
 */
export function TaskDialog({ open, onClose, channelId, parentMessageId }: TaskDialogProps) {
  const create = useCreateTask(channelId)
  return (
    <Dialog open={open} onClose={create.isPending ? () => {} : onClose} title="Tạo công việc">
      <TaskForm channelId={channelId} parentMessageId={parentMessageId} onClose={onClose} create={create} />
    </Dialog>
  )
}

function TaskForm({ channelId, parentMessageId, onClose, create }: {
  channelId: string
  parentMessageId?: string
  onClose: () => void
  create: ReturnType<typeof useCreateTask>
}) {
  const labelId = useId()
  const [title, setTitle] = useState('')
  const [assignee, setAssignee] = useState<Person[]>([])
  const [due, setDue] = useState('')
  const [titleError, setTitleError] = useState<string>()
  const [failure, setFailure] = useState<string>()

  const members = useChannelMembers(channelId)
  const { workspaceId } = useActiveWorkspace()
  const directory = usePeople(workspaceId)
  // Members the directory does not know are still pickable, by the name they sign in with.
  const candidates = useMemo<Person[]>(
    () =>
      (members.data?.members ?? []).map(
        (m) =>
          directory.byUserId.get(m.user_id) ??
          { userId: m.user_id, nodeId: m.ngac_node_id, username: m.username, name: m.username, role: '', avatarUrl: '' },
      ),
    [members.data, directory.byUserId],
  )

  const submit = async () => {
    if (!title.trim()) {
      setTitleError('Nhập việc cần làm.')
      return
    }
    setTitleError(undefined)
    setFailure(undefined)
    try {
      const request: TaskRequest = {
        title: title.trim(),
        ...(assignee[0] ? { assignee_id: assignee[0].userId } : {}),
        ...(due ? { due_date: due } : {}),
        ...(parentMessageId ? { parent_message_id: parentMessageId } : {}),
      }
      await create.mutateAsync(request)
    } catch (err) {
      setFailure(explain(err, 'tạo công việc'))
      return
    }
    void queryClient.invalidateQueries({ queryKey: keys.messaging.messages(channelId) })
    if (parentMessageId) void queryClient.invalidateQueries({ queryKey: keys.messaging.thread(parentMessageId) })
    onClose()
  }

  return (
    <form
      className="grid gap-3.5"
      onSubmit={(e) => {
        e.preventDefault()
        void submit()
      }}
    >
      <TextField
        label="Việc cần làm"
        value={title}
        maxLength={TITLE_MAX}
        counter
        autoFocus
        error={titleError}
        onChange={(e) => setTitle(e.target.value)}
      />
      <div className="grid gap-1.5">
        <span id={labelId} className="text-sm font-semibold text-ink">Giao cho</span>
        <PeoplePicker
          label="Giao cho"
          labelledBy={labelId}
          people={candidates}
          value={assignee}
          onChange={setAssignee}
          max={1}
          placeholder="Tìm thành viên nhóm…"
        />
      </div>
      <TextField label="Hạn" labelHint="(không bắt buộc)" type="date" value={due} onChange={(e) => setDue(e.target.value)} />
      {failure && <AlertBanner variant="error">{failure}</AlertBanner>}
      <div className="flex justify-end gap-2">
        <Button type="button" variant="soft" onClick={onClose} disabled={create.isPending}>Huỷ</Button>
        <Button type="submit" loading={create.isPending}>Gửi công việc</Button>
      </div>
    </form>
  )
}
