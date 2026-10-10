import type { HistoryRecord, RequestStatus, Urgency } from '../api/assets'
import { ApiError } from '../api/client'
import { explain } from './errors'
import { formatDate, formatTime, toDate } from './format'
import { UNKNOWN_PERSON, type PeopleDirectory } from './people'

/**
 * What the asset screens know about states, steps, requests and people, kept
 * out of the components so each rule is stated once and testable without
 * rendering. Nothing here returns a code for display: every state, step,
 * category and person has a word to fall back on.
 */

// --- States (the backend lifecycle: backend/services/asset/internal/domain/lifecycle.go) ---

export const ASSET_STATES = ['requested', 'available', 'assigned', 'maintenance', 'retired', 'disposed'] as const

const STATE_LABEL: Record<string, string> = {
  requested: 'Chờ duyệt',
  available: 'Sẵn sàng',
  assigned: 'Đang giao',
  maintenance: 'Bảo trì',
  retired: 'Ngừng dùng',
  disposed: 'Đã thanh lý',
}

/** A state in words. One the screen was not taught reads "Khác", never its code. */
export const stateLabel = (state: string | undefined) => STATE_LABEL[state ?? ''] ?? 'Khác'

export type AssetTone = 'wait' | 'ok' | 'bad' | 'idle' | 'info' | 'acc'
export interface AssetPillData { tone: AssetTone; label: string }

const STATE_TONE: Record<string, AssetTone> = {
  requested: 'acc',
  available: 'ok',
  assigned: 'info',
  maintenance: 'wait',
  retired: 'idle',
  disposed: 'idle',
}

export const statePill = (state: string | undefined): AssetPillData => ({
  tone: STATE_TONE[state ?? ''] ?? 'idle',
  label: stateLabel(state),
})

/** Swatch class for a state in the dashboard legend. Colour is never the only signal: a label sits beside it. */
export const STATE_SWATCH: Record<string, string> = {
  requested: 'bg-accent',
  available: 'bg-success',
  assigned: 'bg-info',
  maintenance: 'bg-warning',
  retired: 'bg-ink-subtle',
  disposed: 'bg-line',
}

// --- Steps (lifecycle actions) ---

const ACTION_LABEL: Record<string, string> = {
  approve: 'Duyệt nhập kho',
  assign: 'Giao tài sản',
  return: 'Thu hồi',
  flag_maintenance: 'Đưa đi bảo trì',
  complete_maintenance: 'Hoàn tất bảo trì',
  retire: 'Ngừng dùng',
  dispose: 'Thanh lý',
}

/**
 * A lifecycle step as a verb on a button. A step the screen was not taught is
 * named by where it leads, and failing that by "Cập nhật", never as written.
 */
export function actionLabel(action: string, toState?: string): string {
  const known = ACTION_LABEL[action]
  if (known) return known
  return toState && STATE_LABEL[toState] ? `Chuyển sang ${STATE_LABEL[toState]}` : 'Cập nhật'
}

const RIGHT_LABEL: Record<string, string> = { read: 'Xem', write: 'Sửa', approve: 'Duyệt', manage: 'Quản lý', upload: 'Tải lên', share: 'Chia sẻ', invite: 'Mời' }

export interface LifecycleNote { right: string; steps: string[] }

/** Which steps of a lifecycle need which right, in words ("Quản lý: Giao tài sản, Thu hồi"). */
export function lifecycleNotes(transitions: { from_state?: string; operation: string; to_state: string; ngac_permission: string }[]): LifecycleNote[] {
  const byRight = new Map<string, string[]>()
  for (const t of transitions) {
    const right = RIGHT_LABEL[t.ngac_permission] ?? 'Quản lý'
    const steps = byRight.get(right) ?? []
    const label = actionLabel(t.operation, t.to_state)
    if (!steps.includes(label)) steps.push(label)
    byRight.set(right, steps)
  }
  return [...byRight].map(([right, steps]) => ({ right, steps }))
}

const ACTION_DONE: Record<string, string> = {
  approve: 'Đã duyệt nhập kho',
  return: 'Đã thu hồi tài sản',
  flag_maintenance: 'Đã đưa đi bảo trì',
  complete_maintenance: 'Đã hoàn tất bảo trì',
  retire: 'Đã cho ngừng dùng',
  dispose: 'Đã thanh lý',
}

/** The line shown once a step has been taken. */
export const actionDone = (action: string) => ACTION_DONE[action] ?? 'Đã cập nhật tài sản'

/** Steps that cannot be taken back, which ask first. */
export const isFinalAction = (action: string) => action === 'retire' || action === 'dispose'

// --- History as a sentence ---

/** A piece of a sentence: plain text, a name in bold, or a status pill. */
export type Segment = string | { b: string } | { pill: AssetPillData }

/**
 * What a lifecycle step did, as the part of the sentence after the actor's name.
 * `asset` names the asset in a feed that mixes several; in an asset's own
 * history it is left out. The person a step concerned is named by the server;
 * when it could not, a neutral word stands in, never an id.
 */
export function stepSegments(r: HistoryRecord, opts: { asset?: string } = {}): Segment[] {
  if (r.action === 'request_approved' || r.action === 'request_rejected') {
    const by = r.subject_name?.trim() || UNKNOWN_PERSON
    return [r.action === 'request_approved' ? 'đã duyệt yêu cầu ' : 'đã từ chối yêu cầu ', { b: opts.asset ?? 'tài sản' }, ' của ', { b: by }]
  }
  const who = r.subject_name?.trim() || (r.subject_user_id ? UNKNOWN_PERSON : '')
  const asset: Segment[] = opts.asset ? [' ', { b: opts.asset }] : []
  switch (r.action) {
    case 'assign':
      if (opts.asset) return who ? ['đã giao', ...asset, ' cho ', { b: who }] : ['đã giao', ...asset]
      return who ? ['đã giao cho ', { b: who }] : ['đã giao tài sản']
    case 'return':
      return who ? ['đã thu hồi', ...asset, ' từ ', { b: who }] : ['đã thu hồi', ...asset]
    case 'flag_maintenance':
      return ['đã đưa', ...asset, ' đi bảo trì']
    case 'complete_maintenance':
      return ['đã hoàn tất bảo trì', ...asset, ', chuyển sang ', { pill: statePill(r.to_state) }]
    case 'approve':
      return ['đã duyệt nhập kho', ...asset]
    case 'retire':
      return ['đã cho', ...asset, ' ngừng dùng']
    case 'dispose':
      return ['đã thanh lý', ...asset]
    default:
      return STATE_LABEL[r.to_state] ? ['đã chuyển', ...asset, ' sang ', { pill: statePill(r.to_state) }] : ['đã cập nhật', ...asset]
  }
}

// --- Requests ---

const REQUEST_PILL: Record<string, AssetPillData> = {
  pending: { tone: 'wait', label: 'Đang chờ' },
  approved: { tone: 'ok', label: 'Đã duyệt, chờ giao' },
  fulfilled: { tone: 'ok', label: 'Đã giao' },
  rejected: { tone: 'bad', label: 'Từ chối' },
}

export const requestPill = (status: RequestStatus | string | undefined): AssetPillData =>
  REQUEST_PILL[status ?? ''] ?? { tone: 'idle', label: 'Khác' }

const URGENCY_LABEL: Record<string, string> = { low: 'Thấp', normal: 'Bình thường', high: 'Cao', urgent: 'Khẩn' }
export const URGENCIES: Urgency[] = ['low', 'normal', 'high', 'urgent']

export const urgencyLabel = (u: string | undefined) => URGENCY_LABEL[u ?? ''] ?? URGENCY_LABEL.normal!

export function urgencyPill(u: string | undefined): AssetPillData {
  const tone: AssetTone = u === 'urgent' ? 'bad' : u === 'high' ? 'wait' : 'idle'
  return { tone, label: urgencyLabel(u) }
}

export type RequestFilter = 'pending' | 'approved' | 'rejected' | 'all' | 'mine'

/** The filters over the request table, in the order the mockup shows them. */
export const REQUEST_FILTERS: { id: RequestFilter; label: string }[] = [
  { id: 'pending', label: 'Đang chờ' },
  { id: 'approved', label: 'Đã duyệt' },
  { id: 'rejected', label: 'Từ chối' },
  { id: 'all', label: 'Tất cả' },
  { id: 'mine', label: 'Tôi đã gửi' },
]

/** What to ask the server for, for a filter. "Đã duyệt" covers approved requests and the ones already given an asset. */
export function requestFilterStatuses(f: RequestFilter): { status?: string; mine?: boolean } {
  switch (f) {
    case 'pending': return { status: 'pending' }
    case 'approved': return { status: 'approved,fulfilled' }
    case 'rejected': return { status: 'rejected' }
    case 'mine': return { mine: true }
    default: return {}
  }
}

// --- Categories ---

const CATEGORY_LABEL: Record<string, string> = {
  hardware: 'Phần cứng',
  software: 'Phần mềm',
  license: 'Giấy phép',
  furniture: 'Nội thất',
  other: 'Khác',
}

/** The categories a new type can be filed under. The keys are what the server stores; they are never shown. */
export const CATEGORIES = Object.keys(CATEGORY_LABEL)

/** A category in Vietnamese; one somebody typed by hand (through the API) is shown as typed. */
export const categoryLabel = (category: string) => CATEGORY_LABEL[category] ?? category

// --- People ---

export interface AssetPersonView {
  name: string
  /** Colour key (user id). Never rendered. */
  hueKey: string
  avatarUrl: string
  role: string
}

/**
 * A person on an asset screen. Asset records name people by user id; the server's
 * name (read inside this workspace) comes first, then the directory, then a
 * neutral word. An id is never a fallback.
 */
export function assetPerson(people: PeopleDirectory, userId: string | undefined, serverName?: string): AssetPersonView {
  const known = userId ? people.byUserId.get(userId) : undefined
  return {
    name: serverName?.trim() || known?.name || UNKNOWN_PERSON,
    hueKey: userId || serverName || UNKNOWN_PERSON,
    avatarUrl: known?.avatarUrl ?? '',
    role: known?.role ?? '',
  }
}

// --- Time ---

/**
 * When a step happened, for history and the feed: `09:20` today, `Hôm qua 16:10`,
 * `08/10 10:02` this year, `31/12/2025 08:05` before. The full date and time go
 * in the tooltip (`formatDateTime`).
 */
export function formatStamp(value: unknown, now: Date = new Date()): string {
  const d = toDate(value)
  if (!d) return ''
  const day = (x: Date) => new Date(x.getFullYear(), x.getMonth(), x.getDate()).getTime()
  const diff = Math.round((day(now) - day(d)) / 86_400_000)
  if (diff === 0) return formatTime(d)
  if (diff === 1) return `Hôm qua ${formatTime(d)}`
  const date = formatDate(d)
  return d.getFullYear() === now.getFullYear() ? `${date.slice(0, 5)} ${formatTime(d)}` : `${date} ${formatTime(d)}`
}

// --- Why an action failed ---

const REASON_TEXT: Record<string, string> = {
  request_not_open: 'Yêu cầu này vừa được xử lý. Tải lại trang để xem kết quả.',
  asset_unavailable: 'Tài sản này vừa được giao hoặc không còn Sẵn sàng. Chọn tài sản khác.',
  state_changed: 'Tài sản vừa thay đổi trong lúc bạn thao tác. Tải lại trang rồi làm lại.',
  not_a_member: 'Người này không còn là thành viên của không gian làm việc. Chọn người khác.',
  wrong_type: 'Tài sản này không thuộc loại được yêu cầu. Chọn tài sản khác.',
  same_holder: 'Người này đang giữ tài sản này rồi. Chọn người khác.',
}

/**
 * A sentence for a failed asset action. The server says which of several
 * refusals it was (`reason`), because a 409 for "already decided" and a 409 for
 * "just given away" ask for different things; without one, the general sentence
 * for the status.
 */
export function explainAsset(err: unknown, action: string): string {
  const reason = err instanceof ApiError ? (err.body as { reason?: string } | undefined)?.reason : undefined
  return (reason && REASON_TEXT[reason]) || explain(err, action)
}
