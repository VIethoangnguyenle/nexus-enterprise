import { useRef, useState } from 'react'
import { Spinner } from '../primitives'

interface ChatInputProps {
  value: string
  onChange: (val: string) => void
  onSend: () => void
  onTyping?: () => void
  onFileUpload?: (file: File) => Promise<void>
  isPending?: boolean
  error?: string | null
  placeholder?: string
}

/** Chat input bar with textarea, send button, file upload, and typing indicator hook. */
export function ChatInput({ value, onChange, onSend, onTyping, onFileUpload, isPending, error, placeholder = 'Message...' }: ChatInputProps) {
  const fileRef = useRef<HTMLInputElement>(null)
  const [uploading, setUploading] = useState(false)

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      onSend()
    }
  }

  const handleFileSelect = async () => {
    const file = fileRef.current?.files?.[0]
    if (!file || !onFileUpload) return
    setUploading(true)
    try {
      await onFileUpload(file)
    } catch (err: any) {
      alert(`File upload failed: ${err.message}`)
    } finally {
      setUploading(false)
      if (fileRef.current) fileRef.current.value = ''
    }
  }

  return (
    <div className="px-4 py-3 border-t border-border bg-bg-tertiary/60">
      {error && (
        <div className="bg-danger-bg text-danger text-xs px-3 py-1.5 rounded mb-2">{error}</div>
      )}
      <div className="flex items-end gap-2 bg-bg-glass border border-border rounded-[var(--radius-md)] px-3 py-2
        focus-within:border-border-focus focus-within:shadow-[0_0_0_3px_var(--color-accent-glow)]
        transition-colors">

        {/* File upload button */}
        {onFileUpload && (
          <>
            <input
              ref={fileRef}
              type="file"
              className="hidden"
              onChange={handleFileSelect}
            />
            <button
              onClick={() => fileRef.current?.click()}
              disabled={uploading}
              title="Attach file"
              className="w-7 h-7 flex items-center justify-center rounded-[var(--radius-sm)]
                text-text-muted hover:text-text-primary hover:bg-bg-hover
                border-none bg-transparent cursor-pointer transition-all duration-150
                disabled:opacity-30 disabled:cursor-not-allowed flex-shrink-0"
            >
              {uploading ? <Spinner size="sm" /> : '📎'}
            </button>
          </>
        )}

        <textarea
          value={value}
          onChange={e => { onChange(e.target.value); onTyping?.() }}
          onKeyDown={handleKeyDown}
          rows={1}
          placeholder={placeholder}
          className="flex-1 bg-transparent border-none text-sm text-text-primary resize-none
            focus:outline-none placeholder:text-text-muted min-h-[20px] max-h-[120px]
            font-[inherit] leading-relaxed"
        />
        <button
          onClick={onSend}
          disabled={!value.trim() || isPending}
          className="w-7 h-7 flex items-center justify-center rounded-[var(--radius-sm)]
            bg-accent text-white border-none cursor-pointer transition-all duration-150
            disabled:opacity-30 disabled:cursor-not-allowed
            hover:bg-accent-hover hover:shadow-[0_2px_8px_var(--color-accent-glow)]
            flex-shrink-0"
        >
          {isPending ? <Spinner size="sm" /> : '➤'}
        </button>
      </div>
    </div>
  )
}
