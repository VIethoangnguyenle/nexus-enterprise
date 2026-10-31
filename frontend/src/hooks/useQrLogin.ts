import { useCallback, useEffect, useRef, useState } from 'react'
import { useMutation, useQuery } from '@tanstack/react-query'
import { qrApi, sessionUser, type QrStatusResponse } from '../api/auth'
import { ApiError } from '../api/client'
import { useAuthStore } from '../stores/auth.store'
import { keys } from './keys'

/** How often the computer asks about its code while it is showing. */
export const QR_POLL_MS = 2000
/** A fresh code is fetched this long before the old one runs out. */
export const QR_RENEW_EARLY_MS = 5000
/** Codes renewed in a row with nobody scanning before the page stops and waits for a click. */
export const QR_MAX_RENEWALS = 5
/** Failed polls in a row before the screen says the connection is gone. */
export const QR_FAILURES_BEFORE_NOTICE = 3

export type QrPhase =
  | 'loading' | 'pending' | 'scanned' | 'confirm' | 'approving' | 'declined' | 'expired' | 'failed'

export interface QrLogin {
  phase: QrPhase
  token: string | null
  expiresAt: number | null
  scanner: NonNullable<QrStatusResponse['scanner']> | null
  /** Polls are failing; the code is still shown and polling goes on. */
  reconnecting: boolean
  /** A new code, after an expiry, a decline or a failure. */
  renew: () => void
  /** "Tiếp tục với tên …": collects the session the phone approved. */
  confirm: () => void
  /** "Không phải tôi": throws the approved code away and shows a new one. */
  notMe: () => void
}

type Ended = 'expired' | 'declined' | 'failed'

/**
 * The computer's side of sign-in by QR code (design/mockups/qr-login.html §1, §2).
 * Fetches a code, polls it every two seconds while the tab is visible, swaps it
 * for a fresh one before it runs out (up to five times with nobody scanning),
 * and, once the phone approves, asks the person at the computer to confirm who
 * they are signing in as before collecting the session (design §1 note 5).
 * `onSignedIn` gets whether the profile step is still owed.
 */
export function useQrLogin(onSignedIn: (needsProfile: boolean) => void): QrLogin {
  const login = useAuthStore((s) => s.login)
  const [ticket, setTicket] = useState<{ token: string; secret: string; expiresAt: number } | null>(null)
  const [ended, setEnded] = useState<Ended | null>(null)
  const [reconnecting, setReconnecting] = useState(false)
  const renewals = useRef(0)
  const failures = useRef(0)
  const onSignedInRef = useRef(onSignedIn)
  onSignedInRef.current = onSignedIn

  const create = useMutation({
    meta: { silentError: true },
    mutationFn: () => qrApi.create(),
    onSuccess: (r) => {
      failures.current = 0
      setReconnecting(false)
      setEnded(null)
      setTicket({ token: r.token, secret: r.poll_secret, expiresAt: Date.now() + r.expires_in * 1000 })
    },
    onError: () => setEnded('failed'),
  })
  const createRef = useRef(create.mutate)
  createRef.current = create.mutate

  // One code on arrival, even when effects run twice in development.
  const started = useRef(false)
  useEffect(() => {
    if (started.current) return
    started.current = true
    createRef.current()
  }, [])

  const token = ticket?.token ?? null
  const status = useQuery({
    queryKey: keys.auth.qrStatus(token ?? ''),
    enabled: !!token && !ended,
    meta: { silentError: true },
    retry: false,
    gcTime: 0,
    refetchIntervalInBackground: false,
    refetchInterval: (q) => {
      const s = q.state.data?.status
      // An approval stops the polling: the person at the computer decides next.
      return s === undefined || s === 'pending' || s === 'scanned' ? QR_POLL_MS : false
    },
    queryFn: async (): Promise<QrStatusResponse | { status: 'expired' }> => {
      try {
        const r = await qrApi.status(ticket!.token, ticket!.secret)
        failures.current = 0
        setReconnecting(false)
        return r
      } catch (err) {
        // The server's one answer for a code that can no longer be used.
        if (err instanceof ApiError && err.status === 410) return { status: 'expired' }
        failures.current += 1
        if (failures.current >= QR_FAILURES_BEFORE_NOTICE) setReconnecting(true)
        throw err
      }
    },
  })
  const data = status.data

  // What the server said decides what happens next.
  useEffect(() => {
    if (!data) return
    if (data.status === 'expired') setEnded('expired')
    else if (data.status === 'declined') setEnded('declined')
  }, [data])

  const claim = useMutation({
    meta: { silentError: true },
    mutationFn: (t: { token: string; secret: string }) => qrApi.claim(t.token, t.secret),
    onSuccess: (r) => {
      login(r.access_token, sessionUser(r.user))
      onSignedInRef.current(r.needs_profile === true)
    },
    // Too late (the code ran out) or gone: start over with a new code.
    onError: (err) => setEnded(err instanceof ApiError && err.status === 410 ? 'expired' : 'failed'),
  })
  // The code is passed in: `renew` replaces the ticket before the request leaves.
  const cancel = useMutation({
    meta: { silentError: true },
    mutationFn: (t: { token: string; secret: string }) => qrApi.cancel(t.token, t.secret),
  })

  // Before an unscanned code runs out, swap it, a limited number of times.
  const waiting = !ended && !!ticket && (!data || data.status === 'pending')
  useEffect(() => {
    if (!waiting || !ticket) return
    const id = window.setTimeout(() => {
      if (renewals.current >= QR_MAX_RENEWALS) {
        setEnded('expired')
        return
      }
      renewals.current += 1
      createRef.current()
    }, Math.max(0, ticket.expiresAt - Date.now() - QR_RENEW_EARLY_MS))
    return () => window.clearTimeout(id)
  }, [waiting, ticket])

  const renew = useCallback(() => {
    renewals.current = 0
    setTicket(null)
    createRef.current()
  }, [])

  const confirm = () => { if (ticket && !claim.isPending) claim.mutate({ token: ticket.token, secret: ticket.secret }) }
  // The code goes whether or not the server hears it: a new one replaces it at once.
  const notMe = () => {
    if (ticket) cancel.mutate({ token: ticket.token, secret: ticket.secret })
    renew()
  }

  let phase: QrPhase
  if (ended) phase = ended
  else if (!ticket) phase = 'loading'
  else if (claim.isPending || claim.isSuccess) phase = 'approving'
  else if (data?.status === 'approved') phase = 'confirm'
  else if (data?.status === 'scanned') phase = 'scanned'
  else phase = 'pending'

  return {
    phase,
    token,
    expiresAt: ticket?.expiresAt ?? null,
    scanner: (data && 'scanner' in data && data.scanner) || null,
    reconnecting,
    renew,
    confirm,
    notMe,
  }
}
