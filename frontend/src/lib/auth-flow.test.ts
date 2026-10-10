import { describe, it, expect } from 'vitest'
import { ApiError } from '../api/client'
import {
  afterSignIn,
  describeAuthError,
  expiryLabel,
  formatCountdown,
  googleErrorMessage,
  identifierError,
  parseIdentifier,
  displayIdentifier,
  roleLabel,
  workspaceRowDetail,
  verifyErrorMessage,
} from './auth-flow'

describe('parseIdentifier', () => {
  it('accepts an email, trims it, and keeps its case for the server to normalise', () => {
    expect(parseIdentifier('  hoa.le@novapay.vn ')).toEqual({ kind: 'email', value: 'hoa.le@novapay.vn' })
    expect(parseIdentifier('Hoa.Le@NovaPay.vn')).toEqual({ kind: 'email', value: 'Hoa.Le@NovaPay.vn' })
  })

  it.each([
    ['0912345678', '0912345678'],
    ['0912 345 678', '0912345678'],
    ['0912-345-678', '0912345678'],
    ['+84912345678', '+84912345678'],
    ['84912345678', '84912345678'],
  ])('accepts the Vietnamese phone number %s', (raw, value) => {
    expect(parseIdentifier(raw)).toEqual({ kind: 'phone', value })
  })

  it.each(['', '   ', 'hoa.le@novapay', '@novapay.vn', 'hoa le@novapay.vn', '12345', '0212345678', '091234567', 'abc'])(
    'refuses %j',
    (raw) => {
      expect(parseIdentifier(raw)).toBeNull()
    },
  )
})

describe('identifierError', () => {
  it('says nothing about an empty field (validation happens on blur, not before typing)', () => {
    expect(identifierError('')).toBeNull()
    expect(identifierError('   ')).toBeNull()
  })

  it('asks for a value when one is required and the field is empty', () => {
    expect(identifierError('', true)).toMatch(/email hoặc số điện thoại/i)
    expect(identifierError('   ', true)).toMatch(/hoa\.le@novapay\.vn/)
  })

  it('says nothing about a valid value', () => {
    expect(identifierError('hoa.le@novapay.vn')).toBeNull()
    expect(identifierError('0912 345 678')).toBeNull()
  })

  it('explains an email with a missing tail, with an example, never "invalid input"', () => {
    const msg = identifierError('hoa.le@novapay')
    expect(msg).toMatch(/email/i)
    expect(msg).toMatch(/hoa\.le@novapay\.vn/)
    expect(msg).not.toMatch(/invalid/i)
  })

  it('explains a number that is not a Vietnamese mobile number', () => {
    expect(identifierError('12345')).toMatch(/số điện thoại/i)
  })

  it('asks for either when it is neither', () => {
    expect(identifierError('abc')).toMatch(/email hoặc số điện thoại/i)
  })
})

describe('displayIdentifier', () => {
  it('shows an address in full and masks the middle of a number', () => {
    expect(displayIdentifier({ kind: 'email', value: 'hoa.le@novapay.vn' })).toBe('hoa.le@novapay.vn')
    expect(displayIdentifier({ kind: 'phone', value: '0912345678' })).toBe('0912 ••• 678')
    expect(displayIdentifier({ kind: 'phone', value: '+84912345678' })).toBe('0912 ••• 678')
  })
})

describe('formatCountdown', () => {
  it.each([
    [42, '0:42'],
    [60, '1:00'],
    [5, '0:05'],
    [0, '0:00'],
    [-3, '0:00'],
  ])('%i s -> %s', (s, out) => expect(formatCountdown(s)).toBe(out))
})

describe('afterSignIn', () => {
  it('asks a person who owes a profile for it, and sends everyone else to workspace selection', () => {
    expect(afterSignIn(true)).toBe('/register')
    expect(afterSignIn(false)).toBe('/workspace-select')
  })
})

describe('roleLabel', () => {
  it('names roles in Vietnamese and never shows an unknown code', () => {
    expect(roleLabel('owner')).toBe('Chủ sở hữu')
    expect(roleLabel('admin')).toBe('Quản trị viên')
    expect(roleLabel('member')).toBe('Thành viên')
    expect(roleLabel('something_new')).toBe('Thành viên')
    expect(roleLabel('')).toBe('Thành viên')
  })
})

describe('workspaceRowDetail', () => {
  it('lists headcount and the person’s own role', () => {
    expect(workspaceRowDetail({ member_count: 64, role: 'member', domain: '' })).toBe('64 thành viên · Thành viên')
    expect(workspaceRowDetail({ member_count: 1, role: 'owner', domain: '' })).toBe('1 thành viên · Chủ sở hữu')
  })
  it('names the company first when the workspace claimed a domain', () => {
    expect(workspaceRowDetail({ member_count: 64, role: 'member', domain: 'novapay.vn' })).toBe('novapay.vn · 64 thành viên · Thành viên')
  })
  it('says nothing it does not know', () => {
    expect(workspaceRowDetail(undefined)).toBe('')
  })
})

describe('expiryLabel', () => {
  const now = new Date('2026-10-10T08:00:00')
  it.each([
    ['2026-10-15T08:00:00', 'Hết hạn sau 5 ngày'],
    ['2026-10-11T09:00:00', 'Hết hạn sau 1 ngày'],
    ['2026-10-10T20:00:00', 'Hết hạn sau 12 giờ'],
    ['2026-10-10T08:30:00', 'Hết hạn sau 30 phút'],
    ['2026-10-10T08:00:20', 'Sắp hết hạn'],
    ['2026-10-09T08:00:00', 'Đã hết hạn'],
  ])('%s -> %s', (when, out) => expect(expiryLabel(when, now)).toBe(out))

  it('shows nothing for an unreadable date', () => {
    expect(expiryLabel('', now)).toBe('')
    expect(expiryLabel('soon', now)).toBe('')
  })
})

describe('googleErrorMessage', () => {
  it('has a sentence for each code the server sends and never prints the code', () => {
    for (const code of [
      'google_cancelled', 'google_state', 'google_failed', 'google_unverified',
      'google_conflict', 'google_unavailable', 'google_session', 'google_error',
    ]) {
      const msg = googleErrorMessage(code)
      expect(msg, code).toBeTruthy()
      expect(msg).not.toContain(code)
      expect(msg).not.toMatch(/[a-z]+_[a-z]+/)
    }
  })
  it('falls back for an unknown code and is empty without one', () => {
    expect(googleErrorMessage('brand_new')).toMatch(/Google/)
    expect(googleErrorMessage('brand_new')).not.toContain('brand_new')
    expect(googleErrorMessage(null)).toBeNull()
    expect(googleErrorMessage('')).toBeNull()
  })
})

const api = (status: number, body: unknown = {}) => new ApiError('server words', status, body)

describe('describeAuthError', () => {
  it('never echoes the server’s English text or a code', () => {
    const d = describeAuthError(api(500, { message: 'dial tcp 10.0.0.5', code: 'internal' }), 'request-code')
    expect(d.message).not.toContain('dial')
    expect(d.message).not.toContain('internal')
    expect(d.message).not.toContain('server words')
  })

  describe('request-code', () => {
    it('429 says when to come back, in minutes', () => {
      const d = describeAuthError(api(429, { code: 'otp_rate_limited', retry_after_seconds: 700 }), 'request-code')
      expect(d.kind).toBe('rate-limited')
      expect(d.message).toContain('12 phút')
      expect(d.retryAfterSeconds).toBe(700)
    })
    it('429 without a time still says to wait', () => {
      const d = describeAuthError(api(429, { code: 'otp_rate_limited' }), 'request-code')
      expect(d.kind).toBe('rate-limited')
      expect(d.message).toMatch(/thử lại sau/i)
    })
    it('503 says sign-in is paused and offers Google', () => {
      const d = describeAuthError(api(503, { code: 'otp_unavailable' }), 'request-code')
      expect(d.kind).toBe('unavailable')
      expect(d.message).toMatch(/tạm ngưng/i)
    })
    it('400 explains the field with an example', () => {
      const d = describeAuthError(api(400, { code: 'invalid_input' }), 'request-code')
      expect(d.message).toMatch(/hoa\.le@novapay\.vn/)
    })
    it('a request that never reached the server says to check the network', () => {
      const d = describeAuthError(new TypeError('Failed to fetch'), 'request-code')
      expect(d.kind).toBe('offline')
      expect(d.message).toMatch(/mạng/i)
    })
  })

  describe('verify-code', () => {
    it('a wrong code says how many tries are left', () => {
      const d = describeAuthError(api(401, { code: 'otp_invalid', attempts_left: 3 }), 'verify-code')
      expect(d.kind).toBe('wrong-code')
      expect(d.attemptsLeft).toBe(3)
      expect(d.message).toContain('Còn 3 lần thử')
    })
    it('says "1 lần" for the last try', () => {
      expect(describeAuthError(api(401, { code: 'otp_invalid', attempts_left: 1 }), 'verify-code').message).toContain('Còn 1 lần thử')
    })
    it('the wrong code that uses up the tries asks for a new code', () => {
      const d = describeAuthError(api(401, { code: 'otp_invalid', attempts_left: 0 }), 'verify-code')
      expect(d.kind).toBe('locked')
      expect(d.message).toMatch(/Gửi lại/)
    })
    it('a wrong code without a count is still a wrong code', () => {
      const d = describeAuthError(api(401, { code: 'otp_invalid' }), 'verify-code')
      expect(d.kind).toBe('wrong-code')
      expect(d.message).toMatch(/chưa đúng/i)
    })
    it('an expired code points at Gửi lại', () => {
      const d = describeAuthError(api(401, { code: 'otp_expired' }), 'verify-code')
      expect(d.kind).toBe('expired')
      expect(d.message).toMatch(/hết hạn/i)
      expect(d.message).toMatch(/Gửi lại/)
    })
    it('too many wrong codes says the code is cancelled and offers Google', () => {
      const d = describeAuthError(api(429, { code: 'otp_too_many_attempts' }), 'verify-code')
      expect(d.kind).toBe('locked')
      expect(d.message).toMatch(/quá 5 lần/)
      expect(d.message).toMatch(/Google/)
    })
    it('a 503 while verifying is "paused", not "wrong code"', () => {
      expect(describeAuthError(api(503, { code: 'otp_unavailable' }), 'verify-code').kind).toBe('unavailable')
    })
  })

  describe('other screens', () => {
    it('creating a workspace: bad name, rate limit, server fault', () => {
      expect(describeAuthError(api(400), 'create-workspace').message).toMatch(/80 ký tự/)
      const rl = describeAuthError(api(429, { code: 'rate_limited', retry_after_seconds: 3000 }), 'create-workspace')
      expect(rl.kind).toBe('rate-limited')
      expect(rl.message).toContain('50 phút')
      expect(describeAuthError(api(500), 'create-workspace').message).toMatch(/Máy chủ/)
    })
    it('creating a workspace before the address is proved says to prove it', () => {
      const d = describeAuthError(api(403, { code: 'email_unverified' }), 'create-workspace')
      expect(d.message).toMatch(/xác minh email/i)
      expect(d.kind).toBe('other')
    })
    it('the profile: bad name', () => {
      expect(describeAuthError(api(400), 'profile').message).toMatch(/80 ký tự/)
    })
    it('an invitation: gone, already answered, expired, inviter lost the right', () => {
      expect(describeAuthError(api(404), 'accept-invitation').message).toMatch(/không còn/i)
      expect(describeAuthError(api(409), 'accept-invitation').message).toMatch(/đã được xử lý/i)
      expect(describeAuthError(api(400), 'accept-invitation').message).toMatch(/hết hạn/i)
      expect(describeAuthError(api(403), 'accept-invitation').message).toMatch(/quyền/i)
    })
    it('entering a workspace the person no longer belongs to', () => {
      expect(describeAuthError(api(403), 'enter-workspace').message).toMatch(/không còn là thành viên/i)
    })
    it('a list that did not load', () => {
      expect(describeAuthError(api(502), 'load').message).toMatch(/Không tải được/)
    })
  })
})

describe('describeAuthError: proving the address', () => {
  it('no way to send a code says to use Google, and is not a wrong code', () => {
    const d = describeAuthError(api(503, { code: 'verification_unavailable' }), 'verify-email')
    expect(d.kind).toBe('unavailable')
    expect(d.message).toMatch(/Google/)
    expect(d.message).not.toMatch(/tạm ngưng/)
  })
  it('an address that is already proved', () => {
    expect(describeAuthError(api(409, { code: 'already_verified' }), 'verify-email').message).toMatch(/đã được xác minh/i)
  })
  it('asking for too many codes says when to come back', () => {
    const d = describeAuthError(api(429, { code: 'otp_rate_limited', retry_after_seconds: 600 }), 'verify-email')
    expect(d.kind).toBe('rate-limited')
    expect(d.message).toContain('10 phút')
  })
  it('a fault and a lost connection read like the other screens', () => {
    expect(describeAuthError(api(500), 'verify-email').message).toMatch(/Máy chủ/)
    expect(describeAuthError(new TypeError('x'), 'verify-email').kind).toBe('offline')
  })
})

describe('verifyErrorMessage', () => {
  it('has a sentence for each way proving the address with Google can end, and never prints the code', () => {
    for (const code of ['google_cancelled', 'google_mismatch', 'google_unverified', 'google_failed', 'google_error']) {
      const msg = verifyErrorMessage(code, 'hoa.le@novapay.vn')
      expect(msg, code).toBeTruthy()
      expect(msg).not.toContain(code)
    }
  })
  it('tells a person who picked the wrong Google account which one to pick', () => {
    expect(verifyErrorMessage('google_mismatch', 'hoa.le@novapay.vn')).toContain('hoa.le@novapay.vn')
  })
  it('falls back for a code it does not know and is empty without one', () => {
    expect(verifyErrorMessage('brand_new', 'a@b.vn')).toMatch(/xác minh/)
    expect(verifyErrorMessage('brand_new', 'a@b.vn')).not.toContain('brand_new')
    expect(verifyErrorMessage(undefined, 'a@b.vn')).toBeNull()
  })
})
