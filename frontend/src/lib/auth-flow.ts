import { ApiError } from '../api/client'
import { toDate } from './format'

/**
 * The words and decisions of the sign-in screens, kept apart from the markup so
 * they can be tested without rendering. Server error text is English and can
 * carry detail that is not for a person to read; every sentence a screen shows
 * is written here, chosen by status and machine code.
 */

// --- what the person typed -------------------------------------------------

export type Identifier = { kind: 'email' | 'phone'; value: string }

/** Same shape the server accepts (`emailRegex` in the auth service). */
const EMAIL = /^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$/
/** Vietnamese mobile: 0 + 3..9 + 8..9 digits, or the +84 / 84 forms. */
const PHONE = /^(0[3-9][0-9]{8,9}|\+?84[3-9][0-9]{7,8})$/

/** Separators people type inside a number; the server strips the same ones. */
const stripPhone = (raw: string) => raw.replace(/[\s.-]/g, '')

/** The email or phone number in a field, or null when it is neither. */
export function parseIdentifier(raw: string): Identifier | null {
  const trimmed = raw.trim()
  if (!trimmed) return null
  if (EMAIL.test(trimmed)) return { kind: 'email', value: trimmed }
  const digits = stripPhone(trimmed)
  if (PHONE.test(digits)) return { kind: 'phone', value: digits }
  return null
}

/**
 * Why a field is not acceptable, in words that show what to type. Null for a
 * good value, and for an empty field unless `required` (a person who has not
 * typed yet has done nothing wrong; one who submits an empty field has).
 */
export function identifierError(raw: string, required = false): string | null {
  const trimmed = raw.trim()
  if (!trimmed) return required ? 'Nhập email hoặc số điện thoại, ví dụ hoa.le@novapay.vn.' : null
  if (parseIdentifier(trimmed)) return null
  if (trimmed.includes('@')) return 'Email chưa đầy đủ, ví dụ hoa.le@novapay.vn.'
  if (/^[+\d\s.-]+$/.test(trimmed)) return 'Số điện thoại chưa đúng, ví dụ 0912 345 678.'
  return 'Nhập email hoặc số điện thoại, ví dụ hoa.le@novapay.vn.'
}

/** What the verification step says the code was sent to. */
export function displayIdentifier(id: Identifier): string {
  if (id.kind === 'email') return id.value
  const national = id.value.replace(/^\+?84/, '0')
  return `${national.slice(0, 4)} ••• ${national.slice(-3)}`
}

/** `0:42`, for the resend countdown (tabular-nums on screen keeps the digits still). */
export function formatCountdown(seconds: number): string {
  const s = Math.max(0, Math.floor(seconds))
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`
}

// --- where to go next ------------------------------------------------------

/** After a session exists: owe a profile → ask for it; otherwise choose a workspace. */
export function afterSignIn(needsProfile: boolean): '/register' | '/workspace-select' {
  return needsProfile ? '/register' : '/workspace-select'
}

// --- labels ----------------------------------------------------------------

const ROLE_LABELS: Record<string, string> = {
  owner: 'Chủ sở hữu',
  admin: 'Quản trị viên',
  member: 'Thành viên',
}

/** A role code as a person would say it. An unknown code reads as a plain member, never as the code. */
export function roleLabel(role: string): string {
  return ROLE_LABELS[role] ?? 'Thành viên'
}

/** `novapay.vn · 64 thành viên · Thành viên`; the company only when the workspace claimed one. */
export function workspaceRowDetail(w: { member_count: number; role: string; domain: string } | undefined): string {
  if (!w) return ''
  const own = `${w.member_count} thành viên · ${roleLabel(w.role)}`
  return w.domain ? `${w.domain} · ${own}` : own
}

/** "Hết hạn sau 5 ngày", the unit that fits how long is left. */
export function expiryLabel(expiresAt: string, now: Date = new Date()): string {
  const d = toDate(expiresAt)
  if (!d) return ''
  const ms = d.getTime() - now.getTime()
  if (ms <= 0) return 'Đã hết hạn'
  const minutes = Math.floor(ms / 60_000)
  if (minutes < 1) return 'Sắp hết hạn'
  if (minutes < 60) return `Hết hạn sau ${minutes} phút`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `Hết hạn sau ${hours} giờ`
  return `Hết hạn sau ${Math.floor(hours / 24)} ngày`
}

// --- Google ----------------------------------------------------------------

/**
 * One sentence per code the auth service appends as `/login?error=<code>`:
 * what happened and what to do. The code itself is never shown.
 */
const GOOGLE_ERRORS: Record<string, string> = {
  google_cancelled: 'Bạn đã huỷ đăng nhập Google. Thử lại, hoặc nhận mã qua email.',
  google_state: 'Phiên đăng nhập Google đã hết hạn hoặc được mở ở cửa sổ khác. Bấm Tiếp tục với Google để thử lại.',
  google_failed: 'Chưa xác minh được tài khoản Google của bạn. Thử lại, hoặc nhận mã qua email.',
  google_unverified: 'Google chưa xác minh email này. Xác minh email trong tài khoản Google rồi thử lại, hoặc nhận mã qua email.',
  google_conflict: 'Email Google này đã gắn với một tài khoản Google khác. Đăng nhập bằng mã qua email.',
  google_unavailable: 'Đăng nhập bằng Google đang tạm ngưng. Nhận mã qua email hoặc số điện thoại.',
  google_session: 'Chưa hoàn tất đăng nhập Google. Thử lại, hoặc nhận mã qua email.',
  google_error: 'Đăng nhập bằng Google gặp sự cố. Thử lại sau ít phút, hoặc nhận mã qua email.',
}
const GOOGLE_ERROR_FALLBACK = 'Chưa đăng nhập được bằng Google. Thử lại, hoặc nhận mã qua email.'

export function googleErrorMessage(code: string | null): string | null {
  if (!code) return null
  return GOOGLE_ERRORS[code] ?? GOOGLE_ERROR_FALLBACK
}

// --- failures --------------------------------------------------------------

export type AuthErrorContext =
  | 'request-code'
  | 'verify-email'
  | 'verify-code'
  | 'profile'
  | 'create-workspace'
  | 'accept-invitation'
  | 'decline-invitation'
  | 'enter-workspace'
  | 'load'

export type AuthErrorKind =
  | 'wrong-code'
  | 'expired'
  | 'locked'
  | 'rate-limited'
  | 'unavailable'
  | 'offline'
  | 'other'

export interface DescribedError {
  message: string
  kind: AuthErrorKind
  attemptsLeft?: number
  retryAfterSeconds?: number
}

const bodyOf = (err: ApiError): { code?: string; attempts_left?: number; retry_after_seconds?: number } =>
  err.body && typeof err.body === 'object' ? (err.body as never) : {}

/** "12 phút" for a wait the server reported in seconds; a vague phrase without one. */
function waitPhrase(seconds: number | undefined): string {
  if (!seconds || seconds <= 0) return 'ít phút nữa'
  const minutes = Math.ceil(seconds / 60)
  if (minutes < 60) return `${minutes} phút`
  return `${Math.ceil(minutes / 60)} giờ`
}

const OFFLINE = 'Không kết nối được tới máy chủ. Kiểm tra mạng rồi thử lại.'
const SERVER_FAULT: Record<AuthErrorContext, string> = {
  'verify-email': 'Máy chủ đang gặp sự cố nên chưa gửi được mã xác minh. Thử lại sau ít phút.',
  'request-code': 'Máy chủ đang gặp sự cố nên chưa gửi được mã. Thử lại sau ít phút.',
  'verify-code': 'Máy chủ đang gặp sự cố nên chưa kiểm tra được mã. Thử lại sau ít phút.',
  profile: 'Máy chủ đang gặp sự cố nên chưa lưu được tên. Thử lại sau ít phút.',
  'create-workspace': 'Máy chủ đang gặp sự cố nên chưa tạo được workspace. Thử lại sau ít phút.',
  'accept-invitation': 'Máy chủ đang gặp sự cố nên chưa tham gia được. Thử lại sau ít phút.',
  'decline-invitation': 'Máy chủ đang gặp sự cố nên chưa từ chối được. Thử lại sau ít phút.',
  'enter-workspace': 'Máy chủ đang gặp sự cố nên chưa mở được workspace. Thử lại sau ít phút.',
  load: 'Không tải được danh sách. Máy chủ đang gặp sự cố, thử lại sau ít phút.',
}

/**
 * A sentence for a failed request on a sign-in screen, and what kind of failure
 * it was so the screen can react (clear the code boxes, disable resend, show a
 * calmer band for "paused").
 */
export function describeAuthError(err: unknown, ctx: AuthErrorContext): DescribedError {
  if (!(err instanceof ApiError)) return { message: OFFLINE, kind: 'offline' }

  const { status } = err
  const body = bodyOf(err)

  if (status === 401 && ctx !== 'verify-code') {
    return { message: 'Phiên đăng nhập đã hết hạn. Đăng nhập lại để tiếp tục.', kind: 'other' }
  }

  if (ctx === 'verify-email' && body.code === 'verification_unavailable') {
    return {
      message: 'Chưa gửi được mã xác minh qua email lúc này. Dùng Google để xác minh, hoặc thử lại sau.',
      kind: 'unavailable',
    }
  }
  if (ctx === 'verify-email' && body.code === 'already_verified') {
    return { message: 'Email này đã được xác minh. Tải lại trang để tiếp tục.', kind: 'other' }
  }

  if (status === 503) {
    return {
      message: 'Đăng nhập đang tạm ngưng. Thử lại sau ít phút, hoặc dùng cách đăng nhập khác nếu có.',
      kind: 'unavailable',
    }
  }

  if (status === 429) {
    if (ctx === 'verify-code' && body.code === 'otp_too_many_attempts') {
      return {
        message: 'Nhập sai quá 5 lần nên mã này đã bị huỷ. Bấm Gửi lại để nhận mã mới, hoặc dùng Google.',
        kind: 'locked',
      }
    }
    const wait = waitPhrase(body.retry_after_seconds)
    const what =
      ctx === 'create-workspace' ? 'Bạn đã tạo quá nhiều workspace trong một giờ.'
        : ctx === 'request-code' || ctx === 'verify-email' ? 'Bạn đã xin mã quá nhiều lần.'
          : 'Bạn thao tác quá nhanh.'
    return {
      message: `${what} Thử lại sau ${wait}.`,
      kind: 'rate-limited',
      retryAfterSeconds: body.retry_after_seconds,
    }
  }

  if (status >= 500) return { message: SERVER_FAULT[ctx], kind: 'other' }

  switch (ctx) {
    case 'request-code':
      return { message: 'Email hoặc số điện thoại chưa đúng. Kiểm tra lại, ví dụ hoa.le@novapay.vn.', kind: 'other' }

    case 'verify-code': {
      if (body.code === 'otp_expired' || status === 404) {
        return { message: 'Mã đã hết hạn. Bấm Gửi lại để nhận mã mới.', kind: 'expired' }
      }
      const left = body.attempts_left
      if (left === 0) {
        return {
          message: 'Mã chưa đúng và bạn đã hết lượt thử. Bấm Gửi lại để nhận mã mới.',
          kind: 'locked',
          attemptsLeft: 0,
        }
      }
      return {
        message: left ? `Mã chưa đúng. Còn ${left} lần thử.` : 'Mã chưa đúng. Kiểm tra lại rồi nhập lần nữa.',
        kind: 'wrong-code',
        attemptsLeft: left,
      }
    }

    case 'profile':
      return { message: 'Tên chưa hợp lệ. Dùng tối đa 80 ký tự, không xuống dòng.', kind: 'other' }

    case 'create-workspace':
      if (body.code === 'email_unverified') {
        return { message: 'Cần xác minh email trước khi tạo workspace.', kind: 'other' }
      }
      return { message: 'Tên workspace chưa hợp lệ. Dùng tối đa 80 ký tự, không xuống dòng.', kind: 'other' }

    case 'verify-email':
      break

    case 'accept-invitation':
    case 'decline-invitation':
      if (status === 404) return { message: 'Lời mời này không còn nữa. Có thể người mời đã thu hồi.', kind: 'other' }
      if (status === 409) return { message: 'Lời mời này đã được xử lý trước đó.', kind: 'other' }
      if (status === 400) return { message: 'Lời mời đã hết hạn. Nhờ người mời gửi lại.', kind: 'other' }
      if (status === 403) {
        return {
          message: 'Người mời không còn quyền mời vào workspace này. Nhờ họ hoặc quản trị viên mời lại.',
          kind: 'other',
        }
      }
      break

    case 'enter-workspace':
      if (status === 403 || status === 404) {
        return { message: 'Bạn không còn là thành viên của workspace này.', kind: 'other' }
      }
      break

    case 'load':
      break
  }
  return { message: 'Chưa thực hiện được. Kiểm tra kết nối mạng rồi thử lại.', kind: 'other' }
}

/**
 * One sentence for each way "prove my address with Google" can end when Google
 * sends the person back (`?verify_error=<code>`): what happened and what to do.
 * The code is never shown. Null when there is no error.
 */
export function verifyErrorMessage(code: string | undefined, email: string): string | null {
  if (!code) return null
  switch (code) {
    case 'google_cancelled':
      return 'Bạn đã huỷ xác minh bằng Google. Thử lại khi sẵn sàng.'
    case 'google_mismatch':
      return `Tài khoản Google bạn chọn có email khác. Chọn đúng tài khoản Google của ${email}.`
    case 'google_unverified':
      return 'Google chưa xác minh email của tài khoản bạn chọn. Xác minh email trong Google rồi thử lại.'
    default:
      return 'Chưa xác minh được email bằng Google. Thử lại, hoặc nhận mã qua email nếu có.'
  }
}
