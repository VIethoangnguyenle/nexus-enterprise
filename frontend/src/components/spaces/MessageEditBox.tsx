import { useEffect, useRef } from 'react'
import { EditorContent, useEditor } from '@tiptap/react'
import { Button } from '../primitives'
import { chatExtensions } from '../chat/chat-extensions'

interface MessageEditBoxProps {
  /** The message as the editor reads it (HTML). */
  initialHtml: string
  /** The text now in the editor; `empty` when it holds none, which the owner turns into a question about deleting. */
  onSave: (html: string, empty: boolean) => void
  onCancel: () => void
}

/**
 * A message edited where it stands (DESIGN.md "Sửa tin"): the bubble becomes a
 * raised box ringed in the accent, Enter saves, Shift+Enter breaks the line, Esc
 * gives the text back untouched. The same formatting keys as the composer work;
 * Esc is marked taken so a panel behind does not close with it.
 */
export function MessageEditBox({ initialHtml, onSave, onCancel }: MessageEditBoxProps) {
  const boxRef = useRef<HTMLDivElement>(null)
  const saveRef = useRef<() => void>(() => {})
  const cancelRef = useRef(onCancel)
  cancelRef.current = onCancel

  const editor = useEditor({
    extensions: chatExtensions(''),
    content: initialHtml,
    autofocus: 'end',
    editorProps: {
      attributes: { class: 'chat-editor-content', role: 'textbox', 'aria-multiline': 'true', 'aria-label': 'Sửa tin' },
      handleKeyDown: (_view, event) => {
        if (event.key === 'Escape') {
          event.preventDefault()
          event.stopPropagation()
          cancelRef.current()
          return true
        }
        if (event.key === 'Enter' && !event.shiftKey && !event.isComposing) {
          event.preventDefault()
          saveRef.current()
          return true
        }
        return false
      },
    },
  })
  saveRef.current = () => {
    if (editor) onSave(editor.isEmpty ? '' : editor.getHTML(), editor.isEmpty)
  }

  // The message may sit at the foot of the stream, behind the composer: bring the editor into view.
  useEffect(() => {
    boxRef.current?.scrollIntoView?.({ block: 'nearest' })
  }, [])

  return (
    <div ref={boxRef} className="grid gap-2 w-full max-w-[min(560px,85%)] p-2 rounded-xl bg-raised ring-2 ring-inset ring-accent">
      <div className="max-h-50 overflow-y-auto px-1 py-0.5 text-sm" onClick={() => editor?.commands.focus()}>
        <EditorContent editor={editor} />
      </div>
      <div className="flex items-center gap-1.5">
        <small className="mr-auto text-xs text-ink-muted">Esc huỷ · Enter lưu</small>
        <Button type="button" variant="ghost" size="sm" onClick={onCancel}>Huỷ</Button>
        <Button type="button" variant="primary" size="sm" onClick={() => saveRef.current()}>Lưu</Button>
      </div>
    </div>
  )
}
