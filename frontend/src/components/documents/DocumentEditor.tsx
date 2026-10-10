import { forwardRef, useEffect, useImperativeHandle } from 'react'
import { EditorContent, useEditor, type Editor } from '@tiptap/react'
import StarterKit from '@tiptap/starter-kit'
import Placeholder from '@tiptap/extension-placeholder'
import { sanitizeHtml } from '../../lib/sanitize-html'
import { DocumentToolbar } from './DocumentToolbar'

export interface DocumentEditorHandle {
  /** Replace the whole text (after adopting another version). Does not count as an edit. */
  setContent: (html: string) => void
  focus: () => void
}

interface DocumentEditorProps {
  /** The stored HTML. It is sanitized before it reaches the editor. */
  initialHtml: string
  editable: boolean
  /** Called with the editor's HTML on every edit. */
  onChange: (html: string) => void
  /** Ctrl/Cmd+S. */
  onSaveShortcut: () => void
  label: string
}

/**
 * The text of a document (mockup §3): a toolbar and a 720px page. One author at
 * a time; nothing here shows other people's cursors. Stored HTML is untrusted
 * and goes through the same sanitizer as chat messages, and the editor's own
 * schema drops whatever it does not model, so what is saved back is what the
 * editor can produce.
 */
export const DocumentEditor = forwardRef<DocumentEditorHandle, DocumentEditorProps>(
  function DocumentEditor({ initialHtml, editable, onChange, onSaveShortcut, label }, ref) {
    const editor = useEditor({
      extensions: [
        StarterKit.configure({ heading: { levels: [1, 2, 3] } }),
        Placeholder.configure({ placeholder: 'Bắt đầu viết…' }),
      ],
      content: sanitizeHtml(initialHtml),
      editable,
      editorProps: {
        attributes: {
          class: 'doc-prose',
          role: 'textbox',
          'aria-multiline': 'true',
          'aria-label': label,
        },
        handleKeyDown: (_view, event) => {
          if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 's') {
            event.preventDefault()
            onSaveShortcut()
            return true
          }
          return false
        },
      },
      onUpdate: ({ editor: ed }) => onChange(ed.getHTML()),
    })

    useEffect(() => { editor?.setEditable(editable) }, [editor, editable])

    useImperativeHandle(ref, () => ({
      setContent: (html) => { editor?.commands.setContent(sanitizeHtml(html), { emitUpdate: false }) },
      focus: () => { editor?.commands.focus('end') },
    }), [editor])

    return (
      <div className="grid gap-3">
        {editable && <DocumentToolbar editor={editor as Editor | null} />}
        <div className="rounded-surface bg-raised px-6 py-8 md:px-12 md:py-10 min-h-96">
          <EditorContent editor={editor} />
        </div>
      </div>
    )
  },
)
