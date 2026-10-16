import type { ComponentProps } from 'react'
import { FileGrid } from './FileGrid'
import { FileThumb } from './FileThumb'
import { useLongPress, type MenuAnchor } from './use-long-press'

/** A group made only of pictures and videos, laid out like an album: square tiles, tight. */
export function MediaGrid(props: Omit<ComponentProps<typeof FileGrid>, 'variant'>) {
  return <FileGrid {...props} variant="media" />
}

interface MediaTileProps {
  fileId: string
  name: string
  mimeType?: string
  version?: string
  selected?: boolean
  onSelect?: () => void
  /** A double click. Enter and Space go through `FileGrid`'s `onOpen`. */
  onOpen?: () => void
  /** A long press. Right click and Shift+F10 go through `FileGrid`'s `onMenu`. */
  onMenu: (anchor: MenuAnchor) => void
}

/** One square picture or video frame in a `MediaGrid`. */
export function MediaTile({ fileId, name, mimeType, version, selected = false, onSelect, onOpen, onMenu }: MediaTileProps) {
  const press = useLongPress(onMenu)
  return (
    <div
      data-grid-item
      role="button"
      aria-label={name}
      aria-pressed={selected}
      style={{ borderRadius: 6 }}
      className={`cursor-default select-none outline-none focus-visible:ring-2 focus-visible:ring-accent
        ${selected ? 'ring-2 ring-inset ring-accent' : ''}`}
      {...press.handlers}
      onClick={() => { if (!press.takeFired()) onSelect?.() }}
      onDoubleClick={() => onOpen?.()}
      onContextMenu={(e) => {
        if (press.isHoldContextMenu()) { e.preventDefault(); e.stopPropagation() }
      }}
    >
      <FileThumb fileId={fileId} name={name} mimeType={mimeType} version={version} size="card" aspect="1" radius={6} iconSize={28} />
    </div>
  )
}
