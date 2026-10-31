import { useState } from 'react'
import { useCanGoBack, useNavigate, useRouter } from '@tanstack/react-router'
import {
  Bell, BellOff, Building2, Calendar, ClipboardCheck, FileText, Info, LayoutGrid, MessageSquare,
  Package, Palette, ScanLine, Settings, Smartphone, User,
} from 'lucide-react'
import { logoutSession } from '../../api/client'
import { roleLabel } from '../../lib/auth-flow'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { useModuleBar } from '../../hooks/useModuleBar'
import { useMyProfile } from '../../hooks/useMyProfile'
import { isPaused, usePushPreferences } from '../../hooks/usePushPreferences'
import { useWorkspaceType } from '../../hooks/useWorkspaceType'
import { appVersion } from '../../lib/app-version'
import { UNKNOWN_PERSON } from '../../lib/people'
import { workspaceDisplayName } from '../../lib/workspace'
import { useAuthStore } from '../../stores/auth.store'
import { Avatar } from '../primitives'
import { EditBarScreen } from '../patterns/EditBarScreen'
import { ChoicePage, SettingsGroup, SettingsPage, SettingsRow } from './SettingsGroup'
import { PAUSE_CHOICES, pauseLabel, pauseUntil } from './pause'
import { useSettingsNav } from './settings-nav'

const ICON = { size: 18, strokeWidth: 1.75 } as const

/**
 * Tôi (DESIGN.md §6 "Cài đặt", mockup mobile-shell.html §3, §8): below 1024 the one place for a person's settings,
 * opened by the avatar of the top bar. Profile card, Tạm dừng thông báo, the account's options (every workspace),
 * this workspace's options, one row for Cài đặt workspace, Giới thiệu and Đăng xuất (only here). A personal
 * workspace has no Hồ sơ trong tổ chức, Phê duyệt or Tài sản.
 */
export function MeScreen() {
  const navigate = useNavigate()
  const router = useRouter()
  const canGoBack = useCanGoBack()
  const { open } = useSettingsNav()
  const me = useAuthStore((s) => s.user)
  const { workspaceId, workspaceName } = useActiveWorkspace()
  const { isPersonal } = useWorkspaceType()
  const profile = useMyProfile(workspaceId)
  const push = usePushPreferences()
  const bar = useModuleBar()
  const [layer, setLayer] = useState<'pause' | 'bar' | null>(null)

  const person = profile.data?.user
  const tenant = profile.data?.current_tenant
  const name = person?.display_name || UNKNOWN_PERSON
  const wsName = workspaceDisplayName(workspaceName)
  const subtitle = isPersonal
    ? person?.email ?? ''
    : [person?.title, tenant?.department].filter(Boolean).join(' · ')
  const kind = isPersonal ? 'Workspace cá nhân' : tenant?.role ? `${wsName} · ${roleLabel(tenant.role)}` : wsName
  const paused = isPaused(push.prefs)
  const quiet = push.prefs?.quiet_hours

  const close = () => {
    if (canGoBack) router.history.back()
    else void navigate({ to: '/channels', search: (prev: { ws?: string }) => (prev.ws ? { ws: prev.ws } : {}) })
  }

  if (layer === 'pause') {
    return (
      <ChoicePage
        title="Tạm dừng thông báo"
        value={paused ? undefined : 'off'}
        choices={PAUSE_CHOICES}
        note="Trong lúc tạm dừng, không thông báo đẩy nào được gửi, kể cả khi bạn được nhắc tên."
        onPick={(choice) => void push.save((p) => ({ ...p, pause_until: pauseUntil(choice) }))}
        onBack={() => setLayer(null)}
      />
    )
  }

  return (
    <>
      <SettingsPage title="Tôi" onBack={close} backLabel="Đóng">
        <div className="flex items-center gap-3.5 p-3.5 rounded-xl bg-raised">
          <Avatar name={name} hueKey={me?.id} src={person?.avatar_url || undefined} size={64} />
          <div className="grid gap-1 min-w-0 flex-1 justify-items-start">
            <b className="max-w-full text-[1.125rem] font-bold text-ink truncate">{name}</b>
            {subtitle && <span className="max-w-full text-sm text-ink-muted truncate">{subtitle}</span>}
            <span className="flex flex-wrap gap-1.5">
              <span className="inline-flex items-center gap-1.5 h-7 px-2.5 rounded-full bg-success-wash text-[0.8125rem] font-semibold text-ink">
                <span aria-hidden="true" className="w-2 h-2 rounded-full bg-success" />
                Đang hoạt động
              </span>
              <span
                className={`inline-flex items-center h-7 px-2.5 rounded-full text-[0.8125rem] font-semibold
                  ${isPersonal ? 'bg-sunk text-ink' : 'bg-accent-wash text-accent'}`}
              >
                {kind}
              </span>
            </span>
          </div>
        </div>

        <SettingsGroup>
          <SettingsRow
            icon={paused ? <BellOff {...ICON} /> : <Bell {...ICON} />}
            label="Tạm dừng thông báo"
            value={pauseLabel(push.prefs?.pause_until)}
            onPress={push.prefs ? () => setLayer('pause') : undefined}
          />
        </SettingsGroup>

        {/* Sign a computer in by its QR code (design/mockups/qr-login.html §3 note 5). */}
        <SettingsGroup>
          <SettingsRow
            icon={<ScanLine {...ICON} />}
            label="Quét mã QR"
            hint="Đăng nhập Nexus trên máy tính"
            onPress={() => void navigate({ to: '/scan' })}
          />
        </SettingsGroup>

        <SettingsGroup title="Tài khoản · dùng chung mọi workspace">
          <SettingsRow icon={<User {...ICON} />} label="Tài khoản và hồ sơ" onPress={() => open('account')} />
          <SettingsRow
            icon={<Bell {...ICON} />}
            label="Thông báo"
            value={quiet?.enabled ? `Giờ yên lặng ${quiet.start}–${quiet.end}` : undefined}
            onPress={() => open('notifications')}
          />
          <SettingsRow
            icon={<Palette {...ICON} />}
            label="Giao diện, ngôn ngữ"
            value={person?.language === 'en' ? 'English' : 'Tiếng Việt'}
            onPress={() => open('appearance')}
          />
          <SettingsRow icon={<Smartphone {...ICON} />} label="Thiết bị và phiên" onPress={() => open('sessions')} />
        </SettingsGroup>

        <SettingsGroup title={isPersonal ? 'Trong workspace này' : `Trong ${wsName}`}>
          {!isPersonal && (
            <SettingsRow
              icon={<Building2 {...ICON} />}
              label="Hồ sơ trong tổ chức"
              hint="Chức danh, phòng ban, vai trò"
              onPress={() => open('org-profile')}
            />
          )}
          <SettingsRow icon={<MessageSquare {...ICON} />} label="Chat" onPress={() => open('chat')} />
          <SettingsRow
            icon={<Calendar {...ICON} />}
            label="Lịch"
            hint={isPersonal ? 'Chỉ lịch cá nhân' : undefined}
            onPress={() => open('calendar')}
          />
          <SettingsRow icon={<FileText {...ICON} />} label="Tài liệu" onPress={() => open('drive')} />
          {!isPersonal && (
            <>
              <SettingsRow icon={<ClipboardCheck {...ICON} />} label="Phê duyệt" onPress={() => open('approval')} />
              <SettingsRow icon={<Package {...ICON} />} label="Tài sản" onPress={() => open('assets')} />
            </>
          )}
          <SettingsRow
            icon={<LayoutGrid {...ICON} />}
            label="Thanh đáy"
            value={bar.ready ? bar.slots.map((s) => s.label).join(', ') : undefined}
            onPress={bar.ready ? () => setLayer('bar') : undefined}
          />
        </SettingsGroup>

        <SettingsGroup>
          <SettingsRow
            icon={<Settings {...ICON} />}
            label="Cài đặt workspace"
            hint={isPersonal ? 'Tên, dung lượng' : `${wsName} · thành viên, tổ chức, quy định, dung lượng`}
            onPress={() => open('workspace')}
          />
        </SettingsGroup>

        <SettingsGroup>
          <SettingsRow icon={<Info {...ICON} />} label="Giới thiệu" value={appVersion()} />
        </SettingsGroup>

        <SettingsGroup>
          <SettingsRow label="Đăng xuất" danger center onPress={() => void logoutSession()} />
        </SettingsGroup>
      </SettingsPage>

      {layer === 'bar' && (
        <EditBarScreen modules={bar.modules} bar={bar.ids} defaults={bar.defaults} onSave={bar.saveBar} onClose={() => setLayer(null)} />
      )}
    </>
  )
}
