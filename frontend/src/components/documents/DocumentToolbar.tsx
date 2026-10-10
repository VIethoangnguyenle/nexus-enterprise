import { useRef, useState } from 'react'
import { useEditorState, type Editor } from '@tiptap/react'
import {
  Bold, Check, ChevronDown, Italic, List, ListOrdered, Quote, Redo2, Strikethrough, Underline, Undo2,
} from 'lucide-react'
import { MenuItem, Popover, Pressable } from '../primitives'

type Block = 'paragraph' | 1 | 2 | 3

const BLOCKS: { id: Block; label: string }[] = [
  { id: 'paragraph', label: 'Đoạn văn' },
  { id: 1, label: 'Tiêu đề lớn' },
  { id: 2, label: 'Tiêu đề vừa' },
  { id: 3, label: 'Tiêu đề nhỏ' },
]

const button = (on: boolean) =>
  `press grid place-items-center w-8 h-8 rounded-md transition-colors duration-quick
   disabled:opacity-40 disabled:cursor-not-allowed
   ${on ? 'bg-accent-wash text-ink' : 'text-ink-muted hover:bg-hover hover:text-ink'}`

/**
 * The formatting bar of the editor (mockup §3): a raised strip, radius 10, 32px
 * buttons each with a name, the one in effect on the accent wash. Everything
 * here works on the selection; there is no button that does nothing.
 */
export function DocumentToolbar({ editor }: { editor: Editor | null }) {
  const menuAnchor = useRef<HTMLButtonElement>(null)
  const [menuOpen, setMenuOpen] = useState(false)

  const s = useEditorState({
    editor,
    selector: ({ editor: ed }) => ({
      bold: !!ed?.isActive('bold'),
      italic: !!ed?.isActive('italic'),
      underline: !!ed?.isActive('underline'),
      strike: !!ed?.isActive('strike'),
      bullet: !!ed?.isActive('bulletList'),
      ordered: !!ed?.isActive('orderedList'),
      quote: !!ed?.isActive('blockquote'),
      level: ([1, 2, 3] as const).find((l) => ed?.isActive('heading', { level: l })) ?? ('paragraph' as const),
      canUndo: !!ed?.can().undo(),
      canRedo: !!ed?.can().redo(),
    }),
  })
  if (!editor || !s) return null
  const run = () => editor.chain().focus()

  const setBlock = (b: Block) => {
    if (b === 'paragraph') run().setParagraph().run()
    else run().toggleHeading({ level: b }).run()
  }
  const current = BLOCKS.find((b) => b.id === s.level)!

  return (
    <div
      role="toolbar"
      aria-label="Định dạng"
      className="flex flex-wrap items-center gap-1 p-1 rounded-surface bg-raised"
    >
      <Pressable
        ref={menuAnchor}
        aria-haspopup="menu"
        aria-expanded={menuOpen}
        aria-label={`Kiểu đoạn: ${current.label}`}
        onClick={() => setMenuOpen((o) => !o)}
        className="press inline-flex items-center gap-1.5 h-8 px-2.5 rounded-md text-small-ui font-semibold text-ink hover:bg-hover"
      >
        {current.label}
        <ChevronDown size={14} strokeWidth={1.75} aria-hidden="true" />
      </Pressable>
      <Popover open={menuOpen} onClose={() => setMenuOpen(false)} anchorRef={menuAnchor} role="menu" label="Kiểu đoạn">
        {BLOCKS.map((b) => (
          <MenuItem
            key={String(b.id)}
            onClick={() => setBlock(b.id)}
            icon={s.level === b.id ? <Check size={16} strokeWidth={1.75} /> : <span className="w-4" />}
          >
            {b.label}
          </MenuItem>
        ))}
      </Popover>

      <span aria-hidden="true" className="w-px h-5 mx-1 bg-line" />

      <Pressable aria-label="In đậm" title="In đậm" aria-pressed={s.bold} onClick={() => run().toggleBold().run()} className={button(s.bold)}>
        <Bold size={16} strokeWidth={1.75} aria-hidden="true" />
      </Pressable>
      <Pressable aria-label="In nghiêng" title="In nghiêng" aria-pressed={s.italic} onClick={() => run().toggleItalic().run()} className={button(s.italic)}>
        <Italic size={16} strokeWidth={1.75} aria-hidden="true" />
      </Pressable>
      <Pressable aria-label="Gạch chân" title="Gạch chân" aria-pressed={s.underline} onClick={() => run().toggleUnderline().run()} className={button(s.underline)}>
        <Underline size={16} strokeWidth={1.75} aria-hidden="true" />
      </Pressable>
      <Pressable aria-label="Gạch ngang" title="Gạch ngang" aria-pressed={s.strike} onClick={() => run().toggleStrike().run()} className={button(s.strike)}>
        <Strikethrough size={16} strokeWidth={1.75} aria-hidden="true" />
      </Pressable>

      <span aria-hidden="true" className="w-px h-5 mx-1 bg-line" />

      <Pressable aria-label="Danh sách gạch đầu dòng" title="Danh sách gạch đầu dòng" aria-pressed={s.bullet} onClick={() => run().toggleBulletList().run()} className={button(s.bullet)}>
        <List size={16} strokeWidth={1.75} aria-hidden="true" />
      </Pressable>
      <Pressable aria-label="Danh sách đánh số" title="Danh sách đánh số" aria-pressed={s.ordered} onClick={() => run().toggleOrderedList().run()} className={button(s.ordered)}>
        <ListOrdered size={16} strokeWidth={1.75} aria-hidden="true" />
      </Pressable>
      <Pressable aria-label="Trích dẫn" title="Trích dẫn" aria-pressed={s.quote} onClick={() => run().toggleBlockquote().run()} className={button(s.quote)}>
        <Quote size={16} strokeWidth={1.75} aria-hidden="true" />
      </Pressable>

      <span aria-hidden="true" className="w-px h-5 mx-1 bg-line" />

      <Pressable aria-label="Hoàn tác" title="Hoàn tác" disabled={!s.canUndo} onClick={() => run().undo().run()} className={button(false)}>
        <Undo2 size={16} strokeWidth={1.75} aria-hidden="true" />
      </Pressable>
      <Pressable aria-label="Làm lại" title="Làm lại" disabled={!s.canRedo} onClick={() => run().redo().run()} className={button(false)}>
        <Redo2 size={16} strokeWidth={1.75} aria-hidden="true" />
      </Pressable>
    </div>
  )
}
