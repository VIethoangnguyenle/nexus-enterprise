import { useCallback, useEffect, useMemo, useRef, useState, type DragEvent, type UIEvent } from 'react'
import { useRegisterCreateAction } from '../../hooks/useModuleCreateAction'
import { useNavigate, useRouter, useSearch } from '@tanstack/react-router'
import { AnimatePresence, motion } from 'motion/react'
import { FolderTree, MoreHorizontal, Search, Star, StarOff, UserPlus, WifiOff } from 'lucide-react'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { useArrivals } from '../../hooks/useArrivals'
import { useConnectionLost } from '../../hooks/useConnectionLost'
import { useCreateTextDocument } from '../../hooks/useDocuments'
import { useDownloadFile } from '../../hooks/useDownloadUrl'
import { useDriveFolder, useDriveItem, useQuickAccess, useRecentFiles, useSharedWithMe, useStarredItems } from '../../hooks/useDrive'
import { usePeople } from '../../hooks/usePeople'
import { usePermissions } from '../../hooks/usePermissions'
import { useRecentSearches } from '../../hooks/useRecentSearches'
import { useSuggestionPanel, type SuggestionItem } from '../../hooks/useSuggestionPanel'
import { NO_PERMS, type ObjectPerms } from '../../api/access'
import type { DriveItem } from '../../api/drive'
import { folderSearch, sharedSearch, textsSearch, type DriveSearch } from '../../lib/drive-search'
import { statusOf } from '../../lib/errors'
import { formatListTime } from '../../lib/format'
import type { PeopleDirectory } from '../../lib/people'
import { recentlyEdited } from '../../lib/suggestions/drive'
import { useMotionPresets } from '../../lib/motion'
import { workspaceDisplayName } from '../../lib/workspace'
import { useAuthStore } from '../../stores/auth.store'
import { useDriveStore } from '../../stores/drive.store'
import { useFileViewMode } from '../../stores/file-view.store'
import { setUploadNavigator, useUploadQueue } from '../../stores/upload-queue.store'
import { useWebSocketStore } from '../../stores/websocket.store'
import { Heading, IconButton, toast } from '../primitives'
import { ConfirmDialog } from '../composites/ConfirmDialog'
import { ModuleMenu } from '../patterns/ModuleMenu'
import { PullToRefresh } from '../patterns/PullToRefresh'
import type { SwipeAction } from '../patterns/SwipeRow'
import { SearchSuggestions } from '../patterns/SearchSuggestions'
import { DeleteItemDialog } from './DeleteItemDialog'
import { DriveBody } from './DriveBody'
import { DriveBreadcrumbs, type Crumb } from './DriveBreadcrumbs'
import { DriveBurstBanner } from './DriveBurstBanner'
import { DriveDetailPanel } from './DriveDetailPanel'
import { DriveFolderPhone, DriveSelectionHeader, ViewToggle } from './DriveFolderPhone'
import { DriveGallery, GallerySkeleton } from './DriveGallery'
import { DriveAddSheet, DriveItemMenu, DriveItemSheet, type MenuTarget } from './DriveItemMenu'
import { DrivePhoneChips, DriveQuickAccess, DriveRecentGroups, PhoneHomeBody, type PhoneTab } from './DrivePhoneHome'
import { DrivePhoneList, type PhoneSelection } from './DrivePhoneList'
import { DriveSelectionBar } from './DriveSelectionBar'
import { DriveTable, type RowActions } from './DriveTable'
import { DriveToolbar, SEARCH_HINT } from './DriveToolbar'
import { DriveTree } from './DriveTree'
import { DriveTreeDrawer } from './DriveTreeDrawer'
import { MoveItemDialog } from './MoveItemDialog'
import { NameDialog } from './NameDialog'
import { ShareDialog } from './ShareDialog'
import { UploadQueue } from './UploadQueue'
import { useDriveActions } from './useDriveActions'
import { useDriveLayout } from './use-drive-layout'
import {
  filterByType, itemVersion, kindOf, matchesName, ownerOf, sortItems, type SortKey, type TypeFilter,
} from './drive-model'
import { FileViewer, type ViewerFile } from '../files/FileViewer'
import type { MenuAnchor } from '../files/use-long-press'

/** Items without an NGAC node are never checked; they can be seen and nothing else. */
const READ_ONLY: ObjectPerms = { ...NO_PERMS, read: true }

/** How long after my own change the same change from the server still counts as mine. */
const MINE_MS = 15_000

/** What the viewer shows of a file: its name, kind, size, who put it there and when. */
function toViewerFile(f: DriveItem, people: PeopleDirectory): ViewerFile {
  const owner = ownerOf(f, people)
  return {
    id: f.id,
    name: f.name,
    mime: f.mime_type,
    size: f.size_bytes,
    uploaderName: owner.name,
    uploaderAvatarUrl: owner.avatarUrl,
    uploaderId: f.owner_id,
    time: f.updated_at ?? f.created_at,
    parentFolderId: f.parent_id,
  }
}

/** The element that has focus now: where focus returns to when the viewer closes. */
const focused = () => (document.activeElement instanceof HTMLElement ? document.activeElement : null)

const EMPTY: DriveItem[] = []

/**
 * Tài liệu (DESIGN.md "Tài liệu: lưới, danh sách, panel xem trước"): folder
 * tree, a toolbar, the files of the open folder as a gallery or the hairline
 * table, and a detail panel for the selected item. On a phone (mockup §10) the
 * tree gives way to entry chips, a home with Truy cập nhanh and Gần đây, a
 * folder bar with the path, a sheet per item, multi-select and background uploads.
 *
 * Where the user is — workspace, folder, shared view — is the URL's business:
 * `?folder=<id>` makes reload, Back/Forward and a pasted link land in the same
 * folder. Only the selection, the filters, the phone's chip and the tree's open
 * nodes are client state.
 */
export function DriveScreen() {
  const search = useSearch({ strict: false }) as DriveSearch
  const navigate = useNavigate()
  const router = useRouter()
  const m = useMotionPresets()
  const layout = useDriveLayout()
  const phone = layout === 'phone'
  const connectionLost = useConnectionLost()
  const { workspaceId: wsId, workspaceName, workspaces } = useActiveWorkspace()
  const people = usePeople(wsId)
  const me = useAuthStore((s) => s.user)
  const [mode, setMode] = useFileViewMode('drive')

  const shared = search.view === 'shared'
  const folderId = shared ? undefined : search.folder
  const personal = workspaces.find((w) => w.id === wsId)?.type === 'personal'
  const workspaceLabel = personal ? 'Tài liệu của tôi' : workspaceDisplayName(workspaceName)

  // The phone's home: which chip is open. Shared is the URL's (it has a view of its own).
  const [phoneTabState, setPhoneTab] = useState<PhoneTab>('recent')
  const home = phone && !folderId
  const phoneTab: PhoneTab = shared ? 'shared' : phoneTabState
  const homeList = home && (phoneTab === 'recent' || phoneTab === 'starred')

  const folderQ = useDriveFolder(wsId, folderId, !shared)
  const sharedQ = useSharedWithMe(shared)
  const active = shared ? sharedQ : folderQ

  // A link can carry a folder of another workspace. The server may well allow
  // it (the folder is real), but this workspace's screen must not show it or
  // take uploads for it: it is treated as not found. The folder's own record
  // answers even when it is empty; its first item is the fallback.
  const folderRecord = useDriveItem(shared ? '' : folderId ?? '')
  const foreign =
    !shared && !!folderId && !!wsId &&
    (folderRecord.data
      ? folderRecord.data.workspace_id !== wsId
      : !!folderQ.data?.items?.[0] && folderQ.data.items[0].workspace_id !== wsId)
  const all = useMemo(() => (foreign ? [] : active.data?.items ?? []), [foreign, active.data])

  // The phone's home lists: asked for only while their chip is open; the stars mark rows everywhere on a phone.
  const starredQ = useStarredItems(wsId, phone)
  const quickQ = useQuickAccess(wsId, home && phoneTab === 'recent')
  const recentQ = useRecentFiles(wsId, home && phoneTab === 'recent')
  const starredItems = starredQ.data ?? EMPTY
  const starredSet = useMemo(() => new Set(starredItems.map((i) => i.id)), [starredItems])
  const recentItems = recentQ.data ?? EMPTY
  const quickEntries = useMemo(() => quickQ.data ?? [], [quickQ.data])

  // Selection and the tree's open nodes live in the store.
  const selectedId = useDriveStore((s) => s.selectedItemId)
  const selectItem = useDriveStore((s) => s.selectItem)
  const expandFolders = useDriveStore((s) => s.expandFolders)

  const [query, setQuery] = useState('')
  const recent = useRecentSearches('drive')
  // Read when a file is opened, without making every handler that opens one change with each keystroke.
  const queryRef = useRef(query)
  queryRef.current = query
  const [typeFilter, setTypeFilter] = useState<TypeFilter>('all')
  const [sort, setSort] = useState<SortKey>('recent')
  const [dragging, setDragging] = useState(false)
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [menu, setMenu] = useState<MenuTarget | null>(null)
  const [sheetItem, setSheetItem] = useState<DriveItem | null>(null)
  const [infoItem, setInfoItem] = useState<DriveItem | null>(null)
  const [addOpen, setAddOpen] = useState(false)
  // The phone's search opens from the folder bar's magnifier.
  const [phoneSearch, setPhoneSearch] = useState(false)
  // The phone's multi-select: null outside the mode.
  const [chosen, setChosen] = useState<ReadonlySet<string> | null>(null)
  // The upload card folds to a ring while the list scrolls down.
  const [scrolledDown, setScrolledDown] = useState(false)
  const lastScroll = useRef(0)
  // Deleting the last row: keep the table up until the row has folded away,
  // then show the empty state (reduced motion swaps at once).
  const [holdTable, setHoldTable] = useState(false)
  const rowsSeen = useRef({ scope: '', n: 0 })
  const fileInput = useRef<HTMLInputElement>(null)
  const cameraInput = useRef<HTMLInputElement>(null)

  const { download } = useDownloadFile()

  // A new place starts with no selection, no filter and an empty search.
  const view = shared ? 'shared' : folderId ?? (home ? `home-${phoneTab}` : 'root')
  const scope = `${wsId}:${view}`
  useEffect(() => {
    selectItem(null)
    setQuery('')
    setTypeFilter('all')
    setHoldTable(false)
    setDrawerOpen(false)
    setMenu(null)
    setSheetItem(null)
    setInfoItem(null)
    setChosen(null)
    setPhoneSearch(false)
  }, [scope, selectItem])

  const trail = useMemo(() => (folderQ.data?.breadcrumb ?? []).map((c) => c.id), [folderQ.data])
  useEffect(() => {
    // Open the ancestors, so the tree shows where a reloaded or linked folder is.
    if (!shared && trail.length > 1) expandFolders(trail.slice(0, -1))
  }, [shared, trail, expandFolders])

  const items = useMemo(
    () => sortItems(filterByType(all.filter((i) => matchesName(i, query)), typeFilter), sort),
    [all, query, typeFilter, sort],
  )
  if (rowsSeen.current.scope !== scope) rowsSeen.current = { scope, n: 0 }
  // Only the table folds rows away; the gallery has nothing to wait for.
  if (all.length === 0 && rowsSeen.current.n > 0 && !m.reduced && mode === 'list' && !holdTable) setHoldTable(true)
  rowsSeen.current.n = all.length
  const selected = selectedId ? all.find((i) => i.id === selectedId) : undefined

  // What the phone's list shows: the open chip's items on the home, otherwise the folder's.
  const phoneItems = useMemo(() => {
    if (!homeList) return items
    const source = phoneTab === 'starred' ? starredItems : recentItems
    return source.filter((i) => matchesName(i, query))
  }, [homeList, phoneTab, starredItems, recentItems, items, query])

  // Permissions come as one batch for everything on screen.
  const ngacIds = useMemo(() => {
    const ids = new Set(all.map((i) => i.ngac_node_id))
    if (home) for (const i of [...recentItems, ...starredItems, ...quickEntries.map((e) => e.item)]) ids.add(i.ngac_node_id)
    return [...ids].filter(Boolean)
  }, [all, home, recentItems, starredItems, quickEntries])
  const { permsMap } = usePermissions(ngacIds)
  const permsOf = useCallback(
    (item: DriveItem) => (item.ngac_node_id ? permsMap[item.ngac_node_id] ?? NO_PERMS : READ_ONLY),
    [permsMap],
  )

  // Changes I make come back from the server as "new versions"; they are mine,
  // whoever owns the item, so they must not be washed in someone else's colour.
  const recentChanges = useWebSocketStore((s) => s.recentChanges)
  const mine = useRef(new Map<string, number>())
  const markMine = (id: string) => mine.current.set(id, Date.now())

  const crumbsData = folderQ.data?.breadcrumb ?? []
  const folderName = crumbsData[crumbsData.length - 1]?.name
  const status = statusOf(active.error)
  const missing = !!folderId && (foreign || status === 403 || status === 404)
  const title = shared ? 'Được chia sẻ với tôi' : missing ? 'Tài liệu' : folderId ? folderName : 'Tài liệu'
  const targetLabel = folderId && !missing ? folderName ?? 'thư mục' : workspaceLabel

  const {
    dialog, setDialog, closeDialog, uploadFiles, uploading, creatingFolder, renaming, moving, trashing,
    submitNewFolder, submitRename, submitMove, submitDelete, duplicate, setStars,
  } = useDriveActions({
    workspaceId: wsId, folderId, folderLabel: targetLabel, foreign, background: layout !== 'desktop',
    selectedId, selectItem, markMine,
  })
  const { fresh, burst } = useArrivals(all, {
    keyOf: itemVersion,
    authorOf: (i) => {
      const at = mine.current.get(i.id)
      if (at && Date.now() - at < MINE_MS) return me?.id
      // The person who just acted, when a realtime event named them; otherwise the owner.
      return recentChanges[i.id]?.actorUserId ?? ownerOf(i, people).hueKey
    },
    me: me?.id,
    ready: !!active.data,
    scope,
  })

  // Esc closes the detail panel, or leaves the phone's selection. Dialogs and
  // menus claim the key on `document` (preventDefault); listening on `window`
  // puts this after them however early the panel was opened, so one Esc closes one thing.
  useEffect(() => {
    if (!selected && !chosen) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== 'Escape' || e.defaultPrevented) return
      if (chosen) setChosen(null)
      else selectItem(null)
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [selected, chosen, selectItem])

  const [previewIndex, setPreviewIndex] = useState<number | null>(null)
  /** A file opened from the suggestions that is not in the open folder (shared with me): the viewer shows it alone. */
  const [loneFile, setLoneFile] = useState<ViewerFile | null>(null)
  const previewOpenerRef = useRef<HTMLElement | null>(null)

  const folderItems = useMemo(() => items.filter((i) => i.item_type === 'folder'), [items])
  // What the viewer pages through: the files that are showing, not the whole folder.
  const pageFiles = useMemo(
    () => (homeList ? phoneItems : items).filter((i) => i.item_type === 'file'),
    [homeList, phoneItems, items],
  )

  const viewerFiles = useMemo<ViewerFile[]>(() => pageFiles.map((f) => toViewerFile(f, people)), [pageFiles, people])

  const { add: keepSearch } = recent
  const openPreview = useCallback(
    (item: DriveItem, openerEl?: HTMLElement | null) => {
      const idx = pageFiles.findIndex((f) => f.id === item.id)
      previewOpenerRef.current = openerEl ?? focused()
      if (idx >= 0) {
        // Opening a file the search found keeps that search.
        if (queryRef.current.trim()) keepSearch(queryRef.current)
        setPreviewIndex(idx)
      } else if (item.item_type === 'file') {
        setLoneFile(toViewerFile(item, people))
      }
    },
    [pageFiles, keepSearch, people],
  )

  const openFolder = useCallback(
    (id?: string) => void navigate({ to: '/drive', search: (prev: DriveSearch) => folderSearch(prev, id) }),
    [navigate],
  )
  const hrefOf = useCallback(
    (id: string) => router.buildLocation({ to: '/drive', search: (prev: DriveSearch) => folderSearch(prev, id) }).href,
    [router],
  )

  // "Xem" on the done toast of a background upload opens the folder it went to.
  useEffect(() => {
    setUploadNavigator((t) => {
      if (t.workspaceId !== wsId) return
      if (!t.parentId) setPhoneTab('folders')
      openFolder(t.parentId)
    })
    return () => setUploadNavigator(null)
  }, [wsId, openFolder])
  // What a previous page left in the upload queue is picked up once.
  const restoreUploads = useUploadQueue((s) => s.restore)
  useEffect(() => { void restoreUploads() }, [restoreUploads])

  const actions: RowActions = useMemo(
    () => ({
      onSelect: (item) => selectItem(item.id),
      onDownload: (item) => void download(item),
      onShare: (item) => setDialog({ kind: 'share', item }),
      onMove: (item) => setDialog({ kind: 'move', item }),
      onRename: (item) => setDialog({ kind: 'rename', item }),
      onDelete: (item) => setDialog({ kind: 'delete', item }),
      onPreview: (item, el) => openPreview(item, el),
    }),
    [selectItem, download, openPreview, setDialog],
  )

  const toggleStar = useCallback(
    (item: DriveItem) => void setStars([item], !starredSet.has(item.id)),
    [setStars, starredSet],
  )

  const copyLink = useCallback(
    (item: DriveItem) => {
      // A folder links to itself; a file to the folder it is in. Opening either still asks the server.
      const target = item.item_type === 'folder' ? item.id : item.parent_id
      const href = router.buildLocation({ to: '/drive', search: (prev: DriveSearch) => folderSearch(prev, target) }).href
      const url = new URL(href, window.location.origin).toString()
      const write = navigator.clipboard?.writeText(url)
      if (!write) return void toast.error('Không sao chép được liên kết')
      write.then(() => toast('Đã sao chép liên kết'), () => toast.error('Không sao chép được liên kết'))
    },
    [router],
  )

  // A swipe left on a row of the phone list reveals Chia sẻ · Gắn sao · Thêm (DESIGN.md "Tài liệu", §6 "Vuốt");
  // Thêm is the row's sheet. No swipe deletes.
  const swipeOf = useCallback((item: DriveItem, perms: ObjectPerms): SwipeAction[] => [
    ...(perms.share ? [{ key: 'share', label: 'Chia sẻ', tone: 'accent' as const, icon: <UserPlus size={22} strokeWidth={1.75} />, onAction: () => actions.onShare(item) }] : []),
    {
      key: 'star', label: starredSet.has(item.id) ? 'Bỏ sao' : 'Gắn sao', tone: 'warning' as const,
      icon: starredSet.has(item.id) ? <StarOff size={22} strokeWidth={1.75} /> : <Star size={22} strokeWidth={1.75} />,
      onAction: () => toggleStar(item),
    },
    { key: 'more', label: 'Thêm', tone: 'ink', icon: <MoreHorizontal size={22} strokeWidth={1.75} />, onAction: () => setSheetItem(item) },
  ], [actions, starredSet, toggleStar])

  // Gallery: a tap opens on a phone (no hover, no room for a panel) and selects elsewhere.
  const onSelectFile = useCallback(
    (file: DriveItem) => (phone ? openPreview(file) : selectItem(file.id)),
    [phone, openPreview, selectItem],
  )
  const onOpenFile = useCallback((file: DriveItem) => openPreview(file), [openPreview])
  const onOpenFolder = useCallback((folder: DriveItem) => openFolder(folder.id), [openFolder])
  const onMenu = useCallback(
    (item: DriveItem, anchor: MenuAnchor) => (phone ? setSheetItem(item) : setMenu({ item, anchor })),
    [phone],
  )
  const openItem = useCallback(
    (item: DriveItem) => (item.item_type === 'folder' ? openFolder(item.id) : openPreview(item)),
    [openFolder, openPreview],
  )

  // ---- the phone's selection ----
  const byId = useMemo(() => {
    const map = new Map<string, DriveItem>()
    for (const i of [...all, ...recentItems, ...starredItems]) map.set(i.id, i)
    return map
  }, [all, recentItems, starredItems])
  const chosenItems = useMemo(
    () => (chosen ? [...chosen].map((id) => byId.get(id)).filter((i): i is DriveItem => !!i) : EMPTY),
    [chosen, byId],
  )
  const selection: PhoneSelection = useMemo(() => ({
    ids: chosen,
    start: (item) => setChosen(new Set([item.id])),
    toggle: (item) => setChosen((s) => {
      const next = new Set(s ?? [])
      if (next.has(item.id)) next.delete(item.id)
      else next.add(item.id)
      return next
    }),
    add: (id) => setChosen((s) => (s && !s.has(id) ? new Set([...s, id]) : s)),
  }), [chosen])
  const endSelection = () => setChosen(null)

  const onDrop = (e: DragEvent<HTMLDivElement>) => {
    e.preventDefault()
    setDragging(false)
    if (shared || missing || homeList) return
    const files = Array.from(e.dataTransfer?.files ?? [])
    if (files.length) void uploadFiles(files)
  }
  const isFileDrag = (e: DragEvent<HTMLDivElement>) => Array.from(e.dataTransfer?.types ?? []).includes('Files')

  const onScroll = (e: UIEvent<HTMLDivElement>) => {
    const y = e.currentTarget.scrollTop
    setScrolledDown(y > 24 && y > lastScroll.current)
    lastScroll.current = y
  }

  // ---- what to show ----
  const crumbs: Crumb[] = shared
    ? [{ label: workspaceLabel }, { id: 'shared', label: 'Được chia sẻ với tôi' }]
    : folderId && !missing
      ? [{ label: workspaceLabel }, ...crumbsData.map((c) => ({ id: c.id, label: c.name }))]
      : [{ label: workspaceLabel }]
  // Until the workspace is known the query is idle, and that is still loading.
  const loading = homeList
    ? phoneTab === 'starred' ? !starredQ.data && !starredQ.isError : !recentQ.data && !recentQ.isError
    : !active.data && !active.isError
  // Uploads land in the open folder; the phone's Gần đây and Gắn sao take them at the top of the drive.
  const canAdd = !shared && !missing
  const newFolder = () => setDialog({ kind: 'new-folder' })

  const createText = useCreateTextDocument(wsId)
  const newText = async () => {
    try {
      const doc = await createText.mutateAsync(folderId && !missing ? { folder_id: folderId } : {})
      void navigate({ to: '/documents/$docId', params: { docId: doc.id } })
    } catch {
      // Told by the shared handler.
    }
  }

  // Register the create action for the ModuleBar + button.
  const driveCreateHandler = useCallback(() => setAddOpen(true), [])
  const driveCreateAction = useMemo(
    () => canAdd && !chosen ? { label: 'Thêm mới', handler: driveCreateHandler } : null,
    [canAdd, chosen, driveCreateHandler],
  )
  useRegisterCreateAction('drive', driveCreateAction)

  // Before anything is typed: Tìm gần đây, the files of this folder edited lately and what was shared with me
  // (DESIGN.md "Gợi ý trong ô tìm"). The shared list is asked for only once the box is focused.
  const searchRef = useRef<HTMLInputElement>(null)
  const [wantsShared, setWantsShared] = useState(false)
  const sharedSuggestQ = useSharedWithMe(wantsShared && !shared)
  const suggestFile = (f: DriveItem): SuggestionItem => {
    const kind = kindOf(f)
    return {
      key: f.id,
      title: f.name,
      meta: [ownerOf(f, people).name, formatListTime(f.updated_at ?? f.created_at)].filter(Boolean).join(' · '),
      icon: (
        <span className={`grid place-items-center rounded-md ${phone ? 'size-9' : 'size-8'} ${kind.tile}`}>
          <kind.icon size={16} strokeWidth={1.75} aria-hidden="true" />
        </span>
      ),
      // A file of this folder opens among its neighbours; one from elsewhere opens on its own.
      onSelect: () => {
        if (pageFiles.some((x) => x.id === f.id)) openPreview(f, searchRef.current)
        else {
          previewOpenerRef.current = searchRef.current
          setLoneFile(toViewerFile(f, people))
        }
      },
    }
  }
  const panel = useSuggestionPanel({
    value: query,
    variant: phone ? 'screen' : 'popover',
    anchorRef: searchRef,
    showWhenEmpty: true,
    groups: [
      recent.group(setQuery),
      { label: shared ? 'Sửa gần đây' : `Sửa gần đây trong ${title ?? 'thư mục'}`, loading, items: recentlyEdited(all).map(suggestFile) },
      ...(shared ? [] : [{
        label: 'Được chia sẻ với tôi',
        loading: sharedSuggestQ.isLoading,
        items: recentlyEdited(sharedSuggestQ.data?.items ?? []).map(suggestFile),
      }]),
    ],
  })
  const covered = phone && panel.visible

  // The phone's search opens focused; Quay lại closes it and clears it.
  useEffect(() => {
    if (phoneSearch) searchRef.current?.focus()
  }, [phoneSearch])
  const closePhoneSearch = () => {
    panel.close()
    searchRef.current?.blur()
    setQuery('')
    setPhoneSearch(false)
  }

  const tree = (onOpen: (folderId?: string) => void) => (
    <DriveTree
      workspaceId={wsId}
      workspaceLabel={workspaceLabel}
      currentFolderId={folderId}
      trail={trail}
      sharedActive={shared}
      onOpen={onOpen}
    />
  )

  const phoneList = (list: DriveItem[], label: string) => (
    <DrivePhoneList
      label={label}
      items={list}
      people={people}
      permsOf={permsOf}
      starred={starredSet}
      selection={selection}
      onOpenFile={(item, el) => openPreview(item, el)}
      onMore={setSheetItem}
      swipe={swipeOf}
    />
  )

  const gallery = loading ? (
    <GallerySkeleton />
  ) : (
    <DriveGallery
      folders={folderItems}
      files={items.filter((i) => i.item_type === 'file')}
      people={people}
      selectedId={selected?.id ?? null}
      fresh={fresh}
      hrefOf={hrefOf}
      onOpenFolder={onOpenFolder}
      onSelectFile={onSelectFile}
      onOpenFile={onOpenFile}
      onMenu={onMenu}
    />
  )

  const phoneContent = phoneTab === 'recent' && home ? (
    <>
      <DriveQuickAccess entries={query ? [] : quickEntries} onOpen={openItem} />
      <DriveRecentGroups items={phoneItems} renderList={phoneList} />
    </>
  ) : phoneTab === 'starred' && home ? (
    phoneList(phoneItems, 'Đã gắn sao')
  ) : mode === 'grid' && !chosen ? (
    gallery
  ) : (
    phoneList(items, `Tệp trong ${title ?? 'thư mục'}`)
  )

  const content = phone ? phoneContent : mode === 'grid' ? gallery : (
    <DriveTable
      label={`Tệp trong ${title ?? 'thư mục'}`}
      items={items}
      loading={loading}
      people={people}
      permsOf={permsOf}
      selectedId={selected?.id ?? null}
      scope={view}
      fresh={fresh}
      versionOf={itemVersion}
      actions={actions}
      onRowsGone={() => setHoldTable(false)}
    />
  )

  const goUp = () => {
    const parent = crumbsData[crumbsData.length - 2]?.id
    if (!parent) setPhoneTab('folders')
    openFolder(parent)
  }
  const openTab = (tab: PhoneTab) => {
    if (tab === 'shared') return void navigate({ to: '/drive', search: (prev: DriveSearch) => sharedSearch(prev) })
    setPhoneTab(tab)
    if (shared) openFolder(undefined)
  }

  const phoneSearchBar = (
    <DriveToolbar
      layout={layout}
      folderLabel={title ?? 'thư mục'}
      query={query}
      onQuery={setQuery}
      searchRef={searchRef}
      suggestPanel={panel}
      searchProps={{
        ...panel.inputProps,
        onFocus: () => {
          panel.inputProps.onFocus()
          setWantsShared(true)
        },
        onKeyDown: (e) => {
          panel.inputProps.onKeyDown(e)
          if (e.defaultPrevented) return
          if (e.key === 'Enter' && !e.nativeEvent.isComposing) recent.add(query)
          else if (e.key === 'Escape') {
            e.preventDefault()
            closePhoneSearch()
          }
        },
      }}
      onBack={closePhoneSearch}
      typeFilter={typeFilter}
      onTypeFilter={setTypeFilter}
      sort={sort}
      onSort={setSort}
      mode={mode}
      onMode={setMode}
      canAdd={canAdd}
      uploading={uploading}
      onPickFiles={() => fileInput.current?.click()}
      onNewFolder={newFolder}
    />
  )

  const phoneHeader = chosen ? (
    <DriveSelectionHeader
      count={chosen.size}
      onExit={endSelection}
      onAll={() => setChosen(new Set((homeList ? phoneItems : items).map((i) => i.id)))}
    />
  ) : phoneSearch || covered ? (
    phoneSearchBar
  ) : folderId ? (
    <DriveFolderPhone
      title={title}
      crumbs={crumbs}
      onUp={goUp}
      onCrumb={(id) => {
        if (!id) setPhoneTab('folders')
        openFolder(id)
      }}
      onSearch={() => setPhoneSearch(true)}
      onMore={folderRecord.data && !missing ? () => setSheetItem(folderRecord.data!) : undefined}
      sort={sort}
      onSort={setSort}
      mode={mode}
      onMode={setMode}
    />
  ) : (
    <header className="grid gap-1 pt-2">
      <div className="flex items-center gap-1 pl-4 pr-2">
        <Heading as="h1" look="panel" className="flex-1 min-w-0 truncate">Tài liệu</Heading>
        <IconButton size="lg" className="size-11" aria-label={`Tìm trong ${workspaceLabel}`} onClick={() => setPhoneSearch(true)}>
          <Search size={20} strokeWidth={1.75} />
        </IconButton>
        {(phoneTab === 'folders' || phoneTab === 'shared') && <ViewToggle mode={mode} onMode={setMode} />}
        <ModuleMenu section="drive" label="Tài liệu" />
      </div>
      <DrivePhoneChips tab={phoneTab} onTab={openTab} onDrafts={() => void navigate({ to: '/drive', search: (prev: DriveSearch) => textsSearch(prev, 'drafts') })} />
    </header>
  )

  return (
    <div className="relative flex flex-1 min-h-0 min-w-0">
      {layout === 'desktop' && <div className="w-70 shrink-0 min-h-0">{tree(openFolder)}</div>}

      <section className="flex-1 flex flex-col min-w-0 min-h-0 bg-base" aria-label="Tệp">
        {phone ? phoneHeader : (
          <>
            <header className="flex items-center gap-2 px-5 pt-4 pb-3 min-w-0 max-sm:px-4">
              {layout !== 'desktop' && (
                <IconButton aria-label="Mở cây thư mục" aria-haspopup="dialog" className="max-lg:w-11 max-lg:h-11 -ml-2" onClick={() => setDrawerOpen(true)}>
                  <FolderTree size={20} strokeWidth={1.75} />
                </IconButton>
              )}
              <div className="grid gap-0.5 min-w-0">
                <DriveBreadcrumbs crumbs={crumbs} />
                {title ? (
                  <Heading as="h1" look="panel" className="truncate">{title}</Heading>
                ) : (
                  <div className="skeleton h-6 w-40 rounded-sm" aria-busy="true" />
                )}
              </div>
            </header>

            {!missing && (
              <DriveToolbar
                layout={layout}
                folderLabel={title ?? 'thư mục'}
                query={query}
                onQuery={setQuery}
                searchRef={searchRef}
                suggestPanel={panel}
                searchProps={{
                  ...panel.inputProps,
                  onFocus: () => {
                    panel.inputProps.onFocus()
                    setWantsShared(true)
                  },
                  onKeyDown: (e) => {
                    panel.inputProps.onKeyDown(e)
                    if (e.defaultPrevented) return
                    // The Enter that ends a word an input method is composing is not a search.
                    if (e.key === 'Enter' && !e.nativeEvent.isComposing) recent.add(query)
                  },
                }}
                typeFilter={typeFilter}
                onTypeFilter={setTypeFilter}
                sort={sort}
                onSort={setSort}
                mode={mode}
                onMode={setMode}
                canAdd={canAdd}
                uploading={uploading}
                onPickFiles={() => fileInput.current?.click()}
                onNewFolder={newFolder}
              />
            )}
          </>
        )}

        {connectionLost && (
          <div role="status" className="mx-5 mb-2 flex items-center gap-2.5 px-3 py-2 rounded-surface bg-warning-wash text-sm">
            <WifiOff size={16} strokeWidth={1.75} className="shrink-0" aria-hidden="true" />
            Đang kết nối lại… Thay đổi của người khác sẽ hiện khi có mạng.
          </div>
        )}

        <AnimatePresence>
          {burst && <DriveBurstBanner burst={burst} people={people} row={m.row} />}
        </AnimatePresence>

        <div className="relative flex-1 min-h-0 flex flex-col">
        {covered && (
          // The files stay mounted under the phone's suggestions, so Quay lại finds them where they were scrolled.
          <div className="absolute inset-0 z-sticky grid bg-base">
            <SearchSuggestions panel={panel} emptyText={SEARCH_HINT} />
          </div>
        )}
        <PullToRefresh
          enabled={phone && !covered && !chosen}
          onRefresh={() => (homeList ? Promise.all([recentQ.refetch(), quickQ.refetch(), starredQ.refetch()]) : active.refetch())}
          aria-hidden={covered || undefined}
          inert={covered}
          data-testid="drive-dropzone"
          onScroll={phone ? onScroll : undefined}
          onDragEnter={(e) => { if (!shared && isFileDrag(e)) { e.preventDefault(); setDragging(true) } }}
          onDragOver={(e) => { if (!shared && isFileDrag(e)) e.preventDefault() }}
          onDragLeave={(e) => { if (!e.currentTarget.contains(e.relatedTarget as Node | null)) setDragging(false) }}
          onDrop={onDrop}
          className={`relative flex-1 min-h-0 overflow-y-auto transition-colors duration-quick
            ${dragging ? 'bg-accent-wash' : ''}`}
        >
          {dragging && (
            <div className="sticky top-0 z-dropdown grid place-items-center h-10 text-sm font-semibold text-ink" aria-hidden="true">
              Thả tệp để tải lên “{title}”
            </div>
          )}
          {homeList ? (
            <PhoneHomeBody
              tab={phoneTab}
              loading={loading}
              isError={phoneTab === 'starred' ? starredQ.isError : recentQ.isError}
              empty={phoneItems.length === 0 && (phoneTab === 'starred' || quickEntries.length === 0)}
              filtered={!!query}
              onRetry={() => void (phoneTab === 'starred' ? starredQ.refetch() : recentQ.refetch())}
              content={content}
            />
          ) : (
            <DriveBody
              missing={missing}
              isError={active.isError}
              onRetry={() => void active.refetch()}
              loading={loading}
              shared={shared}
              total={all.length}
              visible={items.length}
              holdTable={holdTable}
              onClearQuery={() => { setQuery(''); setTypeFilter('all') }}
              onPickFiles={() => fileInput.current?.click()}
              view={view}
              route={m.route}
              content={content}
            />
          )}
        </PullToRefresh>
        </div>
      </section>

      {/* On a tablet the panel floats over the files, so the files behind it step back. */}
      <AnimatePresence>
        {selected && layout === 'tablet' && (
          <motion.div
            key="scrim"
            data-testid="drive-scrim"
            {...m.scrim}
            className="absolute inset-0 z-dropdown bg-scrim"
            aria-hidden="true"
            onClick={() => selectItem(null)}
          />
        )}
      </AnimatePresence>
      <AnimatePresence>
        {(selected ?? infoItem) && (
          <DriveDetailPanel
            key="detail"
            item={(selected ?? infoItem)!}
            perms={permsOf((selected ?? infoItem)!)}
            people={people}
            location={shared || infoItem ? undefined : folderId ? folderName : workspaceLabel}
            onClose={() => { selectItem(null); setInfoItem(null) }}
            onShare={actions.onShare}
            onOpen={openPreview}
          />
        )}
      </AnimatePresence>

      <AnimatePresence>
        {drawerOpen && layout !== 'desktop' && (
          <DriveTreeDrawer onClose={() => setDrawerOpen(false)}>
            {tree((id) => { setDrawerOpen(false); openFolder(id) })}
          </DriveTreeDrawer>
        )}
      </AnimatePresence>

      {/* The phone's + is the ModuleBar's CreateActionFab, through useRegisterCreateAction. */}
      {addOpen && (
        <DriveAddSheet
          folderLabel={targetLabel}
          onPickFiles={() => fileInput.current?.click()}
          onTakePhoto={() => cameraInput.current?.click()}
          onNewFolder={newFolder}
          onNewText={() => void newText()}
          onClose={() => setAddOpen(false)}
        />
      )}

      <DriveItemMenu target={menu} perms={menu ? permsOf(menu.item) : NO_PERMS} actions={actions} onClose={() => setMenu(null)} />
      {sheetItem && (
        <DriveItemSheet
          item={sheetItem}
          perms={permsOf(sheetItem)}
          people={people}
          actions={actions}
          starred={starredSet.has(sheetItem.id)}
          onStar={toggleStar}
          onCopyLink={copyLink}
          onDuplicate={(item) => void duplicate(item)}
          onInfo={setInfoItem}
          onClose={() => setSheetItem(null)}
        />
      )}

      {chosen && phone && (
        <DriveSelectionBar
          items={chosenItems}
          permsOf={permsOf}
          starred={starredSet}
          actions={{
            onShare: (item) => setDialog({ kind: 'share', item }),
            onMove: (list) => setDialog({ kind: 'move', item: list[0]!, items: list }),
            onDownload: (list) => list.forEach((i) => void download(i)),
            onStar: (list, star) => void setStars(list, star).then(endSelection),
            onDelete: (list) => setDialog(list.length === 1 ? { kind: 'delete', item: list[0]! } : { kind: 'delete-many', items: list }),
          }}
        />
      )}

      {layout !== 'desktop' && !chosen && <UploadQueue collapsed={scrolledDown} />}

      {/* Choose files for the Tải lên controls. Nothing models a hidden file
          input in the primitives, and wrapping one in a form-field adds chrome
          that `sr-only` would then have to hide again. */}
      {/* eslint-disable-next-line no-restricted-syntax */}
      <input
        ref={fileInput}
        type="file"
        multiple
        aria-label="Chọn tệp để tải lên"
        tabIndex={-1}
        className="sr-only"
        onChange={(e) => {
          const files = Array.from(e.target.files ?? [])
          e.target.value = ''
          if (files.length) void uploadFiles(files)
        }}
      />
      {/* The phone's "Chụp ảnh": the same upload, with the camera asked for first. */}
      {/* eslint-disable-next-line no-restricted-syntax */}
      <input
        ref={cameraInput}
        type="file"
        accept="image/*"
        capture="environment"
        aria-label="Chụp ảnh để tải lên"
        tabIndex={-1}
        className="sr-only"
        onChange={(e) => {
          const files = Array.from(e.target.files ?? [])
          e.target.value = ''
          if (files.length) void uploadFiles(files)
        }}
      />

      <NameDialog
        open={dialog?.kind === 'new-folder'}
        onClose={closeDialog}
        title="Thư mục mới"
        submitLabel="Tạo"
        emptyMessage="Đặt tên cho thư mục"
        pending={creatingFolder}
        onSubmit={(name) => void submitNewFolder(name)}
      />
      <NameDialog
        open={dialog?.kind === 'rename'}
        onClose={closeDialog}
        title="Đổi tên"
        submitLabel="Đổi tên"
        emptyMessage="Đặt tên cho mục này"
        initialName={dialog?.kind === 'rename' ? dialog.item.name : ''}
        pending={renaming}
        onSubmit={(name) => dialog?.kind === 'rename' && void submitRename(dialog.item, name)}
      />
      <MoveItemDialog
        item={dialog?.kind === 'move' ? dialog.item : null}
        workspaceId={wsId}
        pending={moving}
        onConfirm={(item, target) => {
          const list = dialog?.kind === 'move' && dialog.items ? dialog.items : [item]
          void submitMove(list, target).then(() => { if (list.length > 1) endSelection() })
        }}
        onClose={closeDialog}
      />
      <ShareDialog item={dialog?.kind === 'share' ? dialog.item : null} people={people} onClose={closeDialog} />
      <DeleteItemDialog
        item={dialog?.kind === 'delete' ? dialog.item : null}
        pending={trashing}
        onConfirm={(item) => void submitDelete([item]).then(endSelection)}
        onClose={closeDialog}
      />
      <ConfirmDialog
        open={dialog?.kind === 'delete-many'}
        onClose={closeDialog}
        onConfirm={() => dialog?.kind === 'delete-many' && void submitDelete(dialog.items).then(endSelection)}
        title={`Xoá ${dialog?.kind === 'delete-many' ? dialog.items.length : ''} mục`}
        description="Mọi thứ trong các thư mục đã chọn cũng bị xoá. Bạn có thể hoàn tác ngay sau khi xoá."
        confirmLabel="Xoá"
        confirmVariant="danger"
        loading={trashing}
      />

      {(previewIndex !== null || loneFile) && (
        <FileViewer
          files={loneFile ? [loneFile] : viewerFiles}
          index={loneFile ? 0 : previewIndex!}
          origin="drive"
          onClose={() => {
            setPreviewIndex(null)
            setLoneFile(null)
          }}
          returnFocusRef={previewOpenerRef}
        />
      )}
    </div>
  )
}
