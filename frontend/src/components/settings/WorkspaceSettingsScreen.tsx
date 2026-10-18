import { useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import {
  CircleAlert, ClipboardCheck, Database, Layers, Package, Shield, ShieldCheck, User, UserPlus, Users,
} from 'lucide-react'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { useDepartments, useInvitations, useMemberDirectory, useRoles } from '../../hooks/useAdmin'
import { useApprovalTemplates } from '../../hooks/useApproval'
import { useAssetTypes } from '../../hooks/useAssets'
import { useDriveQuota } from '../../hooks/useDrive'
import { useMyProfile } from '../../hooks/useMyProfile'
import { useSetGroupCreation, useUpdateWorkspaceDetails, useWorkspaceDetails, useWorkspaceLocks } from '../../hooks/useWorkspaces'
import { useWorkspaceType } from '../../hooks/useWorkspaceType'
import { roleLabel } from '../../lib/auth-flow'
import { formatBytes } from '../../lib/format'
import { personStyle } from '../../lib/person-hue'
import { workspaceDisplayName } from '../../lib/workspace'
import { useAuthStore } from '../../stores/auth.store'
import { Avatar, Button, spaceLetter } from '../primitives'
import { InviteDialog } from '../admin/InviteDialog'
import { useWorkspaceKindLine } from '../patterns/WorkspaceSwitcherSheet'
import { EmptyState } from '../spaces/EmptyState'
import { EditFieldScreen } from './EditFieldScreen'
import { GROUP_CREATION_CHOICES, GroupCreationRow } from './GroupCreationRow'
import { LeaveWorkspace } from './LeaveWorkspace'
import { ChoicePage, SettingsGroup, SettingsPage, SettingsRow } from './SettingsGroup'
import { useSettingsNav } from './settings-nav'

const ICON = { size: 18, strokeWidth: 1.75 } as const
const NAME_MAX = 100
/** A whole number as Vietnamese writes it (1.250), never capped like a badge. */
const count = (n: number) => (Number.isFinite(n) ? n : 0).toLocaleString('vi-VN')
const DESCRIPTION_MAX = 500

const checkName = (value: string) => {
  const v = value.trim()
  if (!v) return 'Nhập tên cho workspace.'
  if (Array.from(v).length > NAME_MAX) return `Tên dài tối đa ${NAME_MAX} ký tự.`
  return undefined
}
const checkDescription = (value: string) =>
  Array.from(value.trim()).length > DESCRIPTION_MAX ? `Mô tả dài tối đa ${DESCRIPTION_MAX} ký tự.` : undefined

type Page = 'name' | 'description' | 'groupCreation' | 'admins' | null

/** Only the workspace follows to an admin screen. */
const keepWorkspace = (prev: { ws?: string }) => (prev.ws ? { ws: prev.ws } : {})

/**
 * Cài đặt workspace (DESIGN.md §6, mockup mobile-shell.html §11): everything that belongs to the workspace itself,
 * on one page, rows by what the viewer may do (the existing admin check, `can_manage`; the server decides on every
 * screen it opens). Organization administrator: Thông tin, Thành viên và tổ chức, Quy định, Phân hệ, Rời.
 * Organization member: Thông tin read only, Quản trị viên, the rule locked, Hồ sơ trong tổ chức, Rời. Personal: the
 * name and storage only. A workspace's kind is fixed at creation; nothing here changes it.
 */
export function WorkspaceSettingsScreen() {
  const navigate = useNavigate()
  const { back, open } = useSettingsNav()
  const { workspaceId: wsId, workspaceName } = useActiveWorkspace()
  const me = useAuthStore((s) => s.user)
  const { isPersonal } = useWorkspaceType()
  const details = useWorkspaceDetails(wsId)
  const profile = useMyProfile(wsId)
  const kindLine = useWorkspaceKindLine()
  const [page, setPage] = useState<Page>(null)
  const [inviting, setInviting] = useState(false)

  const admin = !isPersonal && details.data?.can_manage === true
  const member = !isPersonal && details.data?.can_manage === false
  // Each count is asked only of the people who see its row.
  const people = useMemberDirectory(admin ? wsId : '')
  const invitations = useInvitations(admin ? wsId : '')
  const departments = useDepartments(wsId, admin)
  const roles = useRoles(wsId, admin)
  const templates = useApprovalTemplates(undefined, false, admin)
  const assetTypes = useAssetTypes(admin ? wsId : '')
  const quota = useDriveQuota(admin || isPersonal ? wsId : '')
  const locks = useWorkspaceLocks(isPersonal ? '' : wsId)
  const setRule = useSetGroupCreation(wsId)
  const update = useUpdateWorkspaceDetails(wsId)

  const name = details.data?.name || workspaceDisplayName(workspaceName)
  const role = profile.data?.current_tenant?.role
  const tenant = profile.data?.current_tenant
  const person = profile.data?.user
  const toAdmin = (to: '/admin' | '/admin/users' | '/admin/roles' | '/admin/directory') => void navigate({ to, search: keepWorkspace })
  const saveDetails = (body: { name?: string; description?: string }) => update.mutateAsync(body).then(() => true, () => false)

  if (details.isError) {
    return (
      <SettingsPage title="Cài đặt workspace" onBack={back}>
        <EmptyState
          icon={<CircleAlert size={24} strokeWidth={1.75} />}
          text="Không tải được thông tin workspace. Kiểm tra kết nối rồi thử lại."
          action={<Button variant="soft" size="sm" onClick={() => void details.refetch()}>Thử lại</Button>}
        />
      </SettingsPage>
    )
  }
  if (!details.data) {
    return (
      <SettingsPage title="Cài đặt workspace" onBack={back}>
        <div aria-busy="true" className="grid gap-3">
          <span className="skeleton h-24 rounded-xl" />
          <span className="skeleton h-28 rounded-xl" />
        </div>
      </SettingsPage>
    )
  }

  const canEdit = details.data.can_manage
  if (page === 'name' && canEdit) {
    return (
      <EditFieldScreen
        title="Tên workspace"
        initial={details.data.name}
        maxLength={NAME_MAX + 20}
        validate={checkName}
        hint="Mọi người trong workspace thấy tên này."
        onSave={(v) => saveDetails({ name: v })}
        onBack={() => setPage(null)}
      />
    )
  }
  if (page === 'description' && canEdit) {
    return (
      <EditFieldScreen
        title="Mô tả"
        initial={details.data.description}
        maxLength={DESCRIPTION_MAX}
        validate={checkDescription}
        multiline
        onSave={(v) => saveDetails({ description: v })}
        onBack={() => setPage(null)}
      />
    )
  }
  if (page === 'groupCreation' && admin) {
    return (
      <ChoicePage
        title="Ai được tạo nhóm"
        value={locks.data?.group_creation}
        choices={GROUP_CREATION_CHOICES}
        onPick={(rule) => setRule.mutate(rule)}
        onBack={() => setPage(null)}
      />
    )
  }
  if (page === 'admins') {
    return (
      <SettingsPage title="Quản trị viên" onBack={() => setPage(null)}>
        <SettingsGroup note="Nhờ một quản trị viên khi bạn cần mời người, đổi vai trò hay phòng ban.">
          {(details.data.owner_names ?? []).map((n) => (
            <SettingsRow key={n} icon={<Avatar name={n} size={24} />} label={n} />
          ))}
        </SettingsGroup>
      </SettingsPage>
    )
  }

  const used = quota.data
  const storage = used
    // The quota is protobuf JSON: a field that is zero is left out. No limit set reads as what is used alone.
    ? `${formatBytes(used.used_bytes ?? 0)}${used.max_bytes ? ` / ${formatBytes(used.max_bytes)}` : ' đã dùng'} · ${count(used.used_files ?? 0)} tệp`
    : undefined
  const pending = invitations.data?.length ?? 0
  const owners = details.data.owner_names ?? []

  return (
    <SettingsPage title="Cài đặt workspace" onBack={back}>
      <div className="flex items-center gap-3.5 p-3.5 rounded-xl bg-raised">
        <span
          aria-hidden="true"
          // An organization is the accent; a workspace of one is the person's own hue.
          style={isPersonal && me?.id ? personStyle(me.id) : undefined}
          className={`grid place-items-center w-18 h-18 rounded-[1.25rem] font-display text-[1.75rem] font-bold shrink-0
            ${isPersonal && me?.id ? 'bg-(--p) text-on-person' : 'bg-accent text-on-accent'}`}
        >
          {spaceLetter(isPersonal ? person?.display_name || name : name)}
        </span>
        <div className="grid gap-1 min-w-0 flex-1 justify-items-start">
          <b className="max-w-full text-[1.125rem] font-bold text-ink truncate">{name}</b>
          <span className="max-w-full text-sm text-ink-muted truncate">{kindLine}</span>
          {!isPersonal && role && (
            <span className={`inline-flex items-center h-7 px-2.5 rounded-full text-[0.8125rem] font-semibold
              ${admin ? 'bg-accent-wash text-accent' : 'bg-sunk text-ink'}`}>
              Bạn là {roleLabel(role)}
            </span>
          )}
        </div>
      </div>

      <SettingsGroup title="Thông tin">
        <SettingsRow label="Tên workspace" value={name} onPress={canEdit ? () => setPage('name') : undefined} />
        {!isPersonal && (
          <SettingsRow
            label="Mô tả"
            value={details.data.description || (canEdit ? 'Chưa có' : undefined)}
            onPress={canEdit ? () => setPage('description') : undefined}
          />
        )}
        {member && owners.length > 0 && (
          <SettingsRow
            label="Quản trị viên"
            trailing={
              <span className="flex items-center -space-x-1.5 shrink-0" aria-label={owners.join(', ')}>
                {owners.slice(0, 3).map((n) => <Avatar key={n} name={n} size={24} />)}
              </span>
            }
            onPress={() => setPage('admins')}
          />
        )}
      </SettingsGroup>

      {admin && (
        <SettingsGroup title="Thành viên và tổ chức">
          <SettingsRow
            icon={<Users {...ICON} />}
            label="Thành viên"
            value={people.data ? count(people.data.length) : undefined}
            onPress={() => toAdmin('/admin/users')}
          />
          <SettingsRow
            icon={<UserPlus {...ICON} />}
            label="Mời thành viên"
            hint={pending > 0 ? `${count(pending)} lời mời đang chờ` : undefined}
            onPress={() => setInviting(true)}
          />
          <SettingsRow
            icon={<Layers {...ICON} />}
            label="Phòng ban"
            value={departments.data ? count(departments.data.flat.length) : undefined}
            onPress={() => toAdmin('/admin')}
          />
          <SettingsRow
            icon={<Shield {...ICON} />}
            label="Vai trò và quyền"
            value={roles.data ? count(roles.data.custom.length + roles.data.system.length) : undefined}
            onPress={() => toAdmin('/admin/roles')}
          />
          <SettingsRow
            icon={<User {...ICON} />}
            label="Phạm vi danh bạ"
            hint="Ai thấy ai trong Danh bạ"
            onPress={() => toAdmin('/admin/directory')}
          />
        </SettingsGroup>
      )}

      {!isPersonal && (
        <SettingsGroup title="Quy định">
          <GroupCreationRow
            icon={<ShieldCheck {...ICON} />}
            rule={locks.data?.group_creation}
            onPress={admin && locks.data ? () => setPage('groupCreation') : undefined}
          />
        </SettingsGroup>
      )}

      {member && (
        <SettingsGroup title="Của bạn trong workspace">
          <SettingsRow
            icon={<User {...ICON} />}
            label="Hồ sơ trong tổ chức"
            hint={[person?.title, tenant?.department].filter(Boolean).join(' · ') || undefined}
            onPress={() => open('org-profile')}
          />
        </SettingsGroup>
      )}

      {(admin || isPersonal) && (
        <SettingsGroup
          title="Phân hệ"
          note={isPersonal ? 'Workspace cá nhân chỉ có bạn: không thành viên, phòng ban, vai trò, quy định hay Rời. Muốn làm việc nhóm thì tạo một workspace tổ chức mới.' : undefined}
        >
          {admin && (
            <SettingsRow
              icon={<ClipboardCheck {...ICON} />}
              label="Mẫu phê duyệt"
              value={templates.data ? count(templates.data.templates?.length ?? 0) : undefined}
              onPress={() => void navigate({ to: '/approval', search: (prev: { ws?: string }) => ({ ...keepWorkspace(prev), tab: 'templates' as const }) })}
            />
          )}
          {admin && (
            <SettingsRow
              icon={<Package {...ICON} />}
              label="Loại tài sản"
              value={assetTypes.data ? count(assetTypes.data.types.length) : undefined}
              onPress={() => void navigate({ to: '/assets', search: (prev: { ws?: string }) => ({ ...keepWorkspace(prev), section: 'types' as const }) })}
            />
          )}
          <SettingsRow icon={<Database {...ICON} />} label="Dung lượng Tài liệu" hint={storage} />
        </SettingsGroup>
      )}

      {!isPersonal && <LeaveWorkspace variant="row" />}

      {admin && (
        <InviteDialog
          open={inviting}
          onClose={() => setInviting(false)}
          workspaceId={wsId}
          roles={roles.data?.custom ?? []}
          departments={departments.data?.flat ?? []}
        />
      )}
    </SettingsPage>
  )
}
