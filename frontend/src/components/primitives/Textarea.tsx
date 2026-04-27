import { type TextareaHTMLAttributes, forwardRef } from 'react'

interface TextareaProps extends TextareaHTMLAttributes<HTMLTextAreaElement> {
  label?: string
  error?: string
}

/** Textarea with optional label and error. Auto-resizes via CSS resize. */
export const Textarea = forwardRef<HTMLTextAreaElement, TextareaProps>(
  ({ label, error, className = '', id, ...props }, ref) => {
    const textareaId = id ?? label?.toLowerCase().replace(/\s+/g, '-')

    return (
      <div className="flex flex-col gap-1.5">
        {label && (
          <label
            htmlFor={textareaId}
            className="text-xs font-medium text-text-secondary uppercase tracking-wider"
          >
            {label}
          </label>
        )}
        <textarea
          ref={ref}
          id={textareaId}
          className={`w-full px-3 py-2 bg-bg-glass border border-border rounded-[var(--radius-sm)]
            text-text-primary text-sm font-[inherit] transition-colors duration-200
            placeholder:text-text-muted resize-y min-h-[80px]
            focus:outline-none focus:border-border-focus focus:shadow-[0_0_0_3px_var(--color-accent-glow)]
            disabled:opacity-50 disabled:cursor-not-allowed
            ${error ? 'border-danger' : ''}
            ${className}`}
          {...props}
        />
        {error && <p className="text-xs text-danger mt-0.5">{error}</p>}
      </div>
    )
  },
)

Textarea.displayName = 'Textarea'
