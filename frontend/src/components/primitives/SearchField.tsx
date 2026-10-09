import { forwardRef, type InputHTMLAttributes } from 'react'
import { Search } from 'lucide-react'

interface SearchFieldProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'type'> {
  /** Accessible name; also the placeholder unless one is given. */
  label: string
  /** `base` sits on a raised panel, `sunk` on the page. */
  tone?: 'base' | 'sunk'
}

/** Search box: magnifier + bare input in a 36px control, focus shown as a ring. */
export const SearchField = forwardRef<HTMLInputElement, SearchFieldProps>(
  ({ label, tone = 'base', className = '', placeholder, ...props }, ref) => (
    <label
      className={`flex items-center gap-2 h-9 px-3 rounded-md text-ink-muted cursor-text
        focus-within:outline-2 focus-within:outline-focus focus-within:outline-offset-2
        ${tone === 'sunk' ? 'bg-sunk' : 'bg-base'} ${className}`}
    >
      <Search size={16} strokeWidth={1.75} aria-hidden="true" className="shrink-0" />
      <input
        ref={ref}
        type="search"
        aria-label={label}
        placeholder={placeholder ?? label}
        className="flex-1 min-w-0 h-full bg-transparent border-none outline-none text-sm text-ink
          placeholder:text-ink-muted"
        {...props}
      />
    </label>
  ),
)
SearchField.displayName = 'SearchField'
