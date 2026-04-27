import { type InputHTMLAttributes, forwardRef } from 'react'

interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  label?: string
  error?: string
}

/** Text input with optional label and error message. */
export const Input = forwardRef<HTMLInputElement, InputProps>(
  ({ label, error, className = '', id, ...props }, ref) => {
    const inputId = id ?? label?.toLowerCase().replace(/\s+/g, '-')

    return (
      <div className="flex flex-col gap-1.5">
        {label && (
          <label
            htmlFor={inputId}
            className="text-xs font-medium text-text-secondary uppercase tracking-wider"
          >
            {label}
          </label>
        )}
        <input
          ref={ref}
          id={inputId}
          className={`w-full px-3 py-2 bg-bg-glass border border-border rounded-[var(--radius-sm)]
            text-text-primary text-sm font-[inherit] transition-colors duration-200
            placeholder:text-text-muted
            focus:outline-none focus:border-border-focus focus:shadow-[0_0_0_3px_var(--color-accent-glow)]
            disabled:opacity-50 disabled:cursor-not-allowed
            ${error ? 'border-danger focus:border-danger focus:shadow-[0_0_0_3px_var(--color-danger-bg)]' : ''}
            ${className}`}
          {...props}
        />
        {error && (
          <p className="text-xs text-danger mt-0.5">{error}</p>
        )}
      </div>
    )
  },
)

Input.displayName = 'Input'
