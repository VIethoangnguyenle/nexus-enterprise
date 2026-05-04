import { type TextareaHTMLAttributes, forwardRef } from 'react'

interface TextareaProps extends TextareaHTMLAttributes<HTMLTextAreaElement> {
  error?: string
}

/** Recessed textarea — matches Input styling with surface-container-lowest background. */
export const Textarea = forwardRef<HTMLTextAreaElement, TextareaProps>(
  ({ className = '', error, ...props }, ref) => (
    <div className="flex flex-col gap-1">
      <textarea
        ref={ref}
        className={`w-full px-3 py-2 text-small bg-surface-container-lowest border rounded-md
          text-on-surface placeholder:text-outline resize-y min-h-[72px]
          transition-colors duration-fast focus-ring
          ${error
            ? 'border-danger focus:border-danger'
            : 'border-outline-variant focus:border-primary'
          } ${className}`}
        {...props}
      />
      {error && <span className="text-micro text-danger">{error}</span>}
    </div>
  ),
)

Textarea.displayName = 'Textarea'
