import { useRef, useState, type FormEvent } from 'react'
import { useNavigate, useSearch } from '@tanstack/react-router'
import { AnimatePresence, motion } from 'motion/react'
import { ArrowLeft, Lock, RefreshCw } from 'lucide-react'
import { startGoogleSignIn, type OTPVerifyResponse } from '../../api/auth'
import { useProviders, useRequestOTP } from '../../hooks/useAuth'
import {
  afterSignIn, describeAuthError, displayIdentifier, googleErrorMessage, identifierError, parseIdentifier,
  type DescribedError, type Identifier,
} from '../../lib/auth-flow'
import { useMotionPresets } from '../../lib/motion'
import { Button, Heading, IconButton, Text, TextField } from '../primitives'
import { AddressFlight, canFly, type FlightOrigin } from './AddressFlight'
import { stepPresets, STEP_SHIFT } from './auth-motion'
import { CodeEntry, type CodeSession } from './CodeEntry'
import { GoogleLogo } from './GoogleLogo'
import { Notice } from './Notice'

const WIDE = 'w-full h-11'

type Step = { name: 'identity' } | { name: 'code'; identifier: Identifier; session: CodeSession; from?: FlightOrigin }

type Direction = 1 | -1

/**
 * Sign in (design/mockups/auth.html §1, §2): Google, or a six-digit code sent
 * to an email or phone. A person with no account takes the same road; the
 * account is made when the code is right, and the profile step follows.
 */
export function LoginScreen() {
  const m = useMotionPresets()
  const [step, setStep] = useState<Step>({ name: 'identity' })
  const [direction, setDirection] = useState<Direction>(1)
  // The first screen is introduced by the shell; later steps rise a beat sooner.
  const [moved, setMoved] = useState(false)
  // Held here so the address survives a trip to the code step and back.
  const [raw, setRaw] = useState('')

  const go = (next: Step, dir: Direction) => {
    setDirection(dir)
    setMoved(true)
    setStep(next)
  }

  // Which way a step travels is read when it leaves, so it is passed as `custom`.
  const variants = {
    enter: (d: Direction) => stepPresets(m.reduced, d).initial,
    center: (d: Direction) => stepPresets(m.reduced, d).animate,
    leave: (d: Direction) => stepPresets(m.reduced, d).exit,
  }

  return (
    <AnimatePresence mode="wait" initial={false} custom={direction}>
      <motion.div
        key={step.name}
        custom={direction}
        variants={variants}
        initial="enter"
        animate="center"
        exit="leave"
        className={`auth-rise ${moved ? 'auth-rise--step' : ''} grid gap-5`}
      >
        {step.name === 'identity' ? (
          <IdentityStep
            raw={raw}
            onRaw={setRaw}
            onSent={(identifier, session, from) => go({ name: 'code', identifier, session, from }, 1)}
          />
        ) : (
          <CodeStep step={step} onBack={() => go({ name: 'identity' }, -1)} />
        )}
      </motion.div>
    </AnimatePresence>
  )
}

function IdentityStep({ raw, onRaw, onSent }: {
  raw: string
  onRaw: (value: string) => void
  onSent: (id: Identifier, session: CodeSession, from?: FlightOrigin) => void
}) {
  const search = useSearch({ strict: false }) as { error?: string }
  const providers = useProviders()
  const request = useRequestOTP()

  const [touched, setTouched] = useState(false)
  const [submitted, setSubmitted] = useState(false)
  const [failure, setFailure] = useState<DescribedError | null>(null)

  const googleOn = providers.data?.google === true
  // Codes are on unless the server says otherwise: a failed lookup must not
  // lock people out of the one way in that may still work.
  const codesOn = providers.data?.otp !== false
  const nothingOffered = providers.isSuccess && !googleOn && !codesOn

  const googleProblem = failure ? null : googleErrorMessage(search.error ?? null)
  const fieldError = touched || submitted ? identifierError(raw, submitted) : null

  const submit = (e: FormEvent) => {
    e.preventDefault()
    setSubmitted(true)
    const identifier = parseIdentifier(raw)
    if (!identifier) return
    setFailure(null)
    // Where the address sits now, for it to leave from once the code step is up.
    const typed = e.currentTarget.querySelector('input')
    const from: FlightOrigin | undefined = typed
      ? { rect: typed.getBoundingClientRect(), text: displayIdentifier(identifier) }
      : undefined
    request.mutate(
      { identifier: identifier.value, type: identifier.kind },
      {
        onSuccess: (res) => {
          onSent(identifier, { sessionId: res.session_id, expiresAt: new Date(Date.now() + res.expires_in * 1000) }, from)
        },
        onError: (err) => setFailure(describeAuthError(err, 'request-code')),
      },
    )
  }

  if (nothingOffered) {
    return (
      <div className="grid justify-items-center gap-3 py-10 text-center">
        <span className="grid place-items-center w-10 h-10 rounded-overlay bg-danger-wash text-danger" aria-hidden="true">
          <Lock size={20} strokeWidth={1.75} />
        </span>
        <Heading as="h1" look="panel">Đăng nhập đang tạm ngưng</Heading>
        <Text variant="body" muted className="block max-w-72">
          Quản trị hệ thống đang bảo trì. Thử lại sau ít phút.
        </Text>
        <Button variant="soft" size="sm" onClick={() => void providers.refetch()}>
          <RefreshCw size={16} strokeWidth={1.75} aria-hidden="true" />
          Thử lại
        </Button>
      </div>
    )
  }

  return (
    <>
      <div className="grid gap-1.5">
        <Heading as="h1" look="page">Đăng nhập vào Nexus</Heading>
        <Text variant="body" muted className="block">Dùng email công việc hoặc số điện thoại.</Text>
      </div>

      {googleProblem && <Notice>{googleProblem}</Notice>}
      {failure && <Notice>{failure.message}</Notice>}

      {googleOn && (
        <Button variant="secondary" size="md" className={WIDE} onClick={() => startGoogleSignIn()}>
          <GoogleLogo />
          Tiếp tục với Google
        </Button>
      )}

      {googleOn && codesOn && (
        <div className="flex items-center gap-3 text-sm text-ink-muted">
          <span className="h-px flex-1 bg-line" />
          hoặc nhận mã đăng nhập
          <span className="h-px flex-1 bg-line" />
        </div>
      )}

      {codesOn && (
        <form onSubmit={submit} noValidate className="grid gap-5">
          <TextField
            large
            autoFocus
            label="Email hoặc số điện thoại"
            type="text"
            inputMode="email"
            autoComplete="username"
            autoCapitalize="none"
            spellCheck={false}
            value={raw}
            onChange={(e) => onRaw(e.target.value)}
            onBlur={() => setTouched(true)}
            error={fieldError ?? undefined}
            hint="Chúng tôi gửi mã 6 số, có hiệu lực 5 phút."
          />
          <Button type="submit" variant="primary" size="md" className={WIDE} loading={request.isPending}>
            Nhận mã đăng nhập
          </Button>
          <Text variant="small" muted className="block text-center">
            Chưa có tài khoản? Nhập email ở trên, tài khoản được tạo sau khi xác minh.
          </Text>
        </form>
      )}
    </>
  )
}

function CodeStep({ step, onBack }: { step: Extract<Step, { name: 'code' }>; onBack: () => void }) {
  const navigate = useNavigate()
  const { ws } = useSearch({ strict: false }) as { ws?: string }
  const { identifier, session, from } = step
  const m = useMotionPresets()
  const address = useRef<HTMLElement>(null)
  // The address leaves the field and lands here; until it does, the real text waits.
  const [flying, setFlying] = useState(() => !!from && !m.reduced && canFly())

  const done = (res: OTPVerifyResponse) => {
    // Only the workspace asked for in the link rides along; a Google error
    // from an earlier attempt must not follow the person into the app.
    void navigate({ to: afterSignIn(res.needs_profile), search: ws ? { ws } : {} })
  }

  return (
    <>
      <IconButton aria-label="Quay lại" size="lg" onClick={onBack} className="-ml-2 w-11 h-11 lg:hidden">
        <ArrowLeft size={18} strokeWidth={1.75} />
      </IconButton>
      <div className="grid gap-1.5">
        <Heading as="h1" look="page">Nhập mã 6 số</Heading>
        <Text variant="body" muted className="block">
          Đã gửi tới{' '}
          <b ref={address} className="font-semibold text-ink" style={flying ? { opacity: 0 } : undefined}>
            {displayIdentifier(identifier)}
          </b>.{' '}
          <Button variant="link" size="link" onClick={onBack}>
            {identifier.kind === 'email' ? 'Đổi email' : 'Đổi số'}
          </Button>
        </Text>
      </div>
      <CodeEntry identifier={identifier} session={session} onVerified={done} />
      {identifier.kind === 'email' && (
        <Text variant="small" muted className="block text-center">
          Không thấy thư? Kiểm tra mục Thư rác.
        </Text>
      )}
      {flying && from && (
        <AddressFlight from={from} target={address} shift={m.reduced ? 0 : STEP_SHIFT} onDone={() => setFlying(false)} />
      )}
    </>
  )
}
