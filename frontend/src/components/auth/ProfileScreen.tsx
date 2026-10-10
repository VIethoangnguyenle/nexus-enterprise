import { useEffect, useState, type FormEvent } from 'react'
import { useNavigate, useSearch } from '@tanstack/react-router'
import { RefreshCw, AlertCircle } from 'lucide-react'
import { useMe, useSaveProfile } from '../../hooks/useAuth'
import { describeAuthError, type DescribedError } from '../../lib/auth-flow'
import { statusOf } from '../../lib/errors'
import { Avatar, Button, Heading, Text, TextField } from '../primitives'
import { Notice } from './Notice'
import { Steps } from './Steps'

const WIDE = 'w-full h-11'
const NAME_MAX = 80
const TITLE_MAX = 120

/**
 * "Tạo hồ sơ" (design/mockups/auth.html §3): the one thing asked of a new
 * person is the name colleagues will see. The title is optional. There is no
 * photo upload here because the server keeps no image store a profile could
 * point at; the avatar is the initials on the person's own colour.
 */
export function ProfileScreen() {
  const navigate = useNavigate()
  // The workspace a link named rides through the profile step to the list.
  const { ws } = useSearch({ strict: false }) as { ws?: string }
  const me = useMe()
  const save = useSaveProfile()

  const [typedName, setTypedName] = useState<string | null>(null)
  const [title, setTitle] = useState('')
  const [nameError, setNameError] = useState<string | null>(null)
  const [failure, setFailure] = useState<DescribedError | null>(null)

  // A name already differing from the handle came from Google: start from it.
  const account = me.data
  const suggested = account && account.display_name !== account.username ? account.display_name : ''
  const name = typedName ?? suggested

  // Nothing to ask someone who has been through this step.
  useEffect(() => {
    if (account && !account.needs_profile && !save.isPending && !save.isSuccess) {
      void navigate({ to: '/workspace-select', search: ws ? { ws } : {}, replace: true })
    }
  }, [account, save.isPending, save.isSuccess, navigate, ws])

  const submit = (e: FormEvent) => {
    e.preventDefault()
    if (save.isPending) return
    const trimmed = name.trim()
    if (!trimmed) {
      setNameError('Nhập tên để mọi người nhận ra bạn.')
      return
    }
    setNameError(null)
    setFailure(null)
    const patch: { display_name: string; title?: string } = { display_name: trimmed }
    if (title.trim()) patch.title = title.trim()
    save.mutate(patch, {
      onSuccess: () => void navigate({ to: '/workspace-select', search: ws ? { ws } : {} }),
      onError: (err) => {
        const described = describeAuthError(err, 'profile')
        // A refused name belongs under the field; anything else is the service's.
        if (statusOf(err) === 400) {
          setNameError(described.message)
        } else {
          setFailure(described)
        }
      },
    })
  }

  if (me.isError) {
    return (
      <div className="grid justify-items-center gap-3 py-10 text-center">
        <span className="grid place-items-center w-10 h-10 rounded-overlay bg-danger-wash text-danger" aria-hidden="true">
          <AlertCircle size={20} strokeWidth={1.75} />
        </span>
        <Text variant="body" className="block max-w-72">
          Không tải được thông tin tài khoản. Kiểm tra kết nối rồi thử lại.
        </Text>
        <Button variant="soft" size="sm" onClick={() => void me.refetch()}>
          <RefreshCw size={16} strokeWidth={1.75} aria-hidden="true" />
          Thử lại
        </Button>
      </div>
    )
  }

  if (!account) return <ProfileSkeleton />

  return (
    <form onSubmit={submit} noValidate className="grid gap-5">
      <Steps current={2} />
      <div className="grid gap-1.5">
        <Heading as="h1" look="page">Chào bạn, mọi người sẽ gọi bạn là gì?</Heading>
        <Text variant="body" muted className="block">
          Tên này hiện trong tin nhắn, phê duyệt và danh bạ. Đổi được trong Cài đặt.
        </Text>
      </div>

      <div className="flex items-center gap-4" data-testid="profile-preview">
        <Avatar name={name} hueKey={account.id} size={64} />
        <Text variant="small" muted className="block">Ảnh đại diện lấy chữ cái đầu của tên.</Text>
      </div>

      {failure && <Notice>{failure.message}</Notice>}

      <TextField
        large
        autoFocus
        label="Tên hiển thị"
        autoComplete="name"
        maxLength={NAME_MAX}
        counter
        value={name}
        onChange={(e) => {
          setTypedName(e.target.value)
          if (nameError) setNameError(null)
        }}
        error={nameError ?? undefined}
      />
      <TextField
        large
        label="Chức danh"
        labelHint="(không bắt buộc)"
        placeholder="Ví dụ: Chuyên viên đối soát"
        autoComplete="organization-title"
        maxLength={TITLE_MAX}
        value={title}
        onChange={(e) => setTitle(e.target.value)}
      />
      <Button type="submit" variant="primary" size="md" className={WIDE} loading={save.isPending}>
        Tiếp tục
      </Button>
    </form>
  )
}

/** The form's own shape while the account loads (DESIGN.md §6 Skeleton). */
function ProfileSkeleton() {
  return (
    <div className="grid gap-5" aria-busy="true" data-skeleton>
      <div className="skeleton h-1 rounded-full" />
      <div className="grid gap-2">
        <div className="skeleton h-6 w-4/5 rounded-md" />
        <div className="skeleton h-4 w-3/5 rounded-md" />
      </div>
      <div className="skeleton h-16 w-16 rounded-full" />
      <div className="skeleton h-11 rounded-md" />
      <div className="skeleton h-11 rounded-md" />
      <div className="skeleton h-11 rounded-md" />
    </div>
  )
}
