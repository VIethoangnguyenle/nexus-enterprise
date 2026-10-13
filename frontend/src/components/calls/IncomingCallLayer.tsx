import { useEffect, useRef, useCallback } from 'react'
import { useIncomingCallStore } from '../../stores/incoming-call.store'
import { usePhone } from '../../hooks/usePhone'
import { createRingtone, type Ringtone } from '../../lib/ringtone'
import { IncomingCallToast } from './IncomingCallToast'
import { IncomingCallScreen } from './IncomingCallScreen'

/**
 * Reads the incoming-call store and renders the appropriate UI:
 * - Desktop → floating IncomingCallToast (top-right)
 * - Phone → full-screen IncomingCallScreen
 *
 * Starts/stops the WebAudio ringtone when a `ringing` call is present.
 * Esc silences the ringtone without declining the call.
 *
 * **Do NOT mount this directly**; the coordinator in the root layout handles it.
 */
export function IncomingCallLayer() {
  const incoming = useIncomingCallStore((s) => s.incoming)
  const clearCall = useIncomingCallStore((s) => s.clear)
  const isPhone = usePhone()
  const ringtoneRef = useRef<Ringtone | null>(null)

  const onClear = useCallback(
    (callId: string) => clearCall(callId),
    [clearCall],
  )

  // Ringtone lifecycle
  useEffect(() => {
    if (incoming?.mode === 'ringing') {
      if (!ringtoneRef.current) ringtoneRef.current = createRingtone()
      ringtoneRef.current.start()
    } else {
      ringtoneRef.current?.stop()
    }
    return () => {
      ringtoneRef.current?.stop()
    }
  }, [incoming?.mode, incoming?.callId])

  // Esc silences the ring (does not decline)
  useEffect(() => {
    if (incoming?.mode !== 'ringing') return
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        ringtoneRef.current?.stop()
      }
    }
    document.addEventListener('keydown', handler)
    return () => document.removeEventListener('keydown', handler)
  }, [incoming?.mode])

  if (!incoming) return null

  if (isPhone) {
    return <IncomingCallScreen incoming={incoming} onClear={onClear} />
  }

  // Desktop: fixed position top-right, 16px offset per mockup §3
  return (
    <div className="fixed top-4 right-4 z-50">
      <IncomingCallToast incoming={incoming} onClear={onClear} />
    </div>
  )
}
