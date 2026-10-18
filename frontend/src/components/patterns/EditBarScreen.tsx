import { useId, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { AnimatePresence, Reorder, motion, useDragControls } from 'motion/react'
import { ArrowDown, ArrowLeft, ArrowUp, GripVertical } from 'lucide-react'
import { useModalFocus } from '../../hooks/useModalFocus'
import { useMotionPresets } from '../../lib/motion'
import { BAR_SLOTS, type BarModule, type BarModuleId } from '../../lib/nav-destinations'
import { Button, IconButton, Pressable } from '../primitives'

const sameOrder = (a: BarModuleId[], b: BarModuleId[]) => a.length === b.length && a.every((id, i) => id === b[i])

/**
 * Sửa thanh đáy (DESIGN.md §5, Điều hướng di động; mockup mobile-shell.html §1): a full screen over the
 * app that picks the four modules of the bottom bar and their order, kept per person and per workspace.
 * The bar always holds four, so a module from Trong Thêm is brought up by naming the one it replaces.
 * Order is changed by dragging the grip or with the ↑ ↓ buttons, which are the keyboard's way. Xong saves
 * and closes; Quay lại and Esc leave the bar as it was. Render it only while open.
 */
export function EditBarScreen({ modules, bar, defaults, onSave, onClose }: {
  modules: BarModule[]
  bar: BarModuleId[]
  defaults: BarModuleId[]
  /** True once saved; false when the server refused (the shared handler has said so). */
  onSave: (bar: BarModuleId[]) => Promise<boolean>
  onClose: () => void
}) {
  const m = useMotionPresets()
  const titleId = useId()
  const surfaceRef = useRef<HTMLDivElement>(null)
  useModalFocus({ active: true, surfaceRef, onClose })
  const [draft, setDraft] = useState<BarModuleId[]>(bar)
  const [bringing, setBringing] = useState<BarModuleId | null>(null)
  const [saving, setSaving] = useState(false)

  const byId = (id: BarModuleId) => modules.find((x) => x.id === id)!
  const rest = modules.filter((x) => !draft.includes(x.id))

  const move = (index: number, by: -1 | 1) => {
    const to = index + by
    if (to < 0 || to >= draft.length) return
    const next = [...draft]
    ;[next[index], next[to]] = [next[to]!, next[index]!]
    setDraft(next)
  }
  const replace = (index: number) => {
    if (!bringing) return
    setDraft(draft.map((id, i) => (i === index ? bringing : id)))
    setBringing(null)
  }
  const done = async () => {
    if (sameOrder(draft, bar)) return onClose()
    setSaving(true)
    const ok = await onSave(draft)
    setSaving(false)
    if (ok) onClose()
  }

  return createPortal(
    <motion.div
      ref={surfaceRef}
      role="dialog"
      aria-modal="true"
      aria-labelledby={titleId}
      {...m.panel}
      className="fixed inset-0 z-modal lg:hidden grid grid-rows-[auto_minmax(0,1fr)] bg-base text-ink
        pt-[env(safe-area-inset-top,0px)] pb-[env(safe-area-inset-bottom,0px)]"
    >
      <header className="flex items-center gap-2 h-14 px-2">
        <IconButton size="lg" className="size-11!" aria-label="Quay lại" onClick={onClose}><ArrowLeft size={20} strokeWidth={1.75} /></IconButton>
        <h1 id={titleId} className="flex-1 min-w-0 m-0 truncate font-display text-xl font-bold">Sửa thanh đáy</h1>
        <Button variant="soft" size="sm" onClick={() => void done()} disabled={saving} className="rounded-full">Xong</Button>
      </header>

      <div className="min-h-0 overflow-y-auto px-3 pb-6">
        <p className="m-0 px-1.5 pb-3 text-sm text-ink-muted">
          {bringing
            ? `Chọn phân hệ trên thanh đáy để thay bằng ${byId(bringing).label}.`
            : `Chọn ${BAR_SLOTS} phân hệ hiện ở thanh đáy. Kéo hoặc dùng mũi tên để đổi thứ tự. Thêm luôn ở ô cuối.`}
        </p>
        {bringing && (
          <Pressable onClick={() => setBringing(null)} className="mb-2 h-10 px-3 text-sm font-semibold text-accent">Huỷ</Pressable>
        )}

        <h2 className="m-0 px-1.5 pb-1 text-xs font-semibold text-ink-muted">Ở thanh đáy ({draft.length}/{BAR_SLOTS})</h2>
        <Reorder.Group as="ul" axis="y" values={draft} onReorder={setDraft} className="grid gap-0.5 m-0 p-0 mb-4 rounded-2xl bg-sunk overflow-hidden">
          {draft.map((id, i) => (
            <BarRow
              key={id}
              module={byId(id)}
              first={i === 0}
              last={i === draft.length - 1}
              bringing={bringing ? byId(bringing).label : null}
              onUp={() => move(i, -1)}
              onDown={() => move(i, 1)}
              onReplace={() => replace(i)}
            />
          ))}
        </Reorder.Group>

        <h2 className="m-0 px-1.5 pb-1 text-xs font-semibold text-ink-muted">Trong Thêm</h2>
        <ul className="grid gap-0.5 m-0 p-0 mb-4 rounded-2xl bg-sunk overflow-hidden">
          <AnimatePresence initial={false}>
            {rest.map((x) => (
              <li key={x.id} className="list-none flex items-center gap-3.5 min-h-14 px-3.5">
                <x.icon size={20} strokeWidth={1.75} aria-hidden="true" className="text-ink-muted shrink-0" />
                <span className="flex-1 min-w-0 truncate font-medium">{x.label}</span>
                <Pressable
                  onClick={() => setBringing(x.id)}
                  aria-pressed={bringing === x.id}
                  aria-label={`Đưa ${x.label} lên thanh đáy`}
                  className="h-10 px-3 rounded-full text-sm font-semibold text-accent hover:bg-hover"
                >
                  Đưa lên
                </Pressable>
              </li>
            ))}
          </AnimatePresence>
        </ul>

        <Pressable
          onClick={() => { setDraft(defaults); setBringing(null) }}
          className="grid place-items-center w-full min-h-13 rounded-2xl bg-sunk text-16 font-semibold text-accent"
        >
          Về mặc định
        </Pressable>
      </div>
    </motion.div>,
    document.body,
  )
}

function BarRow({ module: x, first, last, bringing, onUp, onDown, onReplace }: {
  module: BarModule
  first: boolean
  last: boolean
  /** Name of the module waiting to take a slot; the row then offers to give its place up. */
  bringing: string | null
  onUp: () => void
  onDown: () => void
  onReplace: () => void
}) {
  const controls = useDragControls()
  return (
    <Reorder.Item
      as="li"
      value={x.id}
      dragListener={false}
      dragControls={controls}
      className="list-none flex items-center gap-2 min-h-14 pl-3.5 pr-1.5 bg-sunk"
    >
      <x.icon size={20} strokeWidth={1.75} aria-hidden="true" className="text-ink-muted shrink-0" />
      <span className="flex-1 min-w-0 truncate font-medium">{x.label}</span>
      {bringing ? (
        <Pressable
          onClick={onReplace}
          aria-label={`Thay ${x.label} bằng ${bringing}`}
          className="h-10 px-3 rounded-full text-sm font-semibold text-accent hover:bg-hover"
        >
          Thay
        </Pressable>
      ) : (
        <>
          <IconButton size="lg" className="size-11!" aria-label={`Đưa ${x.label} lên trước`} disabled={first} onClick={onUp}><ArrowUp size={18} strokeWidth={1.75} /></IconButton>
          <IconButton size="lg" className="size-11!" aria-label={`Đưa ${x.label} xuống sau`} disabled={last} onClick={onDown}><ArrowDown size={18} strokeWidth={1.75} /></IconButton>
          <span
            aria-hidden="true"
            onPointerDown={(e) => controls.start(e)}
            className="grid place-items-center w-10 h-10 text-ink-muted touch-none cursor-grab"
          >
            <GripVertical size={20} strokeWidth={1.75} />
          </span>
        </>
      )}
    </Reorder.Item>
  )
}
