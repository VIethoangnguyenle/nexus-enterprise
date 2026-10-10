import { useState, type FormEvent } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { ArrowLeft } from 'lucide-react'
import { keys } from '../../hooks/keys'
import { useCreateMyWorkspace, useMe, useMyWorkspaces, useProviders, useSwitchToWorkspace } from '../../hooks/useAuth'
import { queryClient } from '../../lib/query-client'
import { describeAuthError, type DescribedError } from '../../lib/auth-flow'
import { statusOf } from '../../lib/errors'
import { Button, Heading, IconButton, Text, TextField, toast } from '../primitives'
import { Notice } from './Notice'
import { Steps } from './Steps'
import { VerifyEmailNote } from './VerifyEmailNote'

const WIDE = 'w-full h-11'
const NAME_MAX = 80

/**
 * Tạo workspace (design/mockups/auth.html §5): a name, and that is all. A
 * workspace is addressed by its ID, so there is no URL or slug to invent, and
 * colleagues are invited afterwards from Quản trị, where an invitation can
 * name a role and a department.
 */
export function CreateWorkspaceScreen() {
  const navigate = useNavigate()
  const me = useMe()
  const providers = useProviders()
  const mine = useMyWorkspaces()
  const create = useCreateMyWorkspace()
  const switchTo = useSwitchToWorkspace()

  const [name, setName] = useState('')
  const [nameError, setNameError] = useState<string | null>(null)
  const [failure, setFailure] = useState<DescribedError | null>(null)

  // Only someone with nowhere to go yet is mid-way through setting up.
  const firstWorkspace = mine.isSuccess && mine.data.length === 0
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
    create.mutate(trimmed, {
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
      {firstWorkspace && <Steps current={3} />}
      <IconButton aria-label="Quay lại" size="lg" className="-ml-2 w-11 h-11" onClick={() => void navigate({ to: '/workspace-select' })} type="button">
        <ArrowLeft size={18} strokeWidth={1.75} />
      </IconButton>
      <div className="grid gap-1.5">
        <Heading as="h1" look="page">Tạo workspace</Heading>
        <Text variant="body" muted className="block">
          Nơi nhóm của bạn nhắn tin, chia sẻ tài liệu và duyệt đề nghị.
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
      <TextField
        large
        autoFocus
        label="Tên workspace"
        placeholder="Ví dụ: Tổ Đối soát"
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
        Tạo workspace
      </Button>
      </>)}
    </form>
  )
}
