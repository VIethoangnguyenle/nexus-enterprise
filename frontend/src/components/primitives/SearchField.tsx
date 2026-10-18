import { forwardRef, type InputHTMLAttributes, type ReactNode } from 'react'
import { motion } from 'motion/react'
import { Search } from 'lucide-react'
import { DURATION, EASE, useMotionPresets } from '../../lib/motion'

interface SearchFieldProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'type'> {
  /** Accessible name; also the placeholder unless one is given. */
  label: string
  /** `base` sits on a raised panel, `sunk` on the page. */
  tone?: 'base' | 'sunk'
  /** The screen's own search: the phone top bar's search action goes to it. */
  moduleSearch?: boolean
  /**
   * The full-screen search of a phone: a 52px pill ringed in accent, with `leading` (a back button)
   * and `trailing` (a clear button) beside a 16px box and no magnifier.
   */
  pill?: boolean
  /**
   * No fill, no corners, no focus outline of its own: the box sits inside a surface that draws all three
   * (`SearchBox`) and fills it, so the input does not move when the surface opens.
   */
  bare?: boolean
  leading?: ReactNode
  trailing?: ReactNode
}

/** The accent ring of the full-screen search's pill: 2px, fading in over 160ms (120ms under reduced motion). */
function PillRing() {
  const m = useMotionPresets()
  return (
    <motion.span
      aria-hidden="true"
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      transition={{ duration: m.reduced ? DURATION.reducedFade : DURATION.quick, ease: EASE.out }}
      className="absolute inset-0 rounded-full ring-2 ring-inset ring-accent pointer-events-none"
    />
  )
}

/** Search box: magnifier + bare input in a 36px control, focus shown as a ring. */
export const SearchField = forwardRef<HTMLInputElement, SearchFieldProps>(
  ({ label, tone = 'base', moduleSearch, pill, bare, leading, trailing, className = '', placeholder, ...props }, ref) => pill ? (
    <div className={`relative flex items-center gap-1 h-13 pl-1 pr-2 rounded-full bg-sunk ${className}`}>
      {/* The ring is its own layer so it can fade in when the pill turns into the search box. */}
      <PillRing />
      {leading}
      <input
        ref={ref}
        type="search"
        data-module-search={moduleSearch || undefined}
        aria-label={label}
        placeholder={placeholder ?? label}
        className="flex-1 min-w-0 h-11 bg-transparent border-none outline-none text-16 text-ink placeholder:text-ink-muted
          [&::-webkit-search-cancel-button]:hidden"
        {...props}
      />
      {trailing}
    </div>
  ) : (
    <label
      className={bare
        ? `flex items-center gap-2 h-full w-full px-3 text-ink-muted cursor-text ${className}`
        : `flex items-center gap-2 h-9 px-3 rounded-md text-ink-muted cursor-text
        focus-within:outline-2 focus-within:outline-focus focus-within:outline-offset-2
        ${tone === 'sunk' ? 'bg-sunk' : 'bg-base'} ${className}`}
    >
      <Search size={16} strokeWidth={1.75} aria-hidden="true" className="shrink-0" />
      <input
        ref={ref}
        type="search"
        data-module-search={moduleSearch || undefined}
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
