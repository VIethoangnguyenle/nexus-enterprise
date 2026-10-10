import { useId, useRef, type KeyboardEvent } from 'react'
import { usePreferences } from '../../hooks/usePreferences'
import type { MotionPref, ThemePref } from '../../lib/preferences'
import { Pressable } from '../primitives'

const THEMES: { id: ThemePref; label: string }[] = [
  { id: 'system', label: 'Theo hệ thống' },
  { id: 'light', label: 'Sáng' },
  { id: 'dark', label: 'Tối' },
]

const MOTIONS: { id: MotionPref; label: string }[] = [
  { id: 'system', label: 'Theo hệ thống' },
  { id: 'reduce', label: 'Luôn bật' },
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

/** A set of exclusive choices drawn as a segmented control, with arrow-key movement. */
function Segmented<T extends string>({ label, value, options, onChange }: {
  label: string
  value: T
  options: { id: T; label: string }[]
  onChange: (id: T) => void
}) {
  const refs = useRef<Record<string, HTMLButtonElement | null>>({})
  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    const i = options.findIndex((o) => o.id === value)
    let next = -1
    if (e.key === 'ArrowRight' || e.key === 'ArrowDown') next = (i + 1) % options.length
    else if (e.key === 'ArrowLeft' || e.key === 'ArrowUp') next = (i - 1 + options.length) % options.length
    const target = options[next]
    if (!target) return
    e.preventDefault()
    onChange(target.id)
    refs.current[target.id]?.focus()
  }
  return (
    <div role="radiogroup" aria-label={label} onKeyDown={onKeyDown} className="inline-flex gap-1 p-0.75 rounded-lg bg-sunk justify-self-start">
      {options.map((o) => {
        const on = o.id === value
        return (
          <Pressable
            key={o.id}
            ref={(el) => { refs.current[o.id] = el }}
            role="radio"
            aria-checked={on}
            tabIndex={on ? 0 : -1}
            onClick={() => onChange(o.id)}
            className={`h-9 px-3 rounded-md text-small-ui font-semibold transition-colors duration-quick
              ${on ? 'bg-raised text-ink' : 'text-ink-muted hover:text-ink'}`}
          >
            {o.label}
          </Pressable>
        )
      })}
    </div>
  )
}

/**
 * Cài đặt → Giao diện (mockup §5): theme as three cards that apply at once,
 * reduced motion, and the sidebar. The choices live on this device under the
 * signed-in person. Realtime colour is not a setting: it is information, so
 * there is nothing to turn off (DESIGN.md §7).
 */
export function AppearanceTab() {
  const { prefs, update } = usePreferences()
  const themeId = useId()
  const cards = useRef<Record<string, HTMLButtonElement | null>>({})

  const onThemeKeys = (e: KeyboardEvent<HTMLDivElement>) => {
    const i = THEMES.findIndex((t) => t.id === prefs.theme)
    let next = -1
    if (e.key === 'ArrowRight' || e.key === 'ArrowDown') next = (i + 1) % THEMES.length
    else if (e.key === 'ArrowLeft' || e.key === 'ArrowUp') next = (i - 1 + THEMES.length) % THEMES.length
    const target = THEMES[next]
    if (!target) return
    e.preventDefault()
    update({ theme: target.id })
    cards.current[target.id]?.focus()
  }

  return (
    <div className="grid gap-6 max-w-130">
      <section aria-labelledby={themeId} className="grid gap-3">
        <h2 id={themeId} className="m-0 text-label text-ink-muted">Chủ đề</h2>
        <div role="radiogroup" aria-labelledby={themeId} onKeyDown={onThemeKeys} className="grid grid-cols-3 gap-3">
          {THEMES.map((t) => {
            const on = t.id === prefs.theme
            return (
              <Pressable
                key={t.id}
                ref={(el) => { cards.current[t.id] = el }}
                role="radio"
                aria-checked={on}
                tabIndex={on ? 0 : -1}
                onClick={() => update({ theme: t.id })}
                className={`grid gap-2 p-2 rounded-surface bg-raised transition-shadow duration-quick
                  ${on ? 'outline-2 outline-accent -outline-offset-2' : 'hover:bg-hover'}`}
              >
                {t.id === 'system' ? <SystemThumbnail /> : <Thumbnail theme={t.id} />}
                <span className={`text-sm text-center ${on ? 'font-semibold text-ink' : 'text-ink-muted'}`}>{t.label}</span>
              </Pressable>
            )
          })}
        </div>
        <span className="text-xs text-ink-muted">Mặc định là Theo hệ thống. Lưu trên máy này và theo tài khoản.</span>
      </section>

      <section className="grid gap-2.5">
        <div className="grid">
          <span className="font-semibold text-ink">Giảm chuyển động</span>
          <span className="text-sm text-ink-muted">
            Tắt trượt và phóng to; giữ fade ngắn và lớp màu realtime vì đó là thông tin.
          </span>
        </div>
        <Segmented
          label="Giảm chuyển động"
          value={prefs.motion}
          options={MOTIONS}
          onChange={(motion) => update({ motion })}
        />
      </section>

      <section className="flex items-center gap-4">
        <div className="grid flex-1 min-w-0">
          <span id={`${themeId}-rail`} className="font-semibold text-ink">Thu gọn sidebar</span>
          <span className="text-sm text-ink-muted">Chỉ hiện icon, tên mục hiện khi rê chuột.</span>
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
      </section>
    </div>
  )
}
