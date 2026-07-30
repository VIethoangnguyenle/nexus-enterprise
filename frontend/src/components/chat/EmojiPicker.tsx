import { useEffect, useRef } from 'react'
import data from '@emoji-mart/data'
import { Picker } from 'emoji-mart'

interface EmojiPickerProps {
  onSelect: (emoji: string) => void
  onClose: () => void
}

/**
 * Emoji picker popup using emoji-mart.
 *
 * Mounts emoji-mart's core Picker directly rather than via @emoji-mart/react.
 * That wrapper is abandoned upstream at 1.1.1 and declares a peer of
 * react ^16.8 || ^17 || ^18, which blocks install on this project's React 19.
 * The wrapper was only ~15 lines — constructing the Picker against a ref'd
 * div — so it is inlined here instead.
 */
export function EmojiPicker({ onSelect, onClose }: EmojiPickerProps) {
  const ref = useRef<HTMLDivElement>(null)
  const mountRef = useRef<HTMLDivElement>(null)
  const instance = useRef<unknown>(null)

  // Close on click outside
  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        onClose()
      }
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [onClose])

  // Mount the emoji-mart picker once; it renders into mountRef.
  useEffect(() => {
    instance.current = new Picker({
      data,
      onEmojiSelect: (emoji: { native: string }) => onSelect(emoji.native),
      theme: 'dark',
      previewPosition: 'none',
      skinTonePosition: 'none',
      maxFrequentRows: 2,
      perLine: 8,
      ref: mountRef,
    })
    return () => {
      instance.current = null
    }
    // Mount-only, matching the upstream wrapper's behaviour.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  return (
    <div ref={ref} className="shadow-xl rounded-xl overflow-hidden animate-fade-in">
      <div ref={mountRef} />
    </div>
  )
}
