import { type InputHTMLAttributes, forwardRef } from 'react'

type InputVariant = 'default' | 'search'

interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  error?: string
  helpText?: string
  variant?: InputVariant
}

/** `search` theo Stitch §Inputs: pill trên nền surface-container-low. */
const variantStyles: Record<InputVariant, string> = {
  default: 'rounded-md bg-surface-container-lowest',
  search: 'rounded-full bg-surface-container-low',
}

/** Recessed input — surface-container-lowest background for visual depth on elevated surfaces. */
export const Input = forwardRef<HTMLInputElement, InputProps>(
  ({ className = '', error, helpText, variant = 'default', ...props }, ref) => (
    <div className="flex flex-col gap-1">
      <input
        ref={ref}
        className={`w-full px-3 py-2 text-small border
          text-on-surface placeholder:text-outline
          transition-colors duration-fast focus-ring
          ${variantStyles[variant]}
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
