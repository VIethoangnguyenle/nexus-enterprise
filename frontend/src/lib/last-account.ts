/**
 * Who last signed in on this device, so the sign-in page can greet them over
 * the QR code (design/mockups/qr-login.html §1 note 6). Only what the screen
 * shows is kept: a display name and a picture, never an id or an address. It
 * outlives sign-out on purpose (that is when the greeting is useful) and is
 * dropped by "Dùng tài khoản khác". A greeting, not a restriction: the code
 * signs in whoever scans it.
 */
const KEY = 'nexus.login.last'

export interface LastAccount {
  name: string
  avatarUrl: string
}

export function rememberLastAccount(user: { display_name?: string; username?: string; avatar_url?: string }): void {
  const name = (user.display_name || user.username || '').trim()
  if (!name) return
  try {
    localStorage.setItem(KEY, JSON.stringify({ name, avatarUrl: user.avatar_url ?? '' }))
  } catch {
    // A browser that keeps nothing simply shows no greeting.
  }
}

export function lastAccount(): LastAccount | null {
  try {
    const v = JSON.parse(localStorage.getItem(KEY) ?? 'null') as unknown
    if (v && typeof v === 'object' && typeof (v as LastAccount).name === 'string' && (v as LastAccount).name
      && typeof (v as LastAccount).avatarUrl === 'string') {
      return { name: (v as LastAccount).name, avatarUrl: (v as LastAccount).avatarUrl }
    }
  } catch {
    // Unreadable: no greeting.
  }
  return null
}

export function forgetLastAccount(): void {
  try {
    localStorage.removeItem(KEY)
  } catch {
    // Nothing to forget.
  }
}
