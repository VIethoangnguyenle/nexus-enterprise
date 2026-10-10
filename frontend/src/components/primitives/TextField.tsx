import { forwardRef, useId, type InputHTMLAttributes, type ReactNode } from 'react'

interface TextFieldProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'size'> {
  /** Always present; a placeholder is never the label (DESIGN.md §6). */
  label: string
  /** Muted aside after the label, e.g. "(không bắt buộc)". */
  labelHint?: string
  /** Error under the field, wired with aria-describedby. */
  error?: string
  /** Help under the field while there is no error ("Chúng tôi gửi mã 6 số..."). */
  hint?: string
  /** 44px tall, for sign-in screens where touch targets matter (DESIGN.md §9). */
  large?: boolean
  /** Show `length/maxLength` inside the field. Needs `maxLength`. */
  counter?: boolean
  /** Element placed before the input box (e.g. a live icon preview). */
  leading?: ReactNode
}

/**
 * Tín hiệu input: 40px (44px with `large`), 16px text (no zoom on mobile),
 * raised fill with a hairline inset, focus as a 2px ring. Label above, error
 * or hint below.
 */
export const TextField = forwardRef<HTMLInputElement, TextFieldProps>(
  ({ label, labelHint, error, hint, large, counter, leading, maxLength, value, className = '', id, ...props }, ref) => {
    const autoId = useId()
    const inputId = id ?? autoId
    const errId = `${inputId}-err`
    const hintId = `${inputId}-hint`
    const describedBy = error ? errId : hint ? hintId : undefined
    const height = large ? 'min-h-11' : 'min-h-10'
    const inputHeight = large ? 'h-11' : 'h-10'
    const length = typeof value === 'string' ? Array.from(value).length : 0
    return (
      <div className={`grid gap-1.5 ${className}`}>
        <label htmlFor={inputId} className="text-sm font-semibold text-ink">
          {label}
          {labelHint && <span className="font-normal text-ink-muted"> {labelHint}</span>}
        </label>
        <div className="flex items-center gap-2.5">
          {leading}
          <div
            className={`flex-1 flex items-center gap-2 ${height} px-3 rounded-md bg-raised
              ${error ? 'field-danger' : 'field-line'} focus-within:field-focus`}
          >
            <input
              ref={ref}
              id={inputId}
              value={value}
              maxLength={maxLength}
              aria-invalid={!!error || undefined}
              aria-describedby={describedBy}
              className={`flex-1 min-w-0 ${inputHeight} bg-transparent border-none outline-none text-base text-ink
                placeholder:text-ink-muted`}
              {...props}
            />
            {counter && maxLength && (
              <span className="shrink-0 text-xs text-ink-muted tnum" aria-hidden="true">
                {length}/{maxLength}
              </span>
            )}
          </div>
        </div>
        {error && (
          <span id={errId} className="text-xs font-medium text-danger">
            {error}
          </span>
        )}
        {hint && !error && (
          <span id={hintId} className="text-xs text-ink-muted">
            {hint}
          </span>
        )}
      </div>
    )
  },
)
TextField.displayName = 'TextField'
