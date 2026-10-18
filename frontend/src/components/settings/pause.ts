import { formatDate, formatTime } from '../../lib/format'
import { isPaused } from '../../hooks/usePushPreferences'
import type { Choice } from './SettingsGroup'

/** How long Tạm dừng thông báo lasts. */
export type PauseChoice = 'off' | '30m' | '1h' | '2h' | 'morning'

export const PAUSE_CHOICES: Choice<PauseChoice>[] = [
  { value: 'off', label: 'Không tạm dừng' },
  { value: '30m', label: '30 phút' },
  { value: '1h', label: '1 giờ' },
  { value: '2h', label: '2 giờ' },
  { value: 'morning', label: 'Đến 8:00 sáng mai' },
]

/** The moment a pause chosen `now` ends, as RFC 3339; null for no pause. */
export function pauseUntil(choice: PauseChoice, now: Date = new Date()): string | null {
  const minutes = { '30m': 30, '1h': 60, '2h': 120 } as const
  if (choice === 'off') return null
  if (choice === 'morning') {
    const t = new Date(now.getFullYear(), now.getMonth(), now.getDate() + 1, 8, 0, 0, 0)
    return t.toISOString()
  }
  return new Date(now.getTime() + minutes[choice] * 60_000).toISOString()
}

/** The value of the Tạm dừng thông báo row: "Tắt", "Đến 14:30", or "Đến 08:00 20/10/2026" on another day. */
export function pauseLabel(until: string | null | undefined, now: Date = new Date()): string {
  if (!isPaused({ pause_until: until }, now.getTime())) return 'Tắt'
  const end = new Date(until!)
  const sameDay = end.toDateString() === now.toDateString()
  return sameDay ? `Đến ${formatTime(end)}` : `Đến ${formatTime(end)} ${formatDate(end)}`
}
