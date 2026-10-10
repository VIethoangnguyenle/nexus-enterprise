import type {
  ApprovalAssignment, ApprovalRequest, ApprovalStep, AuditEntry, FormFieldDefinition,
} from '../api/approval'
import { UNKNOWN_PERSON, type PeopleDirectory } from './people'

/**
 * What the approval screens know about requests, steps and people, kept out of
 * the components so each rule is stated once and testable without rendering.
 * Nothing here returns an id for display: every name has a word to fall back on.
 */

// --- Snapshot and submitted form ---

export interface ApprovalSnapshot {
  steps: ApprovalStep[]
  formFields: FormFieldDefinition[]
}

const EMPTY_SNAPSHOT: ApprovalSnapshot = { steps: [], formFields: [] }
const snapshots = new Map<string, ApprovalSnapshot>()

/** The template as it was when the request was made. A damaged or missing snapshot reads as empty. */
export function parseSnapshot(json: string | undefined): ApprovalSnapshot {
  if (!json) return EMPTY_SNAPSHOT
  const hit = snapshots.get(json)
  if (hit) return hit
  let parsed: ApprovalSnapshot = EMPTY_SNAPSHOT
  try {
    const raw = JSON.parse(json) as { steps?: ApprovalStep[] | null; form_fields?: FormFieldDefinition[] | null }
    parsed = {
      steps: [...(raw.steps ?? [])].sort((a, b) => a.step_order - b.step_order),
      formFields: [...(raw.form_fields ?? [])].sort((a, b) => a.field_order - b.field_order),
    }
  } catch {
    parsed = EMPTY_SNAPSHOT
  }
  // Lists repeat the same few snapshots; keep the cache from growing without bound.
  if (snapshots.size > 200) snapshots.clear()
  snapshots.set(json, parsed)
  return parsed
}

/** Submitted form values keyed by field label. Anything unreadable is no values. */
export function parseFormData(json: string | null | undefined): Record<string, string> {
  if (!json) return {}
  try {
    const raw = JSON.parse(json) as unknown
    if (!raw || typeof raw !== 'object' || Array.isArray(raw)) return {}
    return Object.fromEntries(Object.entries(raw).map(([k, v]) => [k, String(v ?? '')]))
  } catch {
    return {}
  }
}

/** The figure a request is about: the first currency field that has a value. */
export function amountOf(fields: FormFieldDefinition[], data: Record<string, string>): number | null {
  const field = fields.find((f) => f.field_type === 'currency' && (data[f.label] ?? '').trim() !== '')
  if (!field) return null
  const n = Number(data[field.label])
  return Number.isFinite(n) ? n : null
}

export function amountOfRequest(req: ApprovalRequest): number | null {
  return amountOf(parseSnapshot(req.template_snapshot).formFields, parseFormData(req.form_data_json))
}

/** The step a request currently waits on, when the snapshot knows it. */
export function stepOf(req: ApprovalRequest): ApprovalStep | undefined {
  return parseSnapshot(req.template_snapshot).steps.find((s) => s.step_order === req.current_step)
}

export const requestTitle = (req: ApprovalRequest) => req.template_name || 'Đề nghị'

// --- Labels ---

const ENTITY_LABELS: Record<string, string> = {
  purchase: 'Mua sắm',
  expense: 'Chi phí',
  leave: 'Nghỉ phép',
  hiring: 'Tuyển dụng',
  contract: 'Hợp đồng',
  custom: 'Khác',
}
/** The kinds of request a template can be for. */
export const ENTITY_TYPES = Object.keys(ENTITY_LABELS)

/** A request kind in words. A type the screen was not taught reads "Khác", never its code. */
export const entityLabel = (type: string) => ENTITY_LABELS[type] ?? 'Khác'

const FIELD_TYPES: Record<string, string> = {
  text: 'Văn bản',
  number: 'Số',
  currency: 'Số tiền',
  date: 'Ngày',
  select: 'Danh sách chọn',
  textarea: 'Văn bản dài',
}
/** A form field type in words. */
export const fieldTypeLabel = (type: string) => FIELD_TYPES[type] ?? 'Văn bản'

let uid = 1
/** A key for a row the builder shows before it has any identity of its own. Never sent anywhere. */
export const newUid = () => uid++

export type ApproverType = 'specific_user' | 'role_in_dept' | 'department'

/** The ways a step can name its approver. The server resolves exactly these. */
export const APPROVER_KINDS: { type: ApproverType; label: string; hint: string }[] = [
  { type: 'specific_user', label: 'Một người', hint: 'Đúng người này duyệt' },
  { type: 'role_in_dept', label: 'Vai trò', hint: 'Người giữ vai trò này duyệt' },
  { type: 'department', label: 'Phòng ban', hint: 'Phòng ban này duyệt' },
]

// --- Status pills ---

export type PillTone = 'wait' | 'ok' | 'bad' | 'idle'
export interface Pill { tone: PillTone; label: string }

interface RowLike { request: ApprovalRequest; assignment?: ApprovalAssignment }

/** The status of a request as a row shows it: always a word, never only a colour. */
export function requestPill({ request, assignment }: RowLike): Pill {
  switch (request.status) {
    case 'approved':
      return { tone: 'ok', label: 'Đã duyệt' }
    case 'rejected':
      return { tone: 'bad', label: 'Trả lại' }
    case 'cancelled':
      return { tone: 'idle', label: 'Đã huỷ' }
    default: {
      if (assignment?.status === 'pending' && assignment.step_order === request.current_step) {
        return { tone: 'wait', label: 'Chờ bạn' }
      }
      const step = stepOf(request)
      return { tone: 'idle', label: step?.name ? `Chờ ${step.name}` : 'Đang chờ duyệt' }
    }
  }
}

/** One approver's status in the chain. */
export function assignmentPill(a: ApprovalAssignment, currentStep: number, requestStatus: string): Pill {
  switch (a.status) {
    case 'approved':
      return { tone: 'ok', label: 'Đã duyệt' }
    case 'rejected':
      return { tone: 'bad', label: 'Trả lại' }
    case 'skipped':
      return { tone: 'idle', label: 'Bỏ qua' }
    case 'revoked':
      return { tone: 'idle', label: 'Hết quyền duyệt' }
    default:
      if (requestStatus !== 'pending') return { tone: 'idle', label: 'Bỏ qua' }
      return a.step_order <= currentStep
        ? { tone: 'wait', label: 'Đang chờ' }
        : { tone: 'idle', label: 'Chưa tới' }
  }
}

// --- People ---

export interface PersonView {
  name: string
  /** Colour key (user id when the directory knows the person). Never rendered. */
  hueKey: string
  avatarUrl: string
  /** "Chuyên viên · Vận hành thanh toán", when the directory knows it. */
  role: string
}

/**
 * A person as the screen shows them: the server's name first (it covers people
 * outside the contacts list), then the directory, then a neutral word.
 */
export function personFor(people: PeopleDirectory, nodeId: string, serverName?: string): PersonView {
  const known = nodeId ? people.byNodeId.get(nodeId) : undefined
  return {
    name: serverName?.trim() || known?.name || UNKNOWN_PERSON,
    hueKey: known?.userId || nodeId,
    avatarUrl: known?.avatarUrl ?? '',
    role: known?.role ?? '',
  }
}

// --- Chain ---

export interface ChainItem {
  key: string
  name: string
  hueKey: string
  avatarUrl: string
  /** The step this approver sits on ("Trưởng phòng"). */
  stepName: string
  pill: Pill
  isMe: boolean
  /** When they acted. */
  actedAt?: string
  /** For the step now waiting: since when. */
  since?: string
  comment?: string
  assignmentId?: string
}

interface ChainInput {
  request: ApprovalRequest
  steps: ApprovalStep[] | null | undefined
  assignments: ApprovalAssignment[] | null | undefined
  meNodeId: string | undefined
  /** The server's word that it is the viewer's turn (also through a role or department). */
  canAct?: boolean
  people?: PeopleDirectory
}

const NO_ONE = 'Người duyệt'

/** Display name of whoever (or whatever role) an assignment is for. */
function assigneeName(a: ApprovalAssignment, people?: PeopleDirectory): string {
  const raw = a.user_name || people?.byNodeId.get(a.user_node_id)?.name
  if (!raw) return NO_ONE
  return raw
}

/**
 * The approval chain, step by step: one entry per approver who holds an
 * assignment, and one placeholder for a step nobody has been assigned to yet.
 * Without the steps (an unreadable snapshot) the assignments alone are listed.
 */
/**
 * A role's or department's own row: its user_node_id is the group named by its
 * grant source. A person who acted through the group has a row of their own
 * under the same grant source.
 */
export function isGroupRow(a: ApprovalAssignment): boolean {
  const [, ua] = a.grant_source.split(':')
  return !!ua && ua === a.user_node_id
}

export function chainOf({ request, steps, assignments, meNodeId, canAct, people }: ChainInput): ChainItem[] {
  const all = assignments ?? []
  const byStep = new Map<number, ApprovalAssignment[]>()
  for (const a of all) byStep.set(a.step_order, [...(byStep.get(a.step_order) ?? []), a])

  const order = steps?.length
    ? steps.map((s) => ({ order: s.step_order, step: s as ApprovalStep | undefined }))
    : [...byStep.keys()].sort((a, b) => a - b).map((o) => ({ order: o, step: undefined }))

  const items: ChainItem[] = []
  let previousActed: string | undefined = request.created_at
  for (const { order: stepOrder, step } of order) {
    const here = byStep.get(stepOrder) ?? []
    const stepName = step?.name || `Bước ${stepOrder}`
    if (here.length === 0) {
      const raw = step?.approver_name
      items.push({
        key: `step-${stepOrder}`,
        name: raw || NO_ONE,
        hueKey: step?.approver_value || `step-${stepOrder}`,
        avatarUrl: '',
        stepName,
        pill: request.status === 'pending' ? { tone: 'idle', label: 'Chưa tới' } : { tone: 'idle', label: 'Bỏ qua' },
        isMe: false,
      })
      continue
    }
    let latestActed: string | undefined
    // Once people have acted for a group and the step has moved on, the group's
    // own row (skipped) only repeats them; while it is open it shows who is awaited.
    const acted = here.some((a) => !isGroupRow(a))
    for (const a of here) {
      if (isGroupRow(a) && a.status !== 'pending' && acted) continue
      const known = people?.byNodeId.get(a.user_node_id)
      if (a.acted_at && (!latestActed || a.acted_at > latestActed)) latestActed = a.acted_at
      items.push({
        key: a.id,
        name: assigneeName(a, people),
        hueKey: known?.userId || a.user_node_id,
        avatarUrl: known?.avatarUrl ?? '',
        stepName,
        pill: assignmentPill(a, request.current_step, request.status),
        isMe: (!!meNodeId && a.user_node_id === meNodeId) || (!!canAct && isGroupRow(a) && a.status === 'pending' && a.step_order === request.current_step),
        actedAt: a.acted_at ?? undefined,
        since: a.status === 'pending' && a.step_order === request.current_step ? previousActed : undefined,
        comment: a.comment || undefined,
        assignmentId: a.id,
      })
    }
    previousActed = latestActed ?? previousActed
  }
  return items
}

// --- Audit ---

export interface AuditLine {
  /** Reads after the actor's name ("đã duyệt"), or alone for a system entry. */
  text: string
  /** No person did this; the server did. */
  system: boolean
  comment?: string
}

function detailOf(entry: Pick<AuditEntry, 'detail_json'>): Record<string, string> {
  return parseFormData(entry.detail_json)
}

/** An audit entry as a sentence. A code the screen was not taught still reads as one. */
export function auditLine(entry: Pick<AuditEntry, 'action' | 'detail_json'>): AuditLine {
  const detail = detailOf(entry)
  const comment = detail.comment?.trim() || undefined
  switch (entry.action) {
    case 'created':
      return { text: 'đã tạo đề nghị', system: false }
    case 'approved':
      return { text: 'đã duyệt', system: false, ...(comment ? { comment } : {}) }
    case 'rejected':
      return { text: 'đã trả lại', system: false, ...(comment ? { comment } : {}) }
    case 'assigned':
      return { text: 'được giao duyệt', system: false }
    case 'step_advanced':
      return { text: 'Chuyển sang bước tiếp theo', system: true }
    case 'completed':
      return {
        text: detail.final_status === 'rejected' ? 'Đề nghị đã bị trả lại' : 'Đề nghị đã được duyệt',
        system: true,
      }
    default:
      return { text: 'đã cập nhật đề nghị', system: false }
  }
}
