import { useEffect, useRef } from 'react'
import type { MediaTrack } from '../../lib/call-room'

/**
 * Shows a video track. The SDK attaches the stream and, with adaptive
 * streaming on, watches the element's size to ask the server for just enough
 * resolution, so a small tile costs little.
 */
export function TrackVideo({ track, mirror = false, fit = 'cover', className = '', speakerMark = false }: {
  track: MediaTrack | null
  /** Your own camera is shown flipped, like a mirror. */
  mirror?: boolean
  fit?: 'cover' | 'contain'
  className?: string
  /** Marks the element Picture-in-Picture may pick when the tab is left. */
  speakerMark?: boolean
}) {
  const ref = useRef<HTMLVideoElement>(null)
  useEffect(() => {
    const el = ref.current
    if (!el || !track) return
    track.attach(el)
    return () => { track.detach(el) }
  }, [track])
  if (!track) return null
  return (
    <video
      ref={ref}
      autoPlay
      playsInline
      muted
      data-call-speaker-video={speakerMark ? '' : undefined}
      className={`cs-video ${fit === 'contain' ? 'cs-video-contain' : ''} ${mirror ? 'cs-video-mirror' : ''} ${className}`}
    />
  )
}

function AudioSink({ track }: { track: MediaTrack }) {
  const ref = useRef<HTMLAudioElement>(null)
  useEffect(() => {
    const el = ref.current
    if (!el) return
    track.attach(el)
    return () => { track.detach(el) }
  }, [track])
  return <audio ref={ref} autoPlay />
}

/** Plays everyone else's microphone (and a shared tab's sound). It lives as long as the room, on every screen. */
export function RemoteAudio({ items }: { items: { key: string; track: MediaTrack }[] }) {
  return (
    <div hidden aria-hidden="true">
      {items.map((i) => <AudioSink key={i.key} track={i.track} />)}
    </div>
  )
}
