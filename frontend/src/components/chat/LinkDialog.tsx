import { useEffect, useRef, useState, type RefObject } from 'react'
import { Button, Popover, TextField } from '../primitives'
import { isAllowedLink } from './chat-html'

interface LinkDialogProps {
  open: boolean
  /** The Link button of the toolbar; the box hangs from it. */
  anchorRef: RefObject<HTMLElement | null>
  /** The selected text, which becomes the link's text. */
  text: string
  /** The address of the link under the cursor, when editing one. */
  href: string
  /** Editing a link that already exists: the title changes and "Bỏ liên kết" appears. */
  hasLink?: boolean
  onSubmit: (link: { text: string; href: string }) => void
  onRemove: () => void
  onClose: () => void
}

const RULE = 'Chỉ dùng liên kết bắt đầu bằng https://, http:// hoặc mailto:'

/**
 * "Chèn liên kết" (DESIGN.md "Hộp chèn link"): a small box hung from the Link
 * button with the link's text and its address. Only http, https and mailto
 * addresses are accepted, the same three the server keeps. Enter inserts, Esc
 * closes.
 */
export function LinkDialog({ open, anchorRef, text, href, hasLink, onSubmit, onRemove, onClose }: LinkDialogProps) {
  const [label, setLabel] = useState(text)
  const [url, setUrl] = useState(href)
  const [touched, setTouched] = useState(false)
  const textRef = useRef<HTMLInputElement>(null)
  const urlRef = useRef<HTMLInputElement>(null)

  // Each opening starts from what the editor holds now.
  useEffect(() => {
    if (!open) return
    setLabel(text)
    setUrl(href)
    setTouched(false)
    const id = requestAnimationFrame(() => (text ? urlRef : textRef).current?.focus())
    return () => cancelAnimationFrame(id)
  }, [open, text, href])

  const valid = isAllowedLink(url)
  const submit = () => {
    if (!valid) {
      setTouched(true)
      return
    }
    const address = url.trim()
    onSubmit({ text: label.trim() || address, href: address })
  }

  return (
    <Popover open={open} onClose={onClose} anchorRef={anchorRef} placement="top-start" label={hasLink ? 'Sửa liên kết' : 'Chèn liên kết'} className="w-80 p-4">
      <form
        className="grid gap-3"
        onSubmit={(e) => {
          e.preventDefault()
          submit()
        }}
      >
        <TextField ref={textRef} label="Văn bản" value={label} onChange={(e) => setLabel(e.target.value)} />
        <TextField
          ref={urlRef}
          label="Đường liên kết"
          type="url"
          inputMode="url"
          value={url}
          placeholder="https://"
          error={touched && url && !valid ? RULE : undefined}
          onChange={(e) => setUrl(e.target.value)}
          onBlur={() => setTouched(true)}
        />
        <div className="flex items-center justify-end gap-2">
          {hasLink && (
            <Button type="button" variant="ghost" size="sm" className="mr-auto" onClick={onRemove}>Bỏ liên kết</Button>
          )}
          <Button type="button" variant="ghost" size="sm" onClick={onClose}>Huỷ</Button>
          <Button type="submit" variant="primary" size="sm" disabled={!valid}>Chèn</Button>
        </div>
      </form>
    </Popover>
  )
}
