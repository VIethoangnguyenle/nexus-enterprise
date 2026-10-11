import { useEffect, useRef, type RefObject } from 'react'

/** How long a message opened from a link carries its accent outline. */
export const FLASH_MS = 2400
/** Pages of older messages asked for, at most, while looking for the target. */
const MAX_PAGES = 12
/** Give up looking after this long. */
const GIVE_UP_MS = 15_000
/** The stream settles its own position when it opens; the target is centred again after that. */
const SETTLE_MS = 300

const SAFE_ID = /^[\w-]{1,64}$/

export type FocusScope = 'stream' | 'panel'

/**
 * The element of a message. In the stream a message is an `article` with its
 * id; in the thread panel a reply (or the root) is a block whose text is named
 * `reply-<id>-text` / `thread-root-<id>-text`, and the block is what lights up.
 */
function locate(root: HTMLElement, id: string, scope: FocusScope): HTMLElement | null {
  if (!SAFE_ID.test(id)) return null
  if (scope === 'stream') return root.querySelector<HTMLElement>(`[role="log"] [data-message-id="${id}"]`)
  const text = root.querySelector<HTMLElement>(`[id="reply-${id}-text"], [id="thread-root-${id}-text"]`)
  return text ? (text.closest<HTMLElement>('.group') ?? text) : null
}

const reducedMotion = () =>
  typeof window !== 'undefined' && !!window.matchMedia?.('(prefers-reduced-motion: reduce)').matches

/**
 * Brings the message `target` to the middle of its scroller and outlines it in
 * the accent for 2.4 s. It waits for the message to render (a link can arrive
 * before the list does) and, in the stream, asks `loadMore` for older pages
 * while the message is not among those held; `loadMore` answers whether it
 * started one. A target it never finds is left alone.
 */
export function useFocusMessage(
  root: RefObject<HTMLElement | null>,
  target: string | undefined,
  scope: FocusScope,
  loadMore?: () => boolean,
): void {
  const more = useRef(loadMore)
  more.current = loadMore

  useEffect(() => {
    const host = root.current
    if (!host || !target) return
    let done = false
    let pages = 0
    const timers: ReturnType<typeof setTimeout>[] = []
    let flashed: HTMLElement | null = null

    const center = (el: HTMLElement) =>
      el.scrollIntoView?.({ block: 'center', behavior: reducedMotion() ? 'auto' : 'smooth' })

    const attempt = () => {
      if (done) return
      const el = locate(host, target, scope)
      if (!el) {
        if (scope === 'stream' && pages < MAX_PAGES && more.current?.()) pages += 1
        return
      }
      done = true
      observer.disconnect()
      clearTimeout(giveUp)
      flashed = el
      center(el)
      el.setAttribute('data-flash', '')
      timers.push(setTimeout(() => center(el), SETTLE_MS))
      timers.push(setTimeout(() => el.removeAttribute('data-flash'), FLASH_MS))
    }

    const observer = new MutationObserver(attempt)
    observer.observe(host, { childList: true, subtree: true })
    const giveUp = setTimeout(() => { done = true; observer.disconnect() }, GIVE_UP_MS)
    attempt()

    return () => {
      done = true
      observer.disconnect()
      clearTimeout(giveUp)
      timers.forEach(clearTimeout)
      flashed?.removeAttribute('data-flash')
    }
  }, [root, target, scope])
}
