import { beforeEach, describe, expect, it } from 'vitest'
import { describeDevice, isQrToken, qrLoginUrl, qrTokenFromScan, rememberQrReturn, takeQrReturn } from './qr-login'

const ORIGIN = 'https://nexus.zaneng.xyz'
const TOKEN = 'Abc_def-0123456789abcdefghijklmnopqrstuvwxy'

describe('sign-in QR links', () => {
  it('is a link to this app that reads back to its token', () => {
    expect(TOKEN).toHaveLength(43)
    const url = qrLoginUrl(TOKEN, ORIGIN)
    expect(url).toBe(`${ORIGIN}/qr/${TOKEN}`)
    expect(qrTokenFromScan(url, ORIGIN)).toBe(TOKEN)
    expect(qrTokenFromScan(`  ${url}\n`, ORIGIN)).toBe(TOKEN)
  })

  it.each([
    ['another site', `https://evil.example/qr/${TOKEN}`],
    ['a look-alike host', `https://nexus.zaneng.xyz.evil.example/qr/${TOKEN}`],
    ['plain http to the same host', `http://nexus.zaneng.xyz/qr/${TOKEN}`],
    ['another path', `${ORIGIN}/channels/${TOKEN}`],
    ['a nested path', `${ORIGIN}/qr/${TOKEN}/x`],
    ['a query', `${ORIGIN}/qr/${TOKEN}?next=//evil`],
    ['a fragment', `${ORIGIN}/qr/${TOKEN}#x`],
    ['a short token', `${ORIGIN}/qr/abc`],
    ['a token with other characters', `${ORIGIN}/qr/${TOKEN.slice(0, 42)}.`],
    ['a script link', `javascript:alert(1)//${ORIGIN}/qr/${TOKEN}`],
    ['a protocol-relative link', `//evil.example/qr/${TOKEN}`],
    ['plain text', 'WIFI:S:office;T:WPA;P:secret;;'],
    ['nothing', ''],
  ])('refuses %s', (_, value) => {
    expect(qrTokenFromScan(value, ORIGIN)).toBeNull()
  })

  it('checks a route parameter is shaped like a token', () => {
    expect(isQrToken(TOKEN)).toBe(true)
    expect(isQrToken('../../etc')).toBe(false)
    expect(isQrToken(undefined)).toBe(false)
  })
})

describe('which computer is asking', () => {
  it.each([
    ['Chrome', 'Windows', 'Chrome trên Windows'],
    ['Safari', '', 'Safari'],
    ['', 'Linux', 'Trình duyệt trên Linux'],
    ['', '', 'Trình duyệt không rõ'],
  ])('%s / %s → %s', (b, o, want) => {
    expect(describeDevice(b, o)).toBe(want)
  })
})

describe('coming back to a code after signing in', () => {
  beforeEach(() => sessionStorage.clear())

  it('returns the remembered code once', () => {
    rememberQrReturn(TOKEN)
    expect(takeQrReturn()).toBe(TOKEN)
    expect(takeQrReturn()).toBeNull()
  })

  it('ignores anything stored that is not a token', () => {
    sessionStorage.setItem('nexus.qr.return', '//evil.example')
    expect(takeQrReturn()).toBeNull()
  })
})
