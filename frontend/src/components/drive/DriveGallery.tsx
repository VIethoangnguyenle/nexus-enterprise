import { memo, type ReactNode } from 'react'
import type { DriveItem } from '../../api/drive'
import type { Arrival } from '../../hooks/useArrivals'
import { formatListTime } from '../../lib/format'
import { personStyle } from '../../lib/person-hue'
import type { PeopleDirectory } from '../../lib/people'
import { FileCard } from '../files/FileCard'
import { FileGrid } from '../files/FileGrid'
import { FolderTile } from '../files/FolderTile'
import { useLongPress, type MenuAnchor } from '../files/use-long-press'
import { itemVersion, ownerOf } from './drive-model'

interface DriveGalleryProps {
  folders: DriveItem[]
  files: DriveItem[]
  people: PeopleDirectory
  selectedId: string | null
  /** Realtime arrivals by item version, from `useArrivals`. */
  fresh: Map<string, Arrival>
  /** Address of a folder, so its tile is a real link. */
  hrefOf: (folderId: string) => string
  onOpenFolder: (folder: DriveItem) => void
  /** A click on a card: the screen decides between selecting and opening. */
  onSelectFile: (file: DriveItem) => void
  /** Double click, Enter or Space on a card. */
  onOpenFile: (file: DriveItem) => void
  /** The ⋯, a right click, Shift+F10 or a long press. */
  onMenu: (item: DriveItem, anchor: MenuAnchor) => void
}

/**
 * The gallery of Tài liệu: "Thư mục" as tiles, then "Tệp" as cards. Each group
 * is one tab stop with arrow keys (`FileGrid`); permissions and what happens
 * on a click are the screen's.
 */
export function DriveGallery(props: DriveGalleryProps) {
  const { folders, files, people, fresh, onMenu } = props
  return (
    <div className="grid gap-5 px-5 pb-6 max-sm:px-4 content-start">
      {folders.length > 0 && (
        <Group title="Thư mục">
          <FileGrid
            label="Thư mục"
            variant="folders"
            onOpen={(i) => props.onOpenFolder(folders[i]!)}
            onMenu={(i, anchor) => onMenu(folders[i]!, anchor)}
          >
            {folders.map((f) => (
              <FolderCell key={f.id} folder={f} href={props.hrefOf(f.id)} onOpen={props.onOpenFolder} onMenu={onMenu} />
            ))}
          </FileGrid>
        </Group>
      )}
      {files.length > 0 && (
        <Group title="Tệp">
          <FileGrid
            label="Tệp"
            onOpen={(i) => props.onOpenFile(files[i]!)}
            onMenu={(i, anchor) => onMenu(files[i]!, anchor)}
          >
            {files.map((f) => (
              <FileCell
                key={f.id}
                file={f}
                people={people}
                selected={props.selectedId === f.id}
                arrival={fresh.get(itemVersion(f))}
                onSelect={props.onSelectFile}
                onOpen={props.onOpenFile}
                onMenu={onMenu}
              />
            ))}
          </FileGrid>
        </Group>
      )}
    </div>
  )
}

function Group({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="grid gap-2">
      <span className="text-label text-ink-muted" aria-hidden="true">{title}</span>
      {children}
    </section>
  )
}

/** Placeholders the shape of the cards, while the folder loads. */
export function GallerySkeleton() {
  return (
    <div aria-busy="true" className="grid gap-2 md:gap-3 px-5 pb-6 max-sm:px-4 grid-cols-2 md:grid-cols-[repeat(auto-fill,minmax(200px,1fr))]">
      {[0, 1, 2, 3, 4, 5, 6, 7].map((i) => (
        <div key={i} className="grid gap-2 rounded-xl bg-raised px-1.5 pt-1.5 pb-2.5">
          <div className="skeleton rounded-lg" style={{ aspectRatio: '16 / 10' }} />
          <div className="skeleton h-3.5 w-3/4 rounded-sm mx-1" />
          <div className="skeleton h-3 w-1/2 rounded-sm mx-1" />
        </div>
      ))}
    </div>
  )
}

interface FolderCellProps {
  folder: DriveItem
  href: string
  onOpen: (folder: DriveItem) => void
  onMenu: (item: DriveItem, anchor: MenuAnchor) => void
}

/** A folder tile that also answers a long press with its menu: the tile has no ⋯ of its own. */
const FolderCell = memo(function FolderCell({ folder, href, onOpen, onMenu }: FolderCellProps) {
  const press = useLongPress((anchor) => onMenu(folder, anchor))
  return (
    <div
      {...press.handlers}
      onClickCapture={(e) => {
        // The lift after a hold must not also open the folder.
        if (press.takeFired()) { e.preventDefault(); e.stopPropagation() }
      }}
      onContextMenu={(e) => { if (press.isHoldContextMenu()) { e.preventDefault(); e.stopPropagation() } }}
      className="min-w-0 select-none"
    >
      <FolderTile name={folder.name} href={href} onOpen={() => onOpen(folder)} />
    </div>
  )
})

interface FileCellProps {
  file: DriveItem
  people: PeopleDirectory
  selected: boolean
  arrival?: Arrival
  onSelect: (file: DriveItem) => void
  onOpen: (file: DriveItem) => void
  onMenu: (item: DriveItem, anchor: MenuAnchor) => void
}

/** A card, washed in its owner's colour for a moment when someone else changes the file. */
const FileCell = memo(function FileCell({ file, people, selected, arrival, onSelect, onOpen, onMenu }: FileCellProps) {
  const owner = ownerOf(file, people)
  const byOther = arrival?.source === 'other'
  return (
    <div
      data-arrival={byOther || undefined}
      style={byOther ? personStyle(arrival?.author ?? owner.hueKey) : undefined}
      className={`min-w-0 rounded-xl ${byOther ? 'rt-wash' : ''}`}
    >
      <FileCard
        fileId={file.id}
        name={file.name}
        mimeType={file.mime_type}
        version={itemVersion(file)}
        owner={owner}
        timeLabel={formatListTime(file.updated_at)}
        selected={selected}
        onSelect={() => onSelect(file)}
        onOpen={() => onOpen(file)}
        onMenu={(anchor) => onMenu(file, anchor)}
      />
      {byOther && <span className="rt-tag block px-2.5 pb-1.5 text-xs font-semibold">vừa cập nhật</span>}
    </div>
  )
})
