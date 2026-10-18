import { useId, useRef, useState } from 'react'
import { Building2, ChevronRight, Folder, MoreHorizontal, Shield, UserPlus, Users, UsersRound, type LucideIcon } from 'lucide-react'
import type { Contact } from '../../hooks/useContacts'
import { useLongPress } from '../../hooks/useLongPress'
import { Avatar, IconButton, Pressable } from '../primitives'
import { ModuleHeader } from '../patterns/ModuleHeader'
import { NavSheet, SheetItem } from '../spaces/NavSheet'
import type { MyDepartment } from './CategoryItem'
import { nameOf, type CategoryDef } from './contacts-model'
import { PersonRow } from './PersonRow'

/** People listed under Đang trực tuyến on the first screen; the rest are behind Xem tất cả. */
export const ONLINE_ON_HOME = 5

const ICONS: Record<CategoryDef['id'], LucideIcon> = { org: Building2, roles: Shield, groups: UsersRound, members: Users }

interface ContactsCategoryHomeProps {
  categories: CategoryDef[]
  onOpen: (cat: CategoryDef['id']) => void
  /** The caller's department in an organization; a personal workspace has none. */
  myDepartment?: MyDepartment
  onOpenMine?: () => void
  /** The top bar's search was pressed: the search screen opens. */
  onSearch: () => void
  /** Thường liên lạc: recent direct-message partners the viewer's scope shows (`useFrequentContacts`). */
  frequent: Contact[]
  /** Who is online now, from the scoped directory, the viewer left out. */
  online: Contact[]
  isOnline: (c: Contact) => boolean
  onOpenPerson: (c: Contact) => void
  /** A long press on a Thường liên lạc avatar: the direct message opens. */
  onMessagePerson: (c: Contact) => void
  onSeeAllOnline: () => void
  /** The viewer may invite people (an administrator): ⋯ offers Mời thành viên. */
  canInvite: boolean
  onInvite: () => void
}

/** The name under a Thường liên lạc avatar: the last two words ("Lê Quang Vinh" is "Quang Vinh"). */
const shortName = (name: string) => name.trim().split(/\s+/).slice(-2).join(' ')

function FrequentPerson({ person, online, onOpen, onMessage }: {
  person: Contact; online: boolean; onOpen: () => void; onMessage: () => void
}) {
  // The click that ends a long press is not a tap.
  const held = useRef(false)
  const press = useLongPress(() => { held.current = true; onMessage() }, true, 'input, textarea')
  const name = nameOf(person)
  return (
    <Pressable
      {...press}
      onPointerDown={(e) => { held.current = false; press.onPointerDown(e) }}
      onClick={() => { if (held.current) held.current = false; else onOpen() }}
      aria-label={name}
      className="press grid justify-items-center gap-1.5 w-15 shrink-0 p-0 border-none bg-transparent text-center cursor-pointer rounded-lg focus-ring"
    >
      <Avatar name={name} hueKey={person.user_id} src={person.avatar_url || undefined} size={52} online={online} />
      <span aria-hidden="true" className="w-full text-xs leading-tight text-ink line-clamp-2">{shortName(name)}</span>
    </Pressable>
  )
}

function Tile({ icon: Icon, hue, title, sub, onOpen }: { icon: LucideIcon; hue: number; title: string; sub: string; onOpen: () => void }) {
  return (
    <Pressable
      onClick={onOpen}
      className="press grid content-start gap-2 min-w-0 p-3.5 rounded-2xl border-none bg-raised text-left cursor-pointer focus-ring
        transition-colors duration-quick hover:bg-hover"
    >
      <span
        aria-hidden="true"
        className="grid place-items-center w-10 h-10 rounded-xl"
        style={{ background: `var(--color-person-${hue}-wash)`, color: `var(--color-person-${hue})` }}
      >
        <Icon size={20} strokeWidth={1.75} />
      </span>
      <span className="grid min-w-0">
        <span className="truncate text-md font-semibold text-ink">{title}</span>
        <span className="truncate text-small text-ink-muted">{sub}</span>
      </span>
    </Pressable>
  )
}

/**
 * The first screen of Danh bạ on a phone (DESIGN.md "Danh bạ" › Điện thoại, mockup `mobile-shell.html` §12): the
 * title and ⋯, Thường liên lạc (hidden when the viewer has messaged nobody), the categories as a two-column grid
 * with Phòng của tôi after Phòng ban when the viewer sits in a department, and Đang trực tuyến · N with at most five
 * people and Xem tất cả. Everyone shown comes from the scoped directory: a person the scope hides is never here. The
 * top bar's search (☰ pill) is pointed at a search control here, which opens the search screen.
 */
export function ContactsCategoryHome(p: ContactsCategoryHomeProps) {
  const [menu, setMenu] = useState(false)
  const frequentId = useId()
  const onlineId = useId()

  const tiles = p.categories.flatMap((def) => {
    const tile = <Tile key={def.id} icon={ICONS[def.id]} hue={def.hue} title={def.title} sub={def.sub} onOpen={() => p.onOpen(def.id)} />
    if (def.id !== 'org' || !p.myDepartment || !p.onOpenMine) return [tile]
    const mine = p.myDepartment
    const sub = mine.count !== undefined ? `${mine.name} · ${mine.count} người` : mine.name
    return [tile, <Tile key="mine" icon={Folder} hue={5} title="Phòng của tôi" sub={sub} onOpen={p.onOpenMine} />]
  })
  const online = p.online.slice(0, ONLINE_ON_HOME)

  return (
    <div className="flex flex-col flex-1 min-h-0 overflow-y-auto bg-base">
      <ModuleHeader
        moduleId="contacts"
        title="Danh bạ"
        actions={p.canInvite ? (
          <IconButton aria-label="Thêm tuỳ chọn Danh bạ" title="Thêm tuỳ chọn" shape="round" size="lg" className="w-11 h-11 -mr-2"
            aria-haspopup="dialog" onClick={() => setMenu(true)}>
            <MoreHorizontal size={20} strokeWidth={1.75} aria-hidden="true" />
          </IconButton>
        ) : undefined}
      />

      {p.frequent.length > 0 && (
        <section aria-labelledby={frequentId} className="grid pt-3">
          <h2 id={frequentId} className="m-0 px-4 pb-1.5 text-small font-semibold text-ink-muted">Thường liên lạc</h2>
          <ul
            aria-label="Thường liên lạc"
            className="m-0 flex gap-3.5 overflow-x-auto px-4 pt-1.5 pb-2.5 list-none [scrollbar-width:none]
              [mask-image:linear-gradient(90deg,#000_88%,transparent)]"
          >
            {p.frequent.map((c) => (
              <li key={c.user_id} className="shrink-0">
                <FrequentPerson person={c} online={p.isOnline(c)} onOpen={() => p.onOpenPerson(c)} onMessage={() => p.onMessagePerson(c)} />
              </li>
            ))}
          </ul>
        </section>
      )}

      <nav aria-label="Danh mục" className="px-4 pt-2 pb-2">
        <div className="grid grid-cols-2 gap-2.5">{tiles}</div>
      </nav>

      {p.online.length > 0 && (
        <section aria-labelledby={onlineId} className="grid px-2 pt-2 pb-6">
          <h2 id={onlineId} className="m-0 px-2 pb-1 text-small font-semibold text-ink-muted">
            Đang trực tuyến · <span className="tnum">{p.online.length}</span>
          </h2>
          <ul className="m-0 p-0 list-none grid gap-0.5">
            {online.map((c) => (
              <li key={c.user_id}><PersonRow person={c} online tall onSelect={() => p.onOpenPerson(c)} /></li>
            ))}
          </ul>
          {p.online.length > ONLINE_ON_HOME && (
            <Pressable
              onClick={p.onSeeAllOnline}
              className="flex items-center justify-between min-h-12 mx-0 px-2 rounded-lg border-none bg-transparent text-left font-medium text-accent
                cursor-pointer hover:bg-hover focus-ring"
            >
              Xem tất cả
              <ChevronRight size={16} strokeWidth={1.75} aria-hidden="true" />
            </Pressable>
          )}
        </section>
      )}

      {menu && (
        <NavSheet label="Danh bạ" onClose={() => setMenu(false)}>
          <SheetItem icon={<UserPlus size={18} strokeWidth={1.75} />} onClick={() => { setMenu(false); p.onInvite() }}>Mời thành viên</SheetItem>
        </NavSheet>
      )}

      {/* Not drawn: the pill of the top bar finds the screen's search by this mark and presses it. */}
      <Pressable hidden aria-hidden="true" tabIndex={-1} data-module-search onClick={p.onSearch} />
    </div>
  )
}
