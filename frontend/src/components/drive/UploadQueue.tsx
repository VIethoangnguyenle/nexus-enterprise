import { useEffect, useMemo, useState } from 'react'
import { AlertCircle, RotateCw, X } from 'lucide-react'
import { useUiStore } from '../../stores/ui.store'
import { useUploadQueue, type UploadTask } from '../../stores/upload-queue.store'
import { IconButton, Pressable } from '../primitives'

/** "còn khoảng 10 giây" from what the current file has sent so far. */
export function timeLeft(task: UploadTask, now: number): string | null {
  if (task.status !== 'uploading' || !task.startedAt || task.sent <= 0) return null
  const rate = task.sent / Math.max(1, now - task.startedAt) // bytes per ms
  const s = Math.ceil((task.size - task.sent) / rate / 1000)
  if (!Number.isFinite(s) || s <= 0) return null
  return s < 60 ? `còn khoảng ${s} giây` : `còn khoảng ${Math.ceil(s / 60)} phút`
}

/**
 * The progress card of background uploads (DESIGN.md "Tài liệu", "Nút +"):
 * floats above the phone's bottom bar, says "Đang tải lên N tệp" with the
 * share sent and a 4px bar, the file on its way and the time left; each file
 * that failed gets its reason and Thử lại. While the list scrolls it folds to
 * a ring with the percentage, which unfolds on a tap. It renders nothing when
 * the queue is empty.
 */
export function UploadQueue({ collapsed = false }: { collapsed?: boolean }) {
  const tasks = useUploadQueue((s) => s.tasks)
  const retry = useUploadQueue((s) => s.retry)
  const dismiss = useUploadQueue((s) => s.dismiss)
  const clearDone = useUploadQueue((s) => s.clearDone)
  const aboveBar = useUiStore((s) => s.chatBarShown)
  const [opened, setOpened] = useState(false)
  const [now, setNow] = useState(() => Date.now())

  const active = useMemo(() => tasks.filter((t) => t.status === 'queued' || t.status === 'uploading'), [tasks])
  const problems = useMemo(() => tasks.filter((t) => t.status === 'failed' || t.status === 'lost'), [tasks])
  const sending = tasks.find((t) => t.status === 'uploading')

  // The time left is read again every second while something is on its way.
  useEffect(() => {
    if (!sending) return
    const id = setInterval(() => setNow(Date.now()), 1000)
    return () => clearInterval(id)
  }, [sending])

  // Once only finished files remain, the card goes; the toast has said it.
  useEffect(() => {
    if (active.length === 0 && problems.length === 0 && tasks.length > 0) clearDone()
  }, [active.length, problems.length, tasks.length, clearDone])

  if (active.length === 0 && problems.length === 0) return null

  // The share of this batch sent so far, counting what already finished in it.
  const batch = tasks.filter((t) => t.status !== 'failed' && t.status !== 'lost')
  const total = batch.reduce((n, t) => n + t.size, 0)
  const sent = batch.reduce((n, t) => n + (t.status === 'done' ? t.size : t.sent), 0)
  const pct = total > 0 ? Math.min(100, Math.round((sent / total) * 100)) : 0
  const left = sending ? timeLeft(sending, now) : null
  const place = aboveBar
    ? 'bottom-[calc(6.375rem+env(safe-area-inset-bottom,0px))]'
    : 'bottom-[calc(1rem+env(safe-area-inset-bottom,0px))]'

  if (collapsed && !opened && active.length > 0) {
    return (
      <Pressable
        aria-label={`Đang tải lên ${active.length} tệp, ${pct}%`}
        onClick={() => setOpened(true)}
        className={`fixed right-4 ${place} z-sticky grid place-items-center size-12 rounded-full bg-overlay shadow-overlay
          focus-ring`}
        style={{ background: `conic-gradient(var(--color-accent) ${pct * 3.6}deg, var(--color-overlay) 0deg)` }}
      >
        <span className="grid place-items-center size-10 rounded-full bg-overlay text-xs font-semibold tnum">{pct}%</span>
      </Pressable>
    )
  }

  return (
    <section
      role="status"
      aria-label="Tải lên"
      className={`fixed inset-x-3.5 ${place} z-sticky lg:left-auto lg:right-4 lg:w-90 rounded-2xl bg-overlay
        shadow-overlay ring-1 ring-inset ring-line px-4 py-3 grid gap-2`}
    >
      {active.length > 0 && (
        <div className="grid gap-1.5">
          <div className="flex items-center justify-between gap-3 text-sm font-semibold">
            <span>Đang tải lên {active.length} tệp</span>
            <span className="tnum">{pct}%</span>
          </div>
          <div className="h-1 rounded-full bg-sunk overflow-hidden" aria-hidden="true">
            <i className="block h-full bg-accent rounded-full transition-[width] duration-quick" style={{ width: `${pct}%` }} />
          </div>
          {sending && (
            <small className="text-xs text-ink-muted truncate">
              {[sending.name, left].filter(Boolean).join(' · ')}
            </small>
          )}
        </div>
      )}
      {problems.map((t) => (
        <div key={t.id} className="flex items-center gap-2 min-w-0">
          <AlertCircle size={16} strokeWidth={1.75} className="shrink-0 text-danger" aria-hidden="true" />
          <span className="grid min-w-0 flex-1">
            <b className="text-sm font-medium truncate">{t.name}</b>
            <small className="text-xs text-ink-muted truncate">{t.error}</small>
          </span>
          {t.status === 'failed' && (
            <IconButton size="md" aria-label={`Thử lại ${t.name}`} title="Thử lại" onClick={() => retry(t.id)}>
              <RotateCw size={16} strokeWidth={1.75} />
            </IconButton>
          )}
          <IconButton size="md" aria-label={`Bỏ ${t.name} khỏi danh sách tải lên`} title="Bỏ qua" onClick={() => dismiss(t.id)}>
            <X size={16} strokeWidth={1.75} />
          </IconButton>
        </div>
      ))}
    </section>
  )
}
