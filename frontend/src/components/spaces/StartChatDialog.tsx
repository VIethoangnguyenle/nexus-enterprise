import { useMemo, useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { useCreateDM } from '../../hooks/useMessaging'
import { usePeople } from '../../hooks/usePeople'
import { useAuthStore } from '../../stores/auth.store'
import { explain } from '../../lib/errors'
import type { Person } from '../../lib/people'
import { Dialog } from '../composites/Dialog'
import { Button, PeoplePicker, toast } from '../primitives'

/** "Nhắn tin trực tiếp": pick one person by name, open (or create) the DM with them. */
export function StartChatDialog({ open, onClose, initial }: {
  open: boolean
  onClose: () => void
  /** Pre-picked person, e.g. from a member's menu. */
  initial?: Person
}) {
  const navigate = useNavigate()
  const { workspaceId } = useActiveWorkspace()
  const people = usePeople(workspaceId)
  const me = useAuthStore((s) => s.user)
  const createDM = useCreateDM()
  const [picked, setPicked] = useState<Person[]>(initial ? [initial] : [])
  const exclude = useMemo(() => new Set(me?.id ? [me.id] : []), [me?.id])

  const close = () => {
    onClose()
    setPicked([])
  }

  const start = () => {
    const target = picked[0]
    if (!target) return
    createDM.mutate(
      { userId: target.userId, ngacNodeId: target.nodeId },
      {
        onSuccess: (ch) => {
          close()
          navigate({ to: '/channels/$channelId', params: { channelId: ch.id } })
        },
        onError: (err) => toast.error(explain(err, `mở cuộc trò chuyện với ${target.name}`)),
      },
    )
  }

  return (
    <Dialog
      open={open}
      onClose={close}
      title="Nhắn tin trực tiếp"
      onSubmit={start}
      footer={
        <>
          <Button type="button" variant="ghost" onClick={close}>Huỷ</Button>
          <Button type="submit" variant="primary" disabled={!picked.length} loading={createDM.isPending}>
            Nhắn tin
          </Button>
        </>
      }
    >
      <PeoplePicker
        label="Người nhận"
        people={people.list}
        value={picked}
        onChange={setPicked}
        exclude={exclude}
        max={1}
        placeholder="Tìm theo tên hoặc phòng ban"
      />
    </Dialog>
  )
}
