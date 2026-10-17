import { useState } from 'react'
import { Pressable } from '../primitives'
import { Building2, ChevronRight } from 'lucide-react'
import type { PathSegment } from './contacts-model'

/** A path longer than this folds its middle into "…". */
const MAX_SHOWN = 3

interface ContactsBreadcrumbProps {
  /** Name of the top of the tree: the organisation, or "Phạm vi của bạn". The building icon is named by it. */
  rootLabel?: string
  /** The departments from the top down to the one that is open; empty at the top. */
  segments?: PathSegment[]
  /** A step up to a department, or to the top when no id is given. */
  onOpen?: (id: string | undefined) => void
  /** The nav landmark's name. */
  label?: string
  /** A category as the first step instead of the building ("Vai trò ›", "Nhóm của tôi ›"). */
  leading?: { label: string; onPress: () => void }
  /** The current page when it is not a department (the role or group that is open). */
  current?: string
}

const STEP = `border-none bg-transparent text-inherit px-1.5 py-1 rounded-sm cursor-pointer min-w-0 truncate
  hover:bg-hover hover:text-ink focus-ring transition-colors duration-quick`

/**
 * Where the middle column is (DESIGN.md "Danh bạ"): the building is the top of the tree, every
 * segment a button to that level and the last one the current page, never cut. A long path folds
 * its middle into "…", which unfolds on press. Roles and groups lead with their category instead.
 */
export function ContactsBreadcrumb({
  rootLabel = '', segments = [], onOpen = () => {}, label = 'Đường dẫn', leading, current,
}: ContactsBreadcrumbProps) {
  // Unfolded for the path it was unfolded on: another place starts folded again.
  const pathKey = segments.map((s) => s.id).join('/')
  const [unfoldedKey, setUnfoldedKey] = useState<string | null>(null)
  const unfolded = unfoldedKey === pathKey
  const sep = <ChevronRight size={14} strokeWidth={1.75} aria-hidden="true" className="shrink-0 text-ink-subtle" />
  const currentSpan = (text: string) => (
    <span aria-current="page" className="px-1.5 py-1 font-semibold text-ink shrink-0">{text}</span>
  )

  if (leading) {
    return (
      <nav aria-label={label} className="flex items-center gap-0.5 min-w-0 text-small text-ink-muted">
        <Pressable onClick={leading.onPress} className={STEP}>{leading.label}</Pressable>
        {sep}
        {currentSpan(current ?? '')}
      </nav>
    )
  }

  const upTo = segments.slice(0, -1)
  const last = segments[segments.length - 1]
  const folded = !unfolded && segments.length > MAX_SHOWN
  // Folded: the first level, "…", and the level above the current one.
  const steps: Array<PathSegment | 'fold'> = folded ? [upTo[0]!, 'fold', upTo[upTo.length - 1]!] : upTo
  return (
    <nav aria-label={label} className="flex items-center gap-0.5 min-w-0 text-small text-ink-muted">
      <Pressable
        aria-label={rootLabel}
        onClick={() => onOpen(undefined)}
        className="inline-grid place-items-center shrink-0 p-1 rounded-sm border-none bg-transparent text-inherit cursor-pointer
          hover:bg-hover hover:text-ink focus-ring transition-colors duration-quick"
      >
        <Building2 size={16} strokeWidth={1.75} aria-hidden="true" />
      </Pressable>
      {sep}
      {last ? (
        <>
          {steps.map((step) => (
            <span key={step === 'fold' ? 'fold' : step.id} className="contents">
              {step === 'fold' ? (
                <Pressable aria-label="Hiện các cấp ở giữa" onClick={() => setUnfoldedKey(pathKey)} className={`${STEP} shrink-0`}>…</Pressable>
              ) : (
                <Pressable onClick={() => onOpen(step.id)} className={STEP}>{step.name}</Pressable>
              )}
              {sep}
            </span>
          ))}
          {currentSpan(last.name)}
        </>
      ) : (
        currentSpan(rootLabel)
      )}
    </nav>
  )
}
