import { useEffect, useState } from 'react'
import {
  Bell, ChevronRight, Crown, Search,
  Star, UserPlus, User,
} from 'lucide-react'
import type { Person, PeopleDirectory } from '../../lib/people'
import { avatarErrorMessage } from '../../lib/avatar-file'
import { explain } from '../../lib/errors'
import { optionFor } from '../../lib/notification-level-copy'
import { useRemoveChannelAvatar, useUploadChannelAvatar } from '../../hooks/useChannelAvatar'
import { useCanManageChannel } from '../../hooks/useCanManageChannel'
import { useGroupRole } from '../../hooks/useGroupManagement'
import { useUpdateChannel } from '../../hooks/useMessaging'
import { usePins } from '../../hooks/useMessages'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { useChatSections } from '../../hooks/useChatSections'
import { sectionOfChannel } from '../../lib/chat-sections'
import { AvatarEdit } from '../patterns/AvatarEdit'
import { Avatar, Button, Pressable, TextField, toast } from '../primitives'
import { NotificationLevelSheet } from './NotificationLevelSheet'
import { SidePanel } from './SidePanel'
import { SectionPickerSheet } from './SectionPickerSheet'
import { GroupManagement } from './GroupManagement'
import { OwnerMustTransferDialog } from './group-management/OwnerMustTransferDialog'
import type { InfoNotification } from './SpaceInfoPanel'

const PERMISSION_DENIED = 'Bạn không còn quyền làm việc này'

/**
 * Conversation settings (phase 06, mockup §5): group and DM settings pages.
 * On a phone this is a full-screen page; on desktop it replaces the info panel.
 *
 * Group: avatar · name · description (rename for managers) · quick actions
 * Tìm, Thông báo, Gắn sao, Thêm người · Thành viên · Tệp, ảnh và liên kết ·
 * Tin đã ghim · Thông báo level · Mục (section picker) · Quản lý nhóm (owner and
 * managers only; it opens in place and closes itself when that right goes) · Rời
 * nhóm (the owner is asked to hand the group over first).
 *
 * DM: person header · Hồ sơ, Tìm, Thông báo, Gắn sao · Tệp · Tin đã ghim ·
 * Thông báo · Mục · Tạo nhóm với <tên>.
 */
export function ConversationSettings({
  channelId, kind, title, memberCount, avatarUrl, partner, people, notification,
  starred, canLeave, onOpenTopic, onOpenMembers, onToggleStar, onLeave, onClose,
}: {
  channelId: string
  kind: 'dm' | 'space'
  title: string
  memberCount: number
  avatarUrl?: string
  partner?: Person
  people: PeopleDirectory
  notification?: InfoNotification
  starred: boolean
  canLeave: boolean
  onOpenTopic: (id: string) => void
  onOpenMembers: () => void
  onToggleStar: () => void
  onLeave: () => void
  onClose: () => void
}) {
  const pins = usePins(channelId)
  const rename = useUpdateChannel(channelId)
  const [editing, setEditing] = useState(false)
  const [levelSheet, setLevelSheet] = useState(false)
  const [sectionSheet, setSectionSheet] = useState(false)
  const upload = useUploadChannelAvatar(channelId)
  const removePicture = useRemoveChannelAvatar(channelId)
  const [pictureError, setPictureError] = useState<string | null>(null)
  const canManage = useCanManageChannel(channelId)
  const [name, setName] = useState(title)
  const list = pins.data?.pins ?? []
  const { workspaceId } = useActiveWorkspace()
  const sections = useChatSections(workspaceId)
  const currentSection = sectionOfChannel(sections.sections, channelId)
  const role = useGroupRole(channelId)
  const [managing, setManaging] = useState<null | 'main' | 'transfer'>(null)
  const [ownerLeave, setOwnerLeave] = useState(false)
  const mayManage = kind === 'space' && role.isManager

  // Demoted while the page is open: it goes, and does not come back by itself if the right returns.
  useEffect(() => {
    if (!mayManage) setManaging(null)
  }, [mayManage])

  const handleLeave = () => {
    if (role.isOwner) setOwnerLeave(true)
    else onLeave()
  }

  const save = () => {
    const n = name.trim()
    if (!n || n === title) {
      setEditing(false)
      return
    }
    rename.mutate(
      { name: n },
      {
        onSuccess: () => {
          setEditing(false)
          toast(`Đã đổi tên nhóm thành "${n}"`)
        },
        onError: (err) => toast.error(explain(err, 'đổi tên nhóm')),
      },
    )
  }

  const changePicture = async (file: File) => {
    setPictureError(null)
    try {
      await upload.mutateAsync(file)
    } catch (err) {
      setPictureError(avatarErrorMessage(err))
      return
    }
    toast('Đã đổi ảnh nhóm')
  }
  const clearPicture = async () => {
    setPictureError(null)
    try {
      await removePicture.mutateAsync()
    } catch (err) {
      setPictureError(avatarErrorMessage(err))
      return
    }
    toast('Đã bỏ ảnh nhóm')
  }

  const handleToggleStar = () => {
    try {
      onToggleStar()
    } catch {
      toast.error(PERMISSION_DENIED)
    }
  }

  const handleSectionPick = (sectionId: string) => {
    void sections.placeConversation({ id: channelId, title }, sectionId)
    setSectionSheet(false)
  }
  const handleSectionRemove = () => {
    void sections.unplaceConversation({ id: channelId, title })
    setSectionSheet(false)
  }

  const isGroup = kind === 'space'
  const settingsTitle = isGroup ? 'Cài đặt nhóm' : 'Cài đặt trò chuyện'
  const levelOption = notification ? optionFor(kind, notification.level) : undefined

  // The short name from the full name for "Tạo nhóm với <tên>"
  const shortName = partner?.name?.split(' ').pop() ?? partner?.name ?? title

  if (managing && mayManage) {
    return (
      <GroupManagement
        channelId={channelId}
        groupName={title}
        people={people}
        startWith={managing}
        onBack={() => setManaging(null)}
        onClose={onClose}
      />
    )
  }

  return (
    <SidePanel
      label={settingsTitle}
      title={settingsTitle}
      closeLabel={settingsTitle.toLowerCase()}
      onClose={onClose}
    >
      <div className="grid gap-3 px-3 pb-3">
        {/* Header: avatar + name + description */}
        <div className="grid justify-items-center gap-1.5 py-2 text-center">
          {isGroup ? (
            <AvatarEdit
              shape="square"
              name={title}
              hueKey={channelId}
              src={avatarUrl}
              label={`Đổi ảnh nhóm ${title}`}
              canEdit={canManage}
              busy={upload.isPending || removePicture.isPending}
              error={pictureError}
              onUpload={changePicture}
              onRemove={clearPicture}
              size={64}
            />
          ) : partner ? (
            <Avatar name={partner.name} hueKey={partner.userId} src={partner.avatarUrl} size={64} />
          ) : null}

          {isGroup ? (
            editing ? (
              <form
                className="grid gap-2 w-full text-left"
                onSubmit={(e) => {
                  e.preventDefault()
                  save()
                }}
              >
                <TextField label="Tên nhóm" value={name} maxLength={128} counter autoFocus onChange={(e) => setName(e.target.value)} />
                <div className="flex justify-end gap-2">
                  <Button type="button" variant="ghost" size="sm" onClick={() => { setName(title); setEditing(false) }}>Huỷ</Button>
                  <Button type="submit" variant="primary" size="sm" loading={rename.isPending}>Lưu</Button>
                </div>
              </form>
            ) : (
              <>
                <span className="text-xl font-bold font-display">{title}</span>
                {canManage && (
                  <Pressable
                    onClick={() => setEditing(true)}
                    className="text-small text-ink-muted hover:text-ink cursor-pointer bg-transparent border-none"
                  >
                    Sửa
                  </Pressable>
                )}
              </>
            )
          ) : partner ? (
            <>
              <span className="text-xl font-bold font-display">{partner.name}</span>
              <span className="text-small text-ink-muted">{partner.role}</span>
            </>
          ) : null}
        </div>

        {/* Quick actions: 4 grid */}
        <div className="grid grid-cols-4 gap-2">
          {isGroup ? (
            <>
              <QuickAction icon={Search} label="Tìm" onClick={() => {}} />
              <QuickAction icon={Bell} label="Thông báo" onClick={() => setLevelSheet(true)} />
              <QuickAction icon={Star} label={starred ? 'Bỏ gắn sao' : 'Gắn sao'} active={starred} onClick={handleToggleStar} />
              <QuickAction icon={UserPlus} label="Thêm người" onClick={() => onOpenMembers()} />
            </>
          ) : (
            <>
              <QuickAction icon={User} label="Hồ sơ" onClick={() => {}} />
              <QuickAction icon={Search} label="Tìm" onClick={() => {}} />
              <QuickAction icon={Bell} label="Thông báo" onClick={() => setLevelSheet(true)} />
              <QuickAction icon={Star} label={starred ? 'Bỏ gắn sao' : 'Gắn sao'} active={starred} onClick={handleToggleStar} />
            </>
          )}
        </div>

        {/* Content rows group 1 */}
        <div className="rounded-2xl bg-raised overflow-hidden">
          {isGroup && (
            <SettingsRow label="Thành viên" onClick={onOpenMembers}>
              <MemberAvatars people={people} memberCount={memberCount} />
            </SettingsRow>
          )}
          <SettingsRow label="Tệp, ảnh và liên kết" value={pins.data ? String(list.length || '') : undefined} onClick={() => {}} />
          <SettingsRow label="Tin đã ghim" value={list.length > 0 ? String(list.length) : 'Chưa có'} onClick={() => onOpenTopic('')} />
        </div>

        {/* Settings rows group 2 */}
        <div className="rounded-2xl bg-raised overflow-hidden">
          {notification && (
            <SettingsRow label="Thông báo" value={levelOption?.title} onClick={() => setLevelSheet(true)} />
          )}
          <SettingsRow label="Mục" value={currentSection?.name ?? 'Không có'} onClick={() => setSectionSheet(true)} />
        </div>

        {/* Group management: only the owner and managers see it */}
        {mayManage && (
          <div className="rounded-2xl bg-raised overflow-hidden">
            <SettingsRow
              label="Quản lý nhóm"
              subtitle="Quyền gửi tin, thêm người, quản trị nhóm"
              icon={<Crown size={16} strokeWidth={1.75} />}
              onClick={() => setManaging('main')}
            />
          </div>
        )}

        {/* DM: create group */}
        {!isGroup && partner && (
          <div className="rounded-2xl bg-raised overflow-hidden">
            <Pressable
              onClick={() => {}}
              className="flex items-center gap-3 w-full min-h-13 px-3.5 py-2 text-accent font-medium"
            >
              <span className="grid place-items-center w-8 h-8 rounded-md bg-sunk text-ink-muted shrink-0">
                <UserPlus size={16} strokeWidth={1.75} />
              </span>
              <span className="flex-1 min-w-0">Tạo nhóm với {shortName}</span>
            </Pressable>
          </div>
        )}

        {/* Leave group */}
        {canLeave && (
          <div className="rounded-2xl bg-raised overflow-hidden">
            <Pressable
              onClick={handleLeave}
              className="flex items-center justify-center w-full min-h-13 px-3.5 py-2 text-danger font-semibold"
            >
              Rời nhóm
            </Pressable>
          </div>
        )}
      </div>

      {/* Level sheet */}
      {notification && levelSheet && (
        <NotificationLevelSheet
          conversation={{ kind, title }}
          level={notification.level}
          onChoose={notification.onChoose}
          onClose={() => setLevelSheet(false)}
        />
      )}

      <OwnerMustTransferDialog
        open={ownerLeave}
        groupName={title}
        onTransfer={mayManage ? () => setManaging('transfer') : undefined}
        onClose={() => setOwnerLeave(false)}
      />

      {/* Section picker sheet */}
      {sectionSheet && (
        <SectionPickerSheet
          sections={sections.sections}
          currentSectionId={currentSection?.id}
          onPick={handleSectionPick}
          onRemove={handleSectionRemove}
          onClose={() => setSectionSheet(false)}
        />
      )}
    </SidePanel>
  )
}

/** A quick-action tile in the 4-column grid (mockup §5 .quick). */
function QuickAction({ icon: Icon, label, active, onClick }: {
  icon: typeof Search
  label: string
  active?: boolean
  onClick: () => void
}) {
  return (
    <Pressable
      onClick={onClick}
      className="grid justify-items-center gap-1.5 py-3 px-1 rounded-2xl bg-raised text-ink text-sm font-medium"
    >
      <Icon size={20} strokeWidth={1.75} className={active ? 'text-accent' : ''} />
      {label}
    </Pressable>
  )
}

/** A grouped settings row (mockup .it pattern). */
function SettingsRow({ label, subtitle, value, icon, children, onClick }: {
  label: string
  subtitle?: string
  value?: string
  icon?: React.ReactNode
  children?: React.ReactNode
  onClick: () => void
}) {
  return (
    <Pressable
      onClick={onClick}
      className="flex items-center gap-3 w-full min-h-13 px-3.5 py-2 [&+&]:border-t [&+&]:border-line"
    >
      {icon && (
        <span className="grid place-items-center w-8 h-8 rounded-md bg-sunk text-ink-muted shrink-0">
          {icon}
        </span>
      )}
      <span className="flex-1 min-w-0 grid">
        <span className="font-medium text-md">{label}</span>
        {subtitle && <span className="text-sm text-ink-muted">{subtitle}</span>}
      </span>
      {children}
      {value !== undefined && <span className="text-sm text-ink-muted whitespace-nowrap">{value}</span>}
      <ChevronRight size={18} strokeWidth={1.75} className="text-ink-subtle shrink-0" />
    </Pressable>
  )
}

/** Stacked avatar preview for the members row. */
function MemberAvatars({ people, memberCount }: { people: PeopleDirectory; memberCount: number }) {
  const previewPeople = people.list.slice(0, 3)
  return (
    <span className="flex items-center shrink-0">
      {previewPeople.map((p, i) => (
        <Avatar
          key={p.userId}
          name={p.name}
          hueKey={p.userId}
          src={p.avatarUrl}
          size={24}
          className={`ring-2 ring-raised ${i > 0 ? '-ml-2' : ''}`}
        />
      ))}
      {memberCount > 0 && (
        <span className="text-sm text-ink-muted ml-3.5">{memberCount}</span>
      )}
    </span>
  )
}
