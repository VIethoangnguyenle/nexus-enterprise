import { forwardRef, useEffect, useRef, type InputHTMLAttributes } from 'react'

interface CheckboxProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'type' | 'children'> {
  /** What ticking it does ("Chọn Tạm ứng công tác phí"). Always present: there is no visible label. */
  label: string
  /** Some, not all, of a group are ticked (the "select all" box). */
  indeterminate?: boolean
}

/**
 * Tick box: a real checkbox, 18px, in the accent colour, with the focus ring
 * and a name for assistive tech. Put visible text beside it with a `<label>`
 * of your own when there is some.
 */
export const Checkbox = forwardRef<HTMLInputElement, CheckboxProps>(
  ({ label, indeterminate = false, className = '', ...props }, ref) => {
    const inner = useRef<HTMLInputElement | null>(null)
    useEffect(() => {
      if (inner.current) inner.current.indeterminate = indeterminate
    }, [indeterminate])
    return (
      <input
        ref={(el) => {
          inner.current = el
          if (typeof ref === 'function') ref(el)
          else if (ref) ref.current = el
        }}
        type="checkbox"
        aria-label={label}
        className={`w-4.5 h-4.5 m-0 rounded-sm accent-accent cursor-pointer focus-ring
          disabled:opacity-50 disabled:cursor-not-allowed ${className}`}
        {...props}
      />
    )
  },
)
Checkbox.displayName = 'Checkbox'
