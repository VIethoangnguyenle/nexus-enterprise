import { useMemo, useRef, useState } from 'react'
import { CircleAlert, Lock, UserRoundPlus } from 'lucide-react'
import type { Asset, AssetType, Transition } from '../../api/assets'
import {
  useAsset, useAssetHistory, useAssetTransitions, useHandOverAsset, useTransitionAsset,
} from '../../hooks/useAssets'
import { formatFieldValue, parseFields } from '../../lib/asset-fields'
import {
  actionDone, actionLabel, assetPerson, categoryLabel, explainAsset, isFinalAction, statePill, stepSegments,
} from '../../lib/asset-model'
import { statusOf } from '../../lib/errors'
import { formatDate, formatDateTime } from '../../lib/format'
import type { PeopleDirectory } from '../../lib/people'
import { ConfirmDialog } from '../composites/ConfirmDialog'
import { Button, PeoplePicker, PersonChip, toast } from '../primitives'
import { EmptyState } from '../spaces/EmptyState'
import { SidePanel } from '../spaces/SidePanel'
import { AssetPill } from './AssetPill'
import { StepList, type StepItem } from './StepList'

interface AssetDetailPanelProps {
  assetId: string
  /** The list row, shown while the detail loads. */
  summary?: Asset
  people: PeopleDirectory
  types: AssetType[]
  onClose: () => void
}

/**
 * Right panel of Danh sách (mockup §2): state, holder, the type's own fields,
 * the steps the lifecycle allows from here that the caller may take, and the
 * history with who did each step. A hand-over picks the person from a list; no
 * id is typed or shown.
 */
export function AssetDetailPanel({ assetId, summary, people, types, onClose }: AssetDetailPanelProps) {
  const detail = useAsset(assetId)
  const asset = detail.data ?? summary

  if (detail.isError && !detail.data) {
    const gone = [403, 404].includes(statusOf(detail.error) ?? 0)
    return (
      <SidePanel label="Chi tiết tài sản" title="Tài sản" closeLabel="chi tiết" onClose={onClose}>
        <EmptyState
          compact
          icon={gone ? <Lock size={24} strokeWidth={1.75} /> : <CircleAlert size={24} strokeWidth={1.75} />}
          text={
            gone
              ? 'Không mở được tài sản này. Có thể nó không còn nữa hoặc bạn chưa có quyền xem.'
              : 'Không tải được tài sản. Kiểm tra kết nối rồi thử lại.'
          }
          action={!gone ? <Button variant="soft" size="sm" onClick={() => void detail.refetch()}>Thử lại</Button> : undefined}
        />
      </SidePanel>
    )
  }

  const type = asset ? types.find((t) => t.id === asset.type_id) : undefined
  return (
    <SidePanel
      label="Chi tiết tài sản"
      title={asset?.name ?? 'Tài sản'}
      sub={asset ? [asset.type_name, type ? categoryLabel(type.category) : ''].filter(Boolean).join(' · ') : undefined}
      closeLabel="chi tiết"
      onClose={onClose}
    >
      {!asset ? (
        <div className="grid gap-4 px-3 pt-1 pb-3" aria-busy="true" aria-label="Đang tải tài sản">
          <div className="skeleton h-5.5 w-24 rounded-full" />
          <div className="skeleton h-4 w-2/3 rounded-sm" />
          <div className="skeleton h-4 w-1/2 rounded-sm" />
          <div className="skeleton h-20 rounded-sm" />
        </div>
      ) : (
        // Keyed by asset: another asset starts with no picker open and no pending confirmation.
        <Body key={asset.id} asset={asset} type={type} people={people} />
      )}
    </SidePanel>
  )
}

function Body({ asset, type, people }: { asset: Asset; type: AssetType | undefined; people: PeopleDirectory }) {
  const transitions = useAssetTransitions(asset.id)
  const history = useAssetHistory(asset.id)
  const step = useTransitionAsset()
  const handOver = useHandOverAsset()
  const [picking, setPicking] = useState(false)
  const [confirm, setConfirm] = useState<Transition | null>(null)
  // Keep the last step while the dialog fades out, so its text does not blank.
  const lastConfirm = useRef<Transition | null>(null)
  if (confirm) lastConfirm.current = confirm
  const shown = confirm ?? lastConfirm.current

  const holder = asset.assigned_to_user_id ? assetPerson(people, asset.assigned_to_user_id, asset.assigned_to_name) : null
  const fields = useMemo(() => parseFields(type?.fields_schema), [type])
  // "Giao" is the hand-over below, not a plain button: it needs a person.
  const steps = (transitions.data?.transitions ?? []).filter((t) => t.action !== 'assign')
  const canAssign = transitions.data?.canAssign === true

  const run = (t: Transition) => {
    step.mutate(
      { id: asset.id, action: t.action },
      {
        onSuccess: () => { toast(actionDone(t.action)); setConfirm(null) },
        onError: (err) => { toast.error(explainAsset(err, 'cập nhật tài sản')); setConfirm(null) },
      },
    )
  }
  const give = (userId: string) => {
    const who = assetPerson(people, userId)
    handOver.mutate(
      { id: asset.id, assigneeId: userId },
      {
        onSuccess: () => { toast(`Đã giao cho ${who.name}`); setPicking(false) },
        onError: (err) => toast.error(explainAsset(err, 'giao tài sản')),
      },
    )
  }

  const items: StepItem[] = (history.data ?? []).map((r) => ({
    key: r.id,
    actor: assetPerson(people, r.actor_id, r.actor_name),
    segments: stepSegments(r),
    comment: r.comment,
    at: r.created_at,
  }))
  const busy = step.isPending || handOver.isPending

  return (
    <div className="grid gap-5 px-3 pt-1 pb-3">
      <div className="flex flex-wrap items-center gap-2">
        <AssetPill pill={statePill(asset.state)} />
        <span className="text-xs text-ink-muted">Cập nhật {formatDate(asset.updated_at)}</span>
      </div>

      <dl className="grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-2 text-sm m-0">
        <dt className="text-ink-muted">Người giữ</dt>
        <dd className="m-0 min-w-0">
          {holder ? (
            <>
              <PersonChip className="max-w-full" name={holder.name} hueKey={holder.hueKey} avatarUrl={holder.avatarUrl} />
              {holder.role && <span className="block text-xs text-ink-muted">{holder.role}</span>}
            </>
          ) : (
            <span className="text-ink-muted">Chưa giao</span>
          )}
        </dd>
        {fields.map((f) => {
          const v = formatFieldValue(f, asset.custom_fields?.[f.key], people)
          return v ? (
            <FieldRow key={f.key} label={f.title} value={v} />
          ) : null
        })}
        <dt className="text-ink-muted">Nhập kho</dt>
        <dd className="m-0 tnum" title={formatDateTime(asset.created_at)}>{formatDate(asset.created_at)}</dd>
      </dl>

      {(steps.length > 0 || canAssign) && (
        <section className="grid gap-2.5">
          <h3 className="m-0 text-label text-ink-muted">Thao tác</h3>
          <div className="flex flex-wrap gap-2">
            {canAssign && (
              <Button variant="soft" size="sm" disabled={busy} aria-expanded={picking} onClick={() => setPicking((v) => !v)}>
                <UserRoundPlus size={16} strokeWidth={1.75} aria-hidden="true" />
                {asset.state === 'assigned' ? 'Giao cho người khác' : 'Giao tài sản'}
              </Button>
            )}
            {steps.map((t) => (
              <Button
                key={t.action}
                variant="soft"
                size="sm"
                disabled={busy}
                onClick={() => (isFinalAction(t.action) ? setConfirm(t) : run(t))}
              >
                {actionLabel(t.action, t.to_state)}
              </Button>
            ))}
          </div>
          {canAssign && picking && (
            <div className="grid gap-1.5">
              <PeoplePicker
                label="Chọn người nhận"
                people={people.list}
                value={[]}
                exclude={asset.assigned_to_user_id ? new Set([asset.assigned_to_user_id]) : undefined}
                max={1}
                autoFocus
                placeholder="Tìm theo tên hoặc phòng ban"
                onChange={(next) => next[0] && give(next[0].userId)}
              />
              <span className="text-xs text-ink-muted">
                {handOver.isPending ? 'Đang giao…' : 'Chọn một người trong danh sách để giao ngay.'}
              </span>
            </div>
          )}
        </section>
      )}

      <section className="grid gap-2.5">
        <h3 className="m-0 text-label text-ink-muted">Lịch sử</h3>
        {history.isLoading ? (
          <div className="grid gap-3" aria-busy="true" aria-label="Đang tải lịch sử">
            <div className="skeleton h-4 w-full rounded-sm" />
            <div className="skeleton h-4 w-3/4 rounded-sm" />
          </div>
        ) : history.isError ? (
          <p className="m-0 text-sm text-ink-muted">
            {statusOf(history.error) === 403
              ? 'Bạn không có quyền xem lịch sử của tài sản này.'
              : 'Không tải được lịch sử. Kiểm tra kết nối rồi thử lại.'}{' '}
            {statusOf(history.error) !== 403 && (
              <Button variant="link" size="link" onClick={() => void history.refetch()}>Thử lại</Button>
            )}
          </p>
        ) : items.length === 0 ? (
          <p className="m-0 text-sm text-ink-muted">Chưa có bước nào được ghi lại.</p>
        ) : (
          <StepList items={items} label="Lịch sử tài sản" />
        )}
      </section>

      <ConfirmDialog
        open={!!confirm}
        onClose={() => setConfirm(null)}
        onConfirm={() => confirm && run(confirm)}
        title={shown ? `${actionLabel(shown.action, shown.to_state)}?` : ''}
        description={<>“{asset.name}” sẽ chuyển sang <b className="font-semibold text-ink">{shown ? statePill(shown.to_state).label : ''}</b>. Bước này không hoàn tác được.</>}
        confirmLabel={shown ? actionLabel(shown.action, shown.to_state) : 'Xác nhận'}
        confirmVariant="danger"
        loading={step.isPending}
      />
    </div>
  )
}

function FieldRow({ label, value }: { label: string; value: string }) {
  return (
    <>
      <dt className="text-ink-muted">{label}</dt>
      <dd className="m-0 break-words tnum">{value}</dd>
    </>
  )
}
