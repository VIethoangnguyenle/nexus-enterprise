import { useState } from 'react'
import { Mail } from 'lucide-react'
import { leaveFor, type AuthProviders } from '../../api/auth'
import { useRequestEmailVerification, useStartEmailVerificationWithGoogle } from '../../hooks/useAuth'
import { describeAuthError, type DescribedError } from '../../lib/auth-flow'
import { Button, Text } from '../primitives'
import { CodeEntry, type CodeSession } from './CodeEntry'
import { GoogleLogo } from './GoogleLogo'
import { Notice } from './Notice'

interface VerifyEmailNoteProps {
  email: string
  providers: AuthProviders | undefined
  /** What the person came here to do: it decides the first sentence. */
  purpose: 'invitations' | 'create-workspace'
  /** The address has just been proved. */
  onVerified: () => void
}

/**
 * Why a person cannot yet see their invitations (or create a workspace), and how
 * to change that.
 *
 * An invitation is addressed to an email, and the server shows it only to an
 * account whose address has been proved, so that nobody can read an offer meant
 * for someone else by signing up with their address; for the same reason only
 * such an account may create a workspace. Two things prove an address: a Google
 * account whose own verified email it is, and a code delivered to it by a real
 * mail sender. A route is offered only when the server says it is available: the
 * test-only fixed code proves nothing and is not offered.
 *
 * Both routes prove the address of the account the person is already in. Neither
 * signs anyone in or changes who they are signed in as: choosing another Google
 * account is refused, not obeyed.
 */
export function VerifyEmailNote({ email, providers, purpose, onVerified }: VerifyEmailNoteProps) {
  const request = useRequestEmailVerification()
  const google = useStartEmailVerificationWithGoogle()
  const [session, setSession] = useState<CodeSession | null>(null)
  const [failure, setFailure] = useState<DescribedError | null>(null)

  const byGoogle = providers?.google === true
  const byCode = providers?.otp_proves_email === true && providers.otp !== false

  const sendCode = () => {
    setFailure(null)
    request.mutate(undefined, {
      onSuccess: (res) =>
        setSession({ sessionId: 'this-account', expiresAt: new Date(Date.now() + res.expires_in * 1000) }),
      onError: (err) => setFailure(describeAuthError(err, 'verify-email')),
    })
  }

  const withGoogle = () => {
    setFailure(null)
    google.mutate(undefined, {
      onSuccess: (url) => leaveFor(url),
      onError: (err) => setFailure(describeAuthError(err, 'verify-email')),
    })
  }

  return (
    <div className="grid gap-3 p-3 rounded-surface bg-info-wash">
      <div className="flex items-start gap-2.5">
        <Mail size={16} strokeWidth={1.75} className="mt-0.5 shrink-0 text-info" aria-hidden="true" />
        <div className="grid gap-1 min-w-0">
          <Text variant="body" className="block font-medium break-words">
            {purpose === 'invitations'
              ? `Lời mời gửi tới ${email} sẽ hiện ở đây sau khi bạn xác minh email này.`
              : `Cần xác minh ${email} trước khi tạo workspace.`}
          </Text>
          <Text variant="small" muted className="block">
            {purpose === 'invitations'
              ? 'Chỉ email đã xác minh mới thấy lời mời, để không ai đọc được lời mời gửi cho người khác.'
              : 'Workspace là nơi người khác được mời vào bằng email, nên chỉ tài khoản có email đã xác minh mới tạo được.'}
          </Text>
        </div>
      </div>

      {failure && <Notice>{failure.message}</Notice>}

      {session ? (
        <CodeEntry
          mode="verify-email"
          identifier={{ kind: 'email', value: email }}
          session={session}
          onVerified={onVerified}
        />
      ) : (
        <div className="flex flex-wrap gap-2">
          {byGoogle && (
            <Button variant="secondary" size="md" className="h-11 sm:h-9" loading={google.isPending} onClick={withGoogle}>
              <GoogleLogo />
              Xác minh bằng Google
            </Button>
          )}
          {byCode && (
            <Button variant="soft" size="md" className="h-11 sm:h-9" loading={request.isPending} onClick={sendCode}>
              Gửi mã tới {email}
            </Button>
          )}
          {!byGoogle && !byCode && (
            <Text variant="small" muted className="block">
              Hiện chưa có cách xác minh nào được bật. Liên hệ quản trị hệ thống để bật đăng nhập Google hoặc gửi mã qua email.
            </Text>
          )}
        </div>
      )}
    </div>
  )
}
