import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate, useSearch } from '@tanstack/react-router'
import { AnimatePresence, motion } from 'motion/react'
import { CircleAlert, ClipboardList, Plus, Tag } from 'lucide-react'
import type { Asset, AssetType } from '../../api/assets'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { useAssetActivity, useAssetRequests, useAssetSummary, useAssetTypes, useAssets } from '../../hooks/useAssets'
import { useArrivals } from '../../hooks/useArrivals'
import { usePeople } from '../../hooks/usePeople'
import { parseFields } from '../../lib/asset-fields'
import { REQUEST_FILTERS, requestFilterStatuses, type RequestFilter } from '../../lib/asset-model'
import {
  assetSearch, composeSearch, filterSearch, requestSearch, showSearch, tabOf, tabSearch, typeSearch,
  type AssetsSearch, type AssetsTab,
} from '../../lib/assets-search'
import { toMillis } from '../../lib/format'
import { useMotionPresets } from '../../lib/motion'
import { workspaceDisplayName } from '../../lib/workspace'
import { useAuthStore } from '../../stores/auth.store'
import { Button, FilterChip, Heading, TabBar, type TabItem } from '../primitives'
import { CountPill } from '../approval/StatusPill'
import { EmptyState } from '../spaces/EmptyState'
import { ApproveDialog } from './ApproveDialog'
import { AssetDetailPanel } from './AssetDetailPanel'
import { AssetListBody, PAGE } from './AssetListBody'
import { AssetOverview } from './AssetOverview'
import type { RealtimeTag } from './AssetTable'
import { NewAssetDialog } from './NewAssetDialog'
import { NewRequestPanel } from './NewRequestPanel'
import { NewTypeDialog } from './NewTypeDialog'
import { RejectDialog } from './RejectDialog'
import { RequestDetailPanel } from './RequestDetailPanel'
import { RequestTable } from './RequestTable'
import { TypeDetailPanel } from './TypeDetailPanel'
import { TypesTable } from './TypesTable'
import { useRequestDecisions } from './useRequestDecisions'

/** Shared by the tab's count, the overview and the "Đang chờ" filter, so they are one request. */
const PENDING = { status: 'pending', limit: 100 } as const

const holds = (t: AssetType, op: string) => t.permissions?.includes(op) === true

const REQUEST_EMPTY: Record<RequestFilter, string> = {
  pending: 'Không còn yêu cầu nào chờ bạn. Yêu cầu mới sẽ hiện ở đây và trên bộ đếm của tab.',
  approved: 'Chưa có yêu cầu nào được duyệt.',
  rejected: 'Chưa có yêu cầu nào bị từ chối.',
  all: 'Chưa có yêu cầu nào trong phạm vi bạn xem được.',
  mine: 'Bạn chưa gửi yêu cầu nào. Tạo yêu cầu để người duyệt nhận được ngay.',
}

const SUBTITLE: Record<AssetsTab, string> = {
  overview: 'Thiết bị, giấy phép và nội thất',
  list: 'Tài sản',
  requests: 'Yêu cầu cấp tài sản',
  types: 'Loại tài sản',
}

/**
 * Tài sản (design/mockups/assets.html): tổng quan, danh sách with a detail
 * panel, yêu cầu, and loại tài sản, as tabs of one screen inside the shared
 * shell. Where the user is — workspace, tab, filters, page, the open asset,
 * request or type, the new-request form — is the URL's business, so reload,
 * Back/Forward and a pasted link land in the same place. Only the open dialogs
 * are client state.
 */
export function AssetsScreen() {
  const search = useSearch({ strict: false }) as AssetsSearch
  const navigate = useNavigate()
  const m = useMotionPresets()
  const { workspaceId: wsId, workspaceName } = useActiveWorkspace()
  const people = usePeople(wsId)
  const me = useAuthStore((s) => s.user)
  const tab = tabOf(search)

  // ---- what the caller may do, from what they hold on each type ----
  const typesQ = useAssetTypes(wsId)
  const types = useMemo(() => typesQ.data?.types ?? [], [typesQ.data])
  const canManage = typesQ.data?.canManage === true
  const requestable = useMemo(() => types.filter((t) => holds(t, 'write')), [types])
  const canApprove = types.some((t) => holds(t, 'approve'))

  // ---- requests: the count on the tab and the overview share one query ----
  const show: RequestFilter = search.show ?? 'pending'
  const pendingQ = useAssetRequests(wsId, PENDING)
  const waiting = useMemo(() => (pendingQ.data?.requests ?? []).filter((r) => r.can_decide), [pendingQ.data])
  const urgentCount = (pendingQ.data?.requests ?? []).filter((r) => r.urgency === 'urgent').length
  const reqParams = show === 'pending' ? PENDING : { ...requestFilterStatuses(show), limit: 50 }
  const requestsQ = useAssetRequests(wsId, reqParams, tab === 'requests')
  const requestRows = useMemo(() => requestsQ.data?.requests ?? [], [requestsQ.data])

  // ---- the list ----
  const page = search.page ?? 1
  const listParams = { type_id: search.kind, state: search.state, search: search.q, limit: PAGE, offset: (page - 1) * PAGE }
  const listQ = useAssets(wsId, listParams, tab === 'list')
  const rows = useMemo(() => listQ.data?.assets ?? [], [listQ.data])
  const total = listQ.data?.total ?? 0
  const summaryQ = useAssetSummary(wsId, tab === 'overview' || tab === 'list')
  const activityQ = useAssetActivity(wsId, 10, tab === 'overview' || tab === 'list')

  // ---- dialogs ----
  const [creatingType, setCreatingType] = useState(false)
  const [creatingAsset, setCreatingAsset] = useState(false)
  const {
    deciding, rejecting, decisionError, decidePending, rejectPending, anyPending,
    closeDecision, closeReject, doApprove, startApprove, startReject, startAssign, doReject,
  } = useRequestDecisions()

  // ---- the search box: typed text is local; the URL follows after a pause ----
  const [typed, setTyped] = useState(search.q ?? '')
  useEffect(() => setTyped(search.q ?? ''), [search.q])

  // ---- URL ----
  const go = useCallback(
    (to: (prev: AssetsSearch) => AssetsSearch) => void navigate({ to: '/assets', search: to }),
    [navigate],
  )
  useEffect(() => {
    if (typed.trim() === (search.q ?? '').trim()) return
    const t = setTimeout(() => go((p) => filterSearch(p, { q: typed })), 300)
    return () => clearTimeout(t)
  }, [typed, search.q, go])

  const composing = tab === 'requests' && search.compose === 'request'
  const openAssetId = tab === 'list' ? search.asset ?? null : null
  const openRequestId = tab === 'requests' && !composing ? search.request ?? null : null
  const openTypeId = tab === 'types' ? search.type ?? null : null
  const panelOpen = composing || !!openAssetId || !!openRequestId || !!openTypeId
  const closePanel = useCallback(
    () => go((p) => (p.compose ? composeSearch(p, false) : p.asset ? assetSearch(p) : p.request ? requestSearch(p) : typeSearch(p))),
    [go],
  )
  // Esc closes the panel. Dialogs and menus claim the key on `document`
  // (preventDefault); listening on `window` puts this after them, so one Esc
  // closes one thing.
  useEffect(() => {
    if (!panelOpen) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && !e.defaultPrevented) closePanel()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [panelOpen, closePanel])

  // ---- realtime: who just changed which asset ----
  const authorByAsset = useMemo(() => {
    const out = new Map<string, string>()
    for (const e of activityQ.data ?? []) if (e.actor_id && !out.has(e.asset_id)) out.set(e.asset_id, e.actor_id)
    return out
  }, [activityQ.data])
  const versionOf = useCallback(
    (a: Asset) => `${a.id}:${a.state}:${a.assigned_to_user_id ?? ''}:${toMillis(a.updated_at)}`,
    [],
  )
  const listScope = `${search.state ?? ''}|${search.kind ?? ''}|${search.q ?? ''}|${page}`
  const { fresh } = useArrivals(rows, {
    keyOf: versionOf,
    authorOf: (a) => authorByAsset.get(a.id),
    me: me?.id,
    ready: !!listQ.data && !listQ.isPlaceholderData && !activityQ.isFetching,
    scope: listScope,
  })
  const tagOf = (_a: Asset, arrival: { author?: string }): RealtimeTag | undefined => {
    if (!arrival.author) return undefined
    const full = people.byUserId.get(arrival.author)?.name
    const given = full ? full.split(/\s+/).pop() : 'Một người'
    return { text: `${given} vừa cập nhật`, hueKey: arrival.author }
  }

  // ---- what to show ----
  const tabs: TabItem[] = [
    { id: 'overview', label: 'Tổng quan' },
    { id: 'list', label: 'Danh sách' },
    { id: 'requests', label: 'Yêu cầu', badge: canApprove && waiting.length > 0 ? <CountPill count={waiting.length} /> : undefined },
    { id: 'types', label: 'Loại tài sản' },
  ]
  const workspaceLabel = workspaceDisplayName(workspaceName)
  const sub =
    tab === 'overview' ? `${SUBTITLE.overview} của ${workspaceLabel}`
    : tab === 'list' ? (listQ.data ? `${listQ.data.total} tài sản` : SUBTITLE.list)
    : tab === 'types' ? (typesQ.data ? `${types.length} loại tài sản` : SUBTITLE.types)
    : SUBTITLE.requests

  const newRequest = requestable.length > 0 && tab !== 'types' && (
    <Button size="sm" onClick={() => go((p) => composeSearch(tabSearch(p, 'requests')))}>
      <Plus size={16} strokeWidth={1.75} aria-hidden="true" />
      <span className="max-sm:sr-only">Yêu cầu tài sản</span>
    </Button>
  )

  const retryOf = (q: { refetch: () => unknown }) => <Button variant="soft" size="sm" onClick={() => void q.refetch()}>Thử lại</Button>
  const failed = (text: string, q: { refetch: () => unknown }) => (
    <EmptyState icon={<CircleAlert size={24} strokeWidth={1.75} />} text={text} action={retryOf(q)} />
  )

  const body = (() => {
    if (tab === 'overview') {
      return (
        <AssetOverview
          search={search}
          summary={summaryQ.data}
          summaryLoading={summaryQ.isLoading}
          summaryFailed={summaryQ.isError && !summaryQ.data}
          onRetry={() => void summaryQ.refetch()}
          activity={activityQ.data}
          activityLoading={activityQ.isLoading}
          waiting={waiting}
          pendingCount={pendingQ.data?.total}
          urgentCount={urgentCount}
          canApprove={canApprove}
          canManage={canManage}
          people={people}
          onOpenRequest={(id) => go((p) => requestSearch(tabSearch(p, 'requests'), id))}
          onNewType={() => setCreatingType(true)}
        />
      )
    }

    if (tab === 'list') {
      if (listQ.isError && !listQ.data) return failed('Không tải được danh sách tài sản. Kiểm tra kết nối rồi thử lại.', listQ)
      return (
        <AssetListBody
          search={search}
          go={go}
          typed={typed}
          onTyped={setTyped}
          summary={summaryQ.data}
          types={types}
          requestable={requestable}
          rows={rows}
          total={total}
          page={page}
          loading={!listQ.data && !listQ.isError}
          people={people}
          openId={openAssetId}
          scope={listScope}
          fresh={fresh}
          versionOf={versionOf}
          tagOf={tagOf}
          route={m.route}
          onAddAsset={() => setCreatingAsset(true)}
        />
      )
    }

    if (tab === 'requests') {
      const loading = !requestsQ.data && !requestsQ.isError
      return (
        <>
          <div className="flex flex-wrap items-center gap-2 px-5 pb-3" role="group" aria-label="Lọc yêu cầu">
            {REQUEST_FILTERS.map((f) => (
              <FilterChip key={f.id} pressed={show === f.id} onClick={() => go((p) => showSearch(p, f.id))}>{f.label}</FilterChip>
            ))}
          </div>
          {requestsQ.isError && !requestsQ.data ? (
            failed('Không tải được danh sách yêu cầu. Kiểm tra kết nối rồi thử lại.', requestsQ)
          ) : !loading && requestRows.length === 0 ? (
            <EmptyState
              icon={<ClipboardList size={24} strokeWidth={1.75} />}
              text={REQUEST_EMPTY[show]}
              action={show === 'mine' && requestable.length > 0 ? <Button size="sm" onClick={() => go((p) => composeSearch(p))}>Yêu cầu tài sản</Button> : undefined}
            />
          ) : (
            <motion.div key={show} {...m.route}>
              <RequestTable
                label="Yêu cầu tài sản"
                requests={requestRows}
                loading={loading}
                people={people}
                showStatus={show !== 'pending'}
                openId={openRequestId}
                onOpen={(r) => go((p) => requestSearch(p, r.id))}
                scope={show}
              />
            </motion.div>
          )}
        </>
      )
    }

    // types
    if (typesQ.isError && !typesQ.data) return failed('Không tải được danh sách loại tài sản. Kiểm tra kết nối rồi thử lại.', typesQ)
    if (typesQ.data && types.length === 0) {
      return (
        <EmptyState
          icon={<Tag size={24} strokeWidth={1.75} />}
          text={canManage
            ? 'Chưa có loại tài sản nào. Tạo loại để theo dõi thiết bị, giấy phép và nội thất.'
            : 'Chưa có loại tài sản nào trong phạm vi bạn xem được.'}
          action={canManage ? <Button size="sm" onClick={() => setCreatingType(true)}>Thêm loại</Button> : undefined}
        />
      )
    }
    return (
      <motion.div key="types" {...m.route}>
        <TypesTable types={types} loading={!typesQ.data} openId={openTypeId} onOpen={(t) => go((p) => typeSearch(p, t.id))} />
      </motion.div>
    )
  })()

  const openRequest = requestRows.find((r) => r.id === openRequestId)
  const openType = types.find((t) => t.id === openTypeId)
  const deciderFields = useMemo(
    () => parseFields(types.find((t) => t.id === deciding?.request.type_id)?.fields_schema),
    [types, deciding],
  )

  return (
    <div className="relative flex flex-1 min-h-0 min-w-0">
      <section className="flex-1 flex flex-col min-w-0 min-h-0 bg-base" aria-label="Tài sản">
        <header className="flex items-center gap-4 px-5 pt-4 pb-1 min-w-0">
          <div className="grid gap-0.5 min-w-0">
            <Heading as="h1" look="page" className="truncate">Tài sản</Heading>
            <span className="text-sm text-ink-muted truncate">{sub}</span>
          </div>
          <div className="ml-auto flex items-center gap-2 shrink-0">
            {tab === 'list' && requestable.length > 0 && (
              <Button variant="soft" size="sm" onClick={() => setCreatingAsset(true)}>
                <Plus size={16} strokeWidth={1.75} aria-hidden="true" />
                <span className="max-sm:sr-only">Thêm tài sản</span>
              </Button>
            )}
            {tab === 'types' && canManage && (
              <Button size="sm" onClick={() => setCreatingType(true)}>
                <Plus size={16} strokeWidth={1.75} aria-hidden="true" />
                <span className="max-sm:sr-only">Thêm loại</span>
              </Button>
            )}
            {newRequest}
          </div>
        </header>

        <div className="px-5 overflow-x-auto">
          <TabBar
            className="w-max"
            label="Tài sản"
            idPrefix="assets"
            value={tab}
            tabs={tabs}
            onChange={(id) => go((p) => tabSearch(p, id as AssetsTab))}
          />
        </div>

        <div
          role="tabpanel"
          id={`assets-panel-${tab}`}
          aria-labelledby={`assets-tab-${tab}`}
          className="@container flex-1 min-h-0 overflow-y-auto pt-3"
        >
          {body}
        </div>
      </section>

      <AnimatePresence>
        {composing && (
          <NewRequestPanel
            key="compose"
            workspaceId={wsId}
            types={requestable}
            loading={typesQ.isLoading}
            onSent={() => go((p) => showSearch(composeSearch(p, false), 'mine'))}
            onClose={closePanel}
          />
        )}
        {openAssetId && (
          <AssetDetailPanel
            key={`asset-${openAssetId}`}
            assetId={openAssetId}
            summary={rows.find((a) => a.id === openAssetId)}
            people={people}
            types={types}
            onClose={closePanel}
          />
        )}
        {openRequestId && (
          <RequestDetailPanel
            key={`request-${openRequestId}`}
            requestId={openRequestId}
            summary={openRequest}
            people={people}
            types={types}
            busy={anyPending}
            onApprove={(r) => void startApprove(r)}
            onReject={startReject}
            onAssign={startAssign}
            onClose={closePanel}
          />
        )}
        {openType && (
          // Keyed by what the server holds, so a save (or another editor's) reloads the fields.
          <TypeDetailPanel
            key={`type-${openType.id}:${openType.fields_schema ?? ''}`}
            type={openType}
            workspaceId={wsId}
            canManage={canManage}
            onClose={closePanel}
          />
        )}
      </AnimatePresence>

      <ApproveDialog
        request={deciding?.request ?? null}
        mode={deciding?.mode ?? 'approve'}
        workspaceId={wsId}
        fields={deciderFields}
        people={people}
        pending={decidePending}
        failure={decisionError}
        onConfirm={(r, assetId) => void doApprove(r, assetId)}
        onClose={closeDecision}
      />
      <RejectDialog
        request={rejecting}
        pending={rejectPending}
        failure={decisionError}
        onConfirm={(r, why) => void doReject(r, why)}
        onClose={closeReject}
      />
      <NewTypeDialog
        open={creatingType}
        workspaceId={wsId}
        onCreated={(id) => go((p) => typeSearch(tabSearch(p, 'types'), id))}
        onClose={() => setCreatingType(false)}
      />
      <NewAssetDialog
        open={creatingAsset}
        workspaceId={wsId}
        types={requestable}
        people={people}
        onCreated={(id) => go((p) => assetSearch(tabSearch(p, 'list'), id))}
        onClose={() => setCreatingAsset(false)}
      />
    </div>
  )
}
