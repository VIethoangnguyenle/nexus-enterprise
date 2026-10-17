import { forwardRef, type InputHTMLAttributes, type SelectHTMLAttributes, type TextareaHTMLAttributes } from 'react'
import { ChevronDown } from 'lucide-react'

const BOX = `h-9 rounded-md bg-sunk text-sm text-ink border-none outline-none tnum
  focus-visible:outline-2 focus-visible:outline-focus focus-visible:outline-offset-1 disabled:opacity-50`

/** A compact input of the editor (a date, a time, a number), on the sunk surface. */
export const FieldInput = forwardRef<HTMLInputElement, InputHTMLAttributes<HTMLInputElement> & { invalid?: boolean }>(
  ({ className = '', invalid, ...props }, ref) => (
    <input
      ref={ref}
      aria-invalid={invalid || undefined}
      className={`${BOX} px-3 min-w-0 ${invalid ? 'ring-2 ring-inset ring-danger' : ''} ${className}`}
      {...props}
    />
  ),
)
FieldInput.displayName = 'FieldInput'

/** A native select drawn like the editor's other controls. */
export const FieldSelect = forwardRef<HTMLSelectElement, SelectHTMLAttributes<HTMLSelectElement>>(
  ({ className = '', children, ...props }, ref) => (
    <span className="relative inline-grid">
      <select ref={ref} className={`${BOX} appearance-none pl-3 pr-8 cursor-pointer max-w-full ${className}`} {...props}>
        {children}
      </select>
      <ChevronDown size={16} strokeWidth={1.75} aria-hidden="true" className="absolute right-2 top-1/2 -translate-y-1/2 text-ink-muted pointer-events-none" />
    </span>
  ),
)
FieldSelect.displayName = 'FieldSelect'


/** A multi-line field drawn like the other compact controls. */
export const FieldTextarea = forwardRef<HTMLTextAreaElement, TextareaHTMLAttributes<HTMLTextAreaElement>>(
  ({ className = '', ...props }, ref) => (
    <textarea
      ref={ref}
      className={`w-full resize-y rounded-md bg-sunk px-3 py-2 text-sm text-ink border-none outline-none
        placeholder:text-ink-muted focus-visible:outline-2 focus-visible:outline-focus focus-visible:outline-offset-1 ${className}`}
      {...props}
    />
  ),
)
FieldTextarea.displayName = 'FieldTextarea'
