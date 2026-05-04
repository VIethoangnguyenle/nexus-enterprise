import { type SelectHTMLAttributes, forwardRef } from 'react'

interface SelectProps extends SelectHTMLAttributes<HTMLSelectElement> {
  error?: string
}

/** Recessed select — matches Input styling with surface-container-lowest background. */
export const Select = forwardRef<HTMLSelectElement, SelectProps>(
  ({ className = '', error, children, ...props }, ref) => (
    <div className="flex flex-col gap-1">
      <select
        ref={ref}
        className={`w-full px-3 py-2 text-small bg-surface-container-lowest border rounded-md
          text-on-surface transition-colors duration-fast
          cursor-pointer appearance-none focus-ring
          ${error
            ? 'border-danger focus:border-danger'
            : 'border-outline-variant focus:border-primary'
          } ${className}`}
        {...props}
      >
        {children}
      </select>
      {error && <span className="text-micro text-danger">{error}</span>}
    </div>
  ),
)

Select.displayName = 'Select'
