import { useMemo } from 'react'
import { X } from 'lucide-react'
import type { CalendarSummary, ShareLevel } from '../../api/calendar'
import { useRemoveShare, useSetShare, useShares } from '../../hooks/useCalendarSharing'
import { usePeople } from '../../hooks/usePeople'
import { colorStyle } from '../../lib/calendar-color'
import { useAuthStore } from '../../stores/auth.store'
import { Avatar, Button, IconButton, PeoplePicker, FieldSelect } from '../primitives'
import { Dialog } from '../composites/Dialog'

export const SHARE_LEVELS: { level: ShareLevel; label: string }[] = [
  { level: 'view', label: 'Xem mọi chi tiết' },
  { level: 'edit', label: 'Sửa sự kiện' },
  { level: 'manage', label: 'Sửa và quản lý chia sẻ' },
]

interface Props {
  open: boolean
  wsId: string
  workspaceName: string
  /** The calendar being shared; null while closed. */
  calendar: CalendarSummary | null
  onClose: () => void
}

/**
 * Chia sẻ lịch (DESIGN.md §6): add people with the picker, pick how far each can go, take them away. The
 * pinned row says everyone in the workspace sees busy/free only and cannot be removed. Changes apply at once.
 */
export function ShareCalendarDialog({ open, wsId, workspaceName, calendar, onClose }: Props) {
  const calId = calendar?.id ?? ''
  const shares = useShares(wsId, calId, open && !!calendar)
  const setShare = useSetShare(wsId, calId)
  const removeShare = useRemoveShare(wsId, calId)
  const people = usePeople(wsId)
  const me = useAuthStore((s) => s.user?.id)

  // The picker keys people by account; the shares by node.
  const exclude = useMemo(() => {
    const out = new Set<string>(me ? [me] : [])
    for (const s of shares.data ?? []) {
      const p = people.byNodeId.get(s.grantee.nodeId)
      if (p) out.add(p.userId)
    }
    return out
  }, [shares.data, people.byNodeId, me])

  return (
    <Dialog open={open && !!calendar} onClose={onClose} title={calendar ? `Chia sẻ lịch “${calendar.name}”` : 'Chia sẻ lịch'} footer={
      <Button type="button" className="rounded-full" onClick={onClose}>Xong</Button>
    }>
      {calendar && (
        <div className="grid gap-4" style={colorStyle(calendar.color)}>
          <PeoplePicker
            label="Thêm người"
            placeholder="Thêm người"
            people={people.list}
            value={[]}
            exclude={exclude}
            onChange={(next) => {
              const p = next[0]
              if (p) setShare.mutate({ nodeId: p.nodeId, level: 'view', name: p.name, announce: true })
            }}
          />
          <ul className="m-0 p-0 list-none grid gap-1">
            <li className="grid grid-cols-[2rem_minmax(0,1fr)_auto_2rem] items-center gap-3 min-h-12">
              <span aria-hidden="true" className="size-8 rounded-full bg-accent text-on-accent grid place-items-center text-xs font-semibold">
                {Array.from(workspaceName.trim())[0]?.toLocaleUpperCase('vi') ?? 'N'}
              </span>
              <span className="grid min-w-0 leading-tight">
                <span className="truncate text-sm text-ink">{`Mọi người trong ${workspaceName}`}</span>
              </span>
              <span className="text-[0.8125rem] font-semibold text-ink-muted pr-2">Thấy bận/rảnh</span>
              <span />
            </li>
            {shares.isPending && <li className="skeleton h-12 rounded-md" aria-hidden="true" />}
            {shares.isError && <li><p role="alert" className="m-0 text-sm text-ink-muted">Chưa tải được danh sách chia sẻ.</p></li>}
            {(shares.data ?? []).map((s) => {
              const person = people.byNodeId.get(s.grantee.nodeId)
              const name = s.grantee.displayName
              return (
                <li key={s.grantee.nodeId} className="grid grid-cols-[2rem_minmax(0,1fr)_auto_2rem] items-center gap-3 min-h-12">
                  <Avatar name={name} hue={s.grantee.hue} src={s.grantee.avatarUrl} size={32} />
                  <span className="grid min-w-0 leading-tight">
                    <span className="truncate text-sm text-ink">{name}</span>
                    {person?.role && <span className="truncate text-xs text-ink-muted">{person.role}</span>}
                  </span>
                  <FieldSelect
                    aria-label={`Mức chia sẻ của ${name}`}
                    value={s.level}
                    onChange={(e) => setShare.mutate({ nodeId: s.grantee.nodeId, level: e.target.value as ShareLevel, name })}
                  >
                    {SHARE_LEVELS.map((l) => <option key={l.level} value={l.level}>{l.label}</option>)}
                  </FieldSelect>
                  <IconButton aria-label={`Bỏ chia sẻ với ${name}`} onClick={() => removeShare.mutate({ nodeId: s.grantee.nodeId, name })}>
                    <X size={16} strokeWidth={1.75} />
                  </IconButton>
                </li>
              )
            })}
          </ul>
        </div>
      )}
    </Dialog>
  )
}
