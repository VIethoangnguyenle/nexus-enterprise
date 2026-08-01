import { useRef, useCallback, type KeyboardEvent, type ClipboardEvent } from 'react'

interface OtpInputProps {
  length?: number
  onComplete: (code: string) => void
  disabled?: boolean
}

/** 6-digit OTP input matching Stitch verification.html source.
 *  Key tokens: w-12 h-14 sm:w-14 sm:h-16, bg-surface, border-outline-variant,
 *  focus:border-primary, rounded-lg, text-h2 size. */
export function OtpInput({ length = 6, onComplete, disabled }: OtpInputProps) {
  const inputRefs = useRef<(HTMLInputElement | null)[]>([])

  const handleChange = useCallback(
    (index: number, value: string) => {
      const digit = value.replace(/\D/g, '').slice(-1)
      const input = inputRefs.current[index]
      if (input) input.value = digit

      if (digit && index < length - 1) {
        inputRefs.current[index + 1]?.focus()
      }

      // Auto-submit when all digits filled
      if (digit && index === length - 1) {
        const code = inputRefs.current.map((ref) => ref?.value || '').join('')
        if (code.length === length) {
          onComplete(code)
        }
      }
    },
    [length, onComplete],
  )

  const handleKeyDown = useCallback(
    (index: number, e: KeyboardEvent<HTMLInputElement>) => {
      if (e.key === 'Backspace') {
        const input = inputRefs.current[index]
        if (input && !input.value && index > 0) {
          inputRefs.current[index - 1]?.focus()
        }
      }
    },
    [],
  )

  const handlePaste = useCallback(
    (e: ClipboardEvent<HTMLInputElement>) => {
      e.preventDefault()
      const paste = e.clipboardData.getData('text').replace(/\D/g, '').slice(0, length)
      paste.split('').forEach((digit, i) => {
        const input = inputRefs.current[i]
        if (input) input.value = digit
      })
      if (paste.length === length) {
        onComplete(paste)
      } else if (paste.length > 0) {
        inputRefs.current[paste.length]?.focus()
      }
    },
    [length, onComplete],
  )

  return (
    <div className="flex justify-between gap-2 sm:gap-4">
      {Array.from({ length }).map((_, i) => (
        <input
          key={i}
          ref={(el) => { inputRefs.current[i] = el }}
          type="text"
          inputMode="numeric"
          maxLength={1}
          disabled={disabled}
          autoFocus={i === 0}
          className="w-12 h-14 sm:w-14 sm:h-16 text-center text-h2 rounded-lg
            bg-surface border border-outline-variant text-on-surface
            placeholder:text-outline-variant
            focus:border-primary focus:ring-2 focus:ring-primary/10 outline-none
            transition-all duration-200
            disabled:opacity-50"
          placeholder="•"
          onChange={(e) => handleChange(i, e.target.value)}
          onKeyDown={(e) => handleKeyDown(i, e)}
          onPaste={i === 0 ? handlePaste : undefined}
          aria-label={`Digit ${i + 1}`}
        />
      ))}
    </div>
  )
}
