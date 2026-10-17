/**
 * Keeps phones from zooming the page, which breaks the layout (user decision
 * 2026-10-17, DESIGN.md §9). The viewport meta asks for no scaling, but iOS
 * Safari ignores that for pinch, so its WebKit gesture events are cancelled
 * here. Double-tap zoom is stopped by `touch-action: manipulation` in
 * index.css, and inputs are 16px on touch screens so focusing one never zooms.
 *
 * The image viewer's own pinch zoom is untouched: it is built on pointer
 * events, not on these gesture events. Returns the function that stops it.
 */
export function preventPageZoom(target: Document | HTMLElement = document): () => void {
  const cancel = (e: Event) => e.preventDefault()
  const types = ['gesturestart', 'gesturechange', 'gestureend'] as const
  for (const type of types) target.addEventListener(type, cancel, { passive: false })
  return () => {
    for (const type of types) target.removeEventListener(type, cancel)
  }
}
