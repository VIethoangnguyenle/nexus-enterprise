import { useState } from 'react'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { useMyProfile } from '../../hooks/useMyProfile'
import { useRemoveAvatar, useUpdateProfile, useUploadAvatar } from '../../hooks/useProfile'
import { useWorkspaceType } from '../../hooks/useWorkspaceType'
import { roleLabel } from '../../lib/auth-flow'
import { avatarErrorMessage } from '../../lib/avatar-file'
import { UNKNOWN_PERSON } from '../../lib/people'
import { workspaceDisplayName } from '../../lib/workspace'
import { useAuthStore } from '../../stores/auth.store'
import { TextField, toast } from '../primitives'
import { AvatarEdit } from '../patterns/AvatarEdit'
import { EditFieldScreen } from './EditFieldScreen'
import { ProfileFormBar, ProfileLoadFailed, ProfileLoading, ProfileTab, RoleField, useProfileForm } from './ProfileTab'
import { ReadOnlyField } from './ReadOnlyField'
import { SettingsGroup, SettingsPage, SettingsRow } from './SettingsGroup'
import { useWorkspaceLead } from './modules/workspace-lead'
import { ACCOUNT_LEAD, FIELD_MAX, NAME_MAX, validateProfile, type ProfileForm } from './settings-model'
import { useSettingsWide } from './settings-layout'
import { useSettingsNav } from './settings-nav'

type Field = keyof ProfileForm

const FIELDS: Record<Field, { title: string; api: 'display_name' | 'title' | 'location'; max: number; hint?: string }> = {
  displayName: { title: 'Tên hiển thị', api: 'display_name', max: NAME_MAX, hint: 'Mọi người thấy tên này trong chat, phê duyệt và danh bạ.' },
  title: { title: 'Chức danh', api: 'title', max: FIELD_MAX },
  location: { title: 'Nơi làm việc', api: 'location', max: FIELD_MAX },
}

/** The person's own profile, the field open for editing, and saving one field. */
function useProfilePage() {
  const { workspaceId } = useActiveWorkspace()
  const q = useMyProfile(workspaceId)
  const save = useUpdateProfile()
  const [editing, setEditing] = useState<Field | null>(null)
  const person = q.data?.user
  const values: ProfileForm = { displayName: person?.display_name ?? '', title: person?.title ?? '', location: person?.location ?? '' }

  const editor = editing && (
    <EditFieldScreen
      title={FIELDS[editing].title}
      initial={values[editing]}
      hint={FIELDS[editing].hint}
      maxLength={FIELDS[editing].max + 20}
      validate={(v) => validateProfile({ ...values, [editing]: v })[editing]}
      onSave={(v) => save.mutateAsync({ [FIELDS[editing].api]: v }).then(() => { toast('Đã lưu hồ sơ'); return true }, () => false)}
      onBack={() => setEditing(null)}
    />
  )
  return { q, person, tenant: q.data?.current_tenant, values, editor, edit: setEditing }
}

/** A read that failed: say so and offer to try again, instead of empty rows. */
function ProfileFailed({ title, onBack, retry }: { title: string; onBack: () => void; retry: () => void }) {
  return (
    <SettingsPage title={title} onBack={onBack}>
      <ProfileLoadFailed retry={retry} />
    </SettingsPage>
  )
}

const shown = (v: string | undefined) => v?.trim() || 'Chưa có'

/**
 * Tài khoản và hồ sơ. On a phone (mockup mobile-shell.html §3) the picture, then each field a person sets as a row
 * that opens a page to edit that one field; below, what an administrator sets, read only. Inside the desktop shell
 * (mockup settings-desktop.html §6) the same fields as one form with Lưu thay đổi, and Đăng xuất at the end.
 */
export function AccountScreen() {
  return useSettingsWide() ? <AccountForm /> : <AccountRows />
}

function AccountForm() {
  const { back } = useSettingsNav()
  return (
    <SettingsPage title="Tài khoản và hồ sơ" lead={ACCOUNT_LEAD} onBack={back}>
      <ProfileTab />
    </SettingsPage>
  )
}

function AccountRows() {
  const { back } = useSettingsNav()
  const me = useAuthStore((s) => s.user)
  const { isPersonal } = useWorkspaceType()
  const { q, person, tenant, values, editor, edit } = useProfilePage()
  const upload = useUploadAvatar()
  const remove = useRemoveAvatar()
  const [avatarError, setAvatarError] = useState<string | null>(null)

  if (editor) return editor
  if (q.isError) return <ProfileFailed title="Tài khoản và hồ sơ" onBack={back} retry={() => void q.refetch()} />

  const changePicture = async (file: File) => {
    setAvatarError(null)
    try {
      await upload.mutateAsync(file)
    } catch (err) {
      setAvatarError(avatarErrorMessage(err))
      return
    }
    toast('Đã đổi ảnh')
  }
  const clearPicture = async () => {
    setAvatarError(null)
    try {
      await remove.mutateAsync()
    } catch {
      return // told by the shared handler
    }
    toast('Đã bỏ ảnh')
  }

  return (
    <SettingsPage title="Tài khoản và hồ sơ" onBack={back}>
      <div className="grid justify-items-center gap-1.5 pt-2">
        <AvatarEdit
          name={values.displayName || UNKNOWN_PERSON}
          hueKey={me?.id}
          src={person?.avatar_url || undefined}
          label="Đổi ảnh đại diện"
          canEdit={!!person}
          busy={upload.isPending || remove.isPending}
          error={avatarError}
          onUpload={changePicture}
          onRemove={clearPicture}
          size={64}
        />
      </div>

      <SettingsGroup>
        {(['displayName', 'title', 'location'] as const).map((f) => (
          <SettingsRow
            key={f}
            label={FIELDS[f].title}
            value={person ? shown(values[f]) : undefined}
            onPress={person ? () => edit(f) : undefined}
          />
        ))}
      </SettingsGroup>

      <SettingsGroup title="Quản trị viên đặt" note="Email dùng để nhận mã đăng nhập. Đổi email cần quản trị viên.">
        <SettingsRow label="Email đăng nhập" hint={person?.email} />
        {!isPersonal && <SettingsRow label="Phòng ban" value={person ? shown(tenant?.department) : undefined} />}
        {!isPersonal && <SettingsRow label="Vai trò" value={tenant?.role ? roleLabel(tenant.role) : undefined} />}
      </SettingsGroup>
    </SettingsPage>
  )
}

/**
 * Hồ sơ trong tổ chức (mockup §8, organization only): the title a person gives themselves, and the department and
 * role an administrator of the workspace set for them. On a phone the title is a row that opens its own page; inside
 * the desktop shell it is a form with Lưu thay đổi.
 */
export function OrgProfileScreen() {
  return useSettingsWide() ? <OrgProfileForm /> : <OrgProfileRows />
}

function OrgProfileForm() {
  const { back } = useSettingsNav()
  const lead = useWorkspaceLead()
  const { q, tenant, form, errors, changes, saving, refs, set, blur, submit, reset } = useProfileForm(['title'])

  const body = q.isError
    ? <ProfileLoadFailed retry={() => void q.refetch()} />
    : !q.data
      ? <ProfileLoading rows={3} avatar={false} />
      : (
        <form
          aria-label="Hồ sơ trong tổ chức"
          noValidate
          onSubmit={(e) => { e.preventDefault(); void submit() }}
          className="grid gap-5 max-w-130"
        >
          <TextField
            ref={refs.title}
            label="Chức danh"
            labelHint="(không bắt buộc)"
            value={form.title}
            maxLength={FIELD_MAX + 20}
            autoComplete="organization-title"
            error={errors.title}
            onChange={(e) => set('title')(e.target.value)}
            onBlur={blur('title')}
          />
          <ReadOnlyField label="Phòng ban" value={tenant?.department ?? ''} reason="Do quản trị viên đặt." />
          <RoleField role={tenant?.role} />
          <ProfileFormBar saving={saving} changed={!!changes} onReset={reset} />
        </form>
      )

  return <SettingsPage title="Hồ sơ trong tổ chức" lead={lead} onBack={back}>{body}</SettingsPage>
}

function OrgProfileRows() {
  const { back } = useSettingsNav()
  const { workspaceName } = useActiveWorkspace()
  const { q, person, tenant, values, editor, edit } = useProfilePage()

  if (editor) return editor
  if (q.isError) return <ProfileFailed title="Hồ sơ trong tổ chức" onBack={back} retry={() => void q.refetch()} />

  return (
    <SettingsPage title="Hồ sơ trong tổ chức" onBack={back}>
      <SettingsGroup>
        <SettingsRow label="Chức danh" value={person ? shown(values.title) : undefined} onPress={person ? () => edit('title') : undefined} />
      </SettingsGroup>
      <SettingsGroup title="Quản trị viên đặt" note={`Quản trị viên ${workspaceDisplayName(workspaceName)} đặt phòng ban và vai trò của bạn.`}>
        <SettingsRow label="Phòng ban" value={person ? shown(tenant?.department) : undefined} />
        <SettingsRow label="Vai trò" value={tenant?.role ? roleLabel(tenant.role) : undefined} />
      </SettingsGroup>
    </SettingsPage>
  )
}
