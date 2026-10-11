/** The part of window.visualViewport this module reads. */
export interface VisualViewportLike {
  height: number
  addEventListener(type: 'resize', listener: () => void): void
  removeEventListener(type: 'resize', listener: () => void): void
}

/**
 * Keeps `--app-height` on <html> equal to the visible viewport. On phones the
 * on-screen keyboard shrinks the visual viewport but not `100dvh` (iOS Safari
 * never resizes the layout for it), so a shell sized with `h-app` keeps the
 * message composer above the keyboard. Returns the function that stops it.
 */
export function trackAppHeight(vv: VisualViewportLike | null | undefined): () => void {
  const root = document.documentElement
  if (!vv) return () => {}
  const apply = () => root.style.setProperty('--app-height', `${Math.round(vv.height)}px`)
  apply()
  vv.addEventListener('resize', apply)
  return () => {
    vv.removeEventListener('resize', apply)
    root.style.removeProperty('--app-height')
  }
}
