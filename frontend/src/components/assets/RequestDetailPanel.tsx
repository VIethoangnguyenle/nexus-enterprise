import { Check, CircleAlert, Lock, Package } from 'lucide-react'
import type { AssetRequest, AssetType } from '../../api/assets'
import { useAssetRequest } from '../../hooks/useAssets'
import { assetPerson, requestPill, urgencyPill } from '../../lib/asset-model'
import { statusOf } from '../../lib/errors'
import { formatDateTime } from '../../lib/format'
import type { PeopleDirectory } from '../../lib/people'
import { Button, PersonChip } from '../primitives'
import { EmptyState } from '../spaces/EmptyState'
import { SidePanel } from '../spaces/SidePanel'
import { AssetPill } from './AssetPill'

interface RequestDetailPanelProps {
  requestId: string
  /** The list row, shown while the detail loads. */
  summary?: AssetRequest
  people: PeopleDirectory
  /** The workspace's types, for how many assets of this kind are ready. */
  types: AssetType[]
  busy: boolean
  /** Duyệt (and, where the caller may, giao) — the screen decides what opens. */
  onApprove: (request: AssetRequest) => void
  onReject: (request: AssetRequest) => void
  /** The request was approved without an asset; give it one now. */
  onAssign: (request: AssetRequest) => void
  onClose: () => void
}

/**
 * Right panel of Yêu cầu (mockup §3): who asked, for what, how urgent and why,
 * what became of it, and the decision. What is offered follows what the server
 * says the caller may do with this request (`can_decide`, `can_assign`).
 */
export function RequestDetailPanel({ requestId, summary, people, types, busy, onApprove, onReject, onAssign, onClose }: RequestDetailPanelProps) {
  const detail = useAssetRequest(requestId)
  const request = detail.data ?? summary

  if (detail.isError && !detail.data) {
    const forbidden = [403, 404].includes(statusOf(detail.error) ?? 0)
    return (
      <SidePanel label="Chi tiết yêu cầu" title="Yêu cầu tài sản" closeLabel="chi tiết" onClose={onClose}>
        <EmptyState
          compact
          icon={forbidden ? <Lock size={24} strokeWidth={1.75} /> : <CircleAlert size={24} strokeWidth={1.75} />}
          text={
            forbidden
              ? 'Không mở được yêu cầu này. Có thể nó không còn nữa hoặc bạn chưa có quyền xem.'
              : 'Không tải được yêu cầu. Kiểm tra kết nối rồi thử lại.'
          }
          action={!forbidden ? <Button variant="soft" size="sm" onClick={() => void detail.refetch()}>Thử lại</Button> : undefined}
        />
      </SidePanel>
    )
  }

  const who = request ? assetPerson(people, request.requester_id, request.requester_name) : null
  const type = request ? types.find((t) => t.id === request.type_id) : undefined
  const open = request?.status === 'pending'

  return (
    <SidePanel
      label="Chi tiết yêu cầu"
      title={request ? subjectOf(request) : 'Yêu cầu tài sản'}
      sub={request && (
        <span className="inline-flex items-center gap-1.5">
          <AssetPill pill={requestPill(request.status)} />
          {open && <AssetPill pill={urgencyPill(request.urgency)} />}
        </span>
      )}
      closeLabel="chi tiết"
      onClose={onClose}
      footer={request && (request.can_decide || request.can_assign) ? (
        <div className="flex gap-2 px-4 pt-2 pb-4">
          {request.can_decide && (
            <>
              <Button className="flex-1" disabled={busy} onClick={() => onApprove(request)}>
                <Check size={16} strokeWidth={1.75} aria-hidden="true" />
                {request.can_assign ? 'Duyệt và giao' : 'Duyệt'}
              </Button>
              <Button className="flex-1" variant="soft" disabled={busy} onClick={() => onReject(request)}>Từ chối</Button>
            </>
          )}
          {!request.can_decide && request.status === 'approved' && request.can_assign && (
            <Button className="flex-1" disabled={busy} onClick={() => onAssign(request)}>
              <Package size={16} strokeWidth={1.75} aria-hidden="true" />
              Giao tài sản
            </Button>
          )}
        </div>
      ) : undefined}
    >
      {!request || !who ? (
        <div className="grid gap-4 px-3 pt-1 pb-3" aria-busy="true" aria-label="Đang tải yêu cầu">
          <div className="skeleton h-4 w-2/3 rounded-sm" />
          <div className="skeleton h-4 w-1/2 rounded-sm" />
          <div className="skeleton h-20 rounded-sm" />
        </div>
      ) : (
        <div className="grid gap-5 px-3 pt-1 pb-3">
          <dl className="grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-2 text-sm m-0">
            <dt className="text-ink-muted">Người gửi</dt>
            <dd className="m-0 min-w-0">
              <PersonChip className="max-w-full" name={who.name} hueKey={who.hueKey} avatarUrl={who.avatarUrl} />
              {who.role && <span className="block text-xs text-ink-muted">{who.role}</span>}
            </dd>
            <dt className="text-ink-muted">Loại</dt>
            <dd className="m-0">
              {request.type_name}
              {open && type?.available_count !== undefined && (
                <span className="text-ink-muted"> · {type.available_count} tài sản sẵn sàng</span>
              )}
            </dd>
            <dt className="text-ink-muted">Gửi lúc</dt>
            <dd className="m-0 tnum">{formatDateTime(request.created_at)}</dd>
            {request.approver_name && request.status !== 'pending' && (
              <>
                <dt className="text-ink-muted">{request.status === 'rejected' ? 'Từ chối bởi' : 'Duyệt bởi'}</dt>
                <dd className="m-0">{request.approver_name}</dd>
              </>
            )}
            {request.assigned_asset_name && (
              <>
                <dt className="text-ink-muted">Đã giao</dt>
                <dd className="m-0 break-words">{request.assigned_asset_name}</dd>
              </>
            )}
          </dl>

          {request.justification && (
            <section className="grid gap-2">
              <h3 className="m-0 text-label text-ink-muted">Lý do</h3>
              <p className="m-0 px-3 py-2.5 rounded-md bg-base text-sm leading-relaxed whitespace-pre-line break-words">{request.justification}</p>
            </section>
          )}
          {request.status === 'rejected' && request.approver_comment && (
            <section className="grid gap-2">
              <h3 className="m-0 text-label text-ink-muted">Lý do từ chối</h3>
              <p className="m-0 px-3 py-2.5 rounded-md bg-base text-sm leading-relaxed whitespace-pre-line break-words">{request.approver_comment}</p>
            </section>
          )}
        </div>
      )}
    </SidePanel>
  )
}

/**
 * What the request is about, as the panel's title (mockup §3): the first line of
 * the reason the sender gave, cut to fit, else the type asked for. Requests carry
 * no subject of their own.
 */
export function subjectOf(r: AssetRequest): string {
  const line = (r.justification ?? '').split('\n')[0]?.trim() ?? ''
  if (!line) return r.type_name || 'Yêu cầu tài sản'
  return Array.from(line).length > 60 ? `${Array.from(line).slice(0, 59).join('').trimEnd()}…` : line
}
