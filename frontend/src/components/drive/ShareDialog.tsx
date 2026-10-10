import { useMemo, useState } from 'react'
import { X } from 'lucide-react'
import type { DriveItem, DriveShare, SharePermission } from '../../api/drive'
import { useAuthStore } from '../../stores/auth.store'
import { useCreateShare, useDriveShares, useRevokeShare } from '../../hooks/useDrive'
import type { PeopleDirectory, Person } from '../../lib/people'
import { Button, IconButton, PeoplePicker, PersonChip, Select, toast } from '../primitives'
import { ConfirmDialog } from '../composites/ConfirmDialog'
import { Dialog } from '../composites/Dialog'
import { SHARE_PERMISSIONS } from './SharePermissionSelect'
import { sharePermissionLabel, shareTargetName } from './drive-model'

interface ShareDialogProps {
  /** The item to share; null keeps the dialog closed. */
  item: DriveItem | null
  people: PeopleDirectory
  onClose: () => void
}

/**
 * The one place sharing is managed: who has access now (with a way to take it
 * away) and a picker to add people. People are found by name; there is no field
 * that takes an id.
 */
export function ShareDialog({ item, people, onClose }: ShareDialogProps) {
  const itemId = item?.id ?? ''
  const me = useAuthStore((s) => s.user)
  const shares = useDriveShares(itemId, !!item)
  const create = useCreateShare(itemId)
  const revoke = useRevokeShare(itemId)

  const [picked, setPicked] = useState<Person[]>([])
  const [permission, setPermission] = useState<SharePermission>('read')
  const [revokeTarget, setRevokeTarget] = useState<DriveShare | null>(null)
  const [busy, setBusy] = useState(false)

  const current = shares.data ?? []
  // People who already have access, plus the owner and the user themselves.
  const exclude = useMemo(() => {
    const ids = new Set<string>()
    if (me?.id) ids.add(me.id)
    const owner = item && (people.byUserId.get(item.owner_id) ?? people.byNodeId.get(item.owner_id))
    if (owner) ids.add(owner.userId)
    for (const s of current) {
      const p = people.byNodeId.get(s.target_ngac_id)
      if (p) ids.add(p.userId)
    }
    return ids
  }, [me?.id, item, people, current])

  const close = () => {
    if (busy) return
    setPicked([])
    setPermission('read')
    onClose()
  }

  const submit = async () => {
    if (picked.length === 0 || busy) return
    setBusy(true)
    try {
      const results = await Promise.allSettled(
        picked.map((p) => create.mutateAsync({ shareType: 'user', targetNodeId: p.nodeId, permission })),
      )
      const done = picked.filter((_, i) => results[i]?.status === 'fulfilled')
      // Whoever could not be added stays picked, so one retry covers them.
      setPicked(picked.filter((_, i) => results[i]?.status === 'rejected'))
      if (done.length > 0) {
        toast(`Đã chia sẻ “${item?.name ?? ''}” với ${done.map((p) => p.name).join(', ')}`)
      }
    } finally {
      setBusy(false)
    }
  }

  const confirmRevoke = async () => {
    const target = revokeTarget
    if (!target) return
    setRevokeTarget(null)
    try {
      await revoke.mutateAsync(target.id)
      toast(`Đã bỏ quyền của ${shareTargetName(target, people)}`)
    } catch {
      // The shared mutation handler has told the user.
    }
  }

  return (
    <>
      <Dialog
        // The confirmation takes the screen alone; the picks here are kept.
        open={!!item && !revokeTarget}
        onClose={close}
        title={`Chia sẻ “${item?.name ?? ''}”`}
        footer={
          <>
            <Button type="button" variant="soft" onClick={close} disabled={busy}>Đóng</Button>
            <Button type="button" loading={busy} disabled={picked.length === 0} onClick={() => void submit()}>
              Chia sẻ
            </Button>
          </>
        }
      >
        <section className="grid gap-2.5">
          <h3 className="m-0 text-label text-ink-muted">Đang chia sẻ với</h3>
          {shares.isLoading ? (
            <div className="skeleton h-8 rounded-md" aria-busy="true" />
          ) : shares.isError ? (
            <p className="m-0 text-small text-danger">Không tải được danh sách chia sẻ.</p>
          ) : current.length === 0 ? (
            <p className="m-0 text-small text-ink-muted">Chưa chia sẻ với ai.</p>
          ) : (
            <ul aria-label="Đang chia sẻ với" className="grid gap-1 m-0 p-0 list-none">
              {current.map((s) => {
                const name = shareTargetName(s, people)
                const person = people.byNodeId.get(s.target_ngac_id)
                return (
                  <li key={s.id} className="flex items-center gap-2 min-h-9">
                    <PersonChip
                      name={name}
                      hueKey={person?.userId ?? s.target_ngac_id}
                      avatarUrl={person?.avatarUrl || undefined}
                    />
                    <span className="ml-auto shrink-0 text-small text-ink-muted">{sharePermissionLabel(s.operations)}</span>
                    <IconButton size="sm" aria-label={`Bỏ quyền của ${name}`} onClick={() => setRevokeTarget(s)}>
                      <X size={16} strokeWidth={1.75} />
                    </IconButton>
                  </li>
                )
              })}
            </ul>
          )}
        </section>

        <section className="grid gap-2.5">
          <h3 className="m-0 text-label text-ink-muted">Thêm người</h3>
          <PeoplePicker
            label="Thêm người"
            people={people.list}
            value={picked}
            onChange={setPicked}
            exclude={exclude}
          />
          <label className="flex items-center gap-2.5 text-sm font-semibold text-ink">
            Quyền
            <Select
              aria-label="Quyền"
              value={permission}
              onChange={(e) => setPermission(e.target.value as SharePermission)}
              className="w-auto"
            >
              {SHARE_PERMISSIONS.map((p) => <option key={p.value} value={p.value}>{p.label}</option>)}
            </Select>
          </label>
        </section>
      </Dialog>

      <ConfirmDialog
        open={!!revokeTarget}
        onClose={() => setRevokeTarget(null)}
        onConfirm={() => void confirmRevoke()}
        title="Bỏ quyền truy cập"
        description={
          <>
            <b className="font-semibold text-ink">{revokeTarget ? shareTargetName(revokeTarget, people) : ''}</b>{' '}
            sẽ không còn mở được “{item?.name}”.
          </>
        }
        confirmLabel="Bỏ quyền"
        confirmVariant="danger"
        loading={revoke.isPending}
      />
    </>
  )
}
