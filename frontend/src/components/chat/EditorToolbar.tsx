import type { Editor } from '@tiptap/react'
import {
  Bold, Italic, Strikethrough, Code, Quote, List, ListOrdered,
  FileCode, AtSign,
} from 'lucide-react'
import type { LucideIcon } from 'lucide-react'

interface EditorToolbarProps {
  editor: Editor
}

/** Formatting toolbar for TipTap editor — icon-based buttons matching Stitch design. */
export function EditorToolbar({ editor }: EditorToolbarProps) {
  const buttons: ToolbarBtn[] = [
    { icon: Bold, title: 'Bold', action: () => editor.chain().focus().toggleBold().run(), active: editor.isActive('bold') },
    { icon: Italic, title: 'Italic', action: () => editor.chain().focus().toggleItalic().run(), active: editor.isActive('italic') },
    { icon: Strikethrough, title: 'Strikethrough', action: () => editor.chain().focus().toggleStrike().run(), active: editor.isActive('strike') },
    { icon: Code, title: 'Inline Code', action: () => editor.chain().focus().toggleCode().run(), active: editor.isActive('code') },
    { divider: true },
    { icon: List, title: 'Bullet List', action: () => editor.chain().focus().toggleBulletList().run(), active: editor.isActive('bulletList') },
    { icon: ListOrdered, title: 'Numbered List', action: () => editor.chain().focus().toggleOrderedList().run(), active: editor.isActive('orderedList') },
    { icon: Quote, title: 'Blockquote', action: () => editor.chain().focus().toggleBlockquote().run(), active: editor.isActive('blockquote') },
    { icon: FileCode, title: 'Code Block', action: () => editor.chain().focus().toggleCodeBlock().run(), active: editor.isActive('codeBlock') },
    { divider: true },
    { icon: AtSign, title: 'Mention', action: () => editor.chain().focus().insertContent('@').run(), active: false },
  ]

  return (
    <div className="flex items-center gap-0.5 overflow-x-auto scrollbar-none">
      {buttons.map((btn, i) => {
        if (btn.divider) {
          return <div key={`d-${i}`} className="w-px h-5 bg-outline-variant/30 mx-1" />
        }
        const Icon = btn.icon!
        return (
          <button
            key={btn.title}
            onClick={btn.action}
            title={btn.title}
            className={`p-1.5 flex items-center justify-center rounded-md
              border-none cursor-pointer transition-colors
              ${btn.active
                ? 'bg-surface-container text-on-surface'
                : 'bg-transparent text-on-surface-variant hover:text-on-surface hover:bg-surface-container'}`}
          >
            <Icon size={16} />
          </button>
        )
      })}
    </div>
  )
}

type ToolbarBtn = {
  icon?: LucideIcon
  title?: string
  action?: () => void
  active?: boolean
  divider?: boolean
}
