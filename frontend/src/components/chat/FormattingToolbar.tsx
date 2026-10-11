import { useRef, type KeyboardEvent } from 'react'
import { useEditorState, type Editor } from '@tiptap/react'
import {
  Bold, Code, Italic, Link, List, ListOrdered, Quote, SquareCode, Strikethrough, Underline,
} from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { IconButton } from '../primitives'

const isMac = () => typeof navigator !== 'undefined' && /Mac|iPhone|iPad/.test(navigator.platform)

/** "Ctrl+B" on most keyboards, "⌘B" on a Mac. */
function shortcut(keys: string): string {
  const mac = isMac()
  return keys.replace('Mod+', mac ? '⌘' : 'Ctrl+').replace('Shift+', mac ? '⇧' : 'Shift+')
}

type Format = 'bold' | 'italic' | 'underline' | 'strike' | 'bulletList' | 'orderedList' | 'link' | 'code' | 'codeBlock' | 'blockquote'

interface Item { format: Format; label: string; icon: LucideIcon; keys?: string }

/** The groups of the design, in order; a hairline sits between groups. */
const GROUPS: Item[][] = [
  [
    { format: 'bold', label: 'Đậm', icon: Bold, keys: 'Mod+B' },
    { format: 'italic', label: 'Nghiêng', icon: Italic, keys: 'Mod+I' },
    { format: 'underline', label: 'Gạch chân', icon: Underline, keys: 'Mod+U' },
    { format: 'strike', label: 'Gạch ngang', icon: Strikethrough, keys: 'Mod+Shift+X' },
  ],
  [
    { format: 'bulletList', label: 'Danh sách chấm', icon: List },
    { format: 'orderedList', label: 'Danh sách số', icon: ListOrdered },
  ],
  [{ format: 'link', label: 'Link', icon: Link, keys: 'Mod+K' }],
  [
    { format: 'code', label: 'Code', icon: Code },
    { format: 'codeBlock', label: 'Khối code', icon: SquareCode },
    { format: 'blockquote', label: 'Trích dẫn', icon: Quote },
  ],
]

/** Applies one format to the selection. Lists exclude each other, and so do the code block and the quote. */
function apply(editor: Editor, format: Format) {
  const chain = editor.chain().focus()
  switch (format) {
    case 'bold': chain.toggleBold().run(); break
    case 'italic': chain.toggleItalic().run(); break
    case 'underline': chain.toggleUnderline().run(); break
    case 'strike': chain.toggleStrike().run(); break
    case 'bulletList': chain.toggleBulletList().run(); break
    case 'orderedList': chain.toggleOrderedList().run(); break
    case 'code': chain.toggleCode().run(); break
    case 'codeBlock':
      if (editor.isActive('blockquote')) chain.lift('blockquote').toggleCodeBlock().run()
      else chain.toggleCodeBlock().run()
      break
    case 'blockquote':
      if (editor.isActive('codeBlock')) chain.toggleCodeBlock().toggleBlockquote().run()
      else chain.toggleBlockquote().run()
      break
    case 'link': break
  }
}

interface FormattingToolbarProps {
  editor: Editor | null
  /** For the "A" button's aria-controls. */
  id: string
  /** The Link button was pressed: open the link box hung from it. */
  onLink: (anchor: HTMLElement) => void
}

/**
 * The formatting row above the composer's input (DESIGN.md "Thanh định dạng"):
 * ten buttons in four groups that show what the selection already has. It is
 * one tab stop; the arrow keys move along it. On a phone it scrolls sideways
 * instead of wrapping, and the buttons are 44px.
 */
export function FormattingToolbar({ editor, id, onLink }: FormattingToolbarProps) {
  const barRef = useRef<HTMLDivElement>(null)
  const active = useEditorState({
    editor,
    selector: ({ editor: ed }) =>
      Object.fromEntries(GROUPS.flat().map((i) => [i.format, !!ed && ed.isActive(i.format)])) as Record<Format, boolean>,
  })

  const buttons = () => Array.from(barRef.current?.querySelectorAll<HTMLButtonElement>('button') ?? [])
  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    const list = buttons()
    const i = list.indexOf(document.activeElement as HTMLButtonElement)
    if (i < 0) return
    let next = -1
    if (e.key === 'ArrowRight') next = (i + 1) % list.length
    else if (e.key === 'ArrowLeft') next = (i - 1 + list.length) % list.length
    else if (e.key === 'Home') next = 0
    else if (e.key === 'End') next = list.length - 1
    if (next < 0) return
    e.preventDefault()
    list.forEach((b, k) => { b.tabIndex = k === next ? 0 : -1 })
    list[next]?.focus()
  }

  return (
    <div
      ref={barRef}
      id={id}
      role="toolbar"
      aria-label="Định dạng"
      onKeyDown={onKeyDown}
      className="flex items-center gap-0.5 mr-1 pb-2 border-b border-line overflow-x-auto [scrollbar-width:none]"
    >
      {GROUPS.map((group, g) => (
        <div key={group[0]!.format} className="contents">
          {g > 0 && <span role="separator" aria-orientation="vertical" className="shrink-0 w-px h-5 mx-1 bg-line" />}
          {group.map((item) => {
            const first = g === 0 && item === group[0]
            const on = active?.[item.format] ?? false
            return (
              <IconButton
                key={item.format}
                type="button"
                aria-label={item.label}
                title={item.keys ? `${item.label} (${shortcut(item.keys)})` : item.label}
                aria-pressed={on}
                tabIndex={first ? 0 : -1}
                // Pressing a button must not take the selection out of the editor.
                onMouseDown={(e) => e.preventDefault()}
                onClick={(e) => {
                  if (!editor) return
                  if (item.format === 'link') onLink(e.currentTarget)
                  else apply(editor, item.format)
                }}
                className={`shrink-0 max-md:w-11 max-md:h-11 ${on ? '!bg-accent-wash !text-accent' : ''}`}
              >
                <item.icon size={16} strokeWidth={1.75} aria-hidden="true" />
              </IconButton>
            )
          })}
        </div>
      ))}
    </div>
  )
}
