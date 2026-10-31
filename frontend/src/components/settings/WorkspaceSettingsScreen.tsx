import { useId, useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import {
  CircleAlert, ClipboardCheck, Database, Layers, Package, Plus, Shield, ShieldCheck, User, UserPlus, Users,
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
import { ConfirmDialog } from '../composites/ConfirmDialog'
import { useWorkspaceKindLine } from '../patterns/WorkspaceSwitcherSheet'
import { EmptyState } from '../spaces/EmptyState'
import { EditFieldScreen } from './EditFieldScreen'
import { GROUP_CREATION_CHOICES, GroupCreationRow } from './GroupCreationRow'
import { LeaveWorkspace } from './LeaveWorkspace'
import { ReadOnlyField } from './ReadOnlyField'
import { SettingsChoice } from './SettingsChoice'
import { ChoicePage, SettingsGroup, SettingsPage, SettingsRow } from './SettingsGroup'
import {
  DESCRIPTION_MAX, NAME_MAX, WorkspaceDetailsForm, WorkspaceTypeCard, checkDescription, checkName, editableName,
} from './WorkspaceInfo'
import { useWorkspaceLead } from './modules/workspace-lead'
import { useSettingsWide } from './settings-layout'
import { useSettingsNav } from './settings-nav'

const ICON = { size: 18, strokeWidth: 1.75 } as const
/** A whole number as Vietnamese writes it (1.250), never capped like a badge. */
const count = (n: number) => (Number.isFinite(n) ? n : 0).toLocaleString('vi-VN')
const SET_BY_ADMIN = 'Do quản trị viên đặt'
const ADMINS_NOTE = 'Nhờ một quản trị viên khi bạn cần mời người, đổi vai trò hay phòng ban.'
/** What locking group creation does: the words the phone's choice page shows under Chỉ quản trị viên. */
const LOCK_HINT = GROUP_CREATION_CHOICES.find((c) => c.value === 'admins')?.hint

/** Phone only: the subpages a row opens. On a wide screen there is no way back from one, so all of it is inline. */
type Page = 'name' | 'description' | 'groupCreation' | 'admins' | null

/** Only the workspace follows to an admin screen. */
const keepWorkspace = (prev: { ws?: string }) => (prev.ws ? { ws: prev.ws } : {})

/**
 * Cài đặt workspace (DESIGN.md §6, mockup mobile-shell.html §11): everything that belongs to the workspace itself,
 * on one page, rows by what the viewer may do (the existing admin check, `can_manage`; the server decides on every
 * screen it opens). Organization administrator: Thông tin, Thành viên và tổ chức, Quy định, Phân hệ, Rời.
 * Organization member: Thông tin read only, Quản trị viên, the rule locked, Hồ sơ trong tổ chức, Rời. Personal: the
 * name and storage only. A workspace's kind is fixed at creation; nothing here changes it.
 *
 * Inside the desktop shell (§6 "Cài đặt trên máy tính", mockup settings-desktop.html screen 5) the same rows under
 * the same rules, with nothing on a page of its own: Thông tin is a form (read only with the reason for a member),
 * Ai được tạo nhóm a drop-down, Quản trị viên opens its list in place, then the Loại workspace card and Rời.
 */
export function WorkspaceSettingsScreen() {
  const navigate = useNavigate()
  const wide = useSettingsWide()
  const { back, open } = useSettingsNav()
  const { workspaceId: wsId, workspaceName } = useActiveWorkspace()
  const me = useAuthStore((s) => s.user)
  const { isPersonal } = useWorkspaceType()
  const details = useWorkspaceDetails(wsId)
  const profile = useMyProfile(wsId)
  const kindLine = useWorkspaceKindLine()
  const lead = useWorkspaceLead()
  const [page, setPage] = useState<Page>(null)
  const [inviting, setInviting] = useState(false)
  const [adminsOpen, setAdminsOpen] = useState(false)
  const [confirmLock, setConfirmLock] = useState(false)
  const adminsId = useId()

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

  const name = workspaceDisplayName(details.data?.name || workspaceName)
  const role = profile.data?.current_tenant?.role
  const tenant = profile.data?.current_tenant
  const person = profile.data?.user
  const toAdmin = (to: '/admin' | '/admin/users' | '/admin/roles' | '/admin/directory') => void navigate({ to, search: keepWorkspace })
  const saveDetails = (body: { name?: string; description?: string }) => update.mutateAsync(body).then(() => true, () => false)

  if (details.isError) {
    return (
      <SettingsPage title="Cài đặt workspace" lead={lead} onBack={back}>
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
      <SettingsPage title="Cài đặt workspace" lead={lead} onBack={back}>
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
        initial={editableName(details.data.name)}
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
        <SettingsGroup note={ADMINS_NOTE}>
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
  const ownTile = isPersonal && !!me?.id

  return (
    <SettingsPage title="Cài đặt workspace" lead={lead} onBack={back}>
      <div className={`flex items-center ${wide ? 'gap-3.5 pt-1 pb-2' : 'gap-3.5 p-3.5 rounded-xl bg-raised'}`}>
        <span
          aria-hidden="true"
          // An organization is the accent; a workspace of one is the person's own hue.
          style={ownTile && me ? personStyle(me.id) : undefined}
          className={`grid place-items-center font-display font-bold shrink-0
            ${wide ? 'w-14 h-14 rounded-2xl text-[1.5rem]' : 'w-18 h-18 rounded-[1.25rem] text-[1.75rem]'}
            ${ownTile ? 'bg-(--p) text-on-person' : 'bg-accent text-on-accent'}`}
        >
          {spaceLetter(isPersonal ? person?.display_name || name : name)}
        </span>
        <div className="grid gap-1 min-w-0 flex-1 justify-items-start">
          <b className={`max-w-full text-[1.125rem] font-bold text-ink truncate ${wide ? 'font-display' : ''}`}>{name}</b>
          <span className={`max-w-full text-ink-muted truncate ${wide ? 'text-[0.8125rem]' : 'text-sm'}`}>{kindLine}</span>
          {!isPersonal && role && (
            <span className={`inline-flex items-center h-7 px-2.5 rounded-full text-[0.8125rem] font-semibold
              ${admin ? 'bg-accent-wash text-accent' : 'bg-sunk text-ink'}`}>
              Bạn là {roleLabel(role)}
            </span>
          )}
        </div>
      </div>

      <SettingsGroup title="Thông tin">
        {wide && (canEdit ? (
          <div className="pt-2 pb-4">
            <WorkspaceDetailsForm wsId={wsId} details={details.data} label="Thông tin" withDescription={!isPersonal} />
          </div>
        ) : (
          <div className="grid gap-4 max-w-130 pt-2 pb-4">
            <ReadOnlyField label="Tên workspace" value={name} reason={SET_BY_ADMIN} />
            {!isPersonal && <ReadOnlyField label="Mô tả" value={details.data.description ?? ''} reason={SET_BY_ADMIN} />}
          </div>
        ))}
        {!wide && <SettingsRow label="Tên workspace" value={name} onPress={canEdit ? () => setPage('name') : undefined} />}
        {!wide && !isPersonal && (
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
            onPress={wide ? () => setAdminsOpen((o) => !o) : () => setPage('admins')}
            expanded={wide ? adminsOpen : undefined}
            controls={wide ? adminsId : undefined}
          />
        )}
        {wide && member && adminsOpen && (
          <div id={adminsId} className="grid pl-4 pb-3">
            {owners.map((n) => <SettingsRow key={n} icon={<Avatar name={n} size={24} />} label={n} />)}
            <p className="m-0 pt-1 text-[0.8125rem] text-ink-muted">{ADMINS_NOTE}</p>
          </div>
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
            // A wide screen invites from a button on the row (mockup settings-desktop.html screen 5).
            trailing={wide
              ? <Button type="button" variant="soft" size="sm" onClick={() => setInviting(true)}><Plus size={16} strokeWidth={1.75} aria-hidden="true" />Mời</Button>
              : undefined}
            onPress={wide ? undefined : () => setInviting(true)}
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
          {admin && locks.data ? (
            <SettingsChoice
              icon={wide ? undefined : <ShieldCheck {...ICON} />}
              label="Ai được tạo nhóm"
              hint={wide ? 'Khoá cho cả tổ chức: thành viên không đổi được.' : undefined}
              value={locks.data.group_creation}
              choices={GROUP_CREATION_CHOICES}
              // Locking is org-wide and not fully undone by opening again: it is asked first. Opening is not.
              onPick={(rule) => (rule === 'admins' ? setConfirmLock(true) : setRule.mutate(rule))}
              onOpen={() => setPage('groupCreation')}
              disabled={setRule.isPending}
              menu
            />
          ) : (
            <GroupCreationRow icon={wide ? undefined : <ShieldCheck {...ICON} />} rule={locks.data?.group_creation} />
          )}
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
          {/* On a wide screen the size sits on the right, as a row's value (mockup screen 5). */}
          <SettingsRow icon={<Database {...ICON} />} label="Dung lượng Tài liệu" hint={wide ? undefined : storage} value={wide ? storage : undefined} />
        </SettingsGroup>
      )}

      {wide && (
        <SettingsGroup title="Loại workspace">
          <WorkspaceTypeCard details={details.data} />
        </SettingsGroup>
      )}

      {!isPersonal && <LeaveWorkspace variant={wide ? 'section' : 'row'} />}

      {admin && (
        <ConfirmDialog
          open={confirmLock}
          onClose={() => { if (!setRule.isPending) setConfirmLock(false) }}
          onConfirm={() => setRule.mutate('admins', { onSettled: () => setConfirmLock(false) })}
          title="Khoá quyền tạo nhóm?"
          description={LOCK_HINT}
          confirmLabel="Khoá"
          loading={setRule.isPending}
        />
      )}

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
