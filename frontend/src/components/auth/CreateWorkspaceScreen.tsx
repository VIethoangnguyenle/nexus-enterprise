import { useState, type FormEvent } from 'react'
import { useNavigate } from '@tanstack/react-router'
import {
  ArrowLeft, Building2, ClipboardCheck, FileText, FolderOpen, Info, MessageSquare, Package, User, Users,
} from 'lucide-react'
import type { WorkspaceType } from '../../api/workspaces'
import { keys } from '../../hooks/keys'
import { useCreateMyWorkspace, useMe, useProviders, useSwitchToWorkspace } from '../../hooks/useAuth'
import { queryClient } from '../../lib/query-client'
import { describeAuthError, type DescribedError } from '../../lib/auth-flow'
import { statusOf } from '../../lib/errors'
import { Button, Heading, IconButton, Text, TextField, toast } from '../primitives'
import { Notice } from './Notice'
import { VerifyEmailNote } from './VerifyEmailNote'

const WIDE = 'w-full h-11'
const NAME_MAX = 80

/**
 * Tạo workspace (design/mockups/personal-workspace.html §8): choose type
 * (personal by default) and name.
 */
export function CreateWorkspaceScreen() {
  const navigate = useNavigate()
  const me = useMe()
  const providers = useProviders()
  const create = useCreateMyWorkspace()
  const switchTo = useSwitchToWorkspace()

  const [type, setType] = useState<WorkspaceType>('personal')
  const [name, setName] = useState('')
  const [nameError, setNameError] = useState<string | null>(null)
  const [failure, setFailure] = useState<DescribedError | null>(null)

  const busy = create.isPending || switchTo.isPending

  const submit = (e: FormEvent) => {
    e.preventDefault()
    if (busy) return
    const trimmed = name.trim()
    if (!trimmed) {
      setNameError('Nhập tên workspace.')
      return
    }
    setNameError(null)
    setFailure(null)
    create.mutate({ name: trimmed, type }, {
      onSuccess: (ws) =>
        switchTo.mutate(ws.id, {
          onSuccess: () => void navigate({ to: '/channels', search: { ws: ws.id } }),
          onError: () => {
            toast(`Đã tạo ${ws.name}, nhưng chưa mở được. Chọn workspace trong danh sách.`, { tone: 'error' })
            void navigate({ to: '/workspace-select' })
          },
        }),
      onError: (err) => {
        const described = describeAuthError(err, 'create-workspace')
        if (statusOf(err) === 400) setNameError(described.message)
        else setFailure(described)
      },
    })
  }

  const account = me.data
  const email = account?.email ?? ''
  // The server refuses an account whose address nobody has proved (403
  // email_unverified); the screen shows the way to prove it instead of a form
  // that could only be turned down.
  const mayCreate = !!account && !!email && account.email_verified

  return (
    <form onSubmit={submit} noValidate className="grid gap-5">
      <IconButton
        aria-label="Quay lại"
        size="lg"
        disabled={busy}
        className="-ml-2 w-11 h-11"
        onClick={() => void navigate({ to: '/workspace-select' })}
        type="button"
      >
        <ArrowLeft size={18} strokeWidth={1.75} />
      </IconButton>
      <div className="grid gap-1.5">
        <Heading as="h1" look="page">Tạo workspace</Heading>
        <Text variant="body" muted className="block">
          Mỗi workspace có thành viên, tin nhắn và tài liệu riêng.
        </Text>
      </div>

      {!account && (
        <div className="grid gap-5" aria-busy="true" data-skeleton>
          <div className="skeleton h-11 rounded-md" />
          <div className="skeleton h-11 rounded-md" />
        </div>
      )}
      {account && !email && (
        <Notice tone="info">
          Tài khoản này đăng ký bằng số điện thoại nên chưa tạo được workspace. Đăng nhập bằng email để tạo.
        </Notice>
      )}
      {account && !!email && !account.email_verified && (
        <VerifyEmailNote
          email={email}
          providers={providers.data}
          purpose="create-workspace"
          onVerified={() => {
            toast('Đã xác minh email.')
            void queryClient.invalidateQueries({ queryKey: keys.auth.me() })
          }}
        />
      )}

      {failure && <Notice>{failure.message}</Notice>}

      {mayCreate && (<>
        <fieldset className="grid gap-2 border-0 m-0 p-0" aria-describedby="cw-type-note" disabled={busy}>
          <legend className="p-0 mb-2 font-semibold text-ink text-sm">Loại workspace</legend>
          <label
            className={`relative grid grid-cols-[1.25rem_minmax(0,1fr)] gap-3 items-start p-3.5 rounded-surface cursor-pointer
              transition-colors duration-quick
              ${type === 'personal'
                ? 'bg-accent-wash ring-1.5 ring-accent'
                : 'bg-raised ring-1 ring-line hover:bg-hover'}`}
          >
            {/* eslint-disable-next-line no-restricted-syntax -- Thẻ radio chọn loại workspace */}
            <input
              type="radio"
              name="cw-type"
              value="personal"
              aria-label="Cá nhân"
              checked={type === 'personal'}
              onChange={() => setType('personal')}
              className="absolute top-4 left-3.5 w-4.5 h-4.5 opacity-0"
            />
            <span
              aria-hidden="true"
              className={`w-4.5 h-4.5 rounded-full mt-0.5 transition-shadow duration-quick
                ${type === 'personal' ? 'ring-5 ring-accent' : 'ring-1.5 ring-ink-muted'}`}
            />
            <span className="grid gap-1 min-w-0">
              <span className={`flex items-center gap-2 font-semibold text-base ${type === 'personal' ? 'text-accent' : 'text-ink'}`}>
                <User size={18} strokeWidth={1.75} aria-hidden="true" />
                Cá nhân
              </span>
              <span className="text-sm text-ink-muted">
                Cho việc của bạn, hoặc một nhóm nhỏ không cần quy trình duyệt.
              </span>
              <ul className="list-none m-0 p-0 flex flex-wrap gap-x-3 gap-y-1 text-xs text-ink-muted mt-1">
                <li className="inline-flex items-center gap-1">
                  <MessageSquare size={14} strokeWidth={1.75} aria-hidden="true" />
                  Tin nhắn
                </li>
                <li className="inline-flex items-center gap-1">
                  <FolderOpen size={14} strokeWidth={1.75} aria-hidden="true" />
                  Tài liệu
                </li>
                <li className="inline-flex items-center gap-1">
                  <Users size={14} strokeWidth={1.75} aria-hidden="true" />
                  Danh bạ
                </li>
              </ul>
            </span>
          </label>

          <label
            className={`relative grid grid-cols-[1.25rem_minmax(0,1fr)] gap-3 items-start p-3.5 rounded-surface cursor-pointer
              transition-colors duration-quick
              ${type === 'organization'
                ? 'bg-accent-wash ring-1.5 ring-accent'
                : 'bg-raised ring-1 ring-line hover:bg-hover'}`}
          >
            {/* eslint-disable-next-line no-restricted-syntax -- Thẻ radio chọn loại workspace */}
            <input
              type="radio"
              name="cw-type"
              value="organization"
              aria-label="Tổ chức"
              checked={type === 'organization'}
              onChange={() => setType('organization')}
              className="absolute top-4 left-3.5 w-4.5 h-4.5 opacity-0"
            />
            <span
              aria-hidden="true"
              className={`w-4.5 h-4.5 rounded-full mt-0.5 transition-shadow duration-quick
                ${type === 'organization' ? 'ring-5 ring-accent' : 'ring-1.5 ring-ink-muted'}`}
            />
            <span className="grid gap-1 min-w-0">
              <span className={`flex items-center gap-2 font-semibold text-base ${type === 'organization' ? 'text-accent' : 'text-ink'}`}>
                <Building2 size={18} strokeWidth={1.75} aria-hidden="true" />
                Tổ chức
              </span>
              <span className="text-sm text-ink-muted">
                Cho phòng ban, công ty: có quy trình duyệt và quản lý tài sản.
              </span>
              <ul className="list-none m-0 p-0 flex flex-wrap gap-x-3 gap-y-1 text-xs text-ink-muted mt-1">
                <li className="basis-full text-ink-muted">Như Cá nhân, thêm:</li>
                <li className="font-semibold text-ink inline-flex items-center gap-1">
                  <Package size={14} strokeWidth={1.75} aria-hidden="true" />
                  Tài sản
                </li>
                <li className="font-semibold text-ink inline-flex items-center gap-1">
                  <ClipboardCheck size={14} strokeWidth={1.75} aria-hidden="true" />
                  Phê duyệt
                </li>
                <li className="font-semibold text-ink inline-flex items-center gap-1">
                  <FileText size={14} strokeWidth={1.75} aria-hidden="true" />
                  Duyệt tài liệu
                </li>
              </ul>
            </span>
          </label>
        </fieldset>

        <div id="cw-type-note" className="flex items-start gap-2 text-xs text-ink-muted -mt-2">
          <Info size={16} strokeWidth={1.75} className="shrink-0 mt-0.5" aria-hidden="true" />
          <span>Workspace cá nhân chuyển thành workspace tổ chức được về sau, trong Cài đặt. Không chuyển ngược lại.</span>
        </div>

        <TextField
          large
          readOnly={busy}
          label="Tên workspace"
          placeholder={type === 'personal' ? 'Ví dụ: Việc nhà, Nhóm chạy bộ' : 'Ví dụ: Tổ Đối soát'}
          maxLength={NAME_MAX}
          counter
          value={name}
          onChange={(e) => {
            setName(e.target.value)
            if (nameError) setNameError(null)
          }}
          error={nameError ?? undefined}
          hint="Mời đồng nghiệp sau, trong Quản trị."
        />
        <Button type="submit" variant="primary" size="md" className={WIDE} loading={busy}>
          {type === 'personal' ? 'Tạo workspace cá nhân' : 'Tạo workspace tổ chức'}
        </Button>
      </>)}
    </form>
  )
}
