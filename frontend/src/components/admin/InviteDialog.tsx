import { useEffect, useMemo, useRef, useState } from 'react'
import { KeyRound, X } from 'lucide-react'
import type { Department, RoleSummary } from '../../api/admin'
import { useInvitePeople, type InviteOutcome, type InviteStatus } from '../../hooks/useAdmin'
import { roleName } from '../../lib/admin-model'
import { Button, ChoicePicker, IconButton, TextField, toast, type Choice } from '../primitives'
import { Dialog } from '../composites/Dialog'
import { DepartmentPicker } from './DepartmentPicker'

const EMAIL = /^[^\s@,;]+@[^\s@,;]+\.[^\s@,;]+$/

const REASON: Record<Exclude<InviteStatus, 'invited'>, string> = {
  invalid: 'Địa chỉ email không hợp lệ.',
  forbidden: 'Bạn không có quyền mời thành viên.',
  rate_limited: 'Bạn đã mời quá nhiều địa chỉ trong một giờ. Thử lại sau.',
  failed: 'Chưa gửi được lời mời. Kiểm tra kết nối rồi thử lại.',
}

/** The addresses in a piece of text, lower-cased, and the pieces that are not addresses. */
function readEmails(text: string): { good: string[]; bad: string[] } {
  const good: string[] = []
  const bad: string[] = []
  for (const raw of text.split(/[\s,;]+/)) {
    const t = raw.trim()
    if (!t) continue
    if (EMAIL.test(t)) good.push(t.toLowerCase())
    else bad.push(t)
  }
  return { good, bad }
}

interface InviteDialogProps {
  open: boolean
  onClose: () => void
  workspaceId: string
  /** The administrator's own roles, offered as the role to give. */
  roles: RoleSummary[]
  departments: Department[]
}

/**
 * Mời thành viên (design/mockups/admin.html §3): email addresses as chips, an
 * optional role and department. Each address becomes a pending invitation the
 * person answers when they sign in; nobody is added here, and the answer is the
 * same whether or not an account exists, so this dialog can say nothing about
 * that. An address the server refuses (not an address, over the hourly budget)
 * is named under its chip and kept for another try.
 *
 * There is no way to type an id, and the invitation email itself is not sent
 * by this change: the person finds the offer on their next sign-in.
 */
export function InviteDialog({ open, onClose, workspaceId, roles, departments }: InviteDialogProps) {
  const invite = useInvitePeople(workspaceId)
  const inputRef = useRef<HTMLInputElement>(null)
  const [emails, setEmails] = useState<string[]>([])
  const [draft, setDraft] = useState('')
  const [error, setError] = useState<string | undefined>()
  const [role, setRole] = useState<Choice | null>(null)
  const [dept, setDept] = useState('')
  const [failures, setFailures] = useState<Record<string, string>>({})

  // Each opening starts empty: nothing carries over from the last people invited.
  useEffect(() => {
    if (!open) return
    setEmails([])
    setDraft('')
    setError(undefined)
    setRole(null)
    setDept('')
    setFailures({})
    invite.reset()
    // The reset belongs to opening only, so `invite` is deliberately not a dependency.
  }, [open])

  const choices = useMemo(() => roles.map((r) => ({ id: r.id, name: roleName(r) })), [roles])

  const addAll = (good: string[]) =>
    setEmails((cur) => [...cur, ...good.filter((e, i) => !cur.includes(e) && good.indexOf(e) === i)])

  /** Typing or pasting: an address is taken once a separator follows it; what is still being typed stays. */
  const onType = (text: string) => {
    if (!/[\s,;]/.test(text)) {
      setDraft(text)
      setError(undefined)
      return
    }
    const tokens = text.split(/[\s,;]+/)
    const typing = /[\s,;]$/.test(text) ? '' : (tokens.pop() ?? '')
    const { good, bad } = readEmails(tokens.join(' '))
    addAll(good)
    setError(bad[0] ? `“${bad[0]}” không phải địa chỉ email.` : undefined)
    setDraft([...bad, typing].filter(Boolean).join(' '))
  }

  /** Enter or leaving the field: whatever is typed is an address now. */
  const settle = () => {
    if (!draft.trim()) return
    const { good, bad } = readEmails(draft)
    addAll(good)
    setError(bad[0] ? `“${bad[0]}” không phải địa chỉ email.` : undefined)
    setDraft(bad.join(' '))
  }

  const remove = (email: string) => {
    setEmails((cur) => cur.filter((e) => e !== email))
    setFailures((cur) => {
      const { [email]: _gone, ...rest } = cur
      return rest
    })
  }

  const submit = async () => {
    const { good, bad } = readEmails(draft)
    if (bad[0]) {
      setError(`“${bad[0]}” không phải địa chỉ email.`)
      inputRef.current?.focus()
      return
    }
    const list = [...new Set([...emails, ...good])]
    if (list.length === 0) {
      setError('Nhập ít nhất một địa chỉ email.')
      inputRef.current?.focus()
      return
    }
    const outcomes = await invite.mutateAsync({ emails: list, roleId: role?.id, departmentId: dept || undefined })
    const failed = outcomes.filter((o): o is InviteOutcome & { status: Exclude<InviteStatus, 'invited'> } => o.status !== 'invited')
    const sent = outcomes.length - failed.length

    setEmails(failed.map((o) => o.email))
    setDraft('')
    setFailures(Object.fromEntries(failed.map((o) => [o.email, REASON[o.status]])))

    if (failed.length === 0) {
      toast(sent === 1 ? 'Đã gửi lời mời cho 1 địa chỉ' : `Đã gửi lời mời cho ${sent} địa chỉ`)
      onClose()
      return
    }
    toast.error(sent > 0 ? `Đã gửi ${sent} lời mời, ${failed.length} địa chỉ chưa gửi được.` : 'Chưa gửi được lời mời nào. Xem lý do dưới từng địa chỉ.')
  }

  return (
    <Dialog
      open={open}
      onClose={invite.isPending ? () => {} : onClose}
      title="Mời thành viên"
      initialFocusRef={inputRef}
      onSubmit={() => void submit()}
      footer={
        <>
          <Button type="button" variant="soft" onClick={onClose} disabled={invite.isPending}>Huỷ</Button>
          <Button type="submit" loading={invite.isPending}>
            {emails.length > 0 ? `Mời ${emails.length} địa chỉ` : 'Mời'}
          </Button>
        </>
      }
    >
      <div className="grid gap-2">
        <TextField
          ref={inputRef}
          label="Email"
          inputMode="email"
          autoComplete="off"
          value={draft}
          error={error}
          placeholder="ten@congty.vn"
          onChange={(e) => onType(e.target.value)}
          onBlur={settle}
          onKeyDown={(e) => {
            if (e.key === 'Enter' && draft.trim()) {
              e.preventDefault()
              settle()
            }
          }}
        />
        <p className="m-0 text-xs text-ink-muted">
          Dán nhiều email, cách nhau bằng dấu phẩy. Người được mời tự chấp nhận trong 7 ngày; chỉ khi đó họ mới vào workspace.
        </p>
        {emails.length > 0 && (
          <ul aria-label="Email sẽ mời" className="grid gap-1.5 m-0 p-0 list-none">
            {emails.map((e) => (
              <li key={e} className="grid gap-0.5">
                <span className={`inline-flex items-center gap-1.5 h-7 pl-3 pr-1 rounded-full text-small-ui w-fit max-w-full
                  ${failures[e] ? 'bg-danger-wash text-danger' : 'bg-sunk text-ink'}`}>
                  <span className="truncate">{e}</span>
                  <IconButton size="sm" aria-label={`Bỏ ${e}`} onClick={() => remove(e)}>
                    <X size={14} strokeWidth={1.75} />
                  </IconButton>
                </span>
                {failures[e] && <span role="alert" className="text-xs text-danger">{failures[e]}</span>}
              </li>
            ))}
          </ul>
        )}
      </div>

      <div className="grid gap-1.5">
        <span className="text-sm font-semibold text-ink">
          Vai trò <span className="font-normal text-ink-muted">(không bắt buộc)</span>
        </span>
        <ChoicePicker
          label="Chọn vai trò"
          choices={choices}
          value={role}
          onChange={setRole}
          icon={<KeyRound size={14} strokeWidth={1.75} />}
          placeholder="Mặc định: Thành viên"
          emptyText="Chưa có vai trò tuỳ chỉnh nào."
        />
      </div>

      <div className="grid gap-1.5">
        <span className="text-sm font-semibold text-ink">
          Phòng ban <span className="font-normal text-ink-muted">(không bắt buộc)</span>
        </span>
        <DepartmentPicker
          label="Phòng ban"
          departments={departments}
          value={dept}
          onChange={setDept}
          noneLabel="Chưa chọn phòng ban"
        />
      </div>
    </Dialog>
  )
}
