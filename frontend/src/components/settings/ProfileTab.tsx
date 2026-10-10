import { useEffect, useMemo, useRef, useState } from 'react'
import { CircleAlert, LogOut } from 'lucide-react'
import { logoutSession } from '../../api/client'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { useMyProfile } from '../../hooks/useMyProfile'
import { useUpdateProfile } from '../../hooks/useProfile'
import { UNKNOWN_PERSON } from '../../lib/people'
import { useAuthStore } from '../../stores/auth.store'
import { Avatar, Button, TextField, toast } from '../primitives'
import { EmptyState } from '../spaces/EmptyState'
import { ReadOnlyField } from './ReadOnlyField'
import {
  FIELD_MAX, NAME_MAX, profileChanges, profileOf, validateProfile, type ProfileErrors, type ProfileForm,
} from './settings-model'

/**
 * Cài đặt → Hồ sơ (mockup §4): the three things a person can change about
 * themselves, the two they can only see, and signing out. Errors show when a
 * field is left and when saving; only what changed is sent.
 */
export function ProfileTab() {
  const { workspaceId: wsId } = useActiveWorkspace()
  const me = useAuthStore((s) => s.user)
  const q = useMyProfile(wsId)
  const save = useUpdateProfile()

  // The person's own record (GET /api/me), not a search of the directory.
  const person = q.data?.user
  const department = q.data?.current_tenant?.department ?? ''
  const initial = useMemo(() => profileOf(person), [person])

  const [form, setForm] = useState<ProfileForm>(initial)
  const [errors, setErrors] = useState<ProfileErrors>({})
  const nameRef = useRef<HTMLInputElement>(null)
  const titleRef = useRef<HTMLInputElement>(null)
  const placeRef = useRef<HTMLInputElement>(null)

  // The stored profile arrives (or changes after a save): the form starts from it.
  // A form being edited is left alone; only a pristine one follows.
  const seeded = useRef<ProfileForm | null>(null)
  useEffect(() => {
    if (!person) return
    const pristine = !seeded.current || !profileChanges(seeded.current, form)
    seeded.current = initial
    if (pristine) setForm(initial)
     
  }, [initial, person])

  const changes = profileChanges(initial, form)
  const set = (key: keyof ProfileForm) => (value: string) => {
    setForm((f) => ({ ...f, [key]: value }))
    if (errors[key]) setErrors((e) => ({ ...e, [key]: validateProfile({ ...form, [key]: value })[key] }))
  }
  const blur = (key: keyof ProfileForm) => () => setErrors((e) => ({ ...e, [key]: validateProfile(form)[key] }))

  const submit = async () => {
    const found = validateProfile(form)
    setErrors(found)
    if (found.displayName) return nameRef.current?.focus()
    if (found.title) return titleRef.current?.focus()
    if (found.location) return placeRef.current?.focus()
    if (!changes) return
    try {
      await save.mutateAsync(changes)
    } catch {
      return // told by the shared handler; the form keeps what was typed
    }
    toast('Đã lưu hồ sơ')
  }

  if (q.isError) {
    return (
      <EmptyState
        icon={<CircleAlert size={24} strokeWidth={1.75} />}
        text="Không tải được hồ sơ của bạn. Kiểm tra kết nối rồi thử lại."
        action={<Button variant="soft" size="sm" onClick={() => void q.refetch()}>Thử lại</Button>}
      />
    )
  }
  if (!q.data) {
    return (
      <div aria-busy="true" className="grid gap-5 max-w-130">
        <div className="flex items-center gap-4"><span className="skeleton w-16 h-16 rounded-full" /><span className="skeleton h-4 w-48 rounded-sm" /></div>
        {[0, 1, 2, 3].map((i) => <span key={i} className="skeleton h-10 rounded-md" />)}
      </div>
    )
  }

  const shownName = form.displayName.trim() || initial.displayName || UNKNOWN_PERSON

  return (
    <form
      aria-label="Hồ sơ"
      noValidate
      onSubmit={(e) => { e.preventDefault(); void submit() }}
      className="grid gap-5 max-w-130"
    >
      <div className="flex items-center gap-4">
        <Avatar name={shownName} hueKey={me?.id} src={person?.avatar_url || undefined} size={64} />
        <p className="m-0 text-sm text-ink-muted">
          Không có ảnh thì dùng chữ cái đầu của tên trên màu riêng của bạn.
        </p>
      </div>

      <TextField
        ref={nameRef}
        label="Tên hiển thị"
        value={form.displayName}
        maxLength={NAME_MAX + 20}
        autoComplete="name"
        error={errors.displayName}
        onChange={(e) => set('displayName')(e.target.value)}
        onBlur={blur('displayName')}
      />
      <p className="m-0 -mt-3.5 text-xs text-ink-muted">Mọi người thấy tên này trong chat, phê duyệt và danh bạ.</p>

      <TextField
        ref={titleRef}
        label="Chức danh"
        labelHint="(không bắt buộc)"
        value={form.title}
        maxLength={FIELD_MAX + 20}
        autoComplete="organization-title"
        error={errors.title}
        onChange={(e) => set('title')(e.target.value)}
        onBlur={blur('title')}
      />

      <TextField
        ref={placeRef}
        label="Nơi làm việc"
        labelHint="(không bắt buộc)"
        value={form.location}
        maxLength={FIELD_MAX + 20}
        error={errors.location}
        onChange={(e) => set('location')(e.target.value)}
        onBlur={blur('location')}
      />

      <ReadOnlyField label="Email đăng nhập" value={person?.email ?? ''} reason="Dùng để nhận mã đăng nhập. Đổi email cần quản trị viên." />
      <ReadOnlyField label="Phòng ban" value={department} reason="Do quản trị viên đặt." />

      <div className="flex items-center gap-2">
        <Button type="submit" loading={save.isPending} disabled={!changes}>Lưu thay đổi</Button>
        <Button
          type="button"
          variant="ghost"
          disabled={!changes || save.isPending}
          onClick={() => { setForm(initial); setErrors({}) }}
        >
          Huỷ
        </Button>
      </div>

      <hr className="w-full m-0 border-0 border-t border-line" />
      <div className="flex items-center gap-4">
        <div className="grid flex-1 min-w-0">
          <span className="font-semibold text-ink">Đăng xuất</span>
          <span className="text-sm text-ink-muted">Thoát khỏi Nexus Hub trên trình duyệt này.</span>
        </div>
        <Button type="button" variant="soft" onClick={() => void logoutSession()}>
          <LogOut size={16} strokeWidth={1.75} aria-hidden="true" />
          Đăng xuất
        </Button>
      </div>
    </form>
  )
}
