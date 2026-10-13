import { useEffect, useRef, useState } from 'react'
import { useIsFetching, useIsMutating } from '@tanstack/react-query'
import { RefreshCw } from 'lucide-react'
import { usePwaStore } from '../../lib/pwa'
import { useUiStore } from '../../stores/ui.store'
import { Button } from '../primitives'

/** How often the bar re-reads what is being typed. Programmatic clears (a sent message) raise no event. */
const POLL_MS = 500
/** An edit this recent is unsaved work wherever it happened: documents autosave after ~1.2 s. */
const RECENT_EDIT_MS = 3000

const COMPOSER = '.chat-editor-content'
const NOT_TEXT = new Set(['checkbox', 'radio', 'range', 'file', 'button', 'submit', 'reset', 'color', 'image'])

function fieldOf(target: EventTarget | null): HTMLElement | null {
  if (!(target instanceof HTMLElement)) return null
  if (target instanceof HTMLTextAreaElement) return target
  if (target instanceof HTMLInputElement) return NOT_TEXT.has(target.type) ? null : target
  return target.closest<HTMLElement>('[contenteditable]:not([contenteditable="false"])')
}

function textOf(el: HTMLElement): string {
  return el instanceof HTMLTextAreaElement || el instanceof HTMLInputElement ? el.value : (el.textContent ?? '')
}

/** Fields the person has typed into since they appeared (filled from real input events only). */
interface Edits {
  fields: Set<HTMLElement>
  lastAt: number
}

/**
 * Unsent work the reload would destroy. A field counts only if the person typed in it
 * (so a form pre-filled with saved text never holds the update forever) and it still
 * holds text; any edit in the last few seconds counts too, which covers a document
 * between keystroke and autosave. Fields that left the page are forgotten.
 */
function hasDraft(edits: Edits): boolean {
  if (Date.now() - edits.lastAt < RECENT_EDIT_MS) return true
  for (const el of edits.fields) {
    if (!el.isConnected) { edits.fields.delete(el); continue }
    if (el.closest('.doc-prose')) continue // saved by the document autosave, covered above
    if (textOf(el).trim() !== '') return true
  }
  return false
}

/** What the person is doing right now: unsent text, and whether a composer is on screen. */
function useWorkState(active: boolean): { draft: boolean; composer: boolean } {
  const edits = useRef<Edits>({ fields: new Set(), lastAt: -Infinity })
  const [state, setState] = useState({ draft: false, composer: false })

  // Watch typing from the start, so text typed before the bar shows up is remembered.
  useEffect(() => {
    const record = (e: Event) => {
      const el = fieldOf(e.target)
      if (!el) return
      edits.current.fields.add(el)
      edits.current.lastAt = Date.now()
    }
    document.addEventListener('input', record, true)
    return () => document.removeEventListener('input', record, true)
  }, [])

  // Programmatic clears (a sent message) raise no event, so while the bar shows, look again regularly.
  useEffect(() => {
    if (!active) return
    const sync = () => {
      const next = { draft: hasDraft(edits.current), composer: !!document.querySelector(COMPOSER) }
      setState((cur) => (cur.draft === next.draft && cur.composer === next.composer ? cur : next))
    }
    sync()
    const timer = setInterval(sync, POLL_MS)
    return () => clearInterval(timer)
  }, [active])
  return state
}

/**
 * Floating "new version" bar (design/mockups/push.html §5). The waiting worker is only
 * activated on a tap, and never while it would cost the person something: with unsent text
 * the button reads "Tải lại sau khi gửi", and the reload also waits until no send, upload
 * or fetch is in flight. "Để sau" hides the bar for this visit and cancels a pending reload.
 * Next to a composer the bar sits at the top, so it never covers the send button.
 */
export function UpdateBar() {
  const needRefresh = usePwaStore((s) => s.needRefresh)
  const applyUpdate = usePwaStore((s) => s.applyUpdate)
  const [dismissed, setDismissed] = useState(false)
  const [armed, setArmed] = useState(false)
  const busy = useIsMutating() + useIsFetching() > 0
  const aboveChatBar = useUiStore((s) => s.chatBarShown)
  const visible = needRefresh && !dismissed
  const { draft, composer } = useWorkState(visible)

  const holding = draft || busy
  useEffect(() => {
    if (armed && visible && !holding) applyUpdate()
  }, [armed, visible, holding, applyUpdate])

  const dismiss = () => {
    setDismissed(true)
    setArmed(false)
  }

  // The live region exists from the first render and only its content changes, so
  // screen readers announce the bar when it arrives.
  return (
    <div role="status">
      {visible && (
        <div
          data-placement={composer ? 'top' : 'bottom'}
          style={composer ? { top: 'calc(env(safe-area-inset-top, 0px) + 0.75rem)' } : undefined}
          className={`fixed left-2.5 right-2.5 z-sticky flex items-center gap-2.5 min-h-11 pl-4 pr-2 py-1.5 rounded-2xl text-sm font-semibold bg-overlay text-ink shadow-overlay lg:left-auto lg:right-6 lg:max-w-md ${
            composer ? '' : aboveChatBar
              ? 'bottom-[calc(6.375rem+env(safe-area-inset-bottom,0px))] lg:bottom-6'
              : 'bottom-[calc(1rem+env(safe-area-inset-bottom,0px))] lg:bottom-6'
          }`}
        >
          <RefreshCw size={16} strokeWidth={1.75} className="text-accent shrink-0" aria-hidden />
          <span className="flex-1 min-w-0">Có bản mới của Nexus</span>
          <Button variant="ghost" size="sm" className="min-h-9" onClick={dismiss}>Để sau</Button>
          <Button
            variant="primary"
            size="sm"
            className="min-h-9"
            disabled={armed}
            onClick={() => (holding ? setArmed(true) : applyUpdate())}
          >
            {draft ? 'Tải lại sau khi gửi' : 'Tải lại'}
          </Button>
        </div>
      )}
    </div>
  )
}
