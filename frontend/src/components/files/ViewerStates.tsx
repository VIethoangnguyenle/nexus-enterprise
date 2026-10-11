import { useEffect, useState, type ReactNode } from 'react'
import { AlertTriangle, Download, FileX, Lock } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { Button } from '../primitives'

export type StateTone = 'neutral' | 'bad'

/** A centred message in the stage: icon, what happened, what is left to do. */
export function StateBlock({
  icon: Icon, tone = 'neutral', title, text, children, busy,
}: {
  icon: LucideIcon
  tone?: StateTone
  title: string
  text?: string
  children?: ReactNode
  busy?: boolean
}) {
  return (
    <div className="grid place-items-center h-full p-6 text-center" aria-busy={busy || undefined}>
      <div className="grid justify-items-center gap-3 max-w-90">
        <span
          className={`grid place-items-center w-16 h-16 rounded-lg ${tone === 'bad' ? 'bg-danger-wash text-danger' : 'bg-base text-ink-muted'}`}
          aria-hidden="true"
        >
          <Icon size={32} strokeWidth={1.5} />
        </span>
        <h2 className="text-body-strong text-ink m-0">{title}</h2>
        {text && <p className="text-small text-ink-muted m-0">{text}</p>}
        {children}
      </div>
    </div>
  )
}

export function DownloadButton({
  onClick, variant = 'primary', children = 'Tải xuống',
}: { onClick: () => void; variant?: 'primary' | 'ghost'; children?: ReactNode }) {
  return (
    <Button type="button" variant={variant} size="sm" onClick={onClick}>
      <Download size={16} strokeWidth={1.75} aria-hidden="true" />
      {children}
    </Button>
  )
}

export const DeniedState = () => (
  <StateBlock icon={Lock} title="Bạn không có quyền xem tệp này" text="Hỏi người đã chia sẻ để được cấp quyền." />
)

export const GoneState = () => (
  <StateBlock
    icon={FileX}
    title="Tệp này đã bị xoá"
    text="Người tải lên hoặc chủ thư mục đã xoá tệp, nên không còn bản nào để xem hay tải xuống."
  />
)

export const NetworkState = ({ onRetry }: { onRetry: () => void }) => (
  <StateBlock
    icon={AlertTriangle}
    tone="bad"
    title="Không tải được tệp"
    text="Kết nối tới máy chủ bị gián đoạn. Kiểm tra mạng rồi thử lại."
  >
    <Button type="button" variant="soft" size="sm" onClick={onRetry}>Thử lại</Button>
  </StateBlock>
)

/** After 300 ms of waiting, so a fast file does not flash a skeleton. */
export function useAfter(ms: number) {
  const [elapsed, setElapsed] = useState(false)
  useEffect(() => {
    const id = setTimeout(() => setElapsed(true), ms)
    return () => clearTimeout(id)
  }, [ms])
  return elapsed
}

export function SkeletonStage() {
  const show = useAfter(300)
  if (!show) return <div className="h-full" aria-busy="true" />
  return (
    <div className="grid place-items-center h-full p-6" aria-busy="true">
      <div className="w-full max-w-204 aspect-[3/4] max-h-full rounded-md bg-base animate-pulse" />
    </div>
  )
}

/** A bar with no value: the server reports no progress, so it eases toward 92% and waits. */
function PrepareBar() {
  const [filled, setFilled] = useState(false)
  useEffect(() => {
    const id = requestAnimationFrame(() => setFilled(true))
    return () => cancelAnimationFrame(id)
  }, [])
  return (
    <div role="progressbar" aria-label="Đang chuẩn bị bản xem trước" className="w-60 h-1 rounded-full bg-line overflow-hidden">
      <div
        className="h-full bg-accent motion-reduce:transition-none"
        style={{ width: filled ? '92%' : '0%', transition: 'width 20s cubic-bezier(0.1, 0.8, 0.3, 1)' }}
      />
    </div>
  )
}

/** An Office file being converted for the first time. */
export function PreparingState({ label, onDownload }: { label: string; onDownload: () => void }) {
  const slow = useAfter(15_000)
  return (
    <StateBlock
      icon={Download}
      busy
      title="Đang chuẩn bị bản xem trước…"
      text={
        slow
          ? 'Tệp lớn cần lâu hơn một chút. Bạn có thể tải xuống thay vì chờ.'
          : `Lần đầu mở tệp ${label} cần vài giây. Những lần sau sẽ mở ngay.`
      }
    >
      <PrepareBar />
      <DownloadButton variant="ghost" onClick={onDownload}>Tải xuống thay vì chờ</DownloadButton>
    </StateBlock>
  )
}
