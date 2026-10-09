import { useEffect, useRef, useState } from 'react'

/** Total life of a realtime wash (DESIGN.md §7, --duration-realtime). */
export const REALTIME_MS = 2400
/** Coalescing window: three or more changes inside it become one summary line. */
export const BURST_WINDOW_MS = 2000
export const BURST_MIN = 3

export type ArrivalSource = 'other' | 'mine'

export interface Arrival {
  source: ArrivalSource
  /** Author key of the change (user id). Used for colour; never rendered. */
  author?: string
  at: number
}

export interface Burst {
  count: number
  /** Author keys, most recent first, de-duplicated. */
  authors: string[]
}

interface Options<T> {
  /** Stable identity of an item *version*: a new key means a new change. */
  keyOf: (item: T) => string
  /** Who caused this version. */
  authorOf: (item: T) => string | undefined
  /** The current user; their own changes get no wash. */
  me: string | undefined
  /**
   * False until the list has loaded. Everything present when it turns true is
   * history, not news, and is never washed.
   */
  ready: boolean
  /** Changing this (e.g. the channel id) forgets what has been seen. */
  scope?: string
}

/**
 * Tells realtime arrivals apart from history so the UI can attribute them
 * (DESIGN.md §7): a change by someone else is washed in their hue for 2.4s; a
 * change of your own only gets the insert animation. Three or more changes
 * from others inside 2s also produce a `burst` for a single summary line.
 */
export function useArrivals<T>(items: T[], { keyOf, authorOf, me, ready, scope }: Options<T>) {
  const seen = useRef<Set<string> | null>(null)
  const seenScope = useRef<string | undefined>(scope)
  const [fresh, setFresh] = useState<Map<string, Arrival>>(() => new Map())
  const [burst, setBurst] = useState<Burst | null>(null)
  const timers = useRef<ReturnType<typeof setTimeout>[]>([])

  useEffect(() => () => timers.current.forEach(clearTimeout), [])

  useEffect(() => {
    if (seenScope.current !== scope) {
      seenScope.current = scope
      seen.current = null
      setFresh(new Map())
      setBurst(null)
    }
    if (!ready) return
    if (seen.current === null) {
      seen.current = new Set(items.map(keyOf))
      return
    }
    const now = Date.now()
    const added: [string, Arrival][] = []
    for (const item of items) {
      const k = keyOf(item)
      if (seen.current.has(k)) continue
      seen.current.add(k)
      const author = authorOf(item)
      added.push([k, { source: author && author === me ? 'mine' : 'other', author, at: now }])
    }
    if (added.length === 0) return

    setFresh((prev) => {
      const next = new Map(prev)
      for (const [k, a] of added) next.set(k, a)
      // Coalesce: count other-people arrivals still inside the window.
      const recent = [...next.values()].filter((a) => a.source === 'other' && now - a.at <= BURST_WINDOW_MS)
      if (recent.length >= BURST_MIN) {
        const authors = [...new Set(recent.sort((a, b) => b.at - a.at).map((a) => a.author ?? ''))].filter(Boolean)
        setBurst({ count: recent.length, authors })
      }
      return next
    })

    const t = setTimeout(() => {
      setFresh((prev) => {
        const next = new Map(prev)
        for (const [k] of added) next.delete(k)
        return next
      })
      setBurst((b) => (b && Date.now() - now >= REALTIME_MS - 50 ? null : b))
    }, REALTIME_MS)
    timers.current.push(t)
    // keyOf/authorOf are expected to be pure; re-running on their identity would
    // re-seed nothing but cost a pass, so items/ready/scope/me drive this.
  }, [items, ready, scope, me])

  return { fresh, burst }
}
