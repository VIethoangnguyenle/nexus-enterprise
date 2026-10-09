import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useState, useEffect, useCallback } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useRequestOTP, useVerifyOTP } from '../../hooks/useAuth'
import { authApi, startGoogleSignIn } from '../../api/auth'
import { Spinner, OtpInput, Button } from '../../components/primitives'
import { ArrowLeft, ArrowRight, Clock, Mail } from 'lucide-react'
import { keys } from '../../hooks/keys'

export const Route = createFileRoute('/_auth/login')({
  component: LoginPage,
})

type Step = 'identity' | 'otp'

/**
 * Sentences for the codes the auth service appends as /login?error=<code>
 * when Google sign-in fails. The code itself is never shown.
 */
const GOOGLE_ERROR_MESSAGES: Record<string, string> = {
  google_cancelled: 'Google sign-in was cancelled. You can try again or continue with email.',
  google_state: 'Your Google sign-in took too long or was started in another window. Please try again.',
  google_failed: "We couldn't verify your Google account. Please try again.",
  google_unverified: "Your Google account's email address isn't verified. Verify it with Google, then try again.",
  google_conflict: 'This email is already linked to a different Google account. Sign in with your email code instead.',
  google_unavailable: "Google sign-in isn't available right now. Continue with your email or phone number.",
  google_session: "We couldn't finish signing you in. Please try again.",
}
const GOOGLE_ERROR_FALLBACK = "We couldn't sign you in with Google. Please try again."

function googleErrorMessage(code: string | null): string | null {
  if (!code) return null
  return GOOGLE_ERROR_MESSAGES[code] ?? GOOGLE_ERROR_FALLBACK
}

function LoginPage() {
  const navigate = useNavigate()
  const requestOTP = useRequestOTP()
  const verifyOTP = useVerifyOTP()

  const [step, setStep] = useState<Step>('identity')
  const [identifier, setIdentifier] = useState('')
  const [sessionId, setSessionId] = useState('')
  const [resendCooldown, setResendCooldown] = useState(0)
  const [googleError] = useState(() =>
    googleErrorMessage(new URLSearchParams(window.location.search).get('error')),
  )

  // Only offer Google when the server has it configured; a failed lookup
  // hides the button rather than showing one that cannot work. OTP is the
  // opposite: it stays visible unless the server says it is off.
  const providers = useQuery({
    queryKey: keys.auth.providers(),
    queryFn: () => authApi.providers(),
    staleTime: 5 * 60 * 1000,
    retry: false,
  })
  const googleEnabled = providers.data?.google === true
  const otpEnabled = providers.data?.otp !== false
  // The server's documented test mode: every OTP sign-in accepts a fixed code.
  const otpFixedCode = providers.data?.otp_fixed_code === true
  const noSignInMethod = providers.isSuccess && !otpEnabled && !googleEnabled

  const emailRegex = /^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$/
  const phoneRegex = /^(0[3-9][0-9]{8,9}|\+?84[3-9][0-9]{7,8})$/
  const isEmail = emailRegex.test(identifier)
  const isPhone = phoneRegex.test(identifier)
  const isValid = isEmail || isPhone

  useEffect(() => {
    if (resendCooldown <= 0) return
    const timer = setTimeout(() => setResendCooldown((c) => c - 1), 1000)
    return () => clearTimeout(timer)
  }, [resendCooldown])

  const handleRequestOTP = useCallback(
    (e?: React.FormEvent) => {
      e?.preventDefault()
      const type = isEmail ? 'email' : 'phone'
      requestOTP.mutate(
        { identifier, type },
        {
          onSuccess: (res) => {
            setSessionId(res.session_id)
            setStep('otp')
            setResendCooldown(60)
          },
        },
      )
    },
    [identifier, isEmail, requestOTP],
  )

  const handleVerifyOTP = useCallback(
    (code: string) => {
      verifyOTP.mutate(
        { session_id: sessionId, code },
        {
          onSuccess: () => {
            const search = new URLSearchParams(window.location.search)
            search.delete('error') // a stale Google error must not follow the user
            navigate({ to: '/workspace-select' as any, search: Object.fromEntries(search) })
          },
        },
      )
    },
    [sessionId, verifyOTP, navigate],
  )

  const handleResend = useCallback(() => {
    if (resendCooldown > 0) return
    handleRequestOTP()
  }, [resendCooldown, handleRequestOTP])

  const handleBack = useCallback(() => {
    setStep('identity')
    setSessionId('')
    requestOTP.reset()
    verifyOTP.reset()
  }, [requestOTP, verifyOTP])

  const maskIdentifier = (id: string) => {
    if (id.includes('@')) {
      const [local, domain] = id.split('@')
      return local && domain && local.length > 2
        ? local.slice(0, 2) + '***@' + domain
        : '***'
    }
    return id.length >= 7 ? id.slice(0, 3) + '****' + id.slice(-3) : '***'
  }

  const error = requestOTP.error || verifyOTP.error

  // ─── Step 2: OTP Verification (matches .stitch/designs/verification.html) ───
  if (step === 'otp') {
    return (
      <div className="animate-fade-in">
        {/* Icon header — Stitch: w-16 h-16 bg-surface-container-high rounded-full */}
        <div className="text-center mb-6">
          <div className="w-16 h-16 bg-surface-container-high rounded-full flex items-center justify-center mx-auto mb-4 text-primary">
            <Mail size={32} />
          </div>
          <h1 className="text-h1 text-on-surface mb-1 tracking-tight">Verify your identity</h1>
          <p className="text-body text-on-surface-variant">
            We sent a 6-digit code to{' '}
            <span className="font-medium text-on-surface">{maskIdentifier(identifier)}</span>
          </p>
        </div>

        {error && (
          <div className="bg-error-container/30 border border-error/20 text-error px-4 py-3 rounded-lg text-small mb-6 text-left">
            {error.message}
          </div>
        )}

        <OtpInput
          onComplete={handleVerifyOTP}
          disabled={verifyOTP.isPending}
        />

        {verifyOTP.isPending && (
          <div className="flex justify-center mt-6">
            <Spinner size="sm" />
          </div>
        )}

        {/* Resend section — matches Stitch verification.html */}
        <div className="mt-6 text-center">
          <p className="text-small text-on-surface-variant">
            Didn't receive code?{' '}
            {resendCooldown > 0 ? (
              <span className="text-outline">
                <Clock size={14} className="inline-block align-text-bottom mr-1" />
                Wait 0:{resendCooldown.toString().padStart(2, '0')} before resending
              </span>
            ) : (
              /* eslint-disable-next-line no-restricted-syntax -- Link chữ nằm ngay trong câu văn
                 ("Didn't receive code? Resend"), không phải nút độc lập: size nhỏ nhất của Button
                 vẫn có px-3 py-1.5 + bo góc + focus-ring dạng box-shadow, biến nó thành một viên
                 thuốc nổi giữa dòng chữ thay vì một link trong dòng. Không variant nào khớp màu
                 text-primary/hover:text-primary-hover (primary tô nền, ghost dùng
                 text-on-surface-variant). Task 16. */
              <button
                onClick={handleResend}
                disabled={requestOTP.isPending}
                className="font-semibold text-primary hover:text-primary-hover bg-transparent border-none
                  cursor-pointer transition-colors focus:outline-none focus:underline"
              >
                {requestOTP.isPending ? 'Sending...' : 'Resend'}
              </button>
            )}
          </p>
        </div>

        {/* Back link — Stitch: border-t border-surface-container-high */}
        <div className="mt-8 pt-4 border-t border-surface-container-high text-center">
          <Button variant="ghost" size="sm" onClick={handleBack}>
            <ArrowLeft size={14} />
            Back to log in
          </Button>
        </div>

        {otpFixedCode && (
          <p className="text-caption text-outline mt-4 text-center">
            Test mode — OTP code is <span className="font-mono font-bold text-on-surface-variant">999999</span>
          </p>
        )}
      </div>
    )
  }

  // ─── Step 1: Identity Input (matches .stitch/designs/login.html) ───
  return (
    <div className="animate-fade-in">
      {/* Title — Stitch: font-h2 text-h2 text-on-surface centered */}
      <div className="text-center mb-6">
        <h1 className="text-h2 text-on-surface">Welcome back to Nexus</h1>
        <p className="text-body text-on-surface-variant mt-2">
          Log in to continue your secure workspace.
        </p>
      </div>

      {error && (
        <div className="bg-error-container/30 border border-error/20 text-error px-4 py-3 rounded-lg text-small mb-4 text-left">
          {error.message}
        </div>
      )}
      {!error && googleError && (
        <div role="alert" className="bg-error-container/30 border border-error/20 text-error px-4 py-3 rounded-lg text-small mb-4 text-left">
          {googleError}
        </div>
      )}

      {otpEnabled && (
        <>
          {/* Form — Stitch: flex flex-col gap-6 */}
          <form onSubmit={handleRequestOTP} className="flex flex-col gap-6">
            {/* Input Group — Stitch: flex flex-col gap-2 */}
            <div className="flex flex-col gap-2">
              <label className="font-semibold text-small text-on-surface" htmlFor="login-identifier">
                Email or Phone Number
              </label>
              {/* eslint-disable-next-line no-restricted-syntax -- rounded-lg (12px, token thật của
                  scale) sẽ âm thầm rơi về rounded-md (8px) mặc định của Input — đúng lỗi bo góc đã
                  ghi nhận ở ô OTP và ở hai input đã miễn trừ trong onboarding.tsx. Input còn ép cỡ chữ
                  text-small (13px, không có prop để đổi) trong khi trường này hiện kế thừa cỡ nền
                  16px — một hồi quy kiểu chữ trên ô nhập liệu quan trọng nhất màn hình. bg-surface
                  cũng không có variant tương ứng (variant "default" của Input đóng cứng
                  bg-surface-container-lowest). Task 16. */}
              <input
                id="login-identifier"
                type="text"
                value={identifier}
                onChange={(e) => setIdentifier(e.target.value)}
                placeholder="name@company.com"
                autoFocus
                className="w-full px-4 py-3 rounded-lg border border-outline-variant bg-surface text-on-surface
                  placeholder:text-outline focus:border-primary focus:ring-2 focus:ring-primary/10
                  transition-all outline-none"
              />
              {identifier && !isValid && (
                <p className="text-caption text-error">Enter a valid email or phone number</p>
              )}
            </div>
  
            {/* Primary Action — Stitch: rounded-lg py-3 bg-primary shadow-sm */}
            <Button
              type="submit"
              variant="primary"
              size="cta"
              disabled={!isValid || requestOTP.isPending}
              className="px-4 shadow-sm"
            >
              {requestOTP.isPending ? <Spinner size="sm" /> : (
                <>Continue <ArrowRight size={16} /></>
              )}
            </Button>
          </form>

          {/* Divider — Stitch: flex items-center gap-4 py-2, with 50% opacity lines */}
          <div className="flex items-center gap-4 py-2 my-4">
            <div className="flex-1 h-px bg-outline-variant/50" />
            <span className="text-small text-on-surface-variant">or continue with</span>
            <div className="flex-1 h-px bg-outline-variant/50" />
          </div>
        </>
      )}

      {noSignInMethod && (
        <p className="text-body text-on-surface-variant text-center">
          Sign-in isn't available right now. Please try again later.
        </p>
      )}

      {/* Social buttons — Stitch: rounded-lg py-2.5, border-outline-variant */}
      <div className="flex flex-col gap-3">
        {googleEnabled && (
          <Button variant="secondary" size="cta" className="px-4" onClick={startGoogleSignIn}>
            <svg className="w-5 h-5" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
              <path d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z" fill="#4285F4" />
              <path d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" fill="#34A853" />
              <path d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z" fill="#FBBC05" />
              <path d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z" fill="#EA4335" />
            </svg>
            Google
          </Button>
        )}
        <Button variant="secondary" size="cta" className="px-4">
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
            <path d="M15 3H9a6 6 0 0 0-6 6v6a6 6 0 0 0 6 6h6a6 6 0 0 0 6-6V9a6 6 0 0 0-6-6Z" />
            <circle cx="12" cy="12" r="3" />
          </svg>
          Single Sign-On (SSO)
        </Button>
      </div>

      {otpEnabled && otpFixedCode && (
        <p className="text-caption text-outline mt-6 text-center">
          Test mode — OTP code is <span className="font-mono font-bold text-on-surface-variant">999999</span>
        </p>
      )}
    </div>
  )
}
