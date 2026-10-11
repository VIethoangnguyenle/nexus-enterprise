import { useLayoutEffect, useRef, useState, type ClipboardEvent, type SyntheticEvent } from 'react'
import { motion } from 'motion/react'
import { Check } from 'lucide-react'
import { DURATION, EASE, useMotionPresets } from '../../lib/motion'

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
   * error (wrong code: the error colour and one sideways jolt) · success (the
   * boxes gather into a check badge).
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
 * Motion is the sign-in exception in DESIGN.md §7, transform and opacity only:
 * a typed digit pops (box .92 → 1, digit 6px up, 160ms), a wrong code jolts the
 * row ±6px once (280ms), a right one washes the boxes in turn (40ms apart),
 * gathers them to the middle and lands a check badge. Under reduced motion every
 * one of those is a colour change or a fade of at most 120ms.
 */
const JOLT = [0, -6, 6, -4, 4, 0]
/** The check badge appears once the boxes have started to gather. */
const GATHER_AT = 0.24
const BADGE_AT = 0.4
/** Gap between one box's wash and the next. */
const WASH_STEP = 0.04

export function OtpInput({
  label, value, onChange, onComplete, length = 6, status = 'idle', errorKey = 0,
  disabled, autoFocus, describedBy,
}: OtpInputProps) {
  const m = useMotionPresets()
  const inputRef = useRef<HTMLInputElement>(null)
  const [focused, setFocused] = useState(false)
  const [caret, setCaret] = useState(value.length)
  const rowRef = useRef<HTMLDivElement>(null)
  // Distance between the centres of two neighbouring boxes, read when the code is right.
  const [pitch, setPitch] = useState(0)
  const locked = status === 'pending' || status === 'success'
  const success = status === 'success'

  useLayoutEffect(() => {
    if (!success) return
    const [a, b] = Array.from(rowRef.current?.querySelectorAll('[data-otp-box]') ?? [])
    if (a && b) setPitch(b.getBoundingClientRect().left - a.getBoundingClientRect().left)
  }, [success])

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

  const fade = { duration: DURATION.reducedFade, ease: 'linear' as const }
  const popTransition = { duration: DURATION.quick, ease: EASE.out }

  return (
    <div className="relative">
      {/* Only the row is re-keyed to replay the jolt or fade; the input stays mounted so focus survives a wrong code. */}
      <motion.div
        ref={rowRef}
        key={status === 'error' ? `error-${errorKey}` : 'steady'}
        data-otp-row
        data-jolt={status === 'error' && !m.reduced ? 'true' : undefined}
        {...(status === 'error' && m.reduced ? m.route : {})}
        {...(status === 'error' && !m.reduced
          ? { initial: { x: 0 }, animate: { x: JOLT, transition: { duration: DURATION.layout, ease: EASE.out } } }
          : {})}
        className="grid gap-2"
        style={{ gridTemplateColumns: `repeat(${length}, minmax(0, 1fr))` }}
      >
        {Array.from({ length }, (_, i) => {
          const ch = value[i]
          // A box that holds a digit is keyed by it, so a new digit mounts afresh and pops.
          const gather = success && !m.reduced
            ? { x: -(i - (length - 1) / 2) * pitch, scale: 0.6, opacity: 0,
                transition: { duration: DURATION.base, ease: EASE.in, delay: GATHER_AT } }
            : success
              ? { opacity: 0, transition: { ...fade, delay: DURATION.reducedFade } }
              : { x: 0, scale: 1, opacity: status === 'pending' ? 0.6 : 1, transition: popTransition }
          return (
            <motion.span
              key={ch ? `${i}-${ch}` : i}
              data-otp-box
              data-status={status}
              data-active={focused && !locked && i === activeIndex ? 'true' : 'false'}
              aria-hidden="true"
              initial={ch && !m.reduced ? { scale: 0.92 } : false}
              animate={gather}
              className={`relative grid place-items-center h-13 rounded-md bg-raised font-display text-xl font-semibold tnum
                ${status === 'success' ? 'text-success' : 'text-ink'}
                ${boxTone}
                ${focused && !locked && i === activeIndex && status !== 'error' ? 'field-focus' : ''}`}
            >
              {success && (
                <motion.span
                  className="absolute inset-0 rounded-md bg-success-wash"
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1, transition: m.reduced ? fade : { duration: DURATION.quick, ease: EASE.out, delay: i * WASH_STEP } }}
                />
              )}
              {ch && (
                <motion.span
                  data-otp-digit
                  className="relative"
                  initial={m.reduced ? false : { opacity: 0, y: 6 }}
                  animate={{ opacity: 1, y: 0, transition: popTransition }}
                >
                  {ch}
                </motion.span>
              )}
            </motion.span>
          )
        })}
      </motion.div>
      {success && (
        <motion.span
          data-otp-badge
          aria-hidden="true"
          className="pointer-events-none absolute inset-0 grid place-items-center"
        >
          <motion.span
            className="grid place-items-center w-10 h-10 rounded-full bg-success-wash text-success"
            initial={m.reduced ? { opacity: 0 } : { opacity: 0, scale: 0.6 }}
            animate={m.reduced
              ? { opacity: 1, transition: { ...fade, delay: DURATION.reducedFade } }
              : { opacity: 1, scale: 1, transition: { duration: DURATION.layout, ease: EASE.outExpo, delay: BADGE_AT } }}
          >
            <Check size={22} strokeWidth={2.25} />
          </motion.span>
        </motion.span>
      )}
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
