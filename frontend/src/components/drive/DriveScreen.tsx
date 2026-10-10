import { useCallback, useEffect, useMemo, useRef, useState, type DragEvent } from 'react'
import { useNavigate, useSearch } from '@tanstack/react-router'
import { AnimatePresence } from 'motion/react'
import { Plus, Upload, WifiOff } from 'lucide-react'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { useArrivals } from '../../hooks/useArrivals'
import { useConnectionLost } from '../../hooks/useConnectionLost'
import { useDownloadFile } from '../../hooks/useDownloadUrl'
import { useDriveFolder, useDriveItem, useSharedWithMe } from '../../hooks/useDrive'
import { usePeople } from '../../hooks/usePeople'
import { usePermissions } from '../../hooks/usePermissions'
import { NO_PERMS, type ObjectPerms } from '../../api/access'
import type { DriveItem } from '../../api/drive'
import { folderSearch, sharedSearch, textsSearch, type DriveSearch } from '../../lib/drive-search'
import { statusOf } from '../../lib/errors'
import { useMotionPresets } from '../../lib/motion'
import { workspaceDisplayName } from '../../lib/workspace'
import { useAuthStore } from '../../stores/auth.store'
import { useDriveStore } from '../../stores/drive.store'
import { useWebSocketStore } from '../../stores/websocket.store'
import { Button, FilterChip, Heading, SearchField } from '../primitives'
import { DeleteItemDialog } from './DeleteItemDialog'
import { DriveBody } from './DriveBody'
import { DriveBreadcrumbs, type Crumb } from './DriveBreadcrumbs'
import { DriveBurstBanner } from './DriveBurstBanner'
import { DriveDetailPanel } from './DriveDetailPanel'
import { DriveTable, type RowActions } from './DriveTable'
import { DriveTree } from './DriveTree'
import { MoveItemDialog } from './MoveItemDialog'
import { NameDialog } from './NameDialog'
import { ShareDialog } from './ShareDialog'
import { useDriveActions } from './useDriveActions'
import {
  itemVersion, matchesName, ownerOf, sortItems,
} from './drive-model'

/** Items without an NGAC node are never checked; they can be seen and nothing else. */
const READ_ONLY: ObjectPerms = { ...NO_PERMS, read: true }

/** How long after my own change the same change from the server still counts as mine. */
const MINE_MS = 15_000

/**
 * Tài liệu (design/mockups/core-screens.html §2): folder tree, the hairline
 * table of the open folder, and a detail panel for the selected item.
 *
 * Where the user is — workspace, folder, shared view — is the URL's business:
 * `?folder=<id>` makes reload, Back/Forward and a pasted link land in the same
 * folder. Only the selection and the tree's open nodes are client state.
 */
export function DriveScreen() {
  const search = useSearch({ strict: false }) as DriveSearch
  const navigate = useNavigate()
  const m = useMotionPresets()
  const connectionLost = useConnectionLost()
  const { workspaceId: wsId, workspaceName } = useActiveWorkspace()
  const people = usePeople(wsId)
  const me = useAuthStore((s) => s.user)

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
  const [dragging, setDragging] = useState(false)
  // Deleting the last row: keep the table up until the row has folded away,
  // then show the empty state (reduced motion swaps at once).
  const [holdTable, setHoldTable] = useState(false)
  const rowsSeen = useRef({ scope: '', n: 0 })
  const fileInput = useRef<HTMLInputElement>(null)

  const { download } = useDownloadFile()

  // A new place starts with no selection, no filter and an empty search.
  const view = shared ? 'shared' : folderId ?? 'root'
  const scope = `${wsId}:${view}`
  useEffect(() => {
    selectItem(null)
    setQuery('')
    setHoldTable(false)
  }, [scope, selectItem])

  const trail = useMemo(() => (folderQ.data?.breadcrumb ?? []).map((c) => c.id), [folderQ.data])
  useEffect(() => {
    // Open the ancestors, so the tree shows where a reloaded or linked folder is.
    if (!shared && trail.length > 1) expandFolders(trail.slice(0, -1))
  }, [shared, trail, expandFolders])

  const items = useMemo(() => sortItems(all.filter((i) => matchesName(i, query))), [all, query])
  if (rowsSeen.current.scope !== scope) rowsSeen.current = { scope, n: 0 }
  if (all.length === 0 && rowsSeen.current.n > 0 && !m.reduced && !holdTable) setHoldTable(true)
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

  const openFolder = useCallback(
    (id?: string) => void navigate({ to: '/drive', search: (prev: DriveSearch) => folderSearch(prev, id) }),
    [navigate],
  )

  const actions: RowActions = useMemo(
    () => ({
      onSelect: (item) => selectItem(item.id),
      onDownload: (item) => void download(item),
      onShare: (item) => setDialog({ kind: 'share', item }),
      onMove: (item) => setDialog({ kind: 'move', item }),
      onRename: (item) => setDialog({ kind: 'rename', item }),
      onDelete: (item) => setDialog({ kind: 'delete', item }),
    }),
    [selectItem, download],
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

  return (
    <div className="relative flex flex-1 min-h-0 min-w-0">
      <div className="hidden lg:block w-70 shrink-0 min-h-0">
        <DriveTree
          workspaceId={wsId}
          workspaceLabel={workspaceLabel}
          currentFolderId={folderId}
          trail={trail}
          query={query}
          onQuery={setQuery}
          sharedActive={shared}
          onOpen={openFolder}
        />
      </div>

      <section className="flex-1 flex flex-col min-w-0 min-h-0 bg-base" aria-label="Tệp">
        <header className="flex items-center gap-4 px-5 pt-4 pb-3 min-w-0">
          <div className="grid gap-0.5 min-w-0">
            <DriveBreadcrumbs crumbs={crumbs} />
            {title ? (
              <Heading as="h1" look="panel" className="truncate">{title}</Heading>
            ) : (
              <div className="skeleton h-6 w-40 rounded-sm" aria-busy="true" />
            )}
          </div>
          {!shared && !missing && (
            <div className="ml-auto flex items-center gap-2 shrink-0">
              <Button variant="soft" size="sm" onClick={() => setDialog({ kind: 'new-folder' })}>
                <Plus size={16} strokeWidth={1.75} aria-hidden="true" />
                <span className="max-sm:sr-only">Thư mục mới</span>
              </Button>
              <Button size="sm" loading={uploading} onClick={() => fileInput.current?.click()}>
                <Upload size={16} strokeWidth={1.75} aria-hidden="true" />
                <span className="max-sm:sr-only">Tải lên</span>
              </Button>
            </div>
          )}
        </header>

        <div className="flex flex-wrap items-center gap-2 px-5 pb-3">
          <div className="flex gap-2 lg:hidden">
            <FilterChip pressed={!shared} onClick={() => openFolder()}>Tất cả tệp</FilterChip>
            <FilterChip pressed={shared} onClick={() => void navigate({ to: '/drive', search: (p: DriveSearch) => sharedSearch(p) })}>
              Chia sẻ với tôi
            </FilterChip>
            <FilterChip pressed={false} onClick={() => void navigate({ to: '/drive', search: (p: DriveSearch) => textsSearch(p) })}>
              Văn bản
            </FilterChip>
          </div>
          {/* The list panel carries the search from lg up; below that it sits here. */}
          <SearchField
            label="Tìm tệp hoặc thư mục"
            moduleSearch
            tone="sunk"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            className="ml-auto w-full sm:w-64 lg:hidden"
          />
        </div>

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
            onClearQuery={() => setQuery('')}
            onPickFiles={() => fileInput.current?.click()}
            view={view}
            route={m.route}
            table={
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
            }
          />
        </div>
      </section>

      <AnimatePresence>
        {selected && (
          <DriveDetailPanel
            key="detail"
            item={selected}
            perms={permsOf(selected)}
            people={people}
            onClose={() => selectItem(null)}
            onShare={actions.onShare}
          />
        )}
      </AnimatePresence>

      {/* Chooses files for the Tải lên buttons. Nothing models a hidden file
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
    </div>
  )
}
