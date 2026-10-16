import { useId, useRef, useState } from 'react'
import { Camera, Image as ImageIcon, TriangleAlert, Trash2, Upload } from 'lucide-react'
import { usePhone } from '../../hooks/usePhone'
import { AVATAR_TYPES, validateAvatarFile } from '../../lib/avatar-file'
import { Avatar, MenuItem, Popover, Pressable } from '../primitives'
import { NavSheet, SheetItem, SheetSeparator } from '../spaces/NavSheet'

interface AvatarEditProps {
  /** Display name: the initials while there is no picture. */
  name: string
  hueKey?: string
  /** The stored picture; absent when there is none. */
  src?: string
  /** What the button is called for a screen reader: "Đổi ảnh đại diện", "Đổi ảnh nhóm". */
  label: string
  /** Without the right to change the picture this is only the avatar: no button, no veil. */
  canEdit: boolean
  /** A picture that passed the size and type check. The caller sends it and reports a failure through `error`. */
  onUpload: (file: File) => void | Promise<void>
  onRemove: () => void | Promise<void>
  /** A picture is being sent or removed. */
  busy?: boolean
  /** 0..1 while the upload's size is known; without it the ring only shows that something is happening. */
  progress?: number
  /** A failure the caller knows of (the server's refusal), in words for the person. */
  error?: string | null
  shape?: 'circle' | 'square'
  size?: 44 | 52 | 64
}

const RING = 4

/**
 * "Touch the picture to change it" (design/mockups/avatar-edit.html §1, §2, §5,
 * DESIGN.md AvatarEdit): one button around the avatar and no buttons beside it.
 * A mouse sees a veil with a camera on hover and focus, a touch screen a camera
 * badge that is always there. The button opens a menu (a sheet below 768 px
 * with Chụp ảnh). A chosen picture is checked here and handed to `onUpload` at
 * once; sending it and its errors are the caller's.
 */
export function AvatarEdit({
  name, hueKey, src, label, canEdit, onUpload, onRemove, busy = false, progress, error, shape = 'circle', size = 64,
}: AvatarEditProps) {
  const phone = usePhone()
  const [open, setOpen] = useState(false)
  const [refused, setRefused] = useState<string | null>(null)
  const anchorRef = useRef<HTMLButtonElement>(null)
  const pickRef = useRef<HTMLInputElement>(null)
  const cameraRef = useRef<HTMLInputElement>(null)
  const errorId = useId()

  if (!canEdit) return <Avatar name={name} hueKey={hueKey} src={src} size={size} shape={shape} />

  const shownError = refused ?? error ?? null
  const square = shape === 'square'
  const pct = progress === undefined ? undefined : Math.round(Math.min(1, Math.max(0, progress)) * 100)
  const accessibleName = !busy ? label : pct === undefined ? 'Đang cập nhật ảnh' : `Đang tải ảnh, ${pct}%`

  const choose = (file: File | undefined) => {
    if (!file) return
    const problem = validateAvatarFile(file)
    setRefused(problem)
    if (problem) return
    void onUpload(file)
  }
  const chosen = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    e.target.value = '' // choosing the same file again still uploads it
    choose(file)
  }
  const remove = () => {
    setRefused(null)
    void onRemove()
  }
  const close = () => setOpen(false)

  const box = size + RING * 2
  const radius = square ? size / 4 : size / 2

  return (
    <div className="inline-grid justify-items-center gap-1.5">
      <Pressable
        ref={anchorRef}
        aria-label={accessibleName}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-busy={busy || undefined}
        aria-disabled={busy || undefined}
        aria-describedby={shownError ? errorId : undefined}
        data-shape={shape}
        onClick={() => { if (!busy) setOpen((o) => !o) }}
        style={{ borderRadius: radius }}
        className="group relative inline-grid shrink-0 p-0 border-0 bg-transparent"
      >
        <span className={`inline-grid transition-opacity duration-quick ${busy ? 'opacity-60' : ''}`}>
          <Avatar name={name} hueKey={hueKey} src={src} size={size} shape={shape} />
        </span>
        {!busy && (
          <span
            aria-hidden="true"
            style={{ borderRadius: radius }}
            className="absolute inset-0 grid place-items-center bg-scrim text-on-accent opacity-0 transition-opacity duration-quick
              motion-reduce:transition-none group-hover:opacity-100 group-focus-visible:opacity-100 group-aria-expanded:opacity-100"
          >
            <Camera size={20} strokeWidth={1.75} />
          </span>
        )}
        {!busy && (
          <span
            data-badge=""
            aria-hidden="true"
            className={`${shownError ? 'grid text-danger' : 'hidden pointer-coarse:grid text-ink'} absolute w-6 h-6 rounded-full
              place-items-center bg-overlay shadow-overlay ${square ? '-right-1 -bottom-1' : '-right-0.5 -bottom-0.5'}`}
          >
            {shownError
              ? <TriangleAlert size={14} strokeWidth={1.75} />
              : <Camera size={14} strokeWidth={1.75} />}
          </span>
        )}
        {busy && (
          <svg
            aria-hidden="true"
            viewBox={`0 0 ${box} ${box}`}
            style={{ inset: -RING }}
            className={`absolute pointer-events-none fill-none ${pct === undefined ? 'motion-safe:animate-pulse' : ''}`}
            strokeWidth={3}
          >
            <Track square={square} size={size} radius={radius} className="stroke-line" />
            <Track square={square} size={size} radius={radius} className="stroke-accent" arc={pct === undefined ? 30 : pct} />
          </svg>
        )}
      </Pressable>

      {shownError && (
        <p id={errorId} role="alert" className="m-0 max-w-40 text-center text-xs text-danger">{shownError}</p>
      )}

      {/* Choose what to send. The primitives have no hidden file input (as in
          DriveScreen); the visible control is the avatar. They live here, not in
          the menu or sheet, so closing those does not drop a picture being chosen. */}
      {/* eslint-disable-next-line no-restricted-syntax */}
      <input
        ref={pickRef}
        type="file"
        accept={AVATAR_TYPES.join(',')}
        aria-label="Tệp ảnh từ máy"
        aria-hidden="true"
        className="sr-only"
        tabIndex={-1}
        onChange={chosen}
      />
      {/* eslint-disable-next-line no-restricted-syntax */}
      <input
        ref={cameraRef}
        type="file"
        accept={AVATAR_TYPES.join(',')}
        capture="user"
        aria-label="Ảnh chụp từ máy ảnh"
        aria-hidden="true"
        className="sr-only"
        tabIndex={-1}
        onChange={chosen}
      />

      {phone ? (
        open && (
          <NavSheet label={label} onClose={close}>
            <SheetItem icon={<Camera size={20} strokeWidth={1.75} />} onClick={() => { close(); cameraRef.current?.click() }}>
              Chụp ảnh
            </SheetItem>
            <SheetItem icon={<ImageIcon size={20} strokeWidth={1.75} />} onClick={() => { close(); pickRef.current?.click() }}>
              Chọn ảnh
            </SheetItem>
            {src && (
              <SheetItem tone="danger" icon={<Trash2 size={20} strokeWidth={1.75} />} onClick={() => { close(); remove() }}>
                Bỏ ảnh
              </SheetItem>
            )}
            <SheetSeparator />
            <Pressable
              onClick={close}
              className="flex items-center justify-center w-full min-h-13 px-3.5 rounded-lg text-16 font-semibold text-ink
                hover:bg-hover active:bg-hover transition-colors duration-quick"
            >
              Huỷ
            </Pressable>
          </NavSheet>
        )
      ) : (
        <Popover open={open} onClose={close} anchorRef={anchorRef} role="menu" label={label} placement="bottom-start">
          <MenuItem icon={<Upload size={16} strokeWidth={1.75} />} onClick={() => pickRef.current?.click()}>
            {src ? 'Đổi ảnh' : 'Tải ảnh lên'}
          </MenuItem>
          {src && (
            <MenuItem tone="danger" icon={<Trash2 size={16} strokeWidth={1.75} />} onClick={remove}>
              Bỏ ảnh
            </MenuItem>
          )}
        </Popover>
      )}
    </div>
  )
}

/**
 * The ring of an upload: a circle or a rounded square drawn on a 100-unit path,
 * so `arc` is a percentage whatever the size. Without `arc` it is the full track.
 */
function Track({ square, size, radius, className, arc }: {
  square: boolean
  size: number
  radius: number
  className: string
  arc?: number
}) {
  const common = {
    pathLength: 100,
    strokeLinecap: 'round' as const,
    strokeDasharray: arc === undefined ? undefined : `${arc} 100`,
    className,
  }
  const c = RING + size / 2
  if (!square) {
    return <circle cx={c} cy={c} r={size / 2 + 2} transform={`rotate(-90 ${c} ${c})`} {...common} />
  }
  return <rect x={2} y={2} width={size + 4} height={size + 4} rx={radius + 2} {...common} />
}
