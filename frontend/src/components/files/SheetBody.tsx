import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { TabBar } from '../primitives'
import { NetworkState, SkeletonStage } from './ViewerStates'
import { TruncationStrip, ViewerTable } from './ViewerTable'
import { MAX_LINES } from './viewer-text'

interface ParsedSheet {
  name: string
  /** Header row first, then at most `MAX_LINES` data rows, as the cells display. */
  rows: string[][]
  /** Data rows in the whole sheet. */
  total: number
}

/**
 * Reads a workbook with SheetJS, imported on demand so it is its own chunk and
 * never part of the app bundle. Cells show their values as formatted in the
 * file, not formulas. Only the first rows of each sheet are parsed.
 */
async function parseWorkbook(url: string): Promise<ParsedSheet[]> {
  const [XLSX, res] = await Promise.all([import('xlsx'), fetch(url)])
  if (!res.ok) throw new Error(`sheet preview: ${res.status}`)
  const wb = XLSX.read(await res.arrayBuffer(), { type: 'array', sheetRows: MAX_LINES + 1 })
  return wb.SheetNames.map((name) => {
    const ws = wb.Sheets[name]
    if (!ws) return { name, rows: [], total: 0 }
    const full = ws['!fullref'] ?? ws['!ref']
    const rowCount = full ? XLSX.utils.decode_range(full).e.r + 1 : 0
    const rows = XLSX.utils.sheet_to_json<unknown[]>(ws, { header: 1, raw: false, defval: '', blankrows: false })
    return { name, rows: rows.map((r) => r.map((c) => String(c ?? ''))), total: Math.max(rowCount - 1, 0) }
  })
}

/** xlsx, xls, ods: a tab per sheet, the first 500 rows as a table. */
export function SheetBody({ url, onDownload }: { url: string; onDownload: () => void }) {
  const [active, setActive] = useState(0)
  const { data, isPending, isError, refetch } = useQuery({
    queryKey: ['drive', 'preview-sheet', url],
    queryFn: () => parseWorkbook(url),
    staleTime: Infinity,
    retry: false,
  })

  if (isPending) return <SkeletonStage />
  if (isError) return <NetworkState onRetry={() => void refetch()} />

  const sheet = data[Math.min(active, data.length - 1)]
  if (!sheet) return <NetworkState onRetry={() => void refetch()} />
  const shown = Math.max(sheet.rows.length - 1, 0)
  return (
    <div className="grid grid-rows-[auto_minmax(0,1fr)_auto] h-full">
      {data.length > 1 ? (
        <TabBar
          tabs={data.map((x, i) => ({ id: String(i), label: x.name }))}
          value={String(active)}
          onChange={(id) => setActive(Number(id))}
          label="Trang tính"
          idPrefix="sheet"
          className="px-3 pt-1 bg-base border-b border-line overflow-x-auto"
        />
      ) : (
        <div />
      )}
      <div role="tabpanel" id={`sheet-panel-${active}`} aria-labelledby={`sheet-tab-${active}`} className="min-h-0">
        <ViewerTable rows={sheet.rows} />
      </div>
      {sheet.total > shown && <TruncationStrip shown={shown} total={sheet.total} onDownload={onDownload} />}
    </div>
  )
}
