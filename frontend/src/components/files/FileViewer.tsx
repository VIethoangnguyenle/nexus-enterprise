import { useCallback, useEffect, useRef, useState, type RefObject } from 'react'
import { createPortal } from 'react-dom'
import { ChevronLeft, ChevronRight, Download, FolderOpen, MoreHorizontal, X } from 'lucide-react'
import { useFilePreview } from '../../hooks/useFilePreview'
import { useDownloadFile } from '../../hooks/useDownloadUrl'
import { useModalFocus } from '../../hooks/useModalFocus'
import { fileKind } from '../../lib/file-kind'
import { formatBytes, formatDateTime } from '../../lib/format'
import { Button, IconButton, PersonChip, Pressable } from '../primitives'
import { failureOf, FileViewerBody } from './FileViewerBody'
import { ImageTools, type ImageZoom } from './ImageBody'
import { isConvertible, splitMiddle } from './viewer-text'

/**
 * One viewer for every drive file, opened from a message, a thread, a group's
 * Tệp tab or Tài liệu (design/mockups/file-viewer.html, DESIGN.md §6).
 *
 * Mount it while a file is open and unmount it on `onClose`:
 *
 *   {open && <FileViewer files={list} index={i} onClose={close} returnFocusRef={openerRef} />}
 *
 * Props
 * - `files`    the sequence ←/→ walks, in display order and without wrapping: the
 *              files of one message (images first, then files), or the list the
 *              user is looking at. One file means no previous/next.
 * - `index`    where to start. The viewer moves on its own after that.
 * - `onClose`  Esc, the close button and a click on the scrim. Only closing is the
 *              caller's; the viewer returns focus to whatever had it (or to
 *              `returnFocusRef` when given).
 * - `origin`   `'chat'` (default) words the sender as "gửi lúc"; `'drive'` as
 *              "tải lên" and never offers "Mở trong Tài liệu".
 * - `onOpenInDrive`  called with the shown file when "Mở trong Tài liệu" is
 *              chosen. The button appears only if this is set, the file has a
 *              `parentFolderId` and `origin` is not `'drive'`.
 * - `returnFocusRef`  element to focus on close; defaults to the opener.
 *
 * ViewerFile: `id` is the drive file id (never shown); `uploaderName` /
 * `uploaderAvatarUrl` / `uploaderId` (colour key only) describe who sent or
 * uploaded it; `time` is any timestamp `lib/format` reads. Header data comes
 * from the opener, so only the middle waits for the preview request.
 */
export interface ViewerFile {
  id: string
  name: string
  mime?: string
  size?: number
  uploaderName?: string
  uploaderAvatarUrl?: string
  uploaderId?: string
  time?: unknown
  parentFolderId?: string
}

export interface FileViewerProps {
  files: ViewerFile[]
  index: number
  onClose: () => void
  origin?: 'chat' | 'drive'
  onOpenInDrive?: (file: ViewerFile) => void
  returnFocusRef?: RefObject<HTMLElement | null>
}

const NO_NAV_TARGET = 'INPUT,TEXTAREA,SELECT,VIDEO,AUDIO'

export function FileViewer({ files, index, onClose, origin = 'chat', onOpenInDrive, returnFocusRef }: FileViewerProps) {
  const count = files.length
  const [at, setAt] = useState(index)
  const pos = Math.min(Math.max(at, 0), Math.max(count - 1, 0))
  const file = files[pos]

  const [zoomState, setZoomState] = useState<{ id: string; zoom: ImageZoom }>({ id: '', zoom: 'fit' })
  const zoom = zoomState.id === file?.id ? zoomState.zoom : 'fit'
  const setZoom = useCallback((z: ImageZoom) => file && setZoomState({ id: file.id, zoom: z }), [file])

  const [sheetOpen, setSheetOpen] = useState(false)
  const [live, setLive] = useState('')

  const surfaceRef = useRef<HTMLDivElement>(null)
  const closeRef = useRef<HTMLButtonElement>(null)
  const moreRef = useRef<HTMLButtonElement>(null)
  const sheetRef = useRef<HTMLDivElement>(null)
  const titleId = 'file-viewer-title'

  const preview = useFilePreview(file?.id)
  const failure = preview.isError ? failureOf(preview.error) : null
  const denied = failure === 'denied'
  const gone = failure === 'gone'

  const { download } = useDownloadFile()
  const onDownload = useCallback(() => file && void download({ id: file.id, name: file.name }), [download, file])

  useModalFocus({ active: true, surfaceRef, onClose, initialFocusRef: closeRef })

  const returnTo = useRef(returnFocusRef)
  useEffect(() => {
    returnTo.current = returnFocusRef
  })
  useEffect(() => () => returnTo.current?.current?.focus(), [])

  // Sheet: focus its first action on open, the ⋯ button on close.
  const sheetWasOpen = useRef(false)
  useEffect(() => {
    if (sheetOpen) sheetRef.current?.querySelector<HTMLElement>('button')?.focus()
    else if (sheetWasOpen.current) moreRef.current?.focus()
    sheetWasOpen.current = sheetOpen
  }, [sheetOpen])

  // The phone sheet is a modal layer over the viewer: it takes Esc (so the
  // viewer stays open) and Tab (the viewer behind it is inert).
  useEffect(() => {
    if (!sheetOpen) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault()
        e.stopPropagation()
        setSheetOpen(false)
      } else if (e.key === 'Tab') {
        const nodes = Array.from(sheetRef.current?.querySelectorAll<HTMLElement>('button:not([disabled])') ?? [])
        const first = nodes[0]
        const last = nodes[nodes.length - 1]
        if (!first || !last) return
        if (e.shiftKey && document.activeElement === first) {
          e.preventDefault()
          last.focus()
        } else if (!e.shiftKey && document.activeElement === last) {
          e.preventDefault()
          first.focus()
        }
      }
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [sheetOpen])

  const go = useCallback(
    (to: number) => {
      if (to < 0 || to >= count || to === pos) return
      setAt(to)
      setSheetOpen(false)
    },
    [count, pos],
  )

  useEffect(() => {
    if (count > 1 && file) setLive(`Tệp ${pos + 1} / ${count}, ${file.name}`)
    // Announced when the shown file changes, not on every render.
  }, [file?.id])

  const waited = useRef(false)
  useEffect(() => {
    if (preview.isPending) waited.current = true
    else if (waited.current && preview.data && file && isConvertible(file.name)) {
      waited.current = false
      setLive('Bản xem trước đã sẵn sàng')
    }
  }, [preview.isPending, preview.data, file])

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const surface = surfaceRef.current
      const modals = document.querySelectorAll('[aria-modal="true"]')
      if (!surface || modals[modals.length - 1] !== surface || sheetOpen) return
      if (e.metaKey || e.ctrlKey || e.altKey) return
      const target = e.target as HTMLElement | null
      if (e.key === 'ArrowLeft' || e.key === 'ArrowRight') {
        if (target?.matches?.(NO_NAV_TARGET)) return
        e.preventDefault()
        go(pos + (e.key === 'ArrowRight' ? 1 : -1))
      } else if (e.key === '+' || e.key === '=') setZoom('one')
      else if (e.key === '-' || e.key === '0') setZoom('fit')
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [go, pos, setZoom, sheetOpen])

  if (!file) return null

  const kind = fileKind(file.name, file.mime)
  const Icon = kind.icon
  const [head, tail] = splitMiddle(file.name)
  const typeLine = [kind.label, file.size ? formatBytes(file.size) : ''].filter(Boolean).join(' · ')
  const verb = origin === 'drive' ? 'tải lên' : 'gửi lúc'
  const when = file.time ? formatDateTime(file.time) : ''
  const showActions = !denied && !gone
  const canOpenInDrive = showActions && origin !== 'drive' && !!onOpenInDrive && !!file.parentFolderId
  const openInDrive = () => {
    setSheetOpen(false)
    onOpenInDrive?.(file)
  }
  const isImage = preview.data?.kind === 'native' && preview.data.mime.startsWith('image/')

  const sender = file.uploaderName ? (
    <span className="inline-flex items-center gap-1.5 min-w-0">
      <PersonChip name={file.uploaderName} hueKey={file.uploaderId} avatarUrl={file.uploaderAvatarUrl} />
      {when && <span className="shrink-0">{verb} <time>{when}</time></span>}
    </span>
  ) : null

  const navBtn = 'grid place-items-center rounded-full bg-overlay text-ink shadow-overlay hover:bg-hover'

  return createPortal(
    <div
      className="fixed inset-0 z-modal bg-scrim md:p-8 animate-fade-in"
      onMouseDown={(e) => e.target === e.currentTarget && onClose()}
    >
      <div
        ref={surfaceRef}
        role="dialog"
        aria-modal="true"
        {...(denied ? { 'aria-label': 'Trình xem tệp' } : { 'aria-labelledby': titleId })}
        className="relative h-full grid grid-rows-[auto_minmax(0,1fr)_auto] bg-sunk overflow-hidden md:rounded-lg md:shadow-overlay select-none"
      >
        <div className="contents" inert={sheetOpen}>
          <header className="flex items-center gap-3 min-h-14 px-3 md:px-4 py-2 bg-base border-b border-line">
            <IconButton
              ref={closeRef}
              type="button"
              onClick={onClose}
              title="Đóng (Esc)"
              aria-label="Đóng trình xem"
              className="max-md:order-first max-md:min-w-11 max-md:min-h-11 md:order-last"
            >
              <X size={18} strokeWidth={1.75} />
            </IconButton>

            {!denied && (
              <span className={`max-md:hidden grid place-items-center w-9 h-9 rounded-md shrink-0 ${kind.tile}`} aria-hidden="true">
                <Icon size={18} strokeWidth={1.75} />
              </span>
            )}
            <div className={`min-w-0 flex-1 ${gone ? 'opacity-60' : ''}`}>
              <b id={titleId} className="flex text-sm text-ink leading-tight" title={denied ? undefined : file.name}>
                {denied ? (
                  'Trình xem tệp'
                ) : (
                  <>
                    <span className="truncate min-w-0">{head}</span>
                    {tail && <span className="shrink-0 whitespace-pre">{tail}</span>}
                  </>
                )}
              </b>
              {!denied && (
                <div className="flex items-center gap-1.5 text-xs text-ink-muted tnum mt-0.5 min-w-0">
                  <span className="shrink-0">{typeLine}</span>
                  {sender && <span aria-hidden="true" className="max-md:hidden">·</span>}
                  {sender && <span className="max-md:hidden min-w-0">{sender}</span>}
                  {gone && <><span aria-hidden="true">·</span><span>Đã xoá</span></>}
                </div>
              )}
            </div>

            {showActions && (
              <div className="flex items-center gap-1 shrink-0">
                <div className="max-md:hidden flex items-center gap-1">
                  {canOpenInDrive && (
                    <Button
                      type="button"
                      variant="ghost"
                      size="sm"
                      onClick={openInDrive}
                      aria-label="Mở trong Tài liệu"
                      title="Mở thư mục chứa tệp trong Tài liệu"
                    >
                      <FolderOpen size={16} strokeWidth={1.75} aria-hidden="true" />
                      <span className="max-lg:hidden">Mở trong Tài liệu</span>
                    </Button>
                  )}
                  <Button type="button" variant="soft" size="sm" onClick={onDownload} aria-label="Tải xuống" title="Tải xuống">
                    <Download size={16} strokeWidth={1.75} aria-hidden="true" />
                    <span className="max-lg:hidden">Tải xuống</span>
                  </Button>
                </div>
                <IconButton
                  ref={moreRef}
                  type="button"
                  className="md:hidden max-md:min-w-11 max-md:min-h-11"
                  aria-label="Thao tác với tệp"
                  aria-haspopup="dialog"
                  aria-expanded={sheetOpen}
                  onClick={() => setSheetOpen(true)}
                >
                  <MoreHorizontal size={18} strokeWidth={1.75} />
                </IconButton>
              </div>
            )}
          </header>

          <div className="relative min-h-0">
            <FileViewerBody
              file={file}
              preview={preview}
              zoom={zoom}
              onZoom={setZoom}
              onDownload={onDownload}
              onPrevious={() => go(pos - 1)}
              onNext={() => go(pos + 1)}
            />
            {isImage && <ImageTools zoom={zoom} onZoom={setZoom} />}
            {count > 1 && (
              <>
                <Pressable
                  className={`${navBtn} max-md:hidden absolute left-3 top-1/2 -translate-y-1/2 w-11 h-11`}
                  aria-label="Tệp trước"
                  title="Tệp trước (←)"
                  disabled={pos === 0}
                  onClick={() => go(pos - 1)}
                >
                  <ChevronLeft size={22} strokeWidth={2} />
                </Pressable>
                <Pressable
                  className={`${navBtn} max-md:hidden absolute right-3 top-1/2 -translate-y-1/2 w-11 h-11`}
                  aria-label="Tệp sau"
                  title="Tệp sau (→)"
                  disabled={pos === count - 1}
                  onClick={() => go(pos + 1)}
                >
                  <ChevronRight size={22} strokeWidth={2} />
                </Pressable>
              </>
            )}
          </div>

          {count > 1 ? (
            <div className="flex items-center justify-center gap-4 min-h-11 py-1 pb-[max(0.25rem,env(safe-area-inset-bottom))] bg-base md:bg-sunk border-t border-line md:border-t-0">
              <Pressable
                className={`${navBtn} md:hidden w-11 h-11`}
                aria-label="Tệp trước"
                disabled={pos === 0}
                onClick={() => go(pos - 1)}
              >
                <ChevronLeft size={22} strokeWidth={2} />
              </Pressable>
              <span className="text-xs font-semibold text-ink-muted tnum" aria-label={`Tệp ${pos + 1} trên ${count}`}>
                {pos + 1} / {count}
              </span>
              <Pressable
                className={`${navBtn} md:hidden w-11 h-11`}
                aria-label="Tệp sau"
                disabled={pos === count - 1}
                onClick={() => go(pos + 1)}
              >
                <ChevronRight size={22} strokeWidth={2} />
              </Pressable>
            </div>
          ) : (
            <div />
          )}
        </div>

        <div aria-live="polite" className="sr-only">{live}</div>

        {sheetOpen && (
          <div className="absolute inset-0 z-10 md:hidden flex flex-col justify-end">
            <div className="absolute inset-0 bg-scrim" onClick={() => setSheetOpen(false)} />
            <div
              ref={sheetRef}
              role="dialog"
              aria-modal="true"
              aria-label="Thao tác với tệp"
              className="relative grid gap-1 p-4 pb-[max(1rem,env(safe-area-inset-bottom))] rounded-t-xl bg-overlay shadow-overlay"
            >
              <div className="grid gap-1 pb-3 mb-1 border-b border-line">
                <b className="text-ink break-words">{file.name}</b>
                <small className="text-ink-muted tnum">{typeLine}</small>
                {sender && <small className="text-ink-muted flex">{sender}</small>}
              </div>
              <Button type="button" variant="soft" onClick={() => { setSheetOpen(false); onDownload() }}>
                <Download size={16} strokeWidth={1.75} aria-hidden="true" />
                Tải xuống
              </Button>
              {canOpenInDrive && (
                <Button type="button" variant="ghost" onClick={openInDrive}>
                  <FolderOpen size={16} strokeWidth={1.75} aria-hidden="true" />
                  Mở trong Tài liệu
                </Button>
              )}
              <Button type="button" variant="ghost" onClick={() => setSheetOpen(false)}>Đóng</Button>
            </div>
          </div>
        )}
      </div>
    </div>,
    document.body,
  )
}
