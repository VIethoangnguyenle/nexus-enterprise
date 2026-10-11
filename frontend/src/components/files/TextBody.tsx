import { useQuery } from '@tanstack/react-query'
import { NetworkState } from './ViewerStates'
import { SkeletonStage } from './ViewerStates'
import { TruncationStrip, ViewerTable } from './ViewerTable'
import { MAX_BYTES, MAX_LINES, parseCsv, readCappedText, takeLines, textFlavor } from './viewer-text'

/** A text, code or CSV file fetched from its signed URL, capped at 1 MB / 500 lines, and only ever shown as text. */
export function TextBody({
  url, name, mime, onDownload,
}: { url: string; name: string; mime: string; onDownload: () => void }) {
  const { data, isPending, isError, refetch } = useQuery({
    queryKey: ['drive', 'preview-text', url],
    queryFn: async () => {
      const res = await fetch(url)
      if (!res.ok) throw new Error(`text preview: ${res.status}`)
      return readCappedText(res, MAX_BYTES)
    },
    staleTime: Infinity,
    retry: false,
  })

  if (isPending) return <SkeletonStage />
  if (isError) return <NetworkState onRetry={() => void refetch()} />

  const flavor = textFlavor(name, mime)
  if (flavor === 'csv') {
    const all = parseCsv(data.text)
    // A cut body ends mid-row; the last row may be partial, so it is not counted.
    const rows = data.cut ? all.slice(0, -1) : all
    const shown = rows.slice(0, MAX_LINES + 1)
    const truncated = data.cut || rows.length > shown.length
    return (
      <div className="grid grid-rows-[minmax(0,1fr)_auto] h-full">
        <ViewerTable rows={shown} />
        {truncated && (
          <TruncationStrip shown={shown.length - 1} total={data.cut ? undefined : rows.length - 1} onDownload={onDownload} />
        )}
      </div>
    )
  }

  const { lines, total } = takeLines(data.text, MAX_LINES)
  const truncated = data.cut || total > lines.length
  return (
    <div className="grid grid-rows-[minmax(0,1fr)_auto] h-full">
      <div className="overflow-auto bg-base" tabIndex={0} role="region" aria-label="Nội dung tệp">
        {flavor === 'code' ? (
          <pre className="m-0 py-4 font-mono text-small leading-6 text-ink w-max min-w-full">
            {lines.map((line, i) => (
              <div key={i} className="flex">
                <span className="select-none shrink-0 w-12 pr-4 text-right text-ink-muted tnum" aria-hidden="true">{i + 1}</span>
                <span className="whitespace-pre pr-4">{line}</span>
              </div>
            ))}
          </pre>
        ) : (
          <pre className="m-0 mx-auto p-6 max-w-[88ch] font-mono text-small leading-6 text-ink whitespace-pre-wrap break-words">
            {lines.join('\n')}
          </pre>
        )}
      </div>
      {truncated && <TruncationStrip shown={lines.length} total={data.cut ? undefined : total} onDownload={onDownload} />}
    </div>
  )
}
