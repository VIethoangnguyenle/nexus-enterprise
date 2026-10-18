import { useState } from 'react'
import { Bell, BellOff, MessageSquare, AtSign, MessagesSquare, Moon, AlertTriangle, Phone } from 'lucide-react'
import type { PushPreferences } from '../../lib/push'
import { usePushDevice } from '../../hooks/usePushDevice'
import { browserZone, usePushPreferences } from '../../hooks/usePushPreferences'
import { Button, Pressable, Select } from '../primitives'
import { EmptyState } from '../spaces/EmptyState'

const HOURS = Array.from({ length: 24 }, (_, i) => `${String(i).padStart(2, '0')}:00`)

/**
 * Cài đặt → Thông báo on a wide screen: this device's push, the kinds that push,
 * and quiet hours. The phone's Tôi → Thông báo reads the same preferences
 * (`usePushPreferences`) and the same device state (`usePushDevice`).
 */
export function NotificationSettings() {
  const device = usePushDevice()
  const { prefs, isLoading, isError, refetch, isSaving, save } = usePushPreferences(device.available)
  const [quietError, setQuietError] = useState<string | null>(null)

  if (!device.available) {
    return (
      <div className="flex-1 grid place-items-center bg-base py-12">
        <EmptyState
          icon={<BellOff size={24} strokeWidth={1.75} />}
          text="Thông báo đẩy không khả dụng trên môi trường này."
        />
      </div>
    )
  }

  if (isError || !prefs) {
    return (
      <div className="flex-1 grid place-items-center bg-base py-12">
        <div className="flex flex-col items-center gap-3 text-center">
          <AlertTriangle size={24} strokeWidth={1.75} className="text-danger" />
          <p className="text-sm text-ink-muted">Không tải được cài đặt thông báo.</p>
          <Button variant="ghost" size="sm" onClick={() => void refetch()}>
            Thử lại
          </Button>
        </div>
      </div>
    )
  }

  const updateKinds = (key: keyof PushPreferences['kinds']) => {
    void save((p) => ({ ...p, kinds: { ...p.kinds, [key]: !p.kinds[key] } }))
  }

  const updateQuietHours = (changes: Partial<PushPreferences['quiet_hours']>) => {
    const merged = { ...prefs.quiet_hours, ...changes }
    // The server refuses an empty window too; say so here, in words.
    if (merged.start === merged.end && merged.enabled) {
      setQuietError('Giờ bắt đầu và kết thúc không được trùng.')
      return
    }
    setQuietError(null)
    // Always the browser's zone, so the server applies the window where the person is.
    void save((p) => ({ ...p, quiet_hours: { ...p.quiet_hours, ...changes, time_zone: browserZone() || p.quiet_hours.time_zone } }))
  }

  const isBlocked = device.blocked
  const isIos = device.ios
  const deviceOptedIn = device.optedIn
  const deviceDisabled = device.disabled
  const dimmed = deviceOptedIn === false || deviceOptedIn === null || isBlocked
  const busy = isLoading || isSaving

  return (
    <div className="flex-1 flex flex-col gap-6 max-w-2xl animate-fade-in">
      {/* Banners */}
      {isBlocked && (
        <div
          role="status"
          className="p-3.5 rounded-xl bg-danger-wash text-ink border border-danger/20 text-xs flex flex-col gap-2"
        >
          <strong className="font-semibold text-danger">Trình duyệt đang chặn thông báo của Nexus</strong>
          <ol className="list-decimal pl-4 space-y-1 text-ink-muted">
            <li>Bấm biểu tượng ổ khoá cạnh địa chỉ trang.</li>
            <li>Chọn Quyền → Thông báo → Cho phép.</li>
            <li>Quay lại đây, công tắc sẽ tự bật.</li>
          </ol>
        </div>
      )}

      {isIos && (
        <div
          role="status"
          className="p-3.5 rounded-xl bg-info-wash text-ink border border-info/20 text-xs flex flex-col gap-1.5"
        >
          <strong className="font-semibold text-info">
            Thêm Nexus vào màn hình chính để nhận thông báo trên iPhone.
          </strong>
        </div>
      )}

      {/* Group 1: Thiết bị này */}
      <section aria-labelledby="heading-device" className="flex flex-col gap-3">
        <h2 id="heading-device" className="text-xs font-semibold tracking-wider uppercase text-ink-muted">
          Thiết bị này
        </h2>
        <div className="flex items-center justify-between gap-4 py-2 border-b border-line">
          <div className="flex items-center gap-3 min-w-0">
            <span aria-hidden="true" className="w-8 h-8 rounded-lg bg-sunk text-ink-muted flex items-center justify-center shrink-0">
              <Bell size={16} strokeWidth={1.75} />
            </span>
            <div className="grid gap-0.5">
              <span id="label-dev" className="text-sm font-semibold text-ink">
                Thông báo đẩy
              </span>
              <span className="text-xs text-ink-muted">{device.status}</span>
              {device.failed && !isBlocked && !isIos ? (
                <Button
                  variant="ghost"
                  size="sm"
                  className="justify-self-start"
                  onClick={device.retry}
                >
                  Thử lại
                </Button>
              ) : null}
            </div>
          </div>
          <Pressable
            role="switch"
            aria-checked={deviceOptedIn ?? false}
            aria-label="Thông báo đẩy trên thiết bị này"
            disabled={deviceDisabled}
            onClick={() => void device.toggle()}
            className={`relative shrink-0 w-11 h-6 rounded-full transition-opacity ${
              deviceDisabled ? 'opacity-40 cursor-not-allowed' : 'cursor-pointer'
            }`}
          >
            <span
              aria-hidden="true"
              className={`absolute inset-0 rounded-full transition-colors duration-press ${
                deviceOptedIn ? 'bg-accent' : 'bg-sunk field-line'
              }`}
            />
            <span
              aria-hidden="true"
              className={`absolute top-0.5 left-0.5 w-5 h-5 rounded-full bg-raised shadow-overlay
                transition-transform duration-press ${deviceOptedIn ? 'translate-x-5' : ''}`}
            />
          </Pressable>
        </div>
      </section>

      {/* Group 2: Báo cho tôi khi */}
      <section
        aria-labelledby="heading-kinds"
        className={`flex flex-col gap-3 transition-opacity ${dimmed ? 'opacity-50 pointer-events-none' : ''}`}
      >
        <h2 id="heading-kinds" className="text-xs font-semibold tracking-wider uppercase text-ink-muted">
          Báo cho tôi khi
        </h2>

        {/* Kind 1: Tin nhắn trực tiếp */}
        <div className="flex items-center justify-between gap-4 py-2 border-b border-line">
          <div className="flex items-center gap-3 min-w-0">
            <span aria-hidden="true" className="w-8 h-8 rounded-lg bg-sunk text-ink-muted flex items-center justify-center shrink-0">
              <MessageSquare size={16} strokeWidth={1.75} />
            </span>
            <span id="label-direct" className="text-sm font-semibold text-ink">
              Tin nhắn trực tiếp
            </span>
          </div>
          <Pressable
            role="switch"
            aria-checked={prefs.kinds.direct}
            aria-label="Tin nhắn trực tiếp"
            disabled={busy}
            onClick={() => updateKinds('direct')}
            className="relative shrink-0 w-11 h-6 rounded-full cursor-pointer"
          >
            <span
              aria-hidden="true"
              className={`absolute inset-0 rounded-full transition-colors duration-press ${
                prefs.kinds.direct ? 'bg-accent' : 'bg-sunk field-line'
              }`}
            />
            <span
              aria-hidden="true"
              className={`absolute top-0.5 left-0.5 w-5 h-5 rounded-full bg-raised shadow-overlay
                transition-transform duration-press ${prefs.kinds.direct ? 'translate-x-5' : ''}`}
            />
          </Pressable>
        </div>

        {/* Kind 2: Được nhắc tên */}
        <div className="flex items-center justify-between gap-4 py-2 border-b border-line">
          <div className="flex items-center gap-3 min-w-0">
            <span aria-hidden="true" className="w-8 h-8 rounded-lg bg-sunk text-ink-muted flex items-center justify-center shrink-0">
              <AtSign size={16} strokeWidth={1.75} />
            </span>
            <div className="grid gap-0.5">
              <span id="label-mention" className="text-sm font-semibold text-ink">
                Được nhắc tên
              </span>
              <span className="text-xs text-ink-muted">Trong nhóm và thread</span>
            </div>
          </div>
          <Pressable
            role="switch"
            aria-checked={prefs.kinds.mention}
            aria-label="Được nhắc tên"
            disabled={busy}
            onClick={() => updateKinds('mention')}
            className="relative shrink-0 w-11 h-6 rounded-full cursor-pointer"
          >
            <span
              aria-hidden="true"
              className={`absolute inset-0 rounded-full transition-colors duration-press ${
                prefs.kinds.mention ? 'bg-accent' : 'bg-sunk field-line'
              }`}
            />
            <span
              aria-hidden="true"
              className={`absolute top-0.5 left-0.5 w-5 h-5 rounded-full bg-raised shadow-overlay
                transition-transform duration-press ${prefs.kinds.mention ? 'translate-x-5' : ''}`}
            />
          </Pressable>
        </div>

        {/* Kind 3: Thread bạn theo dõi */}
        <div className="flex items-center justify-between gap-4 py-2 border-b border-line">
          <div className="flex items-center gap-3 min-w-0">
            <span aria-hidden="true" className="w-8 h-8 rounded-lg bg-sunk text-ink-muted flex items-center justify-center shrink-0">
              <MessagesSquare size={16} strokeWidth={1.75} />
            </span>
            <div className="grid gap-0.5">
              <span id="label-threads" className="text-sm font-semibold text-ink">
                Thread bạn theo dõi
              </span>
              <span className="text-xs text-ink-muted">Trả lời mới trong thread đang theo dõi</span>
            </div>
          </div>
          <Pressable
            role="switch"
            aria-checked={prefs.kinds.thread_reply}
            aria-label="Thread bạn theo dõi"
            disabled={busy}
            onClick={() => updateKinds('thread_reply')}
            className="relative shrink-0 w-11 h-6 rounded-full cursor-pointer"
          >
            <span
              aria-hidden="true"
              className={`absolute inset-0 rounded-full transition-colors duration-press ${
                prefs.kinds.thread_reply ? 'bg-accent' : 'bg-sunk field-line'
              }`}
            />
            <span
              aria-hidden="true"
              className={`absolute top-0.5 left-0.5 w-5 h-5 rounded-full bg-raised shadow-overlay
                transition-transform duration-press ${prefs.kinds.thread_reply ? 'translate-x-5' : ''}`}
            />
          </Pressable>
        </div>

        {/* Kind 4: Cuộc gọi đến */}
        <div className="flex items-center justify-between gap-4 py-2 border-b border-line">
          <div className="flex items-center gap-3 min-w-0">
            <span aria-hidden="true" className="w-8 h-8 rounded-lg bg-sunk text-ink-muted flex items-center justify-center shrink-0">
              <Phone size={16} strokeWidth={1.75} />
            </span>
            <span id="label-incoming-call" className="text-sm font-semibold text-ink">
              Cuộc gọi đến
            </span>
          </div>
          <Pressable
            role="switch"
            aria-checked={prefs.kinds.incoming_call}
            aria-label="Cuộc gọi đến"
            disabled={busy}
            onClick={() => updateKinds('incoming_call')}
            className="relative shrink-0 w-11 h-6 rounded-full cursor-pointer"
          >
            <span
              aria-hidden="true"
              className={`absolute inset-0 rounded-full transition-colors duration-press ${
                prefs.kinds.incoming_call ? 'bg-accent' : 'bg-sunk field-line'
              }`}
            />
            <span
              aria-hidden="true"
              className={`absolute top-0.5 left-0.5 w-5 h-5 rounded-full bg-raised shadow-overlay
                transition-transform duration-press ${prefs.kinds.incoming_call ? 'translate-x-5' : ''}`}
            />
          </Pressable>
        </div>
      </section>

      {/* Group 3: Giờ yên lặng */}
      <section
        aria-labelledby="heading-quiet"
        className={`flex flex-col gap-3 transition-opacity ${dimmed ? 'opacity-50 pointer-events-none' : ''}`}
      >
        <h2 id="heading-quiet" className="text-xs font-semibold tracking-wider uppercase text-ink-muted">
          Giờ yên lặng
        </h2>

        <div className="flex flex-col gap-3 py-2">
          <div className="flex items-center justify-between gap-4">
            <div className="flex items-center gap-3 min-w-0">
              <span aria-hidden="true" className="w-8 h-8 rounded-lg bg-sunk text-ink-muted flex items-center justify-center shrink-0">
                <Moon size={16} strokeWidth={1.75} />
              </span>
              <div className="grid gap-0.5">
                <span id="label-quiet" className="text-sm font-semibold text-ink">
                  Giờ yên lặng
                </span>
                <span className="text-xs text-ink-muted">Không báo trong khoảng giờ này</span>
              </div>
            </div>
            <Pressable
              role="switch"
              aria-checked={prefs.quiet_hours.enabled}
              aria-label="Giờ yên lặng"
              disabled={busy}
              onClick={() => updateQuietHours({ enabled: !prefs.quiet_hours.enabled })}
              className="relative shrink-0 w-11 h-6 rounded-full cursor-pointer"
            >
              <span
                aria-hidden="true"
                className={`absolute inset-0 rounded-full transition-colors duration-press ${
                  prefs.quiet_hours.enabled ? 'bg-accent' : 'bg-sunk field-line'
                }`}
              />
              <span
                aria-hidden="true"
                className={`absolute top-0.5 left-0.5 w-5 h-5 rounded-full bg-raised shadow-overlay
                  transition-transform duration-press ${prefs.quiet_hours.enabled ? 'translate-x-5' : ''}`}
              />
            </Pressable>
          </div>

          {prefs.quiet_hours.enabled && (
            <div className="flex items-center gap-3 pl-11 pt-1 animate-fade-in">
              <label htmlFor="q-from" className="text-xs font-medium text-ink-muted">
                Từ
              </label>
              <Select
                id="q-from"
                value={prefs.quiet_hours.start}
                onChange={(e) => updateQuietHours({ start: e.target.value })}
                className="h-9 px-2 rounded-lg bg-raised border border-line text-sm font-medium text-ink focus-ring"
              >
                {HOURS.map((h) => (
                  <option key={h} value={h}>
                    {h}
                  </option>
                ))}
              </Select>

              <span className="text-xs text-ink-muted">đến</span>

              <label htmlFor="q-to" className="text-xs font-medium text-ink-muted sr-only">
                Đến
              </label>
              <Select
                id="q-to"
                value={prefs.quiet_hours.end}
                onChange={(e) => updateQuietHours({ end: e.target.value })}
                className="h-9 px-2 rounded-lg bg-raised border border-line text-sm font-medium text-ink focus-ring"
              >
                {HOURS.map((h) => (
                  <option key={h} value={h}>
                    {h}
                  </option>
                ))}
              </Select>
            </div>
          )}
          {quietError && (
            <p className="text-xs text-danger pl-11 animate-fade-in" role="alert">{quietError}</p>
          )}
        </div>
      </section>
    </div>
  )
}
