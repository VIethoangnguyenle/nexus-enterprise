import { Children, isValidElement, useId, useRef, type ReactNode, type Ref } from 'react'
import { ArrowLeft, MessageSquare, RefreshCw, Search } from 'lucide-react'
import type { BrowseView } from '../../hooks/useContactsBrowse'
import type { Contact } from '../../hooks/useContacts'
import type { PeopleDirectory } from '../../lib/people'
import { Button } from '../primitives'
import { SwipeRow, SwipeRowGroup, type SwipeAction } from '../patterns/SwipeRow'
import { AlphabetIndex } from './AlphabetIndex'
import { indexLettersOf, letterOf } from './alphabet'
import { ContactsBreadcrumb } from './ContactsBreadcrumb'
import { DepartmentRow } from './DepartmentRow'
import { GroupRow, HiddenMemberRow, RoleRow } from './ListRows'
import { InlineAlert, RowSkeletons, StateCard } from './ContactsStates'
import { PersonRow } from './PersonRow'
import { nameOf, pathSegments, rootLabel } from './contacts-model'

interface ContactsBrowserProps {
  view: BrowseView
  /** `column`: the middle column of a desktop or tablet, with breadcrumbs and the title. `phone`: the list under the phone's own back bar. */
  variant: 'column' | 'phone'
  workspaceName: string
  isPersonal: boolean
  /** The words typed in the search box. */
  query: string
  onlineOnly: boolean
  isOnline: (c: Contact) => boolean
  meId?: string
  selectedId?: string
  /** The directory, for a member of a group that it hides. */
  people: PeopleDirectory
  /** The search box and the online chip; not drawn when null. */
  tools: ReactNode
  /** The name of the level, where focus lands when nothing else is left to take it. */
  headingRef?: Ref<HTMLHeadingElement>
  /** A layer over the list (the search suggestions on a phone): the list stays mounted under it, so going back finds it where it was. */
  cover?: ReactNode
  /** Phòng ban failed to load: say it here with a retry (a desktop says it under the category instead). */
  retryTreeHere: boolean
  onOpenDept: (id: string | undefined) => void
  onOpenRoles: () => void
  onOpenRole: (id: string) => void
  onOpenGroups: () => void
  onOpenGroup: (id: string) => void
  onOpenChat: (channelId: string) => void
  onSelectPerson: (c: Contact, opener: HTMLElement) => void
  onClearSearch: () => void
  onShowEveryone: () => void
  /** A phone's rows: 60 high with avatar 40. */
  tallRows?: boolean
  /** What a swipe to the left on a person's row reveals (a phone: Nhắn tin · Gọi); the signed-in user's own row has none. */
  rowActions?: (c: Contact) => SwipeAction[]
  /** From this many people a list of a level gets the A–Z column; never when absent. */
  alphabetFrom?: number
  /** A search with nothing typed still lists what its filters keep (a phone's place or online filter). */
  listWithoutQuery?: boolean
  /** A line over a phone's search results ("Kết quả trong … · N người khớp"). */
  searchHint?: ReactNode
}

/** A labelled group of rows: "Khối · 2", "Thành viên · 14". */
function Section({ label, count, children }: { label: string; count: number; children: ReactNode }) {
  const id = useId()
  return (
    <section aria-labelledby={id} className="grid">
      <h3 id={id} className="m-0 px-2 pt-3 pb-1 text-small font-semibold text-ink-muted">{label} · {count}</h3>
      <ul className="m-0 p-0 list-none grid gap-0.5">
        {/* Keyed by what each row is (a person, a department), never by its place: rows that stay keep their element. */}
        {Children.toArray(children).map((child, i) => <li key={isValidElement(child) && child.key != null ? child.key : i}>{child}</li>)}
      </ul>
    </section>
  )
}

/**
 * The middle column of Danh bạ: one level of the open category per screen, with the way back above it
 * (DESIGN.md "Danh bạ"). Everything it shows comes from `view`; it asks for nothing itself.
 */
export function ContactsBrowser(p: ContactsBrowserProps) {
  const { view } = p
  const showPath = !p.isPersonal && (view.kind === 'search' || view.kind === 'role' || view.kind === 'group')
  const searchQuery = view.kind === 'search' ? p.query : undefined
  const scopeWords = view.limited ? 'trong phạm vi bạn được xem' : `trong cả ${p.workspaceName}`

  const scrollRef = useRef<HTMLDivElement>(null)

  const personRow = (c: Contact) => {
    const row = (
      <PersonRow
        person={c}
        online={p.isOnline(c)}
        path={showPath ? pathSegments(view.tree, c) : undefined}
        selected={c.user_id === p.selectedId}
        isMe={c.user_id === p.meId}
        query={searchQuery}
        onSelect={p.onSelectPerson}
        tall={p.tallRows}
      />
    )
    const actions = c.user_id !== p.meId ? p.rowActions?.(c) : undefined
    return actions && actions.length > 0 ? <SwipeRow id={c.user_id} end={actions}>{row}</SwipeRow> : row
  }

  // --- the A–Z column: the first row of each letter carries it, and the column jumps there ---
  let letters: string[] = []
  const marked = (names: string[]) => {
    const seen = new Set<string>()
    return names.map((n) => {
      const letter = letterOf(n)
      if (seen.has(letter)) return undefined
      seen.add(letter)
      return letter
    })
  }
  const alphabetOn = (count: number) => p.alphabetFrom !== undefined && count >= p.alphabetFrom
    && (view.kind === 'dept' || view.kind === 'role' || view.kind === 'group' || view.kind === 'members')
  const jump = (letter: string) => {
    const target = scrollRef.current?.querySelector<HTMLElement>(`[data-az="${CSS.escape(letter)}"]`)
    target?.scrollIntoView?.({ block: 'start' })
  }

  // --- header (a column only: the phone's back bar carries the title) ---
  const header = (() => {
    if (p.variant !== 'column') return null
    const end = view.kind === 'group' && view.group ? (
      <Button variant="soft" size="sm" onClick={() => p.onOpenChat(view.group!.id)}>
        <MessageSquare size={16} strokeWidth={1.75} aria-hidden="true" />
        Mở nhóm
      </Button>
    ) : null
    return (
      <header className="grid gap-2 min-w-0 px-5 pt-3.5 pb-2">
        {view.kind === 'dept' && (
          <ContactsBreadcrumb rootLabel={rootLabel(p.workspaceName, view.limited)} segments={view.trail} onOpen={p.onOpenDept} />
        )}
        {view.kind === 'role' && <ContactsBreadcrumb leading={{ label: 'Vai trò', onPress: p.onOpenRoles }} current={view.title} />}
        {view.kind === 'group' && <ContactsBreadcrumb leading={{ label: 'Nhóm của tôi', onPress: p.onOpenGroups }} current={view.title} />}
        {view.kind === 'search' && (
          <div className="flex items-center gap-1.5 text-small text-ink-muted">
            <Search size={14} strokeWidth={1.75} aria-hidden="true" />
            <span>{view.limited ? 'Kết quả trong phạm vi bạn được xem' : `Kết quả trong cả ${p.workspaceName}`}</span>
          </div>
        )}
        <div className="flex items-baseline gap-2.5 min-w-0">
          <h2 ref={p.headingRef} tabIndex={-1} className="m-0 min-w-0 truncate font-display text-lg font-bold text-ink outline-none">{view.title}</h2>
          <span className="shrink-0 text-small text-ink-muted tnum">{view.sub}</span>
          {end && <span className="ml-auto self-center shrink-0">{end}</span>}
        </div>
        {p.tools}
      </header>
    )
  })()

  // --- body ---
  const body = (() => {
    // Nothing typed lists nobody (there is no list of everyone to fall back on), and there is nothing to wait for.
    if (view.kind === 'search' && !p.query.trim() && !p.listWithoutQuery) {
      return <p className="m-0 px-2 pt-6 text-center text-ink-muted">Gõ tên, chức danh hoặc email để tìm người.</p>
    }
    if (view.status === 'loading') {
      const shapes = view.kind === 'dept' || view.kind === 'roles' || view.kind === 'groups'
        ? (['dept', 'dept', 'person', 'person', 'person', 'person'] as const) : (['person', 'person', 'person', 'person', 'person', 'person'] as const)
      return <RowSkeletons shapes={[...shapes]} />
    }
    if (view.status === 'error') {
      return (
        <StateCard
          tone="error"
          title={view.kind === 'groups' || view.kind === 'group' ? 'Không tải được nhóm của bạn' : 'Không tải được danh bạ'}
          text="Kết nối tới máy chủ bị gián đoạn. Kiểm tra mạng rồi thử lại."
          action={{ label: 'Thử lại', onPress: view.retry, icon: <RefreshCw size={16} strokeWidth={1.75} aria-hidden="true" /> }}
        />
      )
    }
    if (view.status === 'tree-error') {
      return p.retryTreeHere ? (
        <div className="px-2 pt-3">
          <InlineAlert text="Không tải được cây phòng ban. Bạn vẫn tìm được người bằng ô tìm." onRetry={view.retry} />
        </div>
      ) : (
        <StateCard title="Chưa duyệt được theo phòng ban" text="Bạn vẫn tìm được người bằng ô tìm." />
      )
    }

    const everyone = view.all.length === 0 && view.kind !== 'groups' && view.kind !== 'group' && view.kind !== 'roles'
    if (everyone) {
      return <StateCard icon="users" title="Workspace này chưa có ai khác" text="Mời đồng nghiệp ở Quản trị để họ hiện ở đây." />
    }

    switch (view.kind) {
      case 'roles':
        return view.roles.length === 0
          ? <StateCard icon="users" title="Chưa có vai trò nào" text="Khi quản trị viên tạo vai trò, chúng sẽ hiện ở đây." />
          : <ul className="m-0 p-0 list-none grid gap-0.5">{view.roles.map((r) => <li key={r.id}><RoleRow role={r} onOpen={p.onOpenRole} /></li>)}</ul>
      case 'groups':
        return view.groups.length === 0
          ? <StateCard icon="users" title="Bạn chưa ở nhóm nào" text="Khi bạn vào một nhóm chat, nhóm sẽ hiện ở đây." />
          : <ul className="m-0 p-0 list-none grid gap-0.5">{view.groups.map((g) => <li key={g.id}><GroupRow group={g} onOpen={p.onOpenGroup} /></li>)}</ul>
      case 'group': {
        const byId = new Map(view.all.map((c) => [c.user_id, c]))
        const shown = view.members.filter((m) => {
          if (!p.onlineOnly) return true
          const c = byId.get(m.userId)
          return !!c && p.isOnline(c)
        })
        if (shown.length === 0) {
          return p.onlineOnly
            ? <OnlineEmpty where={view.title} onShowEveryone={p.onShowEveryone} />
            : <StateCard icon="users" title={`${view.title} chưa có ai`} text="Khi có người vào nhóm, họ sẽ hiện ở đây." />
        }
        const groupMarks = alphabetOn(shown.length) ? marked(shown.map((m) => m.name)) : []
        if (groupMarks.length > 0) letters = indexLettersOf(shown.map((m) => m.name))
        return (
          <Section label={p.onlineOnly ? 'Thành viên đang trực tuyến' : 'Thành viên'} count={shown.length}>
            {shown.map((m, i) => {
              const c = byId.get(m.userId)
              return (
                <div key={m.userId} data-az={groupMarks[i]}>
                  {c ? personRow(c) : <HiddenMemberRow member={m} people={p.people} />}
                </div>
              )
            })}
          </Section>
        )
      }
      default:
    }

    // dept, role, search, members: people, and for a department the departments before them
    const shownPeople = p.onlineOnly ? view.people.filter(p.isOnline) : view.people
    const parts: ReactNode[] = []
    if (view.kind === 'dept' && view.depts.length > 0) {
      parts.push(
        <Section key="depts" label={view.atRoot ? 'Khối' : 'Phòng ban'} count={view.depts.length}>
          {view.depts.map((d) => <DepartmentRow key={d.id} node={d} onOpen={(id) => p.onOpenDept(id)} />)}
        </Section>,
      )
    }
    if (shownPeople.length > 0) {
      const base = view.kind === 'search' ? 'Người' : view.kind === 'dept' && view.atRoot ? 'Chưa thuộc phòng ban nào' : 'Thành viên'
      const marks = alphabetOn(shownPeople.length) ? marked(shownPeople.map(nameOf)) : []
      if (marks.length > 0) letters = indexLettersOf(shownPeople.map(nameOf))
      parts.push(
        <Section key="people" label={p.onlineOnly ? `${base} đang trực tuyến` : base} count={shownPeople.length}>
          {shownPeople.map((c, i) => <div key={c.user_id} data-az={marks[i]}>{personRow(c)}</div>)}
        </Section>,
      )
    }
    if (parts.length > 0) return <>{parts}</>

    if (view.kind === 'search') {
      if (p.onlineOnly && view.people.length > 0) return <OnlineEmpty where="đây" onShowEveryone={p.onShowEveryone} />
      return (
        <StateCard
          icon="search"
          title={`Không có ai khớp ${view.title} ${scopeWords}`}
          text="Thử tên khác, chức danh hoặc email."
          action={{ label: 'Xoá tìm kiếm', onPress: p.onClearSearch }}
        />
      )
    }
    if (p.onlineOnly && view.people.length > 0) {
      return <OnlineEmpty where={view.kind === 'dept' ? view.title : 'đây'} onShowEveryone={p.onShowEveryone} />
    }
    if (view.kind === 'dept' && !view.atRoot) {
      const parent = view.trail.length > 1 ? view.trail[view.trail.length - 2]! : undefined
      const back = parent ? parent.name : rootLabel(p.workspaceName, view.limited)
      return (
        <StateCard
          title={`${view.title} chưa có ai`}
          text="Khi quản trị viên xếp người vào phòng này, họ sẽ hiện ở đây."
          action={{
            label: `Về ${back}`, onPress: () => p.onOpenDept(parent?.id),
            icon: <ArrowLeft size={16} strokeWidth={1.75} aria-hidden="true" />,
          }}
        />
      )
    }
    return null
  })()

  return (
    <section aria-label="Danh sách" className="flex flex-col flex-1 min-w-0 min-h-0 bg-base">
      {header}
      {p.variant === 'phone' && p.tools && <div className="px-4 pt-1 pb-2">{p.tools}</div>}
      {p.variant === 'phone' && p.searchHint && !p.cover && <p className="m-0 px-4 pb-1 text-small text-ink-muted">{p.searchHint}</p>}
      <div className="relative flex-1 min-h-0">
        <div
          ref={scrollRef}
          className={`h-full overflow-y-auto overflow-x-hidden px-3 pb-4 ${letters.length > 0 ? 'pr-6' : ''}`}
          aria-hidden={p.cover ? true : undefined}
          inert={p.cover ? true : undefined}
        >
          {p.rowActions ? <SwipeRowGroup>{body}</SwipeRowGroup> : body}
        </div>
        {letters.length > 0 && !p.cover && <AlphabetIndex letters={letters} onJump={jump} />}
        {p.cover && <div className="absolute inset-0 grid bg-base">{p.cover}</div>}
      </div>
    </section>
  )
}

function OnlineEmpty({ where, onShowEveryone }: { where: string; onShowEveryone: () => void }) {
  return (
    <StateCard
      icon="users"
      title={where === 'đây' ? 'Không ai ở đây đang trực tuyến' : `Không ai ở ${where} đang trực tuyến`}
      text="Bạn vẫn nhắn được; họ thấy tin khi quay lại."
      action={{ label: 'Hiện mọi người', onPress: onShowEveryone }}
    />
  )
}
