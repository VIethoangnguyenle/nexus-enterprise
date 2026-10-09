import { useMemo, useRef, useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { useCreateChannel } from '../../hooks/useMessaging'
import { usePeople } from '../../hooks/usePeople'
import { useAuthStore } from '../../stores/auth.store'
import { messagingApi } from '../../api/messaging'
import { queryClient } from '../../lib/query-client'
import { isForbidden, explain } from '../../lib/errors'
import { workspaceDisplayName } from '../../lib/workspace'
import type { Person } from '../../lib/people'
import { Dialog } from '../composites/Dialog'
import { Button, PeoplePicker, SpaceIcon, TextField, toast } from '../primitives'

const NAME_MAX = 128

/**
 * "Tạo nhóm" (design/mockups/spaces.html §3). One step: name, people, access.
 *
 * Shown fields are only those the backend stores. Hidden for now:
 * - Mô tả: POST /workspaces/:id/channels accepts only name and channel_type.
 * - Icon emoji, "Mọi người trong khối" access and the "Thông báo" space type:
 *   no API. Access is therefore fixed to "Chỉ người được thêm".
 */
export function CreateSpaceDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const navigate = useNavigate()
  const { workspaceId, workspaceName } = useActiveWorkspace()
  const people = usePeople(workspaceId)
  const me = useAuthStore((s) => s.user)
  const create = useCreateChannel(workspaceId)
  const nameRef = useRef<HTMLInputElement>(null)

  const [name, setName] = useState('')
  const [members, setMembers] = useState<Person[]>([])
  const [error, setError] = useState<string | undefined>()
  const [busy, setBusy] = useState(false)

  const exclude = useMemo(() => new Set(me?.id ? [me.id] : []), [me?.id])

  const reset = () => {
    setName('')
    setMembers([])
    setError(undefined)
  }
  const close = () => {
    if (busy) return
    onClose()
    reset()
  }

  const submit = async () => {
    const trimmed = name.trim()
    if (!trimmed) {
      setError('Đặt tên cho nhóm')
      nameRef.current?.focus()
      return
    }
    setBusy(true)
    try {
      const channel = await create.mutateAsync({ name: trimmed, channel_type: 'workspace' })
      const results = await Promise.allSettled(
        members.map((p) => messagingApi.addMember(channel.id, p.nodeId)),
      )
      const failed = results.filter((r) => r.status === 'rejected').length
      void queryClient.invalidateQueries({ queryKey: ['channels', workspaceId] })
      void queryClient.invalidateQueries({ queryKey: ['channelMembers', channel.id] })
      onClose()
      reset()
      navigate({ to: '/channels/$channelId', params: { channelId: channel.id } })
      toast(`Đã tạo nhóm “${trimmed}”`)
      if (failed > 0) {
        toast.error(`Chưa thêm được ${failed} người vào nhóm. Mở Thành viên của nhóm để thêm lại.`)
      }
    } catch (err) {
      toast.error(
        isForbidden(err)
          ? `Bạn chưa có quyền tạo nhóm trong ${workspaceDisplayName(workspaceName)}. Nhờ quản trị viên cấp quyền.`
          : explain(err, 'tạo nhóm'),
      )
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog
      open={open}
      onClose={close}
      title="Tạo nhóm"
      initialFocusRef={nameRef}
      onSubmit={() => void submit()}
      footer={
        <>
          <Button type="button" variant="ghost" onClick={close}>Huỷ</Button>
          <Button type="submit" variant="primary" loading={busy}>Tạo nhóm</Button>
        </>
      }
    >
      <TextField
        ref={nameRef}
        label="Tên nhóm"
        value={name}
        maxLength={NAME_MAX}
        counter
        error={error}
        placeholder="Ví dụ: Đối soát tháng 10"
        onChange={(e) => {
          setName(e.target.value)
          if (error) setError(undefined)
        }}
        onBlur={() => {
          if (name && !name.trim()) setError('Đặt tên cho nhóm')
        }}
        leading={<SpaceIcon name={name.trim() || 'N'} hueKey={name.trim() || 'new-space'} size={40} />}
      />

      <div className="grid gap-1.5">
        <span className="text-sm font-semibold text-ink" aria-hidden="true">Thêm người</span>
        <PeoplePicker
          label="Thêm người"
          people={people.list}
          value={members}
          onChange={setMembers}
          exclude={exclude}
        />
      </div>

      <div className="grid gap-1.5">
        <span className="text-sm font-semibold text-ink">Ai có thể tham gia</span>
        <div className="grid grid-cols-[20px_minmax(0,1fr)] gap-2.5 items-start px-3 py-2.5 rounded-surface bg-accent-wash">
          <span aria-hidden="true" className="mt-0.5 w-4.5 h-4.5 rounded-full border-5 border-accent" />
          <span className="grid gap-0.5">
            <span className="font-semibold text-ink">Chỉ người được thêm</span>
            <span className="text-small text-ink-muted">
              Chỉ những người bạn thêm mới thấy và vào được nhóm.
            </span>
          </span>
        </div>
      </div>
    </Dialog>
  )
}
