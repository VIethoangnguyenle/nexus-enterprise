import { useState } from 'react'
import { CircleAlert, Laptop, Smartphone } from 'lucide-react'
import type { Session } from '../../api/auth'
import { useRevokeSession, useSessions } from '../../hooks/useSessions'
import { formatDateTime } from '../../lib/format'
import { Button, toast } from '../primitives'
import { ConfirmDialog } from '../composites/ConfirmDialog'
import { EmptyState } from '../spaces/EmptyState'
import { SettingsGroup, SettingsPage, SettingsRow } from './SettingsGroup'
import { ACCOUNT_LEAD } from './settings-model'
import { useSettingsNav } from './settings-nav'

const ICON = { size: 18, strokeWidth: 1.75 } as const
const MOBILE = /iPhone|iPad|Android/

/** What a session is called on screen: its device, never its id. */
const deviceName = (s: Session) => s.device || 'Trình duyệt không rõ'

/**
 * Tôi → Thiết bị và phiên: where this account is signed in, this device first. Another session can be signed
 * out from here, after a confirmation; this one is ended with Đăng xuất on Tôi (on a wide screen, at the end of
 * Tài khoản và hồ sơ). The server lists only the caller's own sessions and refuses to end anyone else's.
 */
export function SessionsScreen() {
  const { back } = useSettingsNav()
  const sessions = useSessions()
  const revoke = useRevokeSession()
  const [ending, setEnding] = useState<Session | null>(null)

  const confirm = async () => {
    if (!ending) return
    try {
      await revoke.mutateAsync(ending.session_id)
    } catch {
      return // told by the shared handler; the dialog stays so the person can retry or cancel
    }
    toast(`Đã đăng xuất ${deviceName(ending)}`)
    setEnding(null)
  }

  const body = (() => {
    if (sessions.isError) {
      return (
        <EmptyState
          icon={<CircleAlert size={24} strokeWidth={1.75} />}
          text="Không tải được danh sách thiết bị. Kiểm tra kết nối rồi thử lại."
          action={<Button variant="soft" size="sm" onClick={() => void sessions.refetch()}>Thử lại</Button>}
        />
      )
    }
    if (!sessions.data) {
      return (
        <div aria-busy="true" className="grid gap-2">
          <span className="skeleton h-14 rounded-xl" />
          <span className="skeleton h-14 rounded-xl" />
        </div>
      )
    }
    const current = sessions.data.filter((s) => s.current)
    const others = sessions.data.filter((s) => !s.current)
    const row = (s: Session) => (
      <SettingsRow
        key={s.session_id}
        icon={MOBILE.test(s.device) ? <Smartphone {...ICON} /> : <Laptop {...ICON} />}
        label={deviceName(s)}
        hint={`Đăng nhập lúc ${formatDateTime(s.started_at)}`}
        trailing={
          s.current ? (
            <span className="shrink-0 text-[0.8125rem] font-semibold text-accent">Máy này</span>
          ) : (
            <Button variant="soft" size="sm" onClick={() => setEnding(s)} aria-label={`Đăng xuất ${deviceName(s)}`}>
              Đăng xuất
            </Button>
          )
        }
      />
    )
    return (
      <>
        {current.length > 0 && <SettingsGroup title="Thiết bị này">{current.map(row)}</SettingsGroup>}
        <SettingsGroup
          title="Thiết bị khác"
          note={others.length === 0 ? 'Bạn không đăng nhập ở thiết bị nào khác.' : 'Đăng xuất một thiết bị bạn không nhận ra.'}
        >
          {others.map(row)}
        </SettingsGroup>
      </>
    )
  })()

  return (
    <SettingsPage title="Thiết bị và phiên" lead={ACCOUNT_LEAD} onBack={back}>
      {body}
      <ConfirmDialog
        open={!!ending}
        onClose={() => setEnding(null)}
        onConfirm={() => void confirm()}
        title={ending ? `Đăng xuất ${deviceName(ending)}` : 'Đăng xuất'}
        description="Thiết bị đó phải đăng nhập lại mới vào được Nexus."
        confirmLabel="Đăng xuất"
        confirmVariant="danger"
        loading={revoke.isPending}
      />
    </SettingsPage>
  )
}
