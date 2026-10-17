import { highlight } from '../../lib/search-text'

/**
 * `text` with the words of `query` marked as the search screens show them (`--color-warning-wash`), matched without
 * case or accents but marked as written in the text. With no query it is the text alone.
 */
export function Marked({ text, query }: { text: string; query?: string }) {
  if (!query?.trim()) return <>{text}</>
  return (
    <>
      {highlight(text, query).map((p, i) =>
        p.hit ? <mark key={i} className="bg-warning-wash text-inherit rounded-xs px-px">{p.text}</mark> : p.text)}
    </>
  )
}
