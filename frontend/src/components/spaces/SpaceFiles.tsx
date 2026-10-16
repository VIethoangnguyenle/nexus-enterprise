import { useMemo, useRef, useState, type KeyboardEvent, type ReactNode } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { CircleAlert, Eye, FolderOpen, LayoutGrid, List } from 'lucide-react'
import { useChannelDrive, useDriveFolder } from '../../hooks/useDrive'
import type { DriveItem } from '../../api/drive'
import { folderSearch, type DriveSearch } from '../../lib/drive-search'
import { fileKind } from '../../lib/file-kind'
import { groupFiles, isMediaFile } from '../../lib/file-groups'
import { formatBytes, formatDateTime, formatListTime, formatRelative, toMillis } from '../../lib/format'
import type { PeopleDirectory } from '../../lib/people'
import { useFileViewMode } from '../../stores/file-view.store'
import { Button, MenuItem, PersonChip, Popover, Pressable } from '../primitives'
import { itemVersion, ownerOf } from '../drive/drive-model'
import { FileViewer, type ViewerFile } from '../files/FileViewer'
import { FileCard } from '../files/FileCard'
import { FileGrid } from '../files/FileGrid'
import { MediaGrid, MediaTile } from '../files/MediaGrid'
import type { MenuAnchor } from '../files/use-long-press'
import { EmptyState } from './EmptyState'

interface SpaceFilesProps {
  workspaceId: string
  channelId: string
  people: PeopleDirectory
  /** Switch to the conversation tab (where files are attached). */
  onGoToChat: () => void
  /** The moment "today" is measured from. Tests fix it; the app leaves it. */
  now?: Date
}

type KindFilter = 'all' | 'media' | 'docs' | 'other'

const CHIPS: { key: KindFilter; label: string }[] = [
  { key: 'all', label: 'Tất cả' },
  { key: 'media', label: 'Ảnh & video' },
  { key: 'docs', label: 'Tài liệu' },
  { key: 'other', label: 'Khác' },
]

const EMPTY_TEXT: Record<Exclude<KindFilter, 'all'>, string> = {
  media: 'Chưa có ảnh hay video nào trong nhóm.',
  docs: 'Chưa có tài liệu nào trong nhóm.',
  other: 'Chưa có tệp loại khác nào trong nhóm.',
}

/** Kinds that read as documents; pictures and videos are media, everything else is "Khác". */
const DOC_LABELS = new Set(['Văn bản', 'PDF', 'Bảng tính', 'Trình chiếu'])

function filterOf(f: DriveItem): Exclude<KindFilter, 'all'> {
  if (isMediaFile(f.name, f.mime_type)) return 'media'
  return DOC_LABELS.has(fileKind(f.name, f.mime_type).label) ? 'docs' : 'other'
}

const LIST_COLS =
  'grid grid-cols-[minmax(0,1fr)_minmax(0,180px)_120px_88px] max-lg:grid-cols-[minmax(0,1fr)_88px] gap-4 items-center px-3'

/**
 * Files shared in a space (DESIGN.md §6 "Tab Tệp của nhóm"): the same cards and
 * grid as Tài liệu, filtered by kind with chips and grouped by recency. A group
 * made only of pictures and videos is an album of square tiles. The channel
 * drive's root holds one folder per space; chat uploads land inside it, so both
 * levels are listed. The viewer receives the filtered list, so ←/→ never lands
 * on a hidden file.
 *
 * "Tới tin nhắn" is not offered: a drive item records neither the message nor
 * the thread it was attached to, so the data cannot locate it.
 */
export function SpaceFiles({ workspaceId, channelId, people, onGoToChat, now }: SpaceFilesProps) {
  const navigate = useNavigate()
  const root = useChannelDrive(workspaceId, channelId)
  const folder = root.data?.items?.find((i) => i.item_type === 'folder')
  const inner = useDriveFolder(workspaceId, folder?.id)
  const bodyRef = useRef<HTMLDivElement>(null)
  const openerRef = useRef<HTMLElement | null>(null)
  const menuAnchorRef = useRef<HTMLSpanElement>(null)
  const [mode, setMode] = useFileViewMode('space-files')
  const [filter, setFilter] = useState<KindFilter>('all')
  const [openIndex, setOpenIndex] = useState<number | null>(null)
  const [menu, setMenu] = useState<{ index: number; anchor: MenuAnchor } | null>(null)

  const allFiles = useMemo(() => {
    const all = [...(root.data?.items ?? []), ...(folder ? inner.data?.items ?? [] : [])]
    return all
      .filter((i) => i.item_type === 'file' && i.status === 'active')
      .sort((a, b) => toMillis(b.updated_at) - toMillis(a.updated_at))
  }, [root.data, inner.data, folder])

  const files = useMemo(
    () => (filter === 'all' ? allFiles : allFiles.filter((f) => filterOf(f) === filter)),
    [allFiles, filter],
  )

  const groups = useMemo(() => groupFiles(files, (f) => toMillis(f.updated_at), now), [files, now])
  // Groups keep the sort order, so walking them in turn is the order on screen.
  const ordered = useMemo(() => groups.flatMap((g) => g.items), [groups])
  const indexOfFile = useMemo(() => new Map(ordered.map((f, i) => [f.id, i])), [ordered])

  const viewerFiles = useMemo<ViewerFile[]>(() => {
    return ordered.map((f) => {
      const owner = people.byUserId.get(f.owner_id)
      return {
        id: f.id,
        name: f.name,
        mime: f.mime_type,
        size: f.size_bytes,
        uploaderName: owner?.name ?? f.owner_name,
        uploaderAvatarUrl: owner?.avatarUrl,
        uploaderId: f.owner_id,
        time: f.updated_at ?? f.created_at,
        parentFolderId: f.parent_id,
      }
    })
  }, [ordered, people])

  const loading = root.isLoading || (!!folder && inner.isLoading)

  /** The tile or row on screen for the `index`th file; items sit in the DOM in display order. */
  const elementAt = (index: number): HTMLElement | null =>
    bodyRef.current?.querySelectorAll<HTMLElement>('[data-grid-item], [data-row]')[index] ?? null

  const openAt = (index: number) => {
    openerRef.current = elementAt(index)
    setOpenIndex(index)
  }

  const toDrive = (folderId: string | undefined) => {
    if (folderId) void navigate({ to: '/drive', search: (prev: DriveSearch) => folderSearch(prev, folderId) })
  }

  const onRowKeys = (e: KeyboardEvent<HTMLDivElement>) => {
    if (e.key !== 'ArrowDown' && e.key !== 'ArrowUp') return
    const rows = Array.from(bodyRef.current?.querySelectorAll<HTMLElement>('[data-row]') ?? [])
    const i = rows.indexOf(document.activeElement as HTMLElement)
    const next = rows[e.key === 'ArrowDown' ? Math.min(i + 1, rows.length - 1) : Math.max(i - 1, 0)]
    if (next) {
      e.preventDefault()
      next.focus()
    }
  }

  if (root.isError) {
    return (
      <EmptyState
        icon={<CircleAlert size={24} strokeWidth={1.75} />}
        text="Chưa tải được tệp của nhóm."
        action={<Button variant="soft" size="sm" onClick={() => root.refetch()}>Thử lại</Button>}
      />
    )
  }
  if (!loading && allFiles.length === 0) {
    return (
      <EmptyState
        icon={<FolderOpen size={24} strokeWidth={1.75} />}
        text="Chưa có tệp nào. Tệp đính kèm trong trò chuyện sẽ hiện ở đây."
        action={<Button variant="soft" size="sm" onClick={onGoToChat}>Đính kèm trong Trò chuyện</Button>}
      />
    )
  }

  const menuFile = menu ? ordered[menu.index] : undefined
  const toggle = (value: 'grid' | 'list', label: string, icon: ReactNode) => (
    <Pressable
      aria-label={label}
      aria-pressed={mode === value}
      title={label}
      onClick={() => setMode(value)}
      className={`grid place-items-center w-8 h-8 rounded-md ${mode === value ? 'bg-accent-wash text-ink' : 'text-ink-muted hover:bg-hover'}`}
    >
      {icon}
    </Pressable>
  )

  return (
    <div className="px-5 py-2 max-md:px-3">
      <div className="flex items-center gap-2 py-2 max-md:overflow-x-auto">
        <div role="radiogroup" aria-label="Loại tệp" className="flex items-center gap-1.5 shrink-0">
          {CHIPS.map((c) => (
            <Pressable
              key={c.key}
              role="radio"
              aria-checked={filter === c.key}
              onClick={() => setFilter(c.key)}
              className={`h-8 px-3 rounded-md text-small whitespace-nowrap
                ${filter === c.key ? 'bg-accent-wash text-ink font-semibold' : 'bg-sunk text-ink font-medium hover:bg-hover'}`}
            >
              {c.label}
            </Pressable>
          ))}
        </div>
        <span className="flex-1" />
        <div className="flex items-center gap-1 shrink-0">
          {toggle('grid', 'Lưới', <LayoutGrid size={16} aria-hidden="true" />)}
          {toggle('list', 'Danh sách', <List size={16} aria-hidden="true" />)}
        </div>
        {folder && (
          <Button variant="ghost" size="sm" className="shrink-0" onClick={() => toDrive(folder.id)}>
            Mở trong Tài liệu
          </Button>
        )}
      </div>

      <div ref={bodyRef} aria-busy={loading || undefined}>
        {loading ? (
          <div className="grid gap-2 pt-2">
            {[0, 1, 2].map((i) => <div key={i} className="skeleton h-11 w-full rounded-md" />)}
          </div>
        ) : files.length === 0 ? (
          <EmptyState
            icon={<FolderOpen size={24} strokeWidth={1.75} />}
            text={EMPTY_TEXT[filter as Exclude<KindFilter, 'all'>]}
            compact
          />
        ) : (
          groups.map((g) => {
            const album = mode === 'grid' && g.items.every((f) => isMediaFile(f.name, f.mime_type))
            const at = (i: number) => indexOfFile.get(g.items[i]!.id)!
            return (
              <section key={g.key} className="pb-4">
                <h3 className="max-md:sticky max-md:top-0 z-sticky bg-base py-2 text-xs font-semibold text-ink-muted">
                  {g.label}
                </h3>
                {mode === 'list' ? (
                  <div className="grid" onKeyDown={onRowKeys}>
                    {g.items.map((f) => {
                      const kind = fileKind(f.name, f.mime_type)
                      const Icon = kind.icon
                      const owner = ownerOf(f, people)
                      return (
                        <Pressable
                          key={f.id}
                          data-row
                          onClick={() => openAt(indexOfFile.get(f.id)!)}
                          aria-label={`Xem trước ${f.name}`}
                          className={`${LIST_COLS} w-full h-11 border-b border-line text-sm text-ink rounded-none
                            hover:bg-hover transition-colors duration-quick`}
                        >
                          <span className="flex items-center gap-2.5 min-w-0">
                            <span className={`grid place-items-center w-7 h-7 rounded-md shrink-0 ${kind.tile}`} aria-hidden="true">
                              <Icon size={16} strokeWidth={1.75} />
                            </span>
                            <span className="truncate font-medium">{f.name}</span>
                          </span>
                          <span className="max-lg:hidden min-w-0">
                            <PersonChip name={owner.name} hueKey={owner.hueKey} />
                          </span>
                          <span className="max-lg:hidden text-right text-ink-muted tnum" title={formatDateTime(f.updated_at)}>
                            {formatRelative(f.updated_at, now)}
                          </span>
                          <span className="text-right text-ink-muted tnum">{formatBytes(f.size_bytes)}</span>
                        </Pressable>
                      )
                    })}
                  </div>
                ) : album ? (
                  <MediaGrid
                    label={g.label}
                    onOpen={(i) => openAt(at(i))}
                    onMenu={(i, anchor) => setMenu({ index: at(i), anchor })}
                  >
                    {g.items.map((f) => (
                      <MediaTile
                        key={f.id}
                        fileId={f.id}
                        name={f.name}
                        mimeType={f.mime_type}
                        version={itemVersion(f)}
                        onSelect={() => openAt(indexOfFile.get(f.id)!)}
                        onOpen={() => openAt(indexOfFile.get(f.id)!)}
                        onMenu={(anchor) => setMenu({ index: indexOfFile.get(f.id)!, anchor })}
                      />
                    ))}
                  </MediaGrid>
                ) : (
                  <FileGrid
                    label={g.label}
                    onOpen={(i) => openAt(at(i))}
                    onMenu={(i, anchor) => setMenu({ index: at(i), anchor })}
                  >
                    {g.items.map((f) => (
                      <FileCard
                        key={f.id}
                        fileId={f.id}
                        name={f.name}
                        mimeType={f.mime_type}
                        version={itemVersion(f)}
                        owner={ownerOf(f, people)}
                        timeLabel={formatListTime(f.updated_at, now)}
                        onSelect={() => openAt(indexOfFile.get(f.id)!)}
                        onOpen={() => openAt(indexOfFile.get(f.id)!)}
                        onMenu={(anchor) => setMenu({ index: indexOfFile.get(f.id)!, anchor })}
                      />
                    ))}
                  </FileGrid>
                )}
              </section>
            )
          })
        )}
      </div>

      <span
        ref={menuAnchorRef}
        aria-hidden="true"
        className="fixed w-0 h-0 pointer-events-none"
        style={menu ? { left: menu.anchor.x, top: menu.anchor.y } : undefined}
      />
      <Popover
        open={!!menu && !!menuFile}
        onClose={() => {
          const back = menu ? elementAt(menu.index) : null
          setMenu(null)
          back?.focus()
        }}
        anchorRef={menuAnchorRef}
        role="menu"
        label={menuFile ? `Tùy chọn cho ${menuFile.name}` : undefined}
      >
        {menuFile && menu && (
          <>
            <MenuItem icon={<Eye size={16} aria-hidden="true" />} onClick={() => openAt(menu.index)}>
              Xem trước
            </MenuItem>
            {menuFile.parent_id && (
              <MenuItem icon={<FolderOpen size={16} aria-hidden="true" />} onClick={() => toDrive(menuFile.parent_id)}>
                Mở trong Tài liệu
              </MenuItem>
            )}
          </>
        )}
      </Popover>

      {openIndex !== null && (
        <FileViewer
          files={viewerFiles}
          index={openIndex}
          origin="chat"
          onClose={() => setOpenIndex(null)}
          onOpenInDrive={(vf) => toDrive(vf.parentFolderId)}
          returnFocusRef={openerRef}
        />
      )}
    </div>
  )
}
