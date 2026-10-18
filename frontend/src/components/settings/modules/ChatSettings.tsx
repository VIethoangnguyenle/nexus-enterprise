import { useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import type { ChatSettings as ChatOptions } from '../../../api/settings'
import { useActiveWorkspace } from '../../../hooks/useActiveWorkspace'
import { useBottomBarLayout } from '../../../hooks/usePhone'
import { usePushPreferences } from '../../../hooks/usePushPreferences'
import { useServerSettings } from '../../../hooks/useServerSettings'
import { useWorkspaceLocks } from '../../../hooks/useWorkspaces'
import { useWorkspaceType } from '../../../hooks/useWorkspaceType'
import { levelOptions } from '../../../lib/notification-level-copy'
import { CHAT_DEFAULTS } from '../../../lib/settings-defaults'
import type { PushPreferences } from '../../../lib/push'
import { ChoicePage, SettingsGroup, SettingsPage, SettingsRow, SettingsSwitch, choiceLabel, type Choice } from '../SettingsGroup'
import { useSettingsNav } from '../settings-nav'
import { GroupCreationRow } from '../GroupCreationRow'

export { CHAT_DEFAULTS }

type LevelChoice = 'all' | 'main' | 'off'

/** A group's levels; a person who never chose starts at the group default, Tin chính. */
const LEVELS: Choice<LevelChoice>[] = levelOptions('space').map((o) => ({ value: o.level as LevelChoice, label: o.title, hint: o.description }))
const levelOf = (v: ChatOptions['default_level']): LevelChoice => v ?? 'main'
/** The level in a word, for the row (the choice page says it in full). */
const LEVEL_SHORT: Record<LevelChoice, string> = { all: 'Tất cả', main: 'Tin chính', off: 'Tắt' }

const OPEN_AT: Choice<'all' | 'sections'>[] = [
  { value: 'all', label: 'Tất cả', hint: 'Mọi trò chuyện, theo hoạt động mới nhất' },
  { value: 'sections', label: 'Mục', hint: 'Theo các mục của bạn' },
]
const SORT: Choice<'activity' | 'name'>[] = [
  { value: 'activity', label: 'Hoạt động mới nhất' },
  { value: 'name', label: 'Tên' },
]
const DOUBLE_TAP: Choice<string>[] = [
  { value: '👍', label: '👍' },
  { value: '', label: 'Tắt' },
]

type Kind = keyof PushPreferences['kinds']
const KINDS: { kind: Kind; label: string; hint?: string }[] = [
  { kind: 'direct', label: 'Tin nhắn trực tiếp' },
  { kind: 'mention', label: 'Được nhắc tên', hint: 'Trong nhóm và thread, kể cả @Tất cả' },
  { kind: 'thread_reply', label: 'Thread đang theo dõi' },
  { kind: 'incoming_call', label: 'Cuộc gọi đến' },
]

/**
 * Cài đặt Chat (DESIGN.md §6 "Cài đặt", mockup mobile-shell.html §4, §8): the same page from Tôi → Chat and from
 * Chat's ⋯. Notifications are the account's push switches (messaging) plus the level a new group starts at; the
 * rest are this person's options in this workspace. Every switch and choice saves at once and is put back with a
 * toast when the server refuses.
 *
 * Enter để gửi, Mở Chat ở and Sắp xếp change the composer and the chip list below 1024px only (a wide screen keeps
 * Enter sends and its own list), so a wide screen does not offer them. Xem trước liên kết is not offered: messages
 * have no link previews yet, so the switch would change nothing.
 */
export function ChatSettings() {
  const { back } = useSettingsNav()
  const navigate = useNavigate()
  const { workspaceId } = useActiveWorkspace()
  const { isPersonal } = useWorkspaceType()
  const compact = useBottomBarLayout()
  const { settings, save } = useServerSettings('chat', workspaceId, CHAT_DEFAULTS)
  const push = usePushPreferences()
  const locks = useWorkspaceLocks(isPersonal ? '' : workspaceId)
  const [choosing, setChoosing] = useState<'level' | 'openAt' | 'sort' | 'doubleTap' | null>(null)
  const done = () => setChoosing(null)

  if (choosing === 'level') {
    return (
      <ChoicePage
        title="Mức mặc định cho nhóm mới"
        value={levelOf(settings.default_level)}
        choices={LEVELS}
        note="Mức một nhóm bắt đầu khi bạn tạo hoặc được thêm vào. Đổi riêng từng nhóm trong cài đặt nhóm."
        onPick={(default_level) => void save({ default_level })}
        onBack={done}
      />
    )
  }
  if (choosing === 'openAt') {
    return <ChoicePage title="Mở Chat ở" value={settings.open_at} choices={OPEN_AT} onPick={(open_at) => void save({ open_at })} onBack={done} />
  }
  if (choosing === 'sort') {
    return <ChoicePage title="Sắp xếp" value={settings.sort} choices={SORT} onPick={(sort) => void save({ sort })} onBack={done} />
  }
  if (choosing === 'doubleTap') {
    return (
      <ChoicePage
        title="Cảm xúc khi chạm đúp"
        value={settings.double_tap}
        choices={DOUBLE_TAP}
        onPick={(double_tap) => void save({ double_tap })}
        onBack={done}
      />
    )
  }

  const kinds = push.prefs?.kinds
  return (
    <SettingsPage title="Cài đặt Chat" onBack={back}>
      <SettingsGroup title="Thông báo" note={push.isError ? 'Không tải được thông báo đẩy. Thử lại sau.' : undefined}>
        {KINDS.map(({ kind, label, hint }) => (
          <SettingsSwitch
            key={kind}
            label={label}
            hint={hint}
            checked={kinds?.[kind] ?? true}
            disabled={!push.prefs || push.isLoading}
            onChange={(on) => void push.save((p) => ({ ...p, kinds: { ...p.kinds, [kind]: on } }))}
          />
        ))}
        <SettingsRow
          label="Mức mặc định cho nhóm mới"
          value={LEVEL_SHORT[levelOf(settings.default_level)]}
          onPress={() => setChoosing('level')}
        />
      </SettingsGroup>

      <SettingsGroup title="Soạn và đọc">
        {compact && (
          <SettingsSwitch
            label="Enter để gửi"
            hint="Tắt: Enter xuống dòng, chạm nút Gửi để gửi"
            checked={!!settings.enter_to_send}
            onChange={(enter_to_send) => void save({ enter_to_send })}
          />
        )}
        <SettingsRow
          label="Cảm xúc khi chạm đúp"
          value={settings.double_tap ? settings.double_tap : 'Tắt'}
          onPress={() => setChoosing('doubleTap')}
        />
      </SettingsGroup>

      <SettingsGroup title="Danh sách">
        {compact && (
          <>
            <SettingsRow
              label="Mở Chat ở"
              hint="Tất cả: theo hoạt động mới nhất · Mục: theo mục của bạn"
              value={choiceLabel(OPEN_AT, settings.open_at)}
              onPress={() => setChoosing('openAt')}
            />
            <SettingsRow label="Sắp xếp" value={choiceLabel(SORT, settings.sort)} onPress={() => setChoosing('sort')} />
          </>
        )}
        <SettingsRow
          label="Quản lý mục"
          hint="Đổi tên, sắp thứ tự, xoá mục"
          onPress={() => void navigate({ to: '/channels', search: (prev: { ws?: string }) => ({ ...(prev.ws ? { ws: prev.ws } : {}), tab: 'sections' as const }) })}
        />
      </SettingsGroup>

      {!isPersonal && (
        <SettingsGroup title="Nhóm mới">
          <GroupCreationRow rule={locks.data?.group_creation} />
        </SettingsGroup>
      )}
    </SettingsPage>
  )
}
