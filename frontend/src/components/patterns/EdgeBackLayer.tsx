import { useEffect, useLayoutEffect, useRef, useState, type ReactNode } from 'react'
import { useRouter, useRouterState } from '@tanstack/react-router'
import { motion, useTransform } from 'motion/react'
import { isEdgeBackPath, useEdgeBack, useEdgeBackAction } from '../../hooks/useEdgeBack'
import { useBottomBarLayout } from '../../hooks/usePhone'
import { isStandalone } from '../../lib/push'
import { useMotionPresets } from '../../lib/motion'

/** Screens kept as a picture for the peek; older ones are dropped. */
const MAX_SNAPSHOTS = 6
/** If the back somehow does not land (nothing to go to), the screen comes back after this. */
const LAND_TIMEOUT_MS = 800

interface Snapshot {
  node: HTMLElement
  /** Scroll offsets of the scrolled elements, by their index in document order. */
  scrolls: Array<[number, number]>
}

/** A still copy of a screen: the same DOM without ids, with its scroll positions. */
export function takeSnapshot(el: HTMLElement): Snapshot {
  const scrolls: Array<[number, number]> = []
  el.querySelectorAll('*').forEach((n, i) => {
    if (n.scrollTop > 0) scrolls.push([i, n.scrollTop])
  })
  const node = el.cloneNode(true) as HTMLElement
  node.querySelectorAll('[id]').forEach((n) => n.removeAttribute('id'))
  return { node, scrolls }
}

function showSnapshot(host: HTMLElement, snap: Snapshot) {
  host.replaceChildren(snap.node)
  if (snap.scrolls.length === 0) return
  const all = snap.node.querySelectorAll('*')
  for (const [i, top] of snap.scrolls) {
    const n = all[i]
    if (n) n.scrollTop = top
  }
}

/**
 * The phone's edge-back (DESIGN.md §6 "Vuốt", mockup §7): on a detail screen, settings page or thread, a swipe from
 * the left 20px drags the screen right with the finger while the screen before it shows underneath, dimmed and
 * coming in from -30%; past 35% of the width or a fast flick, it goes back.
 *
 * The screen underneath is a still copy of the screen that was left to come here, taken just before the step and
 * shown only while the gesture runs. Opened with nothing before it (a push notification), the layer underneath is
 * just the background, and back goes to the module's root list (`useEdgeBackAction`).
 *
 * `active` is for the installed app on a phone only: in a browser tab Safari keeps the left edge for itself and the
 * ← of the top bar is the way back. When off, the children render with nothing around them.
 */
export function EdgeBackLayer({ active, isBackPath, pathname, locationKey, onBack, children }: {
  active: boolean
  /** Screens with a way back: the swipe works there, and the screen left to reach one is kept for its peek. */
  isBackPath: (pathname: string) => boolean
  pathname: string
  /** The current location (its href); a change means a back has landed. */
  locationKey: string
  onBack: () => void
  children: ReactNode
}) {
  const { reduced } = useMotionPresets()
  const router = useRouter()
  const frontRef = useRef<HTMLDivElement>(null)
  const peekRef = useRef<HTMLDivElement>(null)
  const snapshots = useRef(new Map<string, Snapshot>())
  const enabled = active && isBackPath(pathname)
  const { x, phase, reset } = useEdgeBack({ ref: frontRef, enabled, reduced, onBack })

  // Keep a picture of the screen being left, filed under the screen being opened: that is what its back reveals.
  const backPath = useRef(isBackPath)
  useEffect(() => {
    backPath.current = isBackPath
  })
  useEffect(() => {
    if (!active) return
    return router.subscribe('onBeforeNavigate', (e) => {
      const el = frontRef.current
      if (!el || !e.pathChanged || !backPath.current(e.toLocation.pathname)) return
      const map = snapshots.current
      map.delete(e.toLocation.href)
      map.set(e.toLocation.href, takeSnapshot(el))
      while (map.size > MAX_SNAPSHOTS) map.delete(map.keys().next().value!)
    })
  }, [router, active])

  // A back landed: the new screen is in place, so the moved screen and its peek go.
  useLayoutEffect(() => {
    reset()
  }, [locationKey, reset])

  // A back that never lands gives the screen back.
  useEffect(() => {
    if (phase !== 'leaving') return
    const t = setTimeout(reset, LAND_TIMEOUT_MS)
    return () => clearTimeout(t)
  }, [phase, reset])

  useLayoutEffect(() => {
    const host = peekRef.current
    const snap = snapshots.current.get(locationKey)
    if (host && snap && !host.firstChild) showSnapshot(host, snap)
  }, [phase, locationKey])

  const peekX = useTransform(x, (v) => {
    const w = frontRef.current?.clientWidth || window.innerWidth || 1
    return `${-30 * (1 - Math.min(1, v / w))}%`
  })

  if (!active) return <>{children}</>

  const moving = phase !== 'idle'
  return (
    <div className="relative flex-1 flex flex-col min-h-0 min-w-0 overflow-hidden">
      {moving && (
        <motion.div
          ref={peekRef}
          aria-hidden="true"
          inert
          data-testid="edge-back-peek"
          style={reduced ? undefined : { x: peekX }}
          className="absolute inset-0 flex flex-col overflow-hidden bg-base brightness-[0.92] pointer-events-none"
        />
      )}
      <motion.div
        ref={frontRef}
        data-testid="edge-back-screen"
        style={{ x }}
        className={`relative flex-1 flex flex-col min-h-0 min-w-0 ${moving ? 'bg-base shadow-overlay will-change-transform' : ''}`}
      >
        {children}
      </motion.div>
    </div>
  )
}

/** The installed app, where iOS has no back gesture of its own. A browser without `matchMedia` is not one. */
function installed(): boolean {
  try {
    return isStandalone()
  } catch {
    return false
  }
}

/**
 * The edge-back of the workspace shell: on a phone in the installed app it wraps the screen in `EdgeBackLayer`,
 * elsewhere it renders the screen as it is. `pathname` is the screen's path.
 */
export function PhoneEdgeBack({ pathname, children }: { pathname: string; children: ReactNode }) {
  const phone = useBottomBarLayout()
  const [standalone] = useState(installed)
  if (!phone || !standalone) return <>{children}</>
  return <RoutedEdgeBack pathname={pathname}>{children}</RoutedEdgeBack>
}

function RoutedEdgeBack({ pathname, children }: { pathname: string; children: ReactNode }) {
  const href = useRouterState({ select: (s) => s.location.href })
  const onBack = useEdgeBackAction(pathname)
  return (
    <EdgeBackLayer active isBackPath={isEdgeBackPath} pathname={pathname} locationKey={href} onBack={onBack}>
      {children}
    </EdgeBackLayer>
  )
}
