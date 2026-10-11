import { DownloadButton } from './ViewerStates'

const NUMERIC = /^-?[\d.,\s]+(?:[%₫$€])?$/

/**
 * A ruled table: sticky header row, sticky row-number column, numbers right
 * aligned. The first row of `rows` is the header. Cells are text, never markup.
 */
export function ViewerTable({ rows }: { rows: string[][] }) {
  const [head = [], ...body] = rows
  const width = Math.max(head.length, ...body.map((r) => r.length))
  const cols = Array.from({ length: width }, (_, i) => i)
  const numeric = cols.map((c) => {
    const cells = body.map((r) => r[c] ?? '').filter((v) => v.trim() !== '')
    return cells.length > 0 && cells.every((v) => NUMERIC.test(v.trim()))
  })
  return (
    <div className="h-full overflow-auto bg-base" tabIndex={0} role="region" aria-label="Nội dung tệp">
      <table className="border-separate border-spacing-0 text-small tnum min-w-full">
        <thead>
          <tr>
            <th scope="col" className="sticky top-0 left-0 z-20 bg-sunk border-b border-r border-line w-12 min-w-12">
              <span className="sr-only">Dòng</span>
            </th>
            {cols.map((c) => (
              <th
                key={c}
                scope="col"
                className={`sticky top-0 z-10 bg-sunk border-b border-line px-3 h-9 font-semibold text-ink whitespace-nowrap ${numeric[c] ? 'text-right' : 'text-left'}`}
              >
                {head[c] ?? ''}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {body.map((r, i) => (
            <tr key={i}>
              <td className="sticky left-0 z-10 bg-sunk border-b border-r border-line px-2 h-9 text-right text-ink-muted">{i + 1}</td>
              {cols.map((c) => (
                <td
                  key={c}
                  className={`border-b border-line px-3 h-9 whitespace-nowrap text-ink ${numeric[c] ? 'text-right' : 'text-left'}`}
                >
                  {r[c] ?? ''}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

/** "Đang xem 500 dòng đầu trên 1.204 dòng." plus the way to the whole file. */
export function TruncationStrip({ shown, total, onDownload }: { shown: number; total?: number; onDownload: () => void }) {
  const fmt = (n: number) => n.toLocaleString('vi-VN')
  return (
    <div className="flex items-center justify-between gap-3 px-4 py-2 bg-base border-t border-line text-small text-ink-muted">
      <span className="tnum">
        {total !== undefined ? `Đang xem ${fmt(shown)} dòng đầu trên ${fmt(total)} dòng.` : `Đang xem ${fmt(shown)} dòng đầu.`}
      </span>
      <DownloadButton variant="ghost" onClick={onDownload}>Tải xuống để xem đủ</DownloadButton>
    </div>
  )
}
