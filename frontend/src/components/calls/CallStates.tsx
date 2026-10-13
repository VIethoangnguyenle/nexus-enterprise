import type { ReactNode } from 'react'
import { CircleAlert, Info, WifiOff } from 'lucide-react'
import { formatElapsed } from '../../lib/call-model'
import type { CallSummary } from '../../stores/call.store'
import { Button, Pressable } from '../primitives'
import './call-stage.css'

/** A state on the light page (ended, elsewhere, removed). */
function LightCenter({ children }: { children: ReactNode }) {
  return <div className="cs-center"><div className="cs-state cs-state-light">{children}</div></div>
}

/** A state on the dark stage (connecting, failed). */
function StageCenter({ children }: { children: ReactNode }) {
  return <div className="cs-center cs-center-stage"><div className="cs-state cs-state-stage">{children}</div></div>
}

export function ConnectingState({ onCancel }: { onCancel: () => void }) {
  return (
    <StageCenter>
      <div className="cs-spin" aria-hidden="true" />
      <h2 role="status">Đang kết nối…</h2>
      <p className="cs-sub">Đang vào cuộc gọi.</p>
      <Button variant="soft" className="justify-self-start" onClick={onCancel}>Huỷ</Button>
    </StageCenter>
  )
}

/** Over the stage while the link recovers; the room keeps the seat for 30 seconds. */
export function ReconnectingOverlay({ onLeave }: { onLeave: () => void }) {
  return (
    <div className="cs-overlay" role="alertdialog" aria-labelledby="cs-reconnect-title">
      <div className="cs-state cs-state-stage">
        <div className="cs-spin" aria-hidden="true" />
        <h2 id="cs-reconnect-title">Đang kết nối lại…</h2>
        <p className="cs-sub">Mạng vừa chập chờn. Cuộc gọi vẫn giữ chỗ cho bạn trong 30 giây.</p>
        <Button variant="soft" className="justify-self-start" onClick={onLeave}>Rời cuộc gọi</Button>
      </div>
    </div>
  )
}

export function FailedState({ onRetry, onLeave }: { onRetry: () => void; onLeave: () => void }) {
  return (
    <StageCenter>
      <WifiOff className="cs-icon-l" style={{ color: 'oklch(85% 0.12 75)' }} aria-hidden="true" />
      <h2>Không kết nối được tới máy chủ cuộc gọi</h2>
      <p className="cs-sub">
        Mạng của bạn có thể đang chặn cuộc gọi (thường gặp ở mạng công ty). Thử mạng khác hoặc báo quản trị.
      </p>
      <div className="cs-actions">
        <Button variant="primary" onClick={onRetry}>Thử lại</Button>
        <Button variant="soft" onClick={onLeave}>Rời cuộc gọi</Button>
      </div>
    </StageCenter>
  )
}

export function EndedSummary({ summary, messages, onBack, onRejoin }: {
  summary: CallSummary
  messages: number
  onBack: () => void
  onRejoin: () => void
}) {
  return (
    <LightCenter>
      <h2>Cuộc gọi đã kết thúc</h2>
      <div className="cs-sum">
        <div><b>{formatElapsed(summary.durationMs)}</b><small>Thời lượng</small></div>
        <div><b>{summary.people}</b><small>Người tham gia</small></div>
        <div><b>{messages}</b><small>Tin trong lúc gọi</small></div>
      </div>
      <div className="cs-actions">
        <Button variant="primary" onClick={onBack}>Về cuộc trò chuyện</Button>
        <Button variant="soft" onClick={onRejoin}>Vào lại</Button>
      </div>
    </LightCenter>
  )
}

/** You were taken out of the room because you are no longer in the group. */
export function RemovedState({ onBack }: { onBack: () => void }) {
  return (
    <LightCenter>
      <div className="cs-banner cs-banner-danger" role="alert">
        <CircleAlert className="cs-icon" aria-hidden="true" />
        <span><b>Bạn không còn ở trong nhóm này</b>Cuộc gọi đã ngắt. Người quản trị nhóm có thể mời lại bạn.</span>
      </div>
      <div className="cs-actions"><Button variant="primary" onClick={onBack}>Về danh sách trò chuyện</Button></div>
    </LightCenter>
  )
}

/** Opening a call while another is running. */
export function OtherCallNotice({ currentTitle, onSwitch, onStay }: { currentTitle: string; onSwitch: () => void; onStay: () => void }) {
  return (
    <LightCenter>
      <div className="cs-banner cs-banner-info">
        <Info className="cs-icon" aria-hidden="true" />
        <span><b>Bạn đang ở trong một cuộc gọi khác</b>{`Rời "${currentTitle}" để vào cuộc gọi này?`}</span>
      </div>
      <div className="cs-actions">
        <Button variant="primary" onClick={onSwitch}>Rời và vào</Button>
        <Button variant="soft" onClick={onStay}>Ở lại</Button>
      </div>
    </LightCenter>
  )
}

/** Opening a call that is already over, or that no longer exists. */
export function CallOverState({ onBack }: { onBack: () => void }) {
  return (
    <LightCenter>
      <h2>Cuộc gọi đã kết thúc</h2>
      <p className="cs-sub">Không còn ai trong cuộc gọi này. Bạn có thể bắt đầu một cuộc gọi mới từ cuộc trò chuyện.</p>
      <div className="cs-actions"><Button variant="primary" onClick={onBack}>Về cuộc trò chuyện</Button></div>
    </LightCenter>
  )
}

export function NoAccessState({ onBack }: { onBack: () => void }) {
  return (
    <LightCenter>
      <h2>Bạn không ở trong cuộc trò chuyện này</h2>
      <p className="cs-sub">Nhờ một thành viên thêm bạn vào nhóm rồi vào lại cuộc gọi.</p>
      <div className="cs-actions"><Button variant="primary" onClick={onBack}>Về danh sách trò chuyện</Button></div>
    </LightCenter>
  )
}

/** Safari holds sound back until a tap (an installed iPhone app especially). */
export function AudioBlockedButton({ onTap }: { onTap: () => void }) {
  return (
    <Pressable className="cs-chip cs-sound" onClick={onTap}>
      Chạm để bật âm thanh
    </Pressable>
  )
}
