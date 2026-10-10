import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useSearch } from '@tanstack/react-router'
import { AnimatePresence, motion } from 'motion/react'
import { CircleAlert, ClipboardCheck, FileText, History, Plus, Settings2 } from 'lucide-react'
import type { ApprovalRequest, ApprovalTemplate } from '../../api/approval'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import {
  useApprovalDepartment, useApprovalHistory, useApprovalMyRequests, useApprovalPending, useApprovalPermissions, useApprovalTemplates,
  useApprove, useBatchApprove, useReject, type ApprovalRow,
} from '../../hooks/useApproval'
import { useArrivals, type Arrival } from '../../hooks/useArrivals'
import { usePeople } from '../../hooks/usePeople'
import { requestSearch, editSearch, tabOf, tabSearch, templateSearch, type ApprovalSearch, type ApprovalTab } from '../../lib/approval-search'
import { statusOf } from '../../lib/errors'
import { useMotionPresets } from '../../lib/motion'
import { workspaceDisplayName } from '../../lib/workspace'
import { useAuthStore } from '../../stores/auth.store'
import { useWebSocketStore } from '../../stores/websocket.store'
import { Avatar, Button, Heading, TabBar, toast, type TabItem } from '../primitives'
import { EmptyState } from '../spaces/EmptyState'
import { ApprovalDetailPanel } from './ApprovalDetailPanel'
import { ApprovalTable, type RealtimeTag } from './ApprovalTable'
import { CreateRequestDialog } from './CreateRequestDialog'
import { ReturnDialog } from './ReturnDialog'
import { CountPill } from './StatusPill'
import { TemplateDetailPanel } from './TemplateDetailPanel'
import { TemplateEditor } from './TemplateEditor'
import { TemplatesTable } from './TemplatesTable'

/** A row's version: a new one means the request changed (a decision, a step, completion). */
const versionOf = (r: ApprovalRow) =>
  `${r.request.id}:${r.request.status}:${r.request.current_step}:${r.request.completed_at ?? ''}:${r.assignment?.status ?? ''}`

const VERBS: Record<string, string> = {
  approved: 'vừa duyệt',
  rejected: 'vừa trả lại',
  created: 'vừa gửi',
  step_advanced: 'vừa chuyển bước',
}

const EMPTY: Record<Exclude<ApprovalTab, 'templates'>, string> = {
  pending: 'Không có đề nghị nào đang chờ bạn duyệt. Đề nghị giao cho bạn sẽ hiện ở đây.',
  mine: 'Bạn chưa gửi đề nghị nào. Tạo đề nghị để người duyệt nhận được ngay.',
  history: 'Bạn chưa duyệt hay trả lại đề nghị nào.',
  department: 'Chưa có đề nghị nào trong phạm vi bạn xem được.',
}

const TAB_LABEL: Record<ApprovalTab, string> = {
  pending: 'Đề nghị chờ tôi duyệt',
  mine: 'Đề nghị tôi đã gửi',
  history: 'Đề nghị tôi đã xử lý',
  department: 'Đề nghị trong phòng ban',
  templates: 'Mẫu phê duyệt',
}

/**
 * Phê duyệt (design/mockups/core-screens.html §3): tabs by role, the hairline
 * table of requests, the detail panel with the approval chain, and the one
 * modal that asks for a reason when a request is returned.
 *
 * Where the user is — workspace, tab, open request or template, the builder —
 * is the URL's business, so reload, Back/Forward and a pasted link land in the
 * same place. Only the tick boxes and open dialogs are client state.
 */
export function ApprovalScreen() {
  const search = useSearch({ strict: false }) as ApprovalSearch
  const navigate = useNavigate()
  const m = useMotionPresets()
  const { workspaceId: wsId, workspaceName } = useActiveWorkspace()
  const people = usePeople(wsId)
  const me = useAuthStore((s) => s.user)
  const activity = useWebSocketStore((s) => s.approvalActivity)

  // Templates are an administrator's: the server refuses everyone else, and the
  // screen does not offer what would be refused. Until the answer is in, a link
  // to the templates waits (skeleton) instead of flashing the wrong screen.
  const perms = useApprovalPermissions()
  const canManage = perms.data?.can_manage_templates === true
  const permsPending = perms.isLoading
  const wantsTemplates = tabOf(search) === 'templates' || !!search.edit
  const tab = tabOf(search) === 'templates' && !canManage && !permsPending ? 'pending' : tabOf(search)
  const editId = canManage ? search.edit : undefined
  const pending = useApprovalPending()
  const mine = useApprovalMyRequests(tab === 'mine')
  const history = useApprovalHistory(tab === 'history')
  const department = useApprovalDepartment(tab === 'department')
  const templates = useApprovalTemplates(undefined, false, tab === 'templates' && canManage)

  const approve = useApprove()
  const reject = useReject()
  const batch = useBatchApprove()

  const [ticked, setTicked] = useState<Set<string>>(() => new Set())
  const [creating, setCreating] = useState(false)
  const [returning, setReturning] = useState<ApprovalRequest | null>(null)
  // Removing the last row: keep the table up until the row has folded away.
  const [holdTable, setHoldTable] = useState(false)

  // ---- the list of the open tab ----
  const view = (() => {
    switch (tab) {
      case 'pending': return { q: pending, rows: pending.data?.rows }
      case 'mine': return { q: mine, rows: mine.data }
      case 'history': return { q: history, rows: history.data }
      case 'department': return { q: department, rows: department.data }
      default: return { q: pending, rows: undefined }
    }
  })()
  const rows = useMemo(() => view.rows ?? [], [view.rows])
  const paged = tab === 'mine' ? mine : tab === 'history' ? history : tab === 'department' ? department : undefined
  const loading = tab === 'templates' ? (permsPending || (!templates.data && !templates.isError)) : !view.rows && !view.q.isError
  const pendingCount = pending.data?.total ?? 0
  const templateList = useMemo(() => templates.data?.templates ?? [], [templates.data])

  // A new tab starts with nothing ticked.
  useEffect(() => setTicked(new Set()), [tab])
  // A request that left the list (decided elsewhere) is no longer ticked.
  const selection = useMemo(() => {
    if (tab !== 'pending') return new Set<string>()
    const live = new Set(rows.map((r) => r.request.id))
    return new Set([...ticked].filter((id) => live.has(id)))
  }, [ticked, rows, tab])

  const rowsSeen = useRef({ scope: '', n: 0 })
  if (rowsSeen.current.scope !== tab) rowsSeen.current = { scope: tab, n: 0 }
  if (rows.length === 0 && rowsSeen.current.n > 0 && !m.reduced && !holdTable) setHoldTable(true)
  rowsSeen.current.n = rows.length

  // ---- realtime: who just changed what ----
  const attribute = useCallback(
    (row: ApprovalRow): string | undefined => {
      const a = activity[row.request.id]
      // Nothing known about who acted: treat it as not news rather than blame someone.
      if (!a) return me?.id
      if (a.actorNodeId === me?.ngac_node_id) return me?.id
      return people.byNodeId.get(a.actorNodeId)?.userId || a.actorNodeId
    },
    [activity, me, people],
  )
  const { fresh, burst } = useArrivals(rows, {
    keyOf: versionOf,
    authorOf: attribute,
    me: me?.id,
    ready: !!view.rows,
    scope: tab,
  })
  const tagOf = (row: ApprovalRow, arrival: Arrival): RealtimeTag | undefined => {
    const a = activity[row.request.id]
    if (!a || !arrival.author) return undefined
    const full = people.byNodeId.get(a.actorNodeId)?.name
    const given = full ? full.split(/\s+/).pop() : 'Một người'
    return { text: `${given} ${VERBS[a.action] ?? 'vừa cập nhật'}`, hueKey: arrival.author }
  }

  // ---- URL ----
  const go = useCallback(
    (to: (prev: ApprovalSearch) => ApprovalSearch) => void navigate({ to: '/approval', search: to }),
    [navigate],
  )
  const openRequest = (row: ApprovalRow) => go((p) => requestSearch(p, row.request.id))
  const closePanel = useCallback(() => go((p) => requestSearch(templateSearch(p))), [go])

  // A link to the templates or the builder, followed by someone who may not use
  // them, falls back to the first tab.
  useEffect(() => {
    if (!permsPending && !canManage && (search.tab === 'templates' || search.edit || search.template)) {
      void navigate({ to: '/approval', search: (p: ApprovalSearch) => tabSearch(p, 'pending'), replace: true })
    }
  }, [permsPending, canManage, search.tab, search.edit, search.template, navigate])

  const openId = search.request ?? null
  const openTemplateId = canManage ? search.template ?? null : null
  const panelOpen = !!openId || !!openTemplateId
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

  // ---- decisions ----
  const doApprove = async (request: ApprovalRequest) => {
    try {
      await approve.mutateAsync({ requestId: request.id })
      toast('Đã duyệt đề nghị')
    } catch {
      // The shared mutation handler has told the user what failed.
    }
  }
  const doReturn = async (request: ApprovalRequest, reason: string) => {
    try {
      await reject.mutateAsync({ requestId: request.id, comment: reason })
      setReturning(null)
      toast('Đã trả lại đề nghị')
    } catch {
      // Told by the shared handler; the dialog stays open for another try.
    }
  }
  const doBatch = async () => {
    const ids = [...selection]
    try {
      const res = await batch.mutateAsync({ requestIds: ids })
      setTicked(new Set())
      toast(
        res.approved_count === ids.length
          ? `Đã duyệt ${res.approved_count} đề nghị`
          : `Đã duyệt ${res.approved_count}/${ids.length} đề nghị. Số còn lại chưa duyệt được, mở từng đề nghị để xem.`,
      )
    } catch {
      /* see above */
    }
  }

  // ---- what to show ----
  const tabs: TabItem[] = [
    { id: 'pending', label: 'Chờ tôi duyệt', badge: pendingCount > 0 ? <CountPill count={pendingCount} /> : undefined },
    { id: 'mine', label: 'Tôi đã gửi' },
    { id: 'history', label: 'Đã xử lý' },
    { id: 'department', label: 'Phòng ban' },
    ...(canManage || (permsPending && wantsTemplates) ? [{ id: 'templates', label: 'Mẫu' }] : []),
  ]
  const workspaceLabel = workspaceDisplayName(workspaceName)
  const errorStatus = statusOf(tab === 'templates' ? templates.error : view.q.error)
  const failed = tab === 'templates' ? templates.isError : view.q.isError && !view.rows

  const retry = () => void (tab === 'templates' ? templates.refetch() : view.q.refetch())
  const newRequest = (
    <Button size="sm" onClick={() => setCreating(true)}>
      <Plus size={16} strokeWidth={1.75} aria-hidden="true" />
      Tạo đề nghị
    </Button>
  )

  const body = (() => {
    if (failed) {
      return errorStatus === 404 ? (
        <EmptyState
          icon={<ClipboardCheck size={24} strokeWidth={1.75} />}
          text="Phê duyệt chưa được bật cho không gian này. Liên hệ quản trị viên để bật."
        />
      ) : (
        <EmptyState
          icon={<CircleAlert size={24} strokeWidth={1.75} />}
          text="Không tải được danh sách đề nghị. Kiểm tra kết nối rồi thử lại."
          action={<Button variant="soft" size="sm" onClick={retry}>Thử lại</Button>}
        />
      )
    }
    if (tab === 'templates') {
      if (!loading && templateList.length === 0) {
        return (
          <EmptyState
            icon={<Settings2 size={24} strokeWidth={1.75} />}
            text="Chưa có mẫu phê duyệt nào. Tạo mẫu để mọi người gửi đề nghị theo cùng một quy trình."
            action={<Button size="sm" onClick={() => go((p) => editSearch(p, 'new'))}>Mẫu mới</Button>}
          />
        )
      }
      return (
        <motion.div key="templates" {...m.route}>
          <TemplatesTable
            templates={templateList}
            loading={loading}
            openId={openTemplateId}
            onOpen={(t: ApprovalTemplate) => go((p) => templateSearch(requestSearch(p), t.id))}
            onEdit={(t: ApprovalTemplate) => go((p) => editSearch(p, t.id))}
          />
        </motion.div>
      )
    }
    if (!loading && rows.length === 0 && !holdTable) {
      return (
        <EmptyState
          icon={tab === 'history' ? <History size={24} strokeWidth={1.75} /> : tab === 'mine' ? <FileText size={24} strokeWidth={1.75} /> : <ClipboardCheck size={24} strokeWidth={1.75} />}
          text={EMPTY[tab]}
          action={tab === 'mine' ? newRequest : undefined}
        />
      )
    }
    return (
      <motion.div key={tab} {...m.route}>
        <ApprovalTable
          label={TAB_LABEL[tab]}
          rows={rows}
          loading={loading}
          people={people}
          openId={openId}
          onOpen={openRequest}
          selection={
            tab === 'pending'
              ? {
                  ids: selection,
                  onToggle: (id) => setTicked((prev) => { const n = new Set(prev); if (n.has(id)) n.delete(id); else n.add(id); return n }),
                  onToggleAll: () => setTicked(selection.size === rows.length ? new Set() : new Set(rows.map((r) => r.request.id))),
                }
              : undefined
          }
          scope={tab}
          fresh={fresh}
          versionOf={versionOf}
          tagOf={tagOf}
          onRowsGone={() => setHoldTable(false)}
        />
        {paged?.hasNextPage && (
          <div className="flex justify-center pb-4">
            <Button variant="soft" size="sm" loading={paged.isFetchingNextPage} onClick={() => void paged.fetchNextPage()}>
              Xem thêm
            </Button>
          </div>
        )}
      </motion.div>
    )
  })()

  const burstAuthors = burst?.authors.map((a) => people.byUserId.get(a)?.name ?? 'Một người') ?? []

  return (
    <div className="relative flex flex-1 min-h-0 min-w-0">
      {editId ? (
        <TemplateEditor
          templateId={editId}
          workspaceId={wsId}
          people={people}
          onDone={() => go((p) => tabSearch(p, 'templates'))}
        />
      ) : (
        <section className="flex-1 flex flex-col min-w-0 min-h-0 bg-base" aria-label="Phê duyệt">
          <header className="flex items-center gap-4 px-5 pt-4 pb-1 min-w-0">
            <div className="grid gap-0.5 min-w-0">
              <Heading as="h1" look="page" className="truncate">Phê duyệt</Heading>
              <span className="text-sm text-ink-muted truncate">Đề nghị trong {workspaceLabel}</span>
            </div>
            <div className="ml-auto flex items-center gap-2 shrink-0">
              {tab === 'templates' ? (
                <>
                  <Button variant="soft" size="sm" onClick={() => setCreating(true)}>
                    <Plus size={16} strokeWidth={1.75} aria-hidden="true" />
                    <span className="max-sm:sr-only">Tạo đề nghị</span>
                  </Button>
                  <Button size="sm" onClick={() => go((p) => editSearch(p, 'new'))}>
                    <Plus size={16} strokeWidth={1.75} aria-hidden="true" />
                    <span className="max-sm:sr-only">Mẫu mới</span>
                  </Button>
                </>
              ) : (
                newRequest
              )}
            </div>
          </header>

          <div className="px-5 overflow-x-auto">
            <TabBar
              className="w-max"
              label="Phê duyệt"
              idPrefix="approval"
              value={tab}
              tabs={tabs}
              onChange={(id) => go((p) => tabSearch(p, id as ApprovalTab))}
            />
          </div>

          <AnimatePresence>
            {burst && (
              <motion.div key="burst" role="status" {...m.row} className="mx-5 mt-2 overflow-hidden">
                <div className="flex items-center gap-2.5 mb-1 px-3 py-2 rounded-surface bg-raised text-sm">
                  <span className="flex items-center">
                    {burst.authors.slice(0, 3).map((a, i) => (
                      <Avatar key={a} name={people.byUserId.get(a)?.name ?? 'Một người'} hueKey={a} size={20} className={`ring-2 ring-raised ${i > 0 ? '-ml-1.5' : ''}`} />
                    ))}
                  </span>
                  <span>
                    <b className="font-semibold">{burstAuthors[0]}</b>
                    {burst.authors.length > 1 ? ` và ${burst.authors.length - 1} người khác` : ''} vừa cập nhật các đề nghị
                  </span>
                </div>
              </motion.div>
            )}
            {selection.size > 0 && (
              <motion.div key="batch" {...m.row} className="mx-5 mt-2 overflow-hidden">
                <div role="status" className="flex items-center gap-3 mb-1 px-3 py-2 rounded-surface bg-accent-wash text-sm">
                  <span className="font-semibold">Đã chọn {selection.size} đề nghị</span>
                  <Button variant="ghost" size="sm" onClick={() => setTicked(new Set())}>Bỏ chọn</Button>
                  <Button size="sm" className="ml-auto" loading={batch.isPending} onClick={() => void doBatch()}>
                    Duyệt {selection.size} đề nghị
                  </Button>
                </div>
              </motion.div>
            )}
          </AnimatePresence>

          <div
            role="tabpanel"
            id={`approval-panel-${tab}`}
            aria-labelledby={`approval-tab-${tab}`}
            className="flex-1 min-h-0 overflow-y-auto"
          >
            {body}
          </div>
        </section>
      )}

      <AnimatePresence>
        {!editId && openId && (
          <ApprovalDetailPanel
            key={`request-${openId}`}
            requestId={openId}
            summary={rows.find((r) => r.request.id === openId)?.request}
            people={people}
            meNodeId={me?.ngac_node_id}
            approving={approve.isPending}
            onApprove={(r) => void doApprove(r)}
            onReturn={setReturning}
            onClose={closePanel}
          />
        )}
        {!editId && !openId && openTemplateId && canManage && (
          <TemplateDetailPanel
            key={`template-${openTemplateId}`}
            templateId={openTemplateId}
            people={people}
            onEdit={(id) => go((p) => editSearch(p, id))}
            onClose={closePanel}
          />
        )}
      </AnimatePresence>

      <ReturnDialog request={returning} pending={reject.isPending} onConfirm={(r, why) => void doReturn(r, why)} onClose={() => setReturning(null)} />
      <CreateRequestDialog
        open={creating}
        onClose={() => setCreating(false)}
        onCreated={() => go((p) => tabSearch(p, 'mine'))}
      />
    </div>
  )
}
