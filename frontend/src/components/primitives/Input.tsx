import { type InputHTMLAttributes, forwardRef } from 'react'

interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  error?: string
  helpText?: string
}

/** Recessed input — surface-container-lowest background for visual depth on elevated surfaces. */
export const Input = forwardRef<HTMLInputElement, InputProps>(
  ({ className = '', error, helpText, ...props }, ref) => (
    <div className="flex flex-col gap-1">
      <input
        ref={ref}
        className={`w-full px-3 py-2 text-small bg-surface-container-lowest border rounded-md
          text-on-surface placeholder:text-outline
          transition-colors duration-fast focus-ring
          ${error
            ? 'border-danger focus:border-danger'
            : 'border-outline-variant focus:border-primary'
          } ${className}`}
        {...props}
      />
      {error && <span className="text-micro text-danger">{error}</span>}
      {helpText && !error && <span className="text-micro text-outline">{helpText}</span>}
    </div>
  ),
)

Input.displayName = 'Input'
