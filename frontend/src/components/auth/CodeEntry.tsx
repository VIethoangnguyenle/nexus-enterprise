import { useEffect, useId, useState } from 'react'
import { AlertCircle } from 'lucide-react'
import type { OTPVerifyResponse } from '../../api/auth'
import {
  useConfirmEmailVerification, useRequestEmailVerification, useRequestOTP, useVerifyOTP,
} from '../../hooks/useAuth'
import { describeAuthError, formatCountdown, type DescribedError, type Identifier } from '../../lib/auth-flow'
import { formatTime } from '../../lib/format'
import { Button, OtpInput } from '../primitives'
import { Notice } from './Notice'

export interface CodeSession {
  sessionId: string
  expiresAt: Date
}

/** Seconds before a new code may be asked for (the server also limits it: 5 per 15 minutes). */
export const RESEND_SECONDS = 60

interface CodeEntryCommon {
  identifier: Identifier
  session: CodeSession
  autoFocus?: boolean
}

type CodeEntryProps = CodeEntryCommon &
  (
    | {
        /** Signing in: the code starts a session. */
        mode?: 'sign-in'
        /** The code was right. The caller decides where to go. */
        onVerified: (result: OTPVerifyResponse) => void
      }
    | {
        /**
         * Proving the address of the account already signed in. The code is
         * checked against that account, and nothing is signed in.
         */
        mode: 'verify-email'
        onVerified: () => void
      }
  )

/** Failures that mean "this code" rather than "the service". */
const ABOUT_THE_CODE = new Set<DescribedError['kind']>(['wrong-code', 'locked', 'expired'])

/**
 * Six-digit code entry with its whole life: checking the code, saying why it
 * was refused, the resend countdown and asking for another code. Used by
 * sign-in and by "verify this address".
 */
export function CodeEntry(props: CodeEntryProps) {
  const { identifier, session: initial, autoFocus = true } = props
  const proving = props.mode === 'verify-email'
  const messageId = useId()
  const signInVerify = useVerifyOTP()
  const signInRequest = useRequestOTP()
  const proveVerify = useConfirmEmailVerification()
  const proveRequest = useRequestEmailVerification()
  const verifying = proving ? proveVerify : signInVerify
  const requesting = proving ? proveRequest : signInRequest

  const [session, setSession] = useState(initial)
  const [code, setCode] = useState('')
  const [error, setError] = useState<DescribedError | null>(null)
  const [errorKey, setErrorKey] = useState(0)
  const [resendError, setResendError] = useState<DescribedError | null>(null)
  const [cooldown, setCooldown] = useState(RESEND_SECONDS)

  const counting = cooldown > 0
  useEffect(() => {
    if (!counting) return
    const timer = setInterval(() => setCooldown((c) => Math.max(0, c - 1)), 1000)
    return () => clearInterval(timer)
  }, [counting])

  // A code the server has thrown away cannot be fixed by typing: take no
  // digits, and do not make the person wait out the countdown for a new one.
  const dead = error?.kind === 'locked' || error?.kind === 'expired'
  const aboutTheCode = error !== null && ABOUT_THE_CODE.has(error.kind)

  const submit = (value: string) => {
    setError(null)
    const failed = (err: unknown) => {
      setError(describeAuthError(err, 'verify-code'))
      setErrorKey((k) => k + 1)
      setCode('')
    }
    if (props.mode === 'verify-email') {
      const done = props.onVerified
      proveVerify.mutate(value, { onSuccess: () => done(), onError: failed })
      return
    }
    const done = props.onVerified
    signInVerify.mutate({ session_id: session.sessionId, code: value }, { onSuccess: done, onError: failed })
  }

  const resend = () => {
    const issued = (expiresIn: number, sessionId: string) => {
      setSession({ sessionId, expiresAt: new Date(Date.now() + expiresIn * 1000) })
      setCode('')
      setError(null)
      setResendError(null)
      setCooldown(RESEND_SECONDS)
      verifying.reset()
    }
    const refused = (err: unknown) => {
      const described = describeAuthError(err, proving ? 'verify-email' : 'request-code')
      setResendError(described)
      if (described.kind === 'rate-limited' && described.retryAfterSeconds) {
        setCooldown(described.retryAfterSeconds)
      }
    }
    if (proving) {
      proveRequest.mutate(undefined, { onSuccess: (res) => issued(res.expires_in, session.sessionId + '+'), onError: refused })
      return
    }
    signInRequest.mutate(
      { identifier: identifier.value, type: identifier.kind },
      { onSuccess: (res) => issued(res.expires_in, res.session_id), onError: refused },
    )
  }

  const status = verifying.isPending ? 'pending' : verifying.isSuccess ? 'success' : aboutTheCode ? 'error' : 'idle'
  const resendLocked = counting && !(dead && !resendError)

  return (
    <div className="grid gap-3">
      <div className="grid gap-2">
        <OtpInput
          // A fresh code is a fresh field: empty, focused, enabled.
          key={session.sessionId}
          label="Mã xác minh"
          value={code}
          onChange={setCode}
          onComplete={submit}
          status={status}
          errorKey={errorKey}
          disabled={dead}
          autoFocus={autoFocus}
          describedBy={messageId}
        />
        <div id={messageId} aria-live="polite" className="min-h-5 text-sm">
          {aboutTheCode ? (
            <span className="flex items-center gap-1.5 text-danger">
              <AlertCircle size={16} strokeWidth={1.75} className="shrink-0" aria-hidden="true" />
              {error.message}
            </span>
          ) : (
            <span className="text-ink-muted">Mã có hiệu lực tới {formatTime(session.expiresAt)}.</span>
          )}
        </div>
      </div>

      {error && !aboutTheCode && <Notice>{error.message}</Notice>}
      {resendError && <Notice>{resendError.message}</Notice>}

      <div className="flex items-center justify-between gap-3 text-sm text-ink-muted">
        {counting ? <span className="tnum">Gửi lại mã sau {formatCountdown(cooldown)}</span> : <span />}
        <Button
          variant="link"
          size="link"
          disabled={resendLocked || requesting.isPending}
          onClick={resend}
        >
          Gửi lại
        </Button>
      </div>
    </div>
  )
}
