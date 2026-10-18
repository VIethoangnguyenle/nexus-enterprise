import { useCallback, useDeferredValue, useEffect, useMemo, useRef, useState } from 'react'
import { useCanGoBack, useNavigate, useRouter, useRouterState, useSearch } from '@tanstack/react-router'
import { AnimatePresence, motion } from 'motion/react'
import { ArrowLeft, MessageSquare, Phone, Search, X } from 'lucide-react'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { useDeviceBack } from '../../hooks/useDeviceBack'
import { useConnectionLost } from '../../hooks/useConnectionLost'
import { levelContacts, useContactsBrowse, type BrowseView } from '../../hooks/useContactsBrowse'
import { useDepartmentTree, useGroupMembers } from '../../hooks/useContactCategories'
import type { Contact } from '../../hooks/useContacts'
import { useStartCall } from '../../hooks/useCalls'
import { useCreateDM } from '../../hooks/useMessaging'
import { usePeople } from '../../hooks/usePeople'
import { useBottomBarLayout, usePhone } from '../../hooks/usePhone'
import { useFrequentContacts, useRecentDirects } from '../../hooks/useRecentDirects'
import { useRecentSearches } from '../../hooks/useRecentSearches'
import { useSuggestionPanel } from '../../hooks/useSuggestionPanel'
import { useWorkspaceType } from '../../hooks/useWorkspaceType'
import { useWorkspaceDetails } from '../../hooks/useWorkspaces'
import { callRoute } from '../../lib/call-nav'
import {
  categorySearch, closePersonSearch, deptSearch, groupSearch, homeSearch, openPersonSearch, roleSearch,
  type ContactsCat, type ContactsSearch,
} from '../../lib/contacts-search'
import { explain } from '../../lib/errors'
import { useMotionPresets } from '../../lib/motion'
import { recentlyMessaged } from '../../lib/suggestions/contacts'
import { workspaceDisplayName } from '../../lib/workspace'
import { stashTicket } from '../../stores/call.store'
import { useAuthStore } from '../../stores/auth.store'
import { useWebSocketStore } from '../../stores/websocket.store'
import { Avatar, FilterChip, IconButton, toast } from '../primitives'
import { SearchBox } from '../patterns/SearchBox'
import { SearchSuggestions } from '../patterns/SearchSuggestions'
import { ModuleHeader } from '../patterns/ModuleHeader'
import type { SwipeAction } from '../patterns/SwipeRow'
import { ALPHABET_FROM } from './alphabet'
import type { MyDepartment } from './CategoryItem'
import { ContactProfile, type ProfileMode } from './ContactProfile'
import { ContactPhoneProfile } from './ContactPhoneProfile'
import { ContactProfileSheet } from './ContactProfileSheet'
import { ContactsBrowser } from './ContactsBrowser'
import { ContactsCategoryHome } from './ContactsCategoryHome'
import { ContactsChips } from './ContactsChips'
import { ContactsNav } from './ContactsNav'
import { ContactsSearchFilters } from './ContactsSearchFilters'
import { PhoneBackBar, PhoneLayer, phonePush } from './ContactsPhone'
import { PathTrail } from './PathTrail'
import { categoriesOf, colleaguesOf, nameOf, parentPathLabel, pathSegments, roleLine } from './contacts-model'
import { phoneBarOf } from './phone-bar'
import { inPlace, placeOfLevel, type PlaceFilter } from './place-filter'

/** What the open search says while there is nothing to suggest. */
const SEARCH_HINT = 'Gõ tên, chức danh hoặc email để tìm người.'

/** The category the address names, if it names one this workspace has. */
function namedCategory(search: ContactsSearch, isPersonal: boolean): ContactsCat | undefined {
  const c = search.cat
  if (!c || c === 'search') return c
  return (isPersonal ? c === 'org' || c === 'roles' : c === 'members') ? undefined : c
}

/**
 * Danh bạ (design/mockups/contacts-lark.html): categories, one level of the open category per screen, and the
 * profile. A desktop has the categories in a list panel, the level in the middle and the profile beside it; a
 * tablet turns the list panel into chips and the profile into a sheet; a phone is a stack of screens (the
 * categories, then a category or a department, then a profile), one history entry each.
 *
 * The place is in the address (`cat`, `dept`, `role`, `chat`, `person`, `q`), so back, forward and links land
 * where the person was. Nothing but the directory is asked for until a category is opened.
 */
export function ContactsScreen() {
  const navigate = useNavigate()
  const router = useRouter()
  const canGoBack = useCanGoBack()
  // This entry was pushed by Danh bạ from another place of Danh bạ: the entry before it is a level of the stack.
  const stacked = useRouterState({ select: (st) => !!(st.location.state as unknown as Record<string, unknown>).contactsStack })
  /**
   * The last move of the history was a back by the device (the system gesture or button), and nothing of Danh bạ's own
   * has moved since. The device has drawn that step already; animating it again, a level fading in from nothing and
   * the layer left fading out over the page, is the flash. `ownBack` marks Danh bạ's own back (its button, or Esc), whose
   * animation is Danh bạ's to draw.
   */
  const { byDevice, ownBack, ownStep } = useDeviceBack()
  const m = useMotionPresets()
  const search = useSearch({ strict: false }) as ContactsSearch
  const connectionLost = useConnectionLost()
  const bottomBar = useBottomBarLayout()
  const phone = usePhone()
  const { workspaceId: wsId, workspaceName } = useActiveWorkspace()
  const { isPersonal } = useWorkspaceType()
  const me = useAuthStore((s) => s.user)
  const onlineUsers = useWebSocketStore((s) => s.onlineUsers)
  const people = usePeople(wsId)
  const createDM = useCreateDM()
  const startCall = useStartCall()
  const workspace = workspaceDisplayName(workspaceName)

  // On a phone no category in the address is the first screen; elsewhere the first category is open.
  const named = namedCategory(search, isPersonal)
  const fallback: ContactsCat = isPersonal ? 'members' : 'org'
  // The phone's first screen has no category open: it is asked for nothing (the search with nothing typed is that).
  const cat: ContactsCat = phone ? named ?? 'search' : named === 'search' ? fallback : named ?? fallback
  const inCategory = phone && !!named
  // The phone's search screen (the pill's, or a level's search icon): not its first screen, which is also `search`.
  const onSearchScreen = phone && named === 'search'

  // --- the search box: typing is local, the address follows a moment later ---
  const urlQuery = search.q ?? ''
  const [query, setQuery] = useState(urlQuery)
  const written = useRef(urlQuery)
  const [onlineOnly, setOnlineOnly] = useState(false)
  // A phone's people search narrowed to a place (the level its search icon was pressed on, or a picked chip).
  const [place, setPlace] = useState<PlaceFilter | undefined>()
  useEffect(() => {
    if (!onSearchScreen) setPlace(undefined)
  }, [onSearchScreen])
  useEffect(() => {
    // The address changed from outside (a link, back, forward): the box follows it.
    if (urlQuery !== written.current) {
      written.current = urlQuery
      setQuery(urlQuery)
    }
  }, [urlQuery])
  useEffect(() => {
    if (query === written.current) return
    const timer = setTimeout(() => {
      written.current = query
      void navigate({
        to: '/contacts', search: (prev: ContactsSearch) => ({ ...prev, q: query || undefined }), replace: true,
        state: ((prev: Record<string, unknown>) => prev) as never,
      })
    }, 300)
    return () => clearTimeout(timer)
  }, [query, navigate])
  // A workspace switch is a different directory. (Arriving at the first workspace, or at a link that carries a search, is not one.)
  const previousWs = useRef(wsId)
  useEffect(() => {
    if (previousWs.current && previousWs.current !== wsId) {
      setQuery('')
      written.current = ''
      setOnlineOnly(false)
      setPlace(undefined)
    }
    previousWs.current = wsId
  }, [wsId])

  // The box answers at once; the list follows at its own pace, so a long list never makes typing stutter.
  const deferredQuery = useDeferredValue(query)
  const { view, facts } = useContactsBrowse({ wsId, workspaceName: workspace, isPersonal, cat, place: search, query: deferredQuery })
  const isOnline = useCallback((c: Contact) => c.user_id === me?.id || c.user_id in onlineUsers, [me?.id, onlineUsers])
  const selected = search.person ? view.all.find((c) => c.user_id === search.person) : undefined
  const meContact = view.all.find((c) => c.user_id === me?.id)

  // --- going places: every level is an address ---
  const go = useCallback((next: (prev: ContactsSearch) => ContactsSearch, replace = false) => {
    ownStep()
    return navigate({
      to: '/contacts',
      search: (prev: ContactsSearch) => next(prev),
      replace,
      // A push is marked, so the way back knows the entry before it is a level of Danh bạ; a replace keeps the mark it had.
      state: ((prev: Record<string, unknown>) => (replace ? prev : { ...prev, contactsStack: true })) as never,
    })
  }, [navigate, ownStep])
  const resetFilters = () => {
    setQuery('')
    written.current = ''
    setOnlineOnly(false)
    setPlace(undefined)
  }
  const openCategory = (c: Exclude<ContactsCat, 'search'>) => { resetFilters(); void go((prev) => categorySearch(prev, c)) }
  const openDept = (id: string | undefined) => { resetFilters(); void go((prev) => deptSearch(prev, id)) }
  const openRole = (id: string) => { resetFilters(); void go((prev) => roleSearch(prev, id)) }
  const openGroup = (id: string) => { resetFilters(); void go((prev) => groupSearch(prev, id)) }
  const openSearchScreen = () => { resetFilters(); void go((prev) => categorySearch(prev, 'search')) }
  /** The search icon of a level: the search screen, narrowed to the department, role or group the level shows. */
  const openPlaceSearch = () => {
    resetFilters()
    setPlace(placeOfLevel(view.level, search))
    void go((prev) => categorySearch(prev, 'search'))
  }
  /** Xem tất cả under Đang trực tuyến: the search screen listing everyone online the viewer may see. */
  const seeAllOnline = () => {
    resetFilters()
    setOnlineOnly(true)
    void go((prev) => categorySearch(prev, 'search'))
  }
  const clearSearch = () => {
    setQuery('')
    setOnlineOnly(false)
    searchRef.current?.focus()
  }
  /**
   * One step back on a phone, the same as the system's: the previous entry when it is a level Danh bạ pushed,
   * else (a link opened here, or from another page) the level above, replacing the entry so the page is not left.
   */
  const stepBack = (parent: (prev: ContactsSearch) => ContactsSearch) => {
    if (canGoBack && stacked) {
      ownBack()
      router.history.back()
    } else void go(parent, true)
  }
  const levelUp = (prev: ContactsSearch): ContactsSearch => {
    switch (view.kind) {
      case 'dept': {
        const above = view.trail[view.trail.length - 2]
        return view.atRoot ? homeSearch(prev) : deptSearch(prev, above?.id)
      }
      case 'role': return categorySearch(prev, 'roles')
      case 'group': return categorySearch(prev, 'groups')
      default: return homeSearch(prev)
    }
  }
  const backOneLevel = () => {
    // A search typed over a level is cleared first: it is the top of what was added.
    if (query.trim() && cat !== 'search') { clearSearch(); return }
    stepBack(levelUp)
  }

  // --- the profile ---
  const opener = useRef<HTMLElement | null>(null)
  const recent = useRecentSearches('contacts')
  const openPerson = (c: Contact, from?: HTMLElement) => {
    // Opening a profile from the results keeps the search that found it.
    if (query.trim()) recent.add(query)
    opener.current = from ?? null
    // What is typed goes into the address with the person: the entry made now must not lose a search the pause had not yet written.
    written.current = query
    // On a phone a profile is a screen of the stack (one history entry); elsewhere it is a panel over the same place.
    void go((prev) => ({ ...openPersonSearch(prev, c.user_id), q: query || undefined }), !phone)
  }
  // The rows are handed one function for good, so a row that did not change is not drawn again.
  const openPersonRef = useRef(openPerson)
  openPersonRef.current = openPerson
  const selectPerson = useCallback((c: Contact, from?: HTMLElement) => openPersonRef.current(c, from), [])
  const headingRef = useRef<HTMLHeadingElement>(null)
  const closePerson = useCallback(() => {
    if (phone) {
      if (canGoBack && stacked) {
        ownBack()
        router.history.back()
      } else void go((prev) => closePersonSearch(prev), true)
      return
    }
    void go((prev) => closePersonSearch(prev), true).then(() => {
      // Focus goes back to the row that opened the profile, which the closing button's removal would otherwise lose;
      // a profile opened by a link has no row, and focus goes to the name of the list.
      requestAnimationFrame(() => {
        if (opener.current?.isConnected) opener.current.focus()
        else headingRef.current?.focus()
        opener.current = null
      })
    })
  }, [go, phone, canGoBack, stacked, router])

  // On a desktop Esc closes the profile (a sheet and a phone's screen take the key themselves); menus and dialogs claim it first.
  useEffect(() => {
    if (!selected || bottomBar) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && !e.defaultPrevented) closePerson()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [selected, bottomBar, closePerson])

  // The level of a phone's stack is the place in the address, never what is typed over it: typing must not replace the screen.
  const levelKey = `${cat}:${search.dept ?? ''}:${search.role ?? ''}:${search.chat ?? ''}`
  const shownLevel = useRef<string | null>(null)
  const refocusBar = shownLevel.current !== null && shownLevel.current !== levelKey
  useEffect(() => { shownLevel.current = phone && inCategory ? levelKey : null })

  const [busy, setBusy] = useState<'message' | 'call' | null>(null)
  const message = (c: Contact) => {
    setBusy('message')
    createDM.mutate(
      { userId: c.user_id, ngacNodeId: c.ngac_node_id },
      {
        onSuccess: (ch) => void navigate({ to: '/channels/$channelId', params: { channelId: ch.id } }),
        onError: (err) => toast.error(explain(err, `mở cuộc trò chuyện với ${nameOf(c)}`)),
        onSettled: () => setBusy(null),
      },
    )
  }
  /** A call is made in the direct message with the person, which is opened (or found) first. */
  const call = (c: Contact) => {
    setBusy('call')
    createDM.mutate(
      { userId: c.user_id, ngacNodeId: c.ngac_node_id },
      {
        onSuccess: (ch) => startCall.mutate({ channelId: ch.id, kind: 'audio' }, {
          onSuccess: (ticket) => {
            // The ticket that came with starting is the one the call screen joins with.
            stashTicket(ticket)
            void navigate(callRoute(ticket.call.id, 'audio'))
          },
          onSettled: () => setBusy(null),
        }),
        onError: (err) => {
          toast.error(explain(err, `gọi ${nameOf(c)}`))
          setBusy(null)
        },
      },
    )
  }
  const editProfile = () => void navigate({ to: '/settings', search: (prev: { ws?: string }) => ({ ws: prev.ws }) })
  const openChat = (channelId: string) => void navigate({ to: '/channels/$channelId', params: { channelId } })

  // --- suggestions before anything is typed: Tìm gần đây and Nhắn gần đây (DESIGN.md "Gợi ý trong ô tìm") ---
  const searchRef = useRef<HTMLInputElement>(null)
  // The direct messages are asked for only once the box is focused: most visits never open the suggestions.
  const [wantsDirects, setWantsDirects] = useState(false)
  const { directs, isLoading: directsLoading } = useRecentDirects(wsId, wantsDirects)
  const lately = useMemo(() => recentlyMessaged(directs, view.all, me?.id), [directs, view.all, me?.id])
  // On a phone's search screen a place or the online filter lists people with nothing typed: no suggestions over them.
  const listing = onSearchScreen && (!!place || onlineOnly)
  const panel = useSuggestionPanel({
    value: query,
    variant: bottomBar ? 'screen' : 'popover',
    anchorRef: searchRef,
    showWhenEmpty: !listing,
    groups: [
      recent.group((q) => setQuery(q)),
      {
        label: 'Nhắn gần đây',
        loading: !facts.directoryLoaded || directsLoading,
        items: lately.map((c) => ({
          key: c.user_id,
          title: nameOf(c),
          meta: roleLine(c) || undefined,
          icon: <Avatar name={nameOf(c)} hueKey={c.user_id} src={c.avatar_url} size={bottomBar ? 36 : 32} />,
          onSelect: () => openPerson(c),
        })),
      },
    ],
  })

  // --- the pieces ---
  const categories = categoriesOf({
    isPersonal, workspaceName: workspace, limited: facts.limited, memberCount: view.all.length,
    roleCount: facts.roleCount, groupCount: facts.groupCount, rootCount: facts.rootCount,
  })
  const myDepartment: MyDepartment | undefined = !isPersonal && meContact?.department_id
    ? (() => {
      const node = facts.tree?.byId.get(meContact.department_id)
      return {
        id: meContact.department_id,
        name: node?.name ?? meContact.department,
        path: node && facts.tree ? parentPathLabel(facts.tree, node.id) || undefined : undefined,
        count: node?.peopleCount,
      }
    })()
    : undefined
  const frequent = useFrequentContacts(wsId, view.all, phone)
  const onlineNow = useMemo(
    () => (phone ? view.all.filter((c) => c.user_id !== me?.id && c.user_id in onlineUsers) : []),
    [phone, view.all, me?.id, onlineUsers],
  )
  const details = useWorkspaceDetails(phone && !isPersonal ? wsId : '')
  const canInvite = !isPersonal && details.data?.can_manage === true
  const searching = query.trim() !== ''
  const mineIsCurrent = !!myDepartment && cat === 'org' && search.dept === myDepartment.id && !searching
  const openMine = () => { if (myDepartment) openDept(myDepartment.id) }

  const placeTree = useDepartmentTree(wsId, onSearchScreen && !isPersonal).tree
  const placeGroup = useGroupMembers(wsId, place?.kind === 'group' ? place.id : null)
  const groupMembers = useMemo(
    () => (place?.kind === 'group' && !placeGroup.isLoading && !placeGroup.isError ? new Set(placeGroup.members.map((mm) => mm.userId)) : undefined),
    [place, placeGroup.members, placeGroup.isLoading, placeGroup.isError],
  )
  const shown: BrowseView = useMemo(() => {
    if (!onSearchScreen || view.kind !== 'search') return view
    const typed = deferredQuery.trim() !== ''
    let found = typed ? view.people : listing ? view.all.filter((c) => c.user_id !== me?.id) : []
    if (place) found = inPlace(found, place, { tree: placeTree ?? facts.tree, groupMembers })
    return { ...view, people: found, sub: `${found.length} người khớp` }
  }, [onSearchScreen, view, deferredQuery, listing, me?.id, place, placeTree, facts.tree, groupMembers])
  const searchHint = onSearchScreen && (deferredQuery.trim() !== '' || listing)
    ? `${place ? `Kết quả trong ${place.label}` : view.limited ? 'Kết quả trong phạm vi bạn được xem'
      : isPersonal ? `Kết quả trong ${workspace}` : `Kết quả trong cả ${workspace}`} · ${
      (onlineOnly ? shown.people.filter(isOnline) : shown.people).length} người khớp`
    : undefined

  const searchLabel = view.limited
    ? 'Tìm người trong phạm vi bạn được xem' : isPersonal ? `Tìm người trong ${workspace}` : `Tìm người trong cả ${workspace}`
  const onlineN = levelContacts(shown).filter(isOnline).length
  const showTools = view.kind !== 'roles' && view.kind !== 'groups'
  // While the suggestions cover the list of a phone or a tablet, filters would change a list that cannot be seen.
  // A phone's search listing a place or everyone online is not covered by suggestions.
  const covering = bottomBar && panel.visible && !listing
  const clearButton = (
    <IconButton aria-label="Xoá tìm kiếm" title="Xoá tìm kiếm" size="sm" onClick={clearSearch}
      className={bottomBar ? 'absolute right-1 top-1/2 -translate-y-1/2' : ''}>
      <X size={16} strokeWidth={1.75} aria-hidden="true" />
    </IconButton>
  )
  const onlineChip = (
    <FilterChip pressed={onlineOnly} onClick={() => setOnlineOnly((on) => !on)}>
      <span className="inline-flex items-center gap-1.5">
        <span aria-hidden="true" className="w-2 h-2 rounded-full bg-success" />
        Đang trực tuyến{view.status === 'ready' && <span className="font-normal text-ink-muted tnum">{onlineN}</span>}
      </span>
    </FilterChip>
  )
  const tools = showTools ? (
    <div className={`flex items-center gap-2 min-w-0 ${phone ? 'flex-wrap' : ''}`}>
      {covering && cat !== 'search' && (
        <IconButton aria-label="Quay lại danh sách" title="Quay lại" shape="round" size="lg" className="w-11 h-11 -ml-2 shrink-0"
          onClick={() => { panel.close(); searchRef.current?.blur() }}>
          <ArrowLeft size={20} strokeWidth={1.75} aria-hidden="true" />
        </IconButton>
      )}
      {/* A phone gives the box the whole row and the chip the one below (mockup §7). */}
      <div className={`relative flex-1 min-w-0 [&_input::-webkit-search-cancel-button]:hidden ${bottomBar ? '' : 'max-w-85'} ${phone && !covering ? 'basis-full' : ''}`}>
        <SearchBox
          ref={searchRef}
          panel={panel}
          label={searchLabel}
          placeholder="Tên, chức danh hoặc email"
          moduleSearch
          tone="sunk"
          value={query}
          {...panel.inputProps}
          onFocus={() => {
            panel.inputProps.onFocus()
            setWantsDirects(true)
          }}
          onKeyDown={(e) => {
            panel.inputProps.onKeyDown(e)
            if (e.defaultPrevented) {
              // The key was the box's own (closing its suggestions): it goes no further, so a screen around it does not also step back.
              if (e.key === 'Escape') e.stopPropagation()
              return
            }
            // The Enter that ends a word an input method is composing is not a search.
            if (e.key === 'Enter' && !e.nativeEvent.isComposing) recent.add(query)
            else if (e.key === 'Escape' && query) {
              e.preventDefault()
              e.stopPropagation()
              clearSearch()
            }
          }}
          onChange={(e) => setQuery(e.target.value)}
          slotClassName="w-full"
          className={`w-full ${bottomBar ? 'h-11' : ''} ${query ? 'pr-9' : ''}`}
          trailing={query && !bottomBar ? clearButton : undefined}
          emptyText={SEARCH_HINT}
        />
        {query && bottomBar && clearButton}
      </div>
      {/* The search screen's chips stay over its suggestions: picking a place lists its people with nothing typed. */}
      {(!covering || onSearchScreen) && (phone ? (
        <ContactsSearchFilters
          isPersonal={isPersonal}
          filter={place}
          onFilter={setPlace}
          tree={placeTree ?? facts.tree}
          roles={view.roles}
          onlineOnly={onlineOnly}
          onToggleOnline={() => setOnlineOnly((on) => !on)}
          onlineCount={view.status === 'ready' ? onlineN : undefined}
        />
      ) : onlineChip)}
    </div>
  ) : null
  const levelTools = showTools ? <div className="flex items-center gap-2 min-w-0">{onlineChip}</div> : null
  const cover = covering ? <SearchSuggestions panel={panel} emptyText={SEARCH_HINT} /> : null

  const rowActions = (c: Contact): SwipeAction[] => [
    { key: 'message', label: 'Nhắn tin', tone: 'accent', icon: <MessageSquare size={20} strokeWidth={1.75} aria-hidden="true" />, onAction: () => message(c) },
    { key: 'call', label: 'Gọi', tone: 'success', icon: <Phone size={20} strokeWidth={1.75} aria-hidden="true" />, onAction: () => call(c) },
  ]
  const browser = (variant: 'column' | 'phone') => (
    <ContactsBrowser
      view={variant === 'phone' ? shown : view}
      variant={variant}
      workspaceName={workspace}
      isPersonal={isPersonal}
      query={deferredQuery}
      onlineOnly={onlineOnly}
      isOnline={isOnline}
      meId={me?.id}
      selectedId={selected?.user_id}
      people={people}
      tools={variant === 'phone' && cat !== 'search' ? levelTools : tools}
      cover={cover}
      tallRows={variant === 'phone'}
      rowActions={variant === 'phone' ? rowActions : undefined}
      alphabetFrom={variant === 'phone' ? ALPHABET_FROM : undefined}
      listWithoutQuery={variant === 'phone' && listing}
      searchHint={variant === 'phone' ? searchHint : undefined}
      retryTreeHere={bottomBar}
      onOpenDept={openDept}
      onOpenRoles={() => openCategory('roles')}
      onOpenRole={openRole}
      onOpenGroups={() => openCategory('groups')}
      onOpenGroup={openGroup}
      onOpenChat={openChat}
      onSelectPerson={selectPerson}
      headingRef={headingRef}
      onClearSearch={clearSearch}
      onShowEveryone={() => setOnlineOnly(false)}
    />
  )
  const profileProps = (contact: Contact) => ({
    contact,
    online: isOnline(contact),
    isMe: contact.user_id === me?.id,
    isPersonal,
    path: isPersonal ? [] : pathSegments(facts.tree, contact),
    colleagues: colleaguesOf(view.all, contact),
    busy,
    onMessage: () => message(contact),
    onCall: () => call(contact),
    onClose: closePerson,
    onOpenDept: (id: string) => { if (phone) void go((prev) => deptSearch(closePersonSearch(prev), id)); else openDept(id) },
    onOpenRole: (id: string) => { if (phone) void go((prev) => roleSearch(closePersonSearch(prev), id)); else openRole(id) },
  })
  const profile = (mode: ProfileMode, contact: Contact) => (
    <ContactProfile {...profileProps(contact)} mode={mode} focusHeading={mode === 'panel'} onEdit={editProfile} />
  )
  const banner = connectionLost && (
    <div role="status" className="mx-5 mt-2 px-3 py-2 rounded-surface bg-warning-wash text-sm">
      Đang kết nối lại… Trạng thái trực tuyến có thể chưa mới nhất.
    </div>
  )

  // --- a phone: the first screen, and the screens stacked over it ---
  if (phone) {
    const bar = phoneBarOf(view, workspace)
    const levelPlace = placeOfLevel(view.level, search)
    return (
      <>
        <ContactsCategoryHome
          categories={categories}
          onOpen={openCategory}
          myDepartment={myDepartment}
          onOpenMine={openMine}
          onSearch={openSearchScreen}
          frequent={frequent}
          online={onlineNow}
          isOnline={isOnline}
          onOpenPerson={(c) => openPerson(c)}
          onMessagePerson={message}
          onSeeAllOnline={seeAllOnline}
          canInvite={canInvite}
          onInvite={() => void navigate({ to: '/settings/$section', params: { section: 'workspace' }, search: (prev: { ws?: string }) => (prev.ws ? { ws: prev.ws } : {}) })}
        />
        <AnimatePresence custom={byDevice}>
          {inCategory && (
            <PhoneLayer key="level" label={bar.title} onBack={backOneLevel} initialFocusRef={cat === 'search' ? searchRef : undefined}>
              <motion.div key={levelKey} {...phonePush(m)} initial={byDevice ? false : 'from'} className="flex flex-col flex-1 min-h-0">
                <PhoneBackBar
                  focusBack={refocusBar}
                  title={bar.title}
                  sub={bar.above.length > 0 ? <PathTrail segments={bar.above} /> : bar.text}
                  onBack={backOneLevel}
                  end={cat === 'search' ? undefined : (
                    <span className="flex items-center">
                      <IconButton
                        aria-label={levelPlace ? `Tìm người trong ${levelPlace.label}` : 'Tìm người'} title="Tìm người"
                        shape="round" size="lg" className="w-11 h-11" onClick={openPlaceSearch}
                      >
                        <Search size={20} strokeWidth={1.75} aria-hidden="true" />
                      </IconButton>
                      {view.level.kind === 'group' && view.group && (
                        <IconButton aria-label="Mở nhóm" title="Mở nhóm" shape="round" size="lg" className="w-11 h-11"
                          onClick={() => openChat(view.group!.id)}>
                          <MessageSquare size={20} strokeWidth={1.75} aria-hidden="true" />
                        </IconButton>
                      )}
                    </span>
                  )}
                />
                {banner}
                {browser('phone')}
              </motion.div>
            </PhoneLayer>
          )}
          {selected && (
            <PhoneLayer key="profile" label={`Hồ sơ ${nameOf(selected)}`} onBack={closePerson}>
              <div className="flex-1 min-h-0 overflow-y-auto pb-6">
                <ContactPhoneProfile {...profileProps(selected)} wsId={wsId} onOpenChat={openChat} />
              </div>
            </PhoneLayer>
          )}
        </AnimatePresence>
      </>
    )
  }

  // --- a tablet: chips instead of the list panel, the profile in a sheet ---
  if (bottomBar) {
    return (
      <div className="relative flex flex-col flex-1 min-h-0 min-w-0 bg-base">
        <ModuleHeader moduleId="contacts" title="Danh bạ">
          <ContactsChips
            categories={categories}
            current={cat}
            onOpen={openCategory}
            myDepartment={myDepartment}
            mineIsCurrent={mineIsCurrent}
            onOpenMine={openMine}
          />
        </ModuleHeader>
        {banner}
        {browser('column')}
        <AnimatePresence>
          {selected && (
            <ContactProfileSheet key="profile" label={`Hồ sơ ${nameOf(selected)}`} onClose={closePerson}>
              {profile('sheet', selected)}
            </ContactProfileSheet>
          )}
        </AnimatePresence>
      </div>
    )
  }

  // --- a desktop: the list panel, the level, the profile ---
  return (
    <div className="relative flex flex-1 min-h-0 min-w-0">
      <ContactsNav
        categories={categories}
        current={cat}
        onOpen={openCategory}
        myDepartment={myDepartment}
        mineIsCurrent={mineIsCurrent}
        onOpenMine={openMine}
        treeFailed={facts.treeFailed && !isPersonal}
        onRetryTree={facts.retryTree}
      />
      <div className="flex flex-col flex-1 min-w-0 min-h-0">
        {banner}
        {browser('column')}
      </div>

      <AnimatePresence>
        {selected && (
          <motion.aside
            key="profile"
            role="complementary"
            aria-label={`Hồ sơ ${nameOf(selected)}`}
            {...m.panel}
            className="flex flex-col min-h-0 min-w-0 bg-raised overflow-y-auto
              absolute right-3 inset-y-3 w-90 rounded-surface shadow-overlay z-sticky
              xl:static xl:my-3 xl:mr-3 xl:shadow-none xl:shrink-0"
          >
            {profile('panel', selected)}
          </motion.aside>
        )}
      </AnimatePresence>
    </div>
  )
}
