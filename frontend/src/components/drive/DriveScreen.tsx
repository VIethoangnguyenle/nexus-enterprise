import { useCallback, useEffect, useMemo, useRef, useState, type DragEvent } from 'react'
import { useNavigate, useRouter, useSearch } from '@tanstack/react-router'
import { AnimatePresence, motion } from 'motion/react'
import { Menu, Plus, WifiOff } from 'lucide-react'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { useArrivals } from '../../hooks/useArrivals'
import { useConnectionLost } from '../../hooks/useConnectionLost'
import { useDownloadFile } from '../../hooks/useDownloadUrl'
import { useDriveFolder, useDriveItem, useSharedWithMe } from '../../hooks/useDrive'
import { usePeople } from '../../hooks/usePeople'
import { usePermissions } from '../../hooks/usePermissions'
import { NO_PERMS, type ObjectPerms } from '../../api/access'
import type { DriveItem } from '../../api/drive'
import { folderSearch, type DriveSearch } from '../../lib/drive-search'
import { statusOf } from '../../lib/errors'
import { useMotionPresets } from '../../lib/motion'
import { workspaceDisplayName } from '../../lib/workspace'
import { useAuthStore } from '../../stores/auth.store'
import { useDriveStore } from '../../stores/drive.store'
import { useFileViewMode } from '../../stores/file-view.store'
import { useWebSocketStore } from '../../stores/websocket.store'
import { Heading, IconButton, Pressable } from '../primitives'
import { DeleteItemDialog } from './DeleteItemDialog'
import { DriveBody } from './DriveBody'
import { DriveBreadcrumbs, type Crumb } from './DriveBreadcrumbs'
import { DriveBurstBanner } from './DriveBurstBanner'
import { DriveDetailPanel } from './DriveDetailPanel'
import { DriveGallery, GallerySkeleton } from './DriveGallery'
import { DriveAddSheet, DriveItemMenu, DriveItemSheet, type MenuTarget } from './DriveItemMenu'
import { DriveTable, type RowActions } from './DriveTable'
import { DriveToolbar } from './DriveToolbar'
import { DriveTree } from './DriveTree'
import { DriveTreeDrawer } from './DriveTreeDrawer'
import { MoveItemDialog } from './MoveItemDialog'
import { NameDialog } from './NameDialog'
import { ShareDialog } from './ShareDialog'
import { useDriveActions } from './useDriveActions'
import { useDriveLayout } from './use-drive-layout'
import {
  filterByType, itemVersion, matchesName, ownerOf, sortItems, type SortKey, type TypeFilter,
} from './drive-model'
import { FileViewer, type ViewerFile } from '../files/FileViewer'
import type { MenuAnchor } from '../files/use-long-press'

/** Items without an NGAC node are never checked; they can be seen and nothing else. */
const READ_ONLY: ObjectPerms = { ...NO_PERMS, read: true }

/** How long after my own change the same change from the server still counts as mine. */
const MINE_MS = 15_000

/** The element that has focus now: where focus returns to when the viewer closes. */
const focused = () => (document.activeElement instanceof HTMLElement ? document.activeElement : null)

/**
 * Tài liệu (DESIGN.md "Tài liệu: lưới, danh sách, panel xem trước"): folder
 * tree, a toolbar, the files of the open folder as a gallery or the hairline
 * table, and a detail panel for the selected item.
 *
 * Where the user is — workspace, folder, shared view — is the URL's business:
 * `?folder=<id>` makes reload, Back/Forward and a pasted link land in the same
 * folder. Only the selection, the filters and the tree's open nodes are client state.
 */
export function DriveScreen() {
  const search = useSearch({ strict: false }) as DriveSearch
  const navigate = useNavigate()
  const router = useRouter()
  const m = useMotionPresets()
  const layout = useDriveLayout()
  const phone = layout === 'phone'
  const connectionLost = useConnectionLost()
  const { workspaceId: wsId, workspaceName } = useActiveWorkspace()
  const people = usePeople(wsId)
  const me = useAuthStore((s) => s.user)
  const [mode, setMode] = useFileViewMode('drive')

  const shared = search.view === 'shared'
  const folderId = shared ? undefined : search.folder
  const workspaceLabel = workspaceDisplayName(workspaceName)

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

  // Selection and the tree's open nodes live in the store.
  const selectedId = useDriveStore((s) => s.selectedItemId)
  const selectItem = useDriveStore((s) => s.selectItem)
  const expandFolders = useDriveStore((s) => s.expandFolders)

  const [query, setQuery] = useState('')
  const [typeFilter, setTypeFilter] = useState<TypeFilter>('all')
  const [sort, setSort] = useState<SortKey>('recent')
  const [dragging, setDragging] = useState(false)
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [menu, setMenu] = useState<MenuTarget | null>(null)
  const [sheetItem, setSheetItem] = useState<DriveItem | null>(null)
  const [addOpen, setAddOpen] = useState(false)
  // Deleting the last row: keep the table up until the row has folded away,
  // then show the empty state (reduced motion swaps at once).
  const [holdTable, setHoldTable] = useState(false)
  const rowsSeen = useRef({ scope: '', n: 0 })
  const fileInput = useRef<HTMLInputElement>(null)
  const cameraInput = useRef<HTMLInputElement>(null)

  const { download } = useDownloadFile()

  // A new place starts with no selection, no filter and an empty search.
  const view = shared ? 'shared' : folderId ?? 'root'
  const scope = `${wsId}:${view}`
  useEffect(() => {
    selectItem(null)
    setQuery('')
    setTypeFilter('all')
    setHoldTable(false)
    setDrawerOpen(false)
    setMenu(null)
    setSheetItem(null)
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

  // Permissions come as one batch for everything on screen.
  const ngacIds = useMemo(() => all.map((i) => i.ngac_node_id).filter(Boolean), [all])
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
  const {
    dialog, setDialog, closeDialog, uploadFiles, uploading, creatingFolder, renaming, moving, trashing,
    submitNewFolder, submitRename, submitMove, submitDelete,
  } = useDriveActions({ workspaceId: wsId, folderId, foreign, selectedId, selectItem, markMine })
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

  // Esc closes the detail panel. Dialogs and menus claim the key on `document`
  // (preventDefault); listening on `window` puts this after them however early
  // the panel was opened, so one Esc closes one thing.
  useEffect(() => {
    if (!selected) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && !e.defaultPrevented) selectItem(null)
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [selected, selectItem])

  const [previewIndex, setPreviewIndex] = useState<number | null>(null)
  const previewOpenerRef = useRef<HTMLElement | null>(null)

  const folderItems = useMemo(() => items.filter((i) => i.item_type === 'folder'), [items])
  // What the viewer pages through: the files that are showing, not the whole folder.
  const folderFiles = useMemo(() => items.filter((i) => i.item_type === 'file'), [items])

  const viewerFiles = useMemo<ViewerFile[]>(() => {
    return folderFiles.map((f) => {
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
    })
  }, [folderFiles, people])

  const openPreview = useCallback(
    (item: DriveItem, openerEl?: HTMLElement | null) => {
      const idx = folderFiles.findIndex((f) => f.id === item.id)
      if (idx >= 0) {
        previewOpenerRef.current = openerEl ?? focused()
        setPreviewIndex(idx)
      }
    },
    [folderFiles],
  )

  const openFolder = useCallback(
    (id?: string) => void navigate({ to: '/drive', search: (prev: DriveSearch) => folderSearch(prev, id) }),
    [navigate],
  )
  const hrefOf = useCallback(
    (id: string) => router.buildLocation({ to: '/drive', search: (prev: DriveSearch) => folderSearch(prev, id) }).href,
    [router],
  )

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

  const onDrop = (e: DragEvent<HTMLDivElement>) => {
    e.preventDefault()
    setDragging(false)
    if (shared || missing) return
    const files = Array.from(e.dataTransfer?.files ?? [])
    if (files.length) void uploadFiles(files)
  }
  const isFileDrag = (e: DragEvent<HTMLDivElement>) => Array.from(e.dataTransfer?.types ?? []).includes('Files')

  // ---- what to show ----
  const crumbsData = folderQ.data?.breadcrumb ?? []
  const folderName = crumbsData[crumbsData.length - 1]?.name
  const status = statusOf(active.error)
  const missing = !!folderId && (foreign || status === 403 || status === 404)
  const title = shared ? 'Được chia sẻ với tôi' : missing ? 'Tài liệu' : folderId ? folderName : 'Tài liệu'
  const crumbs: Crumb[] = shared
    ? [{ label: workspaceLabel }, { id: 'shared', label: 'Được chia sẻ với tôi' }]
    : folderId && !missing
      ? [{ label: workspaceLabel }, ...crumbsData.map((c) => ({ id: c.id, label: c.name }))]
      : [{ label: workspaceLabel }]
  // Until the workspace is known the query is idle, and that is still loading.
  const loading = !active.data && !active.isError
  const canAdd = !shared && !missing
  const newFolder = () => setDialog({ kind: 'new-folder' })

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

  const content =
    mode === 'grid' ? (
      loading ? (
        <GallerySkeleton />
      ) : (
        <DriveGallery
          folders={folderItems}
          files={folderFiles}
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
    ) : (
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

  return (
    <div className="relative flex flex-1 min-h-0 min-w-0">
      {layout === 'desktop' && <div className="w-70 shrink-0 min-h-0">{tree(openFolder)}</div>}

      <section className="flex-1 flex flex-col min-w-0 min-h-0 bg-base" aria-label="Tệp">
        <header className="flex items-center gap-2 px-5 pt-4 pb-3 min-w-0 max-sm:px-4">
          {layout !== 'desktop' && (
            <IconButton aria-label="Mở cây thư mục" aria-haspopup="dialog" className="max-lg:w-11 max-lg:h-11 -ml-2" onClick={() => setDrawerOpen(true)}>
              <Menu size={20} strokeWidth={1.75} />
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

        {connectionLost && (
          <div role="status" className="mx-5 mb-2 flex items-center gap-2.5 px-3 py-2 rounded-surface bg-warning-wash text-sm">
            <WifiOff size={16} strokeWidth={1.75} className="shrink-0" aria-hidden="true" />
            Đang kết nối lại… Thay đổi của người khác sẽ hiện khi có mạng.
          </div>
        )}

        <AnimatePresence>
          {burst && <DriveBurstBanner burst={burst} people={people} row={m.row} />}
        </AnimatePresence>

        <div
          data-testid="drive-dropzone"
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
        {selected && (
          <DriveDetailPanel
            key="detail"
            item={selected}
            perms={permsOf(selected)}
            people={people}
            location={shared ? undefined : folderId ? folderName : workspaceLabel}
            onClose={() => selectItem(null)}
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

      {phone && canAdd && (
        <Pressable
          aria-label="Thêm mới"
          aria-haspopup="dialog"
          onClick={() => setAddOpen(true)}
          style={{ borderRadius: 18 }}
          className="absolute right-4 bottom-4 z-sticky grid place-items-center w-14 h-14 bg-accent
            text-on-accent shadow-overlay"
        >
          <Plus size={24} strokeWidth={1.75} aria-hidden="true" />
        </Pressable>
      )}
      {addOpen && (
        <DriveAddSheet
          onPickFiles={() => fileInput.current?.click()}
          onTakePhoto={() => cameraInput.current?.click()}
          onNewFolder={newFolder}
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
          onOpen={openItem}
          onClose={() => setSheetItem(null)}
        />
      )}

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
        onConfirm={(item, target) => void submitMove(item, target)}
        onClose={closeDialog}
      />
      <ShareDialog item={dialog?.kind === 'share' ? dialog.item : null} people={people} onClose={closeDialog} />
      <DeleteItemDialog
        item={dialog?.kind === 'delete' ? dialog.item : null}
        pending={trashing}
        onConfirm={(item) => void submitDelete(item)}
        onClose={closeDialog}
      />

      {previewIndex !== null && (
        <FileViewer
          files={viewerFiles}
          index={previewIndex}
          origin="drive"
          onClose={() => setPreviewIndex(null)}
          returnFocusRef={previewOpenerRef}
        />
      )}
    </div>
  )
}
