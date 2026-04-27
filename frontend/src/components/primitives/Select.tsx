import { type SelectHTMLAttributes, forwardRef } from 'react'

interface SelectProps extends SelectHTMLAttributes<HTMLSelectElement> {
  label?: string
  error?: string
  options: Array<{ value: string; label: string }>
  placeholder?: string
}

/** Select dropdown with consistent styling matching Input/Textarea. */
export const Select = forwardRef<HTMLSelectElement, SelectProps>(
  ({ label, error, options, placeholder, className = '', id, ...props }, ref) => {
    const selectId = id ?? label?.toLowerCase().replace(/\s+/g, '-')

    return (
      <div className="flex flex-col gap-1.5">
        {label && (
          <label
            htmlFor={selectId}
            className="text-xs font-medium text-text-secondary uppercase tracking-wider"
          >
            {label}
          </label>
        )}
        <select
          ref={ref}
          id={selectId}
          className={`w-full px-3 py-2 bg-bg-glass border border-border rounded-[var(--radius-sm)]
            text-text-primary text-sm font-[inherit] transition-colors duration-200
            focus:outline-none focus:border-border-focus focus:shadow-[0_0_0_3px_var(--color-accent-glow)]
            disabled:opacity-50 disabled:cursor-not-allowed
            ${error ? 'border-danger' : ''}
            ${className}`}
          {...props}
        >
          {placeholder && (
            <option value="" disabled>{placeholder}</option>
          )}
          {options.map(opt => (
            <option key={opt.value} value={opt.value}>{opt.label}</option>
          ))}
        </select>
        {error && <p className="text-xs text-danger mt-0.5">{error}</p>}
      </div>
    )
  },
)

Select.displayName = 'Select'
