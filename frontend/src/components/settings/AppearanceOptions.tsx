import { useId, useRef, type KeyboardEvent } from 'react'
import type { AccountLanguage } from '../../api/auth'
import { useAccountLanguage } from '../../hooks/useAccountLanguage'
import { useBottomBarLayout } from '../../hooks/usePhone'
import { usePreferences } from '../../hooks/usePreferences'
import type { MotionPref, ThemePref } from '../../lib/preferences'
import { Pressable } from '../primitives'
import type { Choice } from './SettingsGroup'
import { Segmented } from './SettingsChoice'

export const THEME_CHOICES: Choice<ThemePref>[] = [
  { value: 'system', label: 'Theo hệ thống' },
  { value: 'light', label: 'Sáng' },
  { value: 'dark', label: 'Tối' },
]

export const LANGUAGE_CHOICES: Choice<AccountLanguage>[] = [
  { value: 'vi', label: 'Tiếng Việt' },
  { value: 'en', label: 'English' },
]

const MOTION_CHOICES: Choice<MotionPref>[] = [
  { value: 'system', label: 'Theo hệ thống' },
  { value: 'reduce', label: 'Luôn bật' },
]

/** A miniature app window drawn in one theme's own surfaces. */
function Thumbnail({ theme }: { theme: 'light' | 'dark' }) {
  return (
    <span
      data-preview={theme}
      aria-hidden="true"
      className="grid grid-cols-[28%_1fr] w-full h-16 rounded-md overflow-hidden bg-(--pv-base)"
    >
      <span className="grid content-start gap-1.5 p-2 bg-(--pv-sunk)">
        <span className="h-1.5 rounded-full bg-(--pv-accent)" />
        <span className="h-1.5 rounded-full bg-(--pv-line)" />
        <span className="h-1.5 rounded-full bg-(--pv-line)" />
      </span>
      <span className="grid content-start gap-1.5 p-2">
        <span className="h-1.5 w-2/5 rounded-full bg-(--pv-ink)" />
        <span className="h-4 rounded-sm bg-(--pv-raised)" />
        <span className="h-4 rounded-sm bg-(--pv-raised)" />
      </span>
    </span>
  )
}

/** Both halves of the window, for "Theo hệ thống". */
function SystemThumbnail() {
  return (
    <span aria-hidden="true" className="relative block w-full h-16 rounded-md overflow-hidden">
      <span className="absolute inset-0"><Thumbnail theme="light" /></span>
      <span className="absolute inset-y-0 right-0 w-1/2 overflow-hidden">
        <span className="block h-full w-[200%] -translate-x-1/2"><Thumbnail theme="dark" /></span>
      </span>
    </span>
  )
}

/**
 * Giao diện, ngôn ngữ on a wide screen (mockup settings-desktop.html §5): theme as three cards that apply at
 * once, reduced motion, the account's language, and (where there is one) the sidebar. Theme and motion live on this
 * device under the signed-in person. Realtime colour is not a setting: it is information, so there is nothing to
 * turn off (DESIGN.md §7).
 */
export function AppearanceOptions() {
  const { prefs, update } = usePreferences()
  const language = useAccountLanguage()
  // The sidebar exists only where the bottom bar does not.
  const phoneLayout = useBottomBarLayout()
  const themeId = useId()
  const cards = useRef<Record<string, HTMLButtonElement | null>>({})

  const onThemeKeys = (e: KeyboardEvent<HTMLDivElement>) => {
    const i = THEME_CHOICES.findIndex((t) => t.value === prefs.theme)
    let next = -1
    if (e.key === 'ArrowRight' || e.key === 'ArrowDown') next = (i + 1) % THEME_CHOICES.length
    else if (e.key === 'ArrowLeft' || e.key === 'ArrowUp') next = (i - 1 + THEME_CHOICES.length) % THEME_CHOICES.length
    const target = THEME_CHOICES[next]
    if (!target) return
    e.preventDefault()
    update({ theme: target.value })
    cards.current[target.value]?.focus()
  }

  return (
    <div className="grid gap-6 max-w-130">
      <section aria-labelledby={themeId} className="grid gap-3">
        <h2 id={themeId} className="m-0 text-label text-ink-muted">Chủ đề</h2>
        <div role="radiogroup" aria-labelledby={themeId} onKeyDown={onThemeKeys} className="grid grid-cols-3 gap-3">
          {THEME_CHOICES.map((t) => {
            const on = t.value === prefs.theme
            return (
              <Pressable
                key={t.value}
                ref={(el) => { cards.current[t.value] = el }}
                role="radio"
                aria-checked={on}
                tabIndex={on ? 0 : -1}
                onClick={() => update({ theme: t.value })}
                className={`grid gap-2 p-2 rounded-surface bg-raised transition-shadow duration-quick
                  ${on ? 'outline-2 outline-accent -outline-offset-2' : 'hover:bg-hover'}`}
              >
                {t.value === 'system' ? <SystemThumbnail /> : <Thumbnail theme={t.value} />}
                <span className={`text-sm text-center ${on ? 'font-semibold text-ink' : 'text-ink-muted'}`}>{t.label}</span>
              </Pressable>
            )
          })}
        </div>
        <span className="text-xs text-ink-muted">Mặc định là Theo hệ thống. Lưu trên máy này và theo tài khoản.</span>
      </section>

      <section className="grid gap-2.5 justify-items-start">
        <div className="grid">
          <span className="font-semibold text-ink">Giảm chuyển động</span>
          <span className="text-sm text-ink-muted">
            Tắt trượt và phóng to; giữ fade ngắn và lớp màu realtime vì đó là thông tin.
          </span>
        </div>
        <Segmented
          label="Giảm chuyển động"
          value={prefs.motion}
          choices={MOTION_CHOICES}
          onPick={(motion) => update({ motion })}
        />
      </section>

      <section className="grid gap-2.5 justify-items-start">
        <div className="grid">
          <span className="font-semibold text-ink">Ngôn ngữ</span>
          <span className="text-sm text-ink-muted">Dùng chung mọi workspace và mọi thiết bị của bạn.</span>
        </div>
        <Segmented
          label="Ngôn ngữ"
          value={language.value}
          choices={LANGUAGE_CHOICES}
          onPick={(v) => { if (language.ready && v !== language.value) language.set(v) }}
        />
      </section>
      {!phoneLayout && <section className="flex items-center gap-4">
        <div className="grid flex-1 min-w-0">
          <span id={`${themeId}-rail`} className="font-semibold text-ink">Thu gọn sidebar</span>
          <span className="text-sm text-ink-muted">Menu chính thành dải icon 72px, mỗi mục một nhãn nhỏ bên dưới. Phím tắt Ctrl+\ (⌘\ trên macOS).</span>
        </div>
        <Pressable
          role="switch"
          aria-checked={prefs.sidebarCollapsed}
          aria-labelledby={`${themeId}-rail`}
          onClick={() => update({ sidebarCollapsed: !prefs.sidebarCollapsed })}
          className="relative shrink-0 w-10 h-6 rounded-full before:absolute before:-inset-2.5 before:content-['']"
        >
          <span
            aria-hidden="true"
            className={`absolute inset-0 rounded-full transition-colors duration-press ${prefs.sidebarCollapsed ? 'bg-accent' : 'bg-sunk field-line'}`}
          />
          <span
            aria-hidden="true"
            className={`absolute top-0.5 left-0.5 w-5 h-5 rounded-full bg-raised shadow-overlay
              transition-transform duration-press ${prefs.sidebarCollapsed ? 'translate-x-4' : ''}`}
          />
        </Pressable>
      </section>}
    </div>
  )
}
