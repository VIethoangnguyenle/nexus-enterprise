import { useRef, useState, type ClipboardEvent, type SyntheticEvent } from 'react'
import { motion } from 'motion/react'
import { useMotionPresets } from '../../lib/motion'

type OtpStatus = 'idle' | 'pending' | 'error' | 'success'

interface OtpInputProps {
  /** Accessible name of the field; the boxes themselves are decoration. */
  label: string
  /** The digits entered so far. Controlled: the parent clears it after a wrong code. */
  value: string
  onChange: (value: string) => void
  /** Called once, with the whole code, when the last digit is typed or pasted. */
  onComplete?: (code: string) => void
  length?: number
  /**
   * idle · pending (checking the code: keeps the digits, ignores typing) ·
   * error (wrong code: the error colour, with a fade) · success.
   */
  status?: OtpStatus
  /** Changes with every wrong code so the fade plays again for a second one in a row. */
  errorKey?: number
  disabled?: boolean
  autoFocus?: boolean
  /** Id of the element that explains the state ("Mã chưa đúng. Còn 3 lần thử."). */
  describedBy?: string
}

const digitsOnly = (s: string) => s.replace(/\D/g, '')

/**
 * One-time code field (DESIGN.md §6): six 52px boxes over ONE real input.
 *
 * A row of six inputs cannot take a paste, an SMS autofill or a password
 * manager's fill in one go; one input with `autocomplete="one-time-code"` can,
 * so it sits invisibly over the boxes and the boxes only draw what it holds.
 * Arrow keys, Home and End move its caret, and the box under the caret shows
 * the focus ring.
 *
 * A wrong code is shown with the error colour and a fade (§7 has no preset for
 * a sideways jolt); motion goes through lib/motion, so reduced motion shortens
 * the fade.
 */
export function OtpInput({
  label, value, onChange, onComplete, length = 6, status = 'idle', errorKey = 0,
  disabled, autoFocus, describedBy,
}: OtpInputProps) {
  const m = useMotionPresets()
  const inputRef = useRef<HTMLInputElement>(null)
  const [focused, setFocused] = useState(false)
  const [caret, setCaret] = useState(value.length)
  const locked = status === 'pending' || status === 'success'

  const trackCaret = (e: SyntheticEvent<HTMLInputElement>) => setCaret(e.currentTarget.selectionStart ?? value.length)

  const accept = (raw: string) => {
    if (locked || disabled) return
    const next = digitsOnly(raw).slice(0, length)
    if (next === value) return
    onChange(next)
    if (next.length === length) onComplete?.(next)
  }

  const handlePaste = (e: ClipboardEvent<HTMLInputElement>) => {
    // Take what a person copied from a message ("Mã của bạn là 481 209...")
    // and keep only its digits; the browser would cut it at the first non-digit.
    e.preventDefault()
    const pasted = digitsOnly(e.clipboardData.getData('text')).slice(0, length)
    if (!pasted) return
    accept(pasted)
  }

  const activeIndex = Math.min(Math.max(caret, 0), length - 1)
  const boxTone = status === 'error' ? 'field-danger' : 'field-line'

  return (
    <div className="relative">
      {/* Only the boxes are re-keyed to replay the fade; the input stays mounted so focus survives a wrong code. */}
      <motion.div
        key={status === 'error' ? `error-${errorKey}` : 'steady'}
        {...(status === 'error' ? m.route : {})}
        className="grid gap-2"
        style={{ gridTemplateColumns: `repeat(${length}, minmax(0, 1fr))` }}
      >
        {Array.from({ length }, (_, i) => (
          <span
            key={i}
            data-otp-box
            data-status={status}
            data-active={focused && !locked && i === activeIndex ? 'true' : 'false'}
            aria-hidden="true"
            className={`grid place-items-center h-13 rounded-md bg-raised font-display text-xl font-semibold tnum
              ${status === 'success' ? 'text-success' : 'text-ink'}
              ${boxTone} ${status === 'pending' ? 'opacity-60' : ''}
              ${focused && !locked && i === activeIndex && status !== 'error' ? 'field-focus' : ''}`}
          >
            {value[i] ?? ''}
          </span>
        ))}
      </motion.div>
      <input
        ref={inputRef}
        value={value}
        onChange={(e) => accept(e.target.value)}
        onPaste={handlePaste}
        onSelect={trackCaret}
        onKeyUp={trackCaret}
        onClick={trackCaret}
        onFocus={(e) => {
          setFocused(true)
          trackCaret(e)
        }}
        onBlur={() => setFocused(false)}
        type="text"
        inputMode="numeric"
        pattern="[0-9]*"
        autoComplete="one-time-code"
        autoFocus={autoFocus}
        disabled={disabled}
        readOnly={locked}
        aria-label={label}
        aria-invalid={status === 'error' || undefined}
        aria-busy={status === 'pending' || undefined}
        aria-describedby={describedBy}
        spellCheck={false}
        className="absolute inset-0 w-full h-full opacity-0 cursor-text caret-transparent
          disabled:cursor-not-allowed"
      />
    </div>
  )
}
