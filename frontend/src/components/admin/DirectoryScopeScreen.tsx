import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { CircleAlert, Folder, RefreshCw, Shield, Users } from 'lucide-react'
import type { Department, LimitedKind, RuleSubject } from '../../api/admin'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import {
  useDepartments, useHideDepartment, useLimitSubject, useManagerDirectoryScope, useRoles, useUnhideDepartment, useUnlimitSubject,
} from '../../hooks/useAdmin'
import { childrenIndex, deptPath, deptWithDescendants, DEPT_SEPARATOR, roleName } from '../../lib/admin-model'
import { explain, statusOf } from '../../lib/errors'
import { Button, Heading, toast } from '../primitives'
import { ConfirmDialog } from '../composites/ConfirmDialog'
import { AdminFrame } from './AdminFrame'
import { DirectoryScopePicker, type ScopeGroup, type ScopeOption } from './DirectoryScopePicker'

/** What a person has asked for and is being asked to confirm. */
type Ask =
  | { rule: 'hide'; id: string; name: string; people: number; below: number }
  | { rule: 'unhide'; id: string; name: string; people: number }
  | { rule: 'limit' | 'unlimit'; kind: LimitedKind; id: string; name: string; people: number }

const lowerFirst = (s: string) => s.charAt(0).toLowerCase() + s.slice(1)
const who = (kind: LimitedKind, name: string) => `${kind === 'role' ? 'vai trò' : 'phòng'} ${name}`

/** The words of each question and of what follows it; people, not codes. */
function wording(a: Ask) {
  if (a.rule === 'hide') {
    return {
      title: `Ẩn ${a.name} khỏi danh bạ?`,
      body: `${a.people} người của phòng này chỉ còn được người trong phòng và quản trị viên thấy${a.below ? `, kể cả ${a.below} phòng, nhóm bên dưới` : ''}. Tên và avatar của họ vẫn hiện ở tin nhắn, phê duyệt và tài liệu.`,
      verb: 'Ẩn phòng', done: `Đã ẩn ${a.name} khỏi danh bạ`, fail: `Không ẩn được ${a.name}.`, action: 'ẩn phòng ban khỏi danh bạ',
    }
  }
  if (a.rule === 'unhide') {
    return {
      title: `Bỏ ẩn ${a.name}?`,
      body: `Mọi thành viên sẽ lại thấy ${a.people} người của phòng này trong danh bạ.`,
      verb: 'Bỏ ẩn', done: `Đã bỏ ẩn ${a.name}`, fail: `Không bỏ ẩn được ${a.name}.`, action: 'hiện lại phòng ban trong danh bạ',
    }
  }
  const w = who(a.kind, a.name)
  if (a.rule === 'limit') {
    return {
      title: `Giới hạn ${w}?`,
      body: `${a.people}${a.kind === 'role' ? ' người giữ vai trò này' : ' người của phòng này (cả các phòng bên dưới)'} chỉ còn thấy người trong phòng của họ và quản trị viên. Ai là quản trị viên thì vẫn thấy tất cả.`,
      verb: 'Giới hạn', done: `Đã giới hạn ${w}`, fail: `Không giới hạn được ${w}.`, action: 'giới hạn phạm vi xem danh bạ',
    }
  }
  return {
    title: `Bỏ giới hạn cho ${w}?`,
    body: `${a.people} người sẽ lại thấy toàn bộ danh bạ, trừ các phòng bị ẩn.`,
    verb: 'Bỏ giới hạn', done: `Đã bỏ giới hạn cho ${w}`, fail: `Không bỏ giới hạn được cho ${w}.`, action: 'bỏ giới hạn phạm vi xem danh bạ',
  }
}

const Tile = ({ kind }: { kind: 'department' | 'role' }) => (
  <span
    aria-hidden="true"
    className={`grid place-items-center w-8 h-8 rounded-md shrink-0 ${kind === 'department' ? 'bg-accent-wash text-accent' : 'bg-sunk text-ink-muted'}`}
  >
    {kind === 'department' ? <Folder size={16} strokeWidth={1.75} /> : <Users size={16} strokeWidth={1.75} />}
  </span>
)

function RuleRow({ kind, title, line, button }: { kind: 'department' | 'role'; title: string; line: string; button: ReactNode }) {
  return (
    <li className="flex items-center gap-3 min-h-14 py-2 min-w-0 border-t border-line first:border-t-0">
      <Tile kind={kind} />
      <span className="grid gap-0.5 flex-1 min-w-0">
        <b className="font-medium truncate">{title}</b>
        <small className="text-small text-ink-muted sm:truncate">{line}</small>
      </span>
      {button}
    </li>
  )
}

function RuleList({ label, count, hint, empty, loading, children, add }: {
  label: string
  count: number
  hint: string
  empty: string
  loading: boolean
  children: ReactNode
  add: ReactNode
}) {
  return (
    <section aria-label={label} className="grid gap-2.5 p-4 rounded-surface bg-raised content-start min-w-0">
      <div className="flex items-baseline gap-2">
        <Heading as="h2" look="section">{label}</Heading>
        {!loading && <span className="text-ink-muted tnum font-medium">{count}</span>}
      </div>
      <p className="m-0 text-small text-ink-muted">{hint}</p>
      {loading ? (
        <ul aria-busy="true" aria-label={`Đang tải: ${label}`} className="grid m-0 p-0 list-none">
          {[50, 42].map((w, i) => (
            <li key={i} className="flex items-center gap-3 min-h-14 py-2 border-t border-line first:border-t-0">
              <div className="skeleton w-8 h-8 rounded-md shrink-0" />
              <div className="grid gap-1.5 flex-1">
                <div className="skeleton h-3.5 rounded-sm" style={{ width: `${w}%` }} />
                <div className="skeleton h-2.5 rounded-sm" style={{ width: `${w + 22}%` }} />
              </div>
              <div className="skeleton w-16 h-8 rounded-md" />
            </li>
          ))}
        </ul>
      ) : count === 0 ? (
        <p className="m-0 py-2 text-ink-muted">{empty}</p>
      ) : (
        <ul className="grid m-0 p-0 list-none">{children}</ul>
      )}
      {add}
    </section>
  )
}

/** The departments in tree order, each with how deep it sits. */
function treeOrder(departments: Department[]): { dept: Department; depth: number }[] {
  const index = childrenIndex(departments)
  const out: { dept: Department; depth: number }[] = []
  const seen = new Set<string>()
  const walk = (parent: string, depth: number) => {
    for (const d of index.get(parent) ?? []) {
      if (seen.has(d.id)) continue
      seen.add(d.id)
      out.push({ dept: d, depth })
      walk(d.id, depth + 1)
    }
  }
  walk('', 0)
  return out
}

/**
 * Quản trị › Danh bạ (design/mockups/admin.html 7b): who sees whom in Danh bạ. Two lists of rules,
 * "Phòng ẩn khỏi danh bạ" and "Chỉ thấy phòng của mình", each changed by choosing a department or a
 * role by name and answering a question that says what it means in people. Nothing changes on the
 * screen until the server has said yes. The server decides who sees whom; this only writes the rules.
 * A workspace with no directory yet has no such screen: the link goes to Tổng quan.
 */
export function DirectoryScopeScreen() {
  const navigate = useNavigate()
  const { workspaceId: wsId } = useActiveWorkspace()
  const rules = useManagerDirectoryScope(wsId)
  const departments = useDepartments(wsId)
  const roles = useRoles(wsId)
  const hide = useHideDepartment(wsId)
  const unhide = useUnhideDepartment(wsId)
  const limit = useLimitSubject(wsId)
  const unlimit = useUnlimitSubject(wsId)
  const [ask, setAsk] = useState<Ask | null>(null)

  const notReady = rules.data?.ready === false
  useEffect(() => {
    if (notReady) void navigate({ to: '/admin', replace: true, search: (prev: { ws?: string }) => (prev.ws ? { ws: prev.ws } : {}) })
  }, [notReady, navigate])

  const all = useMemo(() => departments.data?.flat ?? [], [departments.data])
  const data = rules.data?.ready ? rules.data : undefined
  const hidden = data?.hidden_departments ?? []
  const limited = data?.limited ?? []
  const loading = rules.isPending
  const failed = rules.isError && !rules.data

  const hiddenIds = useMemo(() => new Set(hidden.map((d) => d.id)), [hidden])
  const limitedKey = useMemo(() => new Set(limited.map((l) => `${l.kind}:${l.id}`)), [limited])
  const byId = useMemo(() => new Map(all.map((d) => [d.id, d])), [all])
  const parentPath = (d: RuleSubject) => (d.parent_id ? deptPath(all, d.parent_id).join(DEPT_SEPARATOR) : '')

  const hideOptions = useMemo<ScopeGroup[]>(() => [{
    options: treeOrder(all).map(({ dept, depth }) => {
      let why: string | undefined
      if (hiddenIds.has(dept.id)) why = 'Đã ẩn'
      else {
        for (let p = byId.get(dept.parent_id), seen = new Set<string>(); p && !seen.has(p.id); p = byId.get(p.parent_id)) {
          seen.add(p.id)
          if (hiddenIds.has(p.id)) { why = `Ẩn theo ${p.name}`; break }
        }
      }
      return { kind: 'department' as const, id: dept.id, name: dept.name, hint: `${dept.member_count} người`, depth, why }
    }),
  }], [all, byId, hiddenIds])

  const limitOptions = useMemo<ScopeGroup[]>(() => [
    {
      label: 'Vai trò',
      options: (roles.data?.custom ?? []).map((r): ScopeOption => ({
        kind: 'role', id: r.id, name: roleName(r), hint: `${r.member_count} người`,
        why: limitedKey.has(`role:${r.id}`) ? 'Đã giới hạn' : undefined,
      })),
    },
    {
      label: 'Phòng ban',
      options: treeOrder(all).map(({ dept, depth }): ScopeOption => ({
        kind: 'department', id: dept.id, name: dept.name, hint: `${dept.member_count} người`, depth,
        why: limitedKey.has(`department:${dept.id}`) ? 'Đã giới hạn' : undefined,
      })),
    },
  ].filter((g) => g.options.length > 0), [all, roles.data, limitedKey])

  const run = useCallback((a: Ask) => {
    const w = wording(a)
    const done = () => {
      setAsk(null)
      toast(w.done)
    }
    const failed = (err: unknown) => {
      setAsk(null)
      // A refusal has its own sentence; only a lost connection or a server fault is "check the network".
      const status = statusOf(err)
      const network = status === undefined || status >= 500
      toast.error(network ? `${w.fail} Kiểm tra mạng rồi thử lại.` : explain(err, w.action), {
        action: { label: 'Thử lại', onClick: () => run(a) },
      })
    }
    const opts = { onSuccess: done, onError: failed }
    if (a.rule === 'hide') hide.mutate(a.id, opts)
    else if (a.rule === 'unhide') unhide.mutate(a.id, opts)
    else if (a.rule === 'limit') limit.mutate({ kind: a.kind, id: a.id }, opts)
    else unlimit.mutate({ kind: a.kind, id: a.id }, opts)
  }, [hide, unhide, limit, unlimit])

  const askHide = (o: ScopeOption) => {
    const d = byId.get(o.id)
    setAsk({ rule: 'hide', id: o.id, name: o.name, people: d?.member_count ?? 0, below: Math.max(0, deptWithDescendants(all, o.id).size - 1) })
  }
  const askLimit = (o: ScopeOption) => {
    const people = o.kind === 'role' ? (roles.data?.custom.find((r) => r.id === o.id)?.member_count ?? 0) : (byId.get(o.id)?.member_count ?? 0)
    setAsk({ rule: 'limit', kind: o.kind, id: o.id, name: o.name, people })
  }

  const pending = hide.isPending || unhide.isPending || limit.isPending || unlimit.isPending
  const text = ask ? wording(ask) : null

  return (
    <AdminFrame section="directory">
      <div className="flex-1 min-w-0 min-h-0 overflow-y-auto">
        {failed ? (
          <div className="grid justify-items-center gap-2 px-6 pt-12 text-center">
            <span aria-hidden="true" className="grid place-items-center w-10 h-10 rounded-full bg-sunk text-danger">
              <CircleAlert size={20} strokeWidth={1.75} />
            </span>
            <b>Không tải được phạm vi hiển thị</b>
            <p className="m-0 text-sm text-ink-muted max-w-80">Kết nối tới máy chủ bị gián đoạn. Kiểm tra mạng rồi thử lại.</p>
            <Button variant="soft" size="sm" className="max-sm:h-11" onClick={() => void rules.refetch()}>
              <RefreshCw size={16} strokeWidth={1.75} aria-hidden="true" />
              Thử lại
            </Button>
          </div>
        ) : (
          <div className="grid gap-4 px-5 pt-3 pb-5 content-start">
            <p className="m-0 text-ink-muted max-w-[65ch]">
              Mặc định mọi thành viên thấy toàn bộ danh bạ. Quản trị viên luôn thấy tất cả.
            </p>
            <div className="grid grid-cols-[repeat(auto-fit,minmax(min(320px,100%),1fr))] gap-3 items-start">
              <RuleList
                label="Phòng ẩn khỏi danh bạ"
                count={hidden.length}
                hint="Chỉ người trong phòng (cả các phòng bên dưới) và quản trị viên thấy."
                empty="Chưa ẩn phòng nào. Mọi thành viên thấy mọi phòng."
                loading={loading || notReady}
                add={
                  <DirectoryScopePicker
                    label="Ẩn một phòng"
                    icon={<Folder size={16} strokeWidth={1.75} aria-hidden="true" />}
                    dialogLabel="Chọn phòng để ẩn"
                    sheetTitle="Ẩn một phòng"
                    searchLabel="Tìm phòng"
                    listLabel="Phòng ban"
                    groups={hideOptions}
                    disabled={loading || !departments.data}
                    onPick={askHide}
                  />
                }
              >
                {hidden.map((d) => {
                  const path = parentPath(d)
                  return (
                    <RuleRow
                      key={d.id}
                      kind="department"
                      title={d.name}
                      line={`${path ? `${path} · ` : ''}Chỉ ${d.member_count} người trong phòng và quản trị viên thấy`}
                      button={
                        <Button
                          variant="soft"
                          size="sm"
                          className="max-sm:h-11"
                          aria-label={`Bỏ ẩn ${d.name}`}
                          onClick={() => setAsk({ rule: 'unhide', id: d.id, name: d.name, people: d.member_count })}
                        >
                          Bỏ ẩn
                        </Button>
                      }
                    />
                  )
                })}
              </RuleList>

              <RuleList
                label="Chỉ thấy phòng của mình"
                count={limited.length}
                hint="Người giữ vai trò hoặc ở phòng này chỉ thấy người trong phòng của họ và quản trị viên."
                empty="Chưa giới hạn ai. Mọi thành viên thấy toàn bộ danh bạ."
                loading={loading || notReady}
                add={
                  <DirectoryScopePicker
                    label="Thêm vai trò hoặc phòng"
                    icon={<Shield size={16} strokeWidth={1.75} aria-hidden="true" />}
                    dialogLabel="Chọn vai trò hoặc phòng"
                    sheetTitle="Thêm vai trò hoặc phòng"
                    searchLabel="Tìm vai trò hoặc phòng"
                    listLabel="Vai trò hoặc phòng"
                    groups={limitOptions}
                    footer="Chủ sở hữu và Thành viên không giới hạn được. Quản trị viên luôn thấy tất cả."
                    disabled={loading || !departments.data || !roles.data}
                    onPick={askLimit}
                  />
                }
              >
                {limited.map((l) => {
                  const path = l.kind === 'department' ? parentPath(l) : ''
                  const title = `${l.kind === 'role' ? 'Vai trò' : 'Phòng'} ${l.name}`
                  return (
                    <RuleRow
                      key={`${l.kind}:${l.id}`}
                      kind={l.kind}
                      title={title}
                      line={`${path ? `${path} · ` : ''}${l.member_count} người · thấy người trong phòng của họ`}
                      button={
                        <Button
                          variant="soft"
                          size="sm"
                          className="max-sm:h-11"
                          aria-label={`Bỏ giới hạn cho ${lowerFirst(title)}`}
                          onClick={() => setAsk({ rule: 'unlimit', kind: l.kind, id: l.id, name: l.name, people: l.member_count })}
                        >
                          Bỏ
                        </Button>
                      }
                    />
                  )
                })}
              </RuleList>
            </div>
          </div>
        )}
      </div>

      <ConfirmDialog
        open={!!ask}
        onClose={() => !pending && setAsk(null)}
        title={text?.title ?? ''}
        description={text?.body}
        confirmLabel={text?.verb}
        loading={pending}
        onConfirm={() => ask && run(ask)}
      />
    </AdminFrame>
  )
}
