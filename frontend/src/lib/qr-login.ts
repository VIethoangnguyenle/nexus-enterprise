/**
 * Sign-in QR codes are plain links to this app (`/qr/<token>`), so a phone's own
 * camera opens them in the browser and the in-app scanner reads the same thing.
 */

/** Tokens are 32 random bytes in base64url: 43 characters. Anything else is not ours. */
const TOKEN = /^[A-Za-z0-9_-]{43}$/

/** The link a sign-in QR code carries. */
export function qrLoginUrl(token: string, origin: string = window.location.origin): string {
  return `${origin}/qr/${token}`
}

/** Whether a route parameter can be a sign-in token at all. */
export function isQrToken(raw: unknown): raw is string {
  return typeof raw === 'string' && TOKEN.test(raw)
}

/**
 * The sign-in token in what a scanner read, or null when it is not a Nexus
 * sign-in code: another site, another path, a `javascript:` link, plain text.
 */
export function qrTokenFromScan(value: string, origin: string = window.location.origin): string | null {
  let url: URL
  try {
    url = new URL(value.trim())
  } catch {
    return null
  }
  if (url.origin !== origin || url.search || url.hash) return null
  const m = /^\/qr\/([^/]+)$/.exec(url.pathname)
  return m && isQrToken(m[1]) ? m[1] : null
}

/** "Chrome trên Windows", for the phone to say which computer is asking. */
export function describeDevice(browser: string, os: string): string {
  if (browser && os) return `${browser} trên ${os}`
  if (browser) return browser
  if (os) return `Trình duyệt trên ${os}`
  return 'Trình duyệt không rõ'
}

const RETURN_KEY = 'nexus.qr.return'

/**
 * A phone that opened a code while signed out signs in first. The code is kept
 * for this tab only, so the sign-in that follows can come back to it.
 */
export function rememberQrReturn(token: string): void {
  try {
    sessionStorage.setItem(RETURN_KEY, token)
  } catch {
    // Without storage the person scans again after signing in.
  }
}

/** The code to return to after signing in, once; null when there is none. */
export function takeQrReturn(): string | null {
  try {
    const token = sessionStorage.getItem(RETURN_KEY)
    sessionStorage.removeItem(RETURN_KEY)
    return isQrToken(token) ? token : null
  } catch {
    return null
  }
}
