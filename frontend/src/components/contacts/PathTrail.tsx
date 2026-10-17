import { ChevronRight } from 'lucide-react'
import { Marked } from '../patterns/Marked'
import type { PathSegment } from './contacts-model'

/**
 * A department path on one line. The earlier segments give way with an ellipsis, the last never does,
 * so two departments called "Đối soát" in different branches stay readable. A reader hears the steps
 * as a list ("Khối Vận hành, Đối soát"), not as a run of words.
 */
export function PathTrail({ segments, query }: { segments: PathSegment[]; query?: string }) {
  // No overflow clip on the container: the least it can shrink to is its earlier segments at 1.5em each and the last one whole.
  return (
    <span className="inline-flex items-center">
      {segments.map((seg, i) => {
        const last = i === segments.length - 1
        return (
          <span key={`${seg.id}-${i}`} className={`inline-flex items-center ${last ? 'shrink-0 text-ink' : 'min-w-6 shrink'}`}>
            {i > 0 && (
              <>
                <ChevronRight size={12} strokeWidth={1.75} aria-hidden="true" className="shrink-0 mx-0.5 text-ink-subtle" />
                <span className="sr-only">, </span>
              </>
            )}
            <span className={last ? '' : 'truncate'}><Marked text={seg.name} query={query} /></span>
          </span>
        )
      })}
    </span>
  )
}
