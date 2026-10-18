import { Link } from '@tanstack/react-router'
import { ChevronDown, Settings } from 'lucide-react'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { useOpenInvitationCount } from '../../hooks/useInvitations'
import { countUnit, type BarModule, type BarModuleId } from '../../lib/nav-destinations'
import { workspaceDisplayName } from '../../lib/workspace'
import { useUiStore } from '../../stores/ui.store'
import { Pressable, spaceLetter } from '../primitives'
import { useSettingsNav } from '../settings/settings-nav'
import { NavSheet } from '../spaces/NavSheet'
import { CountPill } from './CountPill'
import { useWorkspaceKindLine } from './WorkspaceSwitcherSheet'

const TILE = `relative grid place-items-center w-14 h-14 rounded-2xl bg-sunk text-ink transition-colors duration-quick
  motion-reduce:transition-none`
const CELL = `grid justify-items-center content-start gap-1.5 min-h-24 py-1 rounded-xl text-center text-[0.8125rem] font-medium
  leading-tight text-ink no-underline focus-ring`

/**
 * Thêm (DESIGN.md §5, Điều hướng di động; mockup mobile-shell.html §1): the workspace you are in, with its ⚙
 * opening Cài đặt workspace (where Quản trị lives on a phone), and every module the workspace offers as a grid
 * with Thông báo. The ones on the bar carry an accent ring. It replaces the ☰ menu; Sửa thanh đáy opens from
 * its heading. Render it only while open.
 */
export function MoreSheet({ modules, pinned, news, onClose, onEdit, onSwitchWorkspace, onOpenNotifications }: {
  modules: BarModule[]
  pinned: BarModuleId[]
  news: Record<BarModuleId, number>
  onClose: () => void
  onEdit: () => void
  onSwitchWorkspace: () => void
  onOpenNotifications: () => void
}) {
  const { workspaceName } = useActiveWorkspace()
  const setActiveModule = useUiStore((s) => s.setActiveModule)
  const settings = useSettingsNav()
  const offers = useOpenInvitationCount()
  const kind = useWorkspaceKindLine()
  const name = workspaceDisplayName(workspaceName)

  const tile = (m: BarModule) => {
    const count = news[m.id]
    const isPinned = pinned.includes(m.id)
    const label = count > 0 ? `${m.label}, ${count} ${countUnit(m.id)}` : m.label
    const inner = (
      <>
        <span className={`${TILE} ${isPinned ? 'ring-2 ring-inset ring-accent' : ''}`}>
          <m.icon size={24} strokeWidth={1.75} aria-hidden="true" />
          {count > 0 && (
            <span className="absolute -top-1 -right-1"><CountPill n={count} /></span>
          )}
        </span>
        {m.label}
      </>
    )
    if (m.to) {
      return (
        <Link
          key={m.id}
          to={m.to}
          aria-label={label}
          onClick={() => { if (m.module) setActiveModule(m.module); onClose() }}
          className={CELL}
        >
          {inner}
        </Link>
      )
    }
    return (
      <Pressable key={m.id} aria-label={label} onClick={() => { onClose(); onOpenNotifications() }} className={CELL}>
        {inner}
      </Pressable>
    )
  }

  return (
    <NavSheet label="Thêm" onClose={onClose}>
      {/* The row switches workspace; the ⚙ sits on top of it, a sibling, so neither button is inside the other. */}
      <div className="grid grid-cols-[2.5rem_minmax(0,1fr)_2.75rem_1.25rem] items-center gap-3 min-h-16 px-3 py-2 rounded-2xl bg-sunk">
        <Pressable
          onClick={onSwitchWorkspace}
          aria-haspopup="dialog"
          aria-label={`Đổi workspace, đang ở ${name}${offers > 0 ? ', có lời mời' : ''}`}
          className="col-span-full row-start-1 grid grid-cols-subgrid items-center -mx-3 -my-2 px-3 py-2 min-h-16 rounded-2xl text-left"
        >
          <span aria-hidden="true" className="relative grid place-items-center w-10 h-10 rounded-surface bg-accent text-on-accent font-display text-lg font-bold">
            {spaceLetter(name)}
            {offers > 0 && <span className="absolute -top-0.5 -right-0.5 w-3 h-3 rounded-full bg-accent ring-2 ring-sunk" />}
          </span>
          <span className="grid min-w-0 gap-0.5">
            <span className="font-display text-[0.9375rem] font-bold text-ink truncate">{name}</span>
            <span className="text-xs text-ink-muted truncate">{kind}</span>
          </span>
          <span />
          <ChevronDown size={20} strokeWidth={1.75} aria-hidden="true" className="text-ink-muted" />
        </Pressable>
        <Pressable
          onClick={() => { settings.open('workspace'); onClose() }}
          aria-label="Cài đặt workspace"
          className="col-start-3 row-start-1 relative grid place-items-center w-11 h-11 rounded-full text-ink-muted
            hover:bg-hover active:bg-hover focus-ring"
        >
          <Settings size={20} strokeWidth={1.75} aria-hidden="true" />
        </Pressable>
      </div>

      <div className="flex items-center justify-between gap-3 px-3.5 pt-3 pb-1">
        <h2 className="m-0 font-display text-[1.0625rem] font-bold text-ink">Tất cả phân hệ</h2>
        <Pressable onClick={onEdit} aria-haspopup="dialog" className="h-9 px-2 text-sm font-semibold text-accent">
          Sửa thanh đáy
        </Pressable>
      </div>
      <nav aria-label="Tất cả phân hệ" className="grid grid-cols-4 gap-x-1 px-1">
        {modules.map(tile)}
      </nav>
      <p className="m-0 px-3.5 pt-1 pb-2 text-xs text-ink-muted">
        Viền màu là phân hệ đang ở thanh đáy. Cài đặt nằm ở Tôi (chạm avatar) và trong ⋯ của từng phân hệ.
      </p>
    </NavSheet>
  )
}
