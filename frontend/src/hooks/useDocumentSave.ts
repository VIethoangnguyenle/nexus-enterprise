import { useCallback, useEffect, useRef, useState } from 'react'
import { ApiError } from '../api/client'
import { conflictOf, type TextDocument, type TextSave, type TextStatus } from '../api/documents'
import type { SaveState } from '../components/documents/document-model'

export interface SaveOptions {
  /** The document as it was loaded. */
  initial: { version: number; title: string; content: string; status: TextStatus }
  /** Sends one save. A refused-stale save must reject with the 409 ApiError. */
  save: (body: TextSave) => Promise<TextDocument>
  /** Quiet time after the last keystroke before a save is sent. */
  delay?: number
  /**
   * Called when text could not be saved after the editor was already gone (or
   * was going): the person has left the page, so this is the only way to tell
   * them. The text is lost either way; saying so is the point.
   */
  onUnsaved?: (reason: 'conflict' | 'offline' | 'error') => void
}

/** How long to wait before trying again after the network was down. */
const RETRY_MS = 4000

interface Fields { title: string; content: string; status: TextStatus }

/**
 * Saves a document as it is written, with no Save button (mockup §3).
 *
 * Every save names the version it is based on. If the server has moved on, the
 * save is refused with the current document and this hook *stops*: it does not
 * retry and does not overwrite, whatever is typed afterwards. The person
 * chooses, through `takeTheirs` (adopt the other version, drop mine) or
 * `keepMine` (write mine on top of theirs, deliberately).
 *
 * One save is in flight at a time; text typed during it is saved right after.
 * A network failure keeps the text, says it is waiting, and retries on a timer
 * and as soon as the browser reports it is online.
 */
export function useDocumentSave({ initial, save, delay = 1200, onUnsaved }: SaveOptions) {
  const [state, setState] = useState<SaveState>('saved')
  const [savedAt, setSavedAt] = useState<Date | null>(null)
  const [conflict, setConflict] = useState<TextDocument | null>(null)
  const [errorStatus, setErrorStatus] = useState<number | undefined>()
  const [status, setStatusState] = useState<TextStatus>(initial.status)

  const base = useRef(initial.version)
  const saved = useRef<Fields>({ title: initial.title, content: initial.content, status: initial.status })
  const local = useRef<Fields>({ ...saved.current })
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const inFlight = useRef(false)
  const rerun = useRef(false)
  const stopped = useRef(false) // a conflict is open: nothing is sent
  const paused = useRef(false) // something else (a delete) is using the document
  const alive = useRef(true) // false once the editor is gone: nothing is scheduled again
  const saveRef = useRef(save)
  const unsavedRef = useRef(onUnsaved)
  useEffect(() => {
    saveRef.current = save
    unsavedRef.current = onUnsaved
  })

  const changed = () =>
    local.current.title.trim() !== saved.current.title.trim() ||
    local.current.content !== saved.current.content ||
    local.current.status !== saved.current.status

  const clear = () => {
    if (timer.current) clearTimeout(timer.current)
    timer.current = null
  }
  const schedule = (ms: number) => {
    clear()
    if (!alive.current) return
    timer.current = setTimeout(() => void run(), ms)
  }

  const run = useCallback(async (): Promise<void> => {
    clear()
    if (stopped.current || paused.current) return
    if (inFlight.current) {
      rerun.current = true
      return
    }
    if (!changed()) {
      setState('saved')
      return
    }
    const sent: Fields = { ...local.current }
    const body: TextSave = { base_version: base.current }
    if (sent.title.trim() !== saved.current.title.trim()) body.title = sent.title
    if (sent.content !== saved.current.content) body.content = sent.content
    if (sent.status !== saved.current.status) body.status = sent.status

    inFlight.current = true
    setState('saving')
    try {
      const doc = await saveRef.current(body)
      base.current = doc.version
      saved.current = {
        title: body.title !== undefined ? sent.title : saved.current.title,
        content: body.content !== undefined ? sent.content : saved.current.content,
        status: body.status !== undefined ? sent.status : saved.current.status,
      }
      setSavedAt(new Date())
      setErrorStatus(undefined)
      if (changed() && alive.current) {
        setState('dirty')
        schedule(rerun.current ? 0 : delay)
      } else {
        setState('saved')
      }
    } catch (err) {
      const current = conflictOf(err)
      if (current) {
        stopped.current = true
        if (!alive.current) unsavedRef.current?.('conflict')
        setConflict(current)
        setState('conflict')
      } else if (!(err instanceof ApiError)) {
        // fetch itself failed: the network is down. The text stays; try again,
        // unless nobody is here to see it any more.
        if (!alive.current) unsavedRef.current?.('offline')
        setState('offline')
        schedule(RETRY_MS)
      } else {
        if (!alive.current) unsavedRef.current?.('error')
        setErrorStatus(err.status)
        setState('error')
      }
    } finally {
      inFlight.current = false
      rerun.current = false
    }
  }, [delay])

  const touch = () => {
    if (stopped.current) {
      setState('conflict')
      return
    }
    if (!changed()) {
      clear()
      setState('saved')
      return
    }
    setState('dirty')
    schedule(delay)
  }

  const setTitle = useCallback((title: string) => { local.current.title = title; touch() }, [delay])  
  const setContent = useCallback((content: string) => { local.current.content = content; touch() }, [delay])  
  const setStatus = useCallback((next: TextStatus) => {
    local.current.status = next
    setStatusState(next)
    if (stopped.current) return setState('conflict')
    void run()
  }, [run])

  const flush = useCallback(() => run(), [run])

  /** What is on the page right now, saved or not. */
  const getLocal = useCallback((): Fields => ({ ...local.current }), [])

  /** The document is gone (deleted): nothing more is sent. */
  const discard = useCallback(() => {
    stopped.current = true
    clear()
  }, [])

  /** Hold saving while something else is done to the document (a delete in flight). */
  const pause = useCallback(() => {
    paused.current = true
    clear()
  }, [])

  /** That something failed: the document is still here, so saving carries on. */
  const resume = useCallback(() => {
    paused.current = false
    if (!stopped.current && changed()) schedule(delay)
  }, [delay])  

  /** Drop my unsaved changes and adopt the other version; saving builds on it. */
  const takeTheirs = useCallback((): TextDocument | undefined => {
    const theirs = conflict
    if (!theirs) return undefined
    base.current = theirs.version
    saved.current = { title: theirs.title, content: theirs.content ?? '', status: theirs.status }
    local.current = { ...saved.current }
    setStatusState(theirs.status)
    stopped.current = false
    setConflict(null)
    setState('saved')
    return theirs
  }, [conflict])

  /** Write my version over theirs, on top of theirs: the person asked for exactly this. */
  const keepMine = useCallback(() => {
    const theirs = conflict
    if (!theirs) return
    base.current = theirs.version
    saved.current = { title: theirs.title, content: theirs.content ?? '', status: theirs.status }
    stopped.current = false
    setConflict(null)
    void run()
  }, [conflict, run])

  // Back online: try the waiting save at once.
  useEffect(() => {
    const onOnline = () => { if (!stopped.current && changed()) void run() }
    window.addEventListener('online', onOnline)
    return () => window.removeEventListener('online', onOnline)
  }, [run])

  // Leaving with unsaved text: ask the browser to hold the page, and on unmount save it.
  useEffect(() => {
    const onUnload = (e: BeforeUnloadEvent) => {
      if (stopped.current || changed()) e.preventDefault()
    }
    window.addEventListener('beforeunload', onUnload)
    return () => window.removeEventListener('beforeunload', onUnload)
  }, [])
  useEffect(() => {
    alive.current = true
    return () => {
      alive.current = false
      clear()
      if (stopped.current || paused.current || !changed()) return
      // Going away while offline: the browser has already been asked to hold the
      // page, and a request now would only fail unseen.
      if (typeof navigator !== 'undefined' && navigator.onLine === false) unsavedRef.current?.('offline')
      else void run()
    }
  }, [run])

  return {
    state, savedAt, conflict, errorStatus, status, setTitle, setContent, setStatus, flush, getLocal, discard, pause, resume, takeTheirs, keepMine,
  }
}
