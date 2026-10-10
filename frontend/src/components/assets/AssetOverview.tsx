import { useMemo } from 'react'
import { Link } from '@tanstack/react-router'
import { CircleAlert, ClipboardList, Package, Plus, Users, Wrench } from 'lucide-react'
import type { ActivityEntry, AssetRequest } from '../../api/assets'
import { useArrivals } from '../../hooks/useArrivals'
import { ASSET_STATES, STATE_SWATCH, assetPerson, stateLabel, stepSegments, urgencyPill } from '../../lib/asset-model'
import { filterSearch, tabSearch, type AssetsSearch } from '../../lib/assets-search'
import { formatListTime } from '../../lib/format'
import type { PeopleDirectory } from '../../lib/people'
import { useAuthStore } from '../../stores/auth.store'
import { Button, Pressable } from '../primitives'
import { EmptyState } from '../spaces/EmptyState'
import { AssetPill } from './AssetPill'
import { StepList, type StepItem } from './StepList'

export interface Summary {
  total: number
  byState: Record<string, number>
  byType: { typeId: string; name: string; count: number }[]
  holders: number
  maintenanceOverdue: number
}

interface AssetOverviewProps {
  search: AssetsSearch
  summary: Summary | undefined
  summaryLoading: boolean
  summaryFailed: boolean
  onRetry: () => void
  activity: ActivityEntry[] | undefined
  activityLoading: boolean
  /** Requests waiting on this caller's decision (shown only to people who may decide). */
  waiting: AssetRequest[]
  /** Pending requests the caller can see, for the card. */
  pendingCount: number | undefined
  urgentCount: number
  canApprove: boolean
  canManage: boolean
  people: PeopleDirectory
  onOpenRequest: (id: string) => void
  onNewType: () => void
}

/**
 * Tổng quan (mockup §1): four figures that each lead to the matching list, the
 * spread over states and types, requests waiting for the caller and recent
 * activity with who did it. The figures stay skeletons until the data is in —
 * a "0" is never shown for "not loaded".
 */
export function AssetOverview(p: AssetOverviewProps) {
  const { summary, search } = p

  if (p.summaryFailed) {
    return (
      <EmptyState
        icon={<CircleAlert size={24} strokeWidth={1.75} />}
        text="Không tải được tổng quan tài sản. Kiểm tra kết nối rồi thử lại."
        action={<Button variant="soft" size="sm" onClick={p.onRetry}>Thử lại</Button>}
      />
    )
  }
  if (summary && summary.total === 0 && summary.byType.length === 0 && !p.activity?.length) {
    return (
      <EmptyState
        icon={<Package size={24} strokeWidth={1.75} />}
        text={p.canManage
          ? 'Chưa có tài sản nào. Thêm loại tài sản để bắt đầu theo dõi thiết bị, giấy phép và nội thất.'
          : 'Chưa có tài sản nào trong phạm vi bạn xem được.'}
        action={p.canManage ? (
          <Button size="sm" onClick={p.onNewType}>
            <Plus size={16} strokeWidth={1.75} aria-hidden="true" />
            Thêm loại tài sản
          </Button>
        ) : undefined}
      />
    )
  }

  const by = summary?.byState ?? {}
  const urgent = p.urgentCount
  const cards: { key: string; icon: typeof Package; label: string; value: number | undefined; note: string; to: AssetsSearch }[] = [
    { key: 'total', icon: Package, label: 'Tổng tài sản', value: summary?.total, note: summary ? `${summary.byType.length} loại` : '', to: tabSearch(search, 'list') },
    { key: 'assigned', icon: Users, label: 'Đang giao', value: summary ? by.assigned ?? 0 : undefined, note: summary ? `cho ${summary.holders} người` : '', to: filterSearch(tabSearch(search, 'list'), { state: 'assigned' }) },
    { key: 'requests', icon: ClipboardList, label: 'Yêu cầu chờ duyệt', value: p.pendingCount, note: p.pendingCount === undefined ? '' : urgent > 0 ? `${urgent} khẩn` : '', to: tabSearch(search, 'requests') },
    { key: 'maintenance', icon: Wrench, label: 'Đang bảo trì', value: summary ? by.maintenance ?? 0 : undefined, note: summary && summary.maintenanceOverdue > 0 ? `${summary.maintenanceOverdue} quá 14 ngày` : '', to: filterSearch(tabSearch(search, 'list'), { state: 'maintenance' }) },
  ]

  return (
    <div className="grid gap-3 px-5 pb-6">
      <div className="grid grid-cols-2 @3xl:grid-cols-4 gap-3">
        {cards.map((c) => (
          <Link
            key={c.key}
            to="/assets"
            search={c.to}
            className="grid gap-1.5 p-4 rounded-surface bg-raised no-underline text-ink focus-ring
              transition-colors duration-quick hover:bg-hover"
          >
            <span className="flex items-center gap-1.5 text-sm text-ink-muted">
              <c.icon size={16} strokeWidth={1.75} aria-hidden="true" />
              {c.label}
            </span>
            {c.value === undefined ? (
              <span className="skeleton h-8.5 w-16 rounded-sm" aria-busy="true" aria-label={`Đang tải ${c.label.toLocaleLowerCase('vi')}`} />
            ) : (
              <span className="font-display text-2xl font-semibold tnum">{c.value}</span>
            )}
            <span className="text-xs text-ink-muted min-h-4">{c.note}</span>
          </Link>
        ))}
      </div>

      <div className="grid gap-3 @3xl:grid-cols-[minmax(0,1.4fr)_minmax(0,1fr)] items-start">
        <Spread summary={summary} loading={p.summaryLoading} />
        <div className="grid gap-3">
          {p.canApprove && (
            <section className="grid gap-3 p-4 rounded-surface bg-raised" aria-label="Chờ bạn duyệt">
              <div className="flex items-center gap-2">
                <h2 className="m-0 font-display text-lg font-bold">Chờ bạn duyệt</h2>
                <Link to="/assets" search={tabSearch(search, 'requests')} className="ml-auto text-sm font-semibold text-accent no-underline focus-ring hover:underline">
                  Xem tất cả
                </Link>
              </div>
              {p.waiting.length === 0 ? (
                <p className="m-0 text-sm text-ink-muted">Không còn yêu cầu nào chờ bạn.</p>
              ) : (
                <ul className="grid m-0 p-0 list-none">
                  {p.waiting.slice(0, 3).map((r) => {
                    const who = assetPerson(p.people, r.requester_id, r.requester_name)
                    return (
                      <li key={r.id} className="border-b border-line last:border-b-0">
                        <Pressable
                          onClick={() => p.onOpenRequest(r.id)}
                          className="w-full grid grid-cols-[minmax(0,1fr)_auto] gap-3 items-center py-2.5 rounded-md px-1
                            transition-colors duration-quick hover:bg-hover"
                        >
                          <span className="grid min-w-0">
                            <span className="truncate font-medium text-ink">{r.type_name}</span>
                            <small className="text-xs text-ink-muted truncate">{who.name} · {formatListTime(r.created_at)}</small>
                          </span>
                          <AssetPill pill={urgencyPill(r.urgency)} />
                        </Pressable>
                      </li>
                    )
                  })}
                </ul>
              )}
            </section>
          )}
          <Activity entries={p.activity} loading={p.activityLoading} people={p.people} />
        </div>
      </div>
    </div>
  )
}

function Spread({ summary, loading }: { summary: Summary | undefined; loading: boolean }) {
  const states = ['assigned', 'available', 'maintenance', 'requested', 'retired', 'disposed'].filter(
    (s) => (ASSET_STATES as readonly string[]).includes(s) && (summary?.byState[s] ?? 0) > 0,
  )
  const total = summary?.total ?? 0
  const top = Math.max(1, ...(summary?.byType ?? []).map((t) => t.count))
  return (
    <section className="grid gap-3 p-4 rounded-surface bg-raised" aria-label="Phân bố tài sản">
      <h2 className="m-0 font-display text-lg font-bold">Theo trạng thái</h2>
      {loading || !summary ? (
        <div className="grid gap-2" aria-busy="true" aria-label="Đang tải phân bố">
          <div className="skeleton h-2.5 rounded-full" />
          <div className="skeleton h-4 w-1/2 rounded-sm" />
          <div className="skeleton h-4 w-2/5 rounded-sm" />
        </div>
      ) : (
        <>
          <div
            role="img"
            aria-label={`Phân bố trạng thái: ${states.map((s) => `${summary.byState[s]} ${stateLabel(s).toLocaleLowerCase('vi')}`).join(', ')}`}
            className="flex h-2.5 rounded-full overflow-hidden gap-0.5 bg-sunk"
          >
            {states.map((s) => (
              <span key={s} className={`block h-full ${STATE_SWATCH[s]}`} style={{ width: `${((summary.byState[s] ?? 0) / total) * 100}%` }} />
            ))}
          </div>
          <ul className="grid gap-2 m-0 p-0 list-none">
            {states.map((s) => (
              <li key={s} className="grid grid-cols-[10px_minmax(0,1fr)_auto] gap-2.5 items-center text-sm">
                <span className={`w-2.5 h-2.5 rounded-xs ${STATE_SWATCH[s]}`} aria-hidden="true" />
                <span>{stateLabel(s)}</span>
                <b className="font-semibold tnum">{summary.byState[s]}</b>
              </li>
            ))}
          </ul>
          {summary.byType.length > 0 && (
            <>
              <h3 className="m-0 mt-1 text-label text-ink-muted">Theo loại</h3>
              <ul className="grid gap-2.5 m-0 p-0 list-none">
                {summary.byType.slice(0, 6).map((t) => (
                  <li key={t.typeId} className="grid grid-cols-[96px_minmax(0,1fr)_32px] @3xl:grid-cols-[120px_minmax(0,1fr)_40px] gap-2.5 items-center text-sm">
                    <span className="truncate">{t.name}</span>
                    <span className="h-2 rounded-full bg-sunk overflow-hidden" aria-hidden="true">
                      <span className="block h-full rounded-full bg-accent" style={{ width: `${(t.count / top) * 100}%` }} />
                    </span>
                    <span className="text-right tnum text-ink-muted">{t.count}</span>
                  </li>
                ))}
              </ul>
            </>
          )}
        </>
      )}
    </section>
  )
}

function Activity({ entries, loading, people }: { entries: ActivityEntry[] | undefined; loading: boolean; people: PeopleDirectory }) {
  const me = useAuthStore((s) => s.user)
  const list = useMemo(() => entries ?? [], [entries])
  const { fresh } = useArrivals(list, {
    keyOf: (e) => e.id,
    authorOf: (e) => e.actor_id,
    me: me?.id,
    ready: !!entries,
  })

  const items: StepItem[] = list.map((e) => ({
    key: e.id,
    actor: assetPerson(people, e.actor_id, e.actor_name),
    segments: stepSegments(e, { asset: e.asset_name }),
    comment: e.comment,
    at: e.created_at,
    fresh: fresh.get(e.id)?.source === 'other',
  }))

  return (
    <section className="grid gap-3 p-4 rounded-surface bg-raised" aria-label="Hoạt động gần đây">
      <h2 className="m-0 font-display text-lg font-bold">Hoạt động gần đây</h2>
      {loading ? (
        <div className="grid gap-3" aria-busy="true" aria-label="Đang tải hoạt động">
          <div className="skeleton h-4 w-full rounded-sm" />
          <div className="skeleton h-4 w-4/5 rounded-sm" />
        </div>
      ) : items.length === 0 ? (
        <p className="m-0 text-sm text-ink-muted">Chưa có hoạt động nào. Giao, thu hồi hay bảo trì tài sản sẽ hiện ở đây, kèm tên người làm.</p>
      ) : (
        <StepList items={items} label="Hoạt động gần đây" />
      )}
    </section>
  )
}
