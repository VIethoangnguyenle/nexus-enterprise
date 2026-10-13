import { useCallback, useEffect, useState } from 'react'

interface DocumentPip {
  requestWindow: (opts?: { width?: number; height?: number }) => Promise<Window>
}
const documentPip = (): DocumentPip | undefined =>
  (window as unknown as { documentPictureInPicture?: DocumentPip }).documentPictureInPicture

/** Gives a new window the page's styles, so what is portalled into it looks the same. */
function copyStyles(target: Window): void {
  for (const sheet of Array.from(document.styleSheets)) {
    try {
      const style = target.document.createElement('style')
      style.textContent = Array.from(sheet.cssRules).map((r) => r.cssText).join('\n')
      target.document.head.appendChild(style)
    } catch {
      // A cross-origin sheet cannot be read: link it instead.
      if (sheet.href) {
        const link = target.document.createElement('link')
        link.rel = 'stylesheet'
        link.href = sheet.href
        target.document.head.appendChild(link)
      }
    }
  }
}

/**
 * Picture-in-Picture for a call on a desktop: when the tab is left, a small
 * always-on-top window shows the speaker and the leave button. The document
 * kind (Chrome, Edge) can hold our own controls; elsewhere the speaker's
 * `<video>` goes into the browser's plain video PiP. Browsers allow this
 * without a click only through the media session "enter picture-in-picture"
 * action, so that is what is registered; where even that is missing, nothing
 * happens and the call simply carries on in its tab.
 */
export function useCallPictureInPicture(enabled: boolean): { window: Window | null; exit: () => void } {
  const [win, setWin] = useState<Window | null>(null)

  const exit = useCallback(() => {
    setWin((w) => { w?.close(); return null })
    if (document.pictureInPictureElement) void document.exitPictureInPicture().catch(() => {})
  }, [])

  const enter = useCallback(async () => {
    try {
      const api = documentPip()
      if (api) {
        const w = await api.requestWindow({ width: 320, height: 260 })
        copyStyles(w)
        w.document.body.className = 'cs-pip-body'
        w.addEventListener('pagehide', () => setWin(null), { once: true })
        setWin(w)
        return
      }
      const video = document.querySelector<HTMLVideoElement>('[data-call-speaker-video]')
      if (video && document.pictureInPictureEnabled && !document.pictureInPictureElement) await video.requestPictureInPicture()
    } catch {
      /* needs a user gesture, or not supported: the call carries on in its tab */
    }
  }, [])

  useEffect(() => {
    if (!enabled) return
    try {
      navigator.mediaSession?.setActionHandler('enterpictureinpicture' as MediaSessionAction, () => { void enter() })
    } catch {
      /* the action is not supported here */
    }
    const onVisibility = () => {
      if (document.visibilityState === 'hidden') void enter()
      else exit()
    }
    document.addEventListener('visibilitychange', onVisibility)
    return () => {
      document.removeEventListener('visibilitychange', onVisibility)
      try { navigator.mediaSession?.setActionHandler('enterpictureinpicture' as MediaSessionAction, null) } catch { /* unsupported */ }
    }
  }, [enabled, enter, exit])

  // The call ended: the window goes with it.
  useEffect(() => {
    if (!enabled) exit()
  }, [enabled, exit])

  return { window: win, exit }
}
