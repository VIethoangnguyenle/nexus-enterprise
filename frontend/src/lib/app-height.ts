/** The part of window.visualViewport this module reads. */
export interface VisualViewportLike {
  height: number
  offsetTop: number
  addEventListener(type: 'resize' | 'scroll', listener: () => void): void
  removeEventListener(type: 'resize' | 'scroll', listener: () => void): void
}

/**
 * Keeps `--app-height` and `--app-shift` on <html> equal to the visible viewport.
 * On phones the on-screen keyboard shrinks the visual viewport but not
 * `100dvh` (iOS Safari never resizes the layout for it) and iOS also scrolls
 * the page so the focused field stays visible, which moves the visual viewport
 * down by `offsetTop`. A shell sized with `h-app` (height and a translate by
 * the offset) therefore stays exactly on screen, with the message composer
 * above the keyboard. Returns the function that stops it.
 */
export function trackAppHeight(vv: VisualViewportLike | null | undefined): () => void {
  const root = document.documentElement
  if (!vv) return () => {}
  const apply = () => {
    root.style.setProperty('--app-height', `${Math.round(vv.height)}px`)
    // Set only while the page is scrolled: any translate makes the shell the
    // containing block of its fixed-position descendants (tab bar, popovers).
    const top = Math.round(vv.offsetTop)
    if (top > 0) root.style.setProperty('--app-shift', `0 ${top}px`)
    else root.style.removeProperty('--app-shift')
  }
  apply()
  vv.addEventListener('resize', apply)
  vv.addEventListener('scroll', apply)
  return () => {
    vv.removeEventListener('resize', apply)
    vv.removeEventListener('scroll', apply)
    root.style.removeProperty('--app-height')
    root.style.removeProperty('--app-shift')
  }
}
