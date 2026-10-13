/**
 * What a person chose last time on this device: mic and camera on or off, and
 * which device each one uses. Kept per user in localStorage; a private-mode or
 * full store just means the choices are not remembered.
 */
export interface CallPrefs {
  micOn: boolean
  camOn: boolean
  micId?: string
  camId?: string
  speakerId?: string
}

export const DEFAULT_CALL_PREFS: CallPrefs = { micOn: true, camOn: true }

const keyOf = (userId: string | undefined) => `ngac-call-prefs:${userId ?? ''}`

const text = (v: unknown): string | undefined => (typeof v === 'string' && v ? v : undefined)

export function loadCallPrefs(userId: string | undefined): CallPrefs {
  try {
    const raw = localStorage.getItem(keyOf(userId))
    if (!raw) return { ...DEFAULT_CALL_PREFS }
    const v = JSON.parse(raw) as Record<string, unknown>
    return {
      micOn: typeof v.micOn === 'boolean' ? v.micOn : true,
      camOn: typeof v.camOn === 'boolean' ? v.camOn : true,
      micId: text(v.micId),
      camId: text(v.camId),
      speakerId: text(v.speakerId),
    }
  } catch {
    return { ...DEFAULT_CALL_PREFS }
  }
}

export function saveCallPrefs(userId: string | undefined, prefs: CallPrefs): void {
  try {
    localStorage.setItem(keyOf(userId), JSON.stringify(prefs))
  } catch {
    /* private mode or a full store: the choice is simply not remembered */
  }
}
