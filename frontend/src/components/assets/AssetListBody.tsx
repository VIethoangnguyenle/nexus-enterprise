import { motion } from 'motion/react'
import { Package, SearchX } from 'lucide-react'
import type { Asset, AssetType } from '../../api/assets'
import type { Arrival } from '../../hooks/useArrivals'
import type { PeopleDirectory } from '../../lib/people'
import { ASSET_STATES, stateLabel } from '../../lib/asset-model'
import { assetSearch, filterSearch, pageSearch, type AssetsSearch } from '../../lib/assets-search'
import type { MotionPresets } from '../../lib/motion'
import { Button, FilterChip, SearchField } from '../primitives'
import { EmptyState } from '../spaces/EmptyState'
import { AssetTable, type RealtimeTag } from './AssetTable'
import type { Summary } from './AssetOverview'
import { TypeFilter } from './TypeFilter'

export const PAGE = 25

interface Props {
  search: AssetsSearch
  go: (to: (prev: AssetsSearch) => AssetsSearch) => void
  /** Search box text, ahead of the URL by a short pause. */
  typed: string
  onTyped: (text: string) => void
  summary: Summary | undefined
  types: AssetType[]
  /** Types the caller may add assets or requests to. */
  requestable: AssetType[]
  rows: Asset[]
  total: number
  page: number
  loading: boolean
  people: PeopleDirectory
  openId: string | null
  scope: string
  fresh: Map<string, Arrival>
  versionOf: (a: Asset) => string
  tagOf: (a: Asset, arrival: { author?: string }) => RealtimeTag | undefined
  route: MotionPresets['route']
  onAddAsset: () => void
}

/** The Danh sách tab: filters, the table or why it is empty, and the pager. */
export function AssetListBody({
  search, go, typed, onTyped, summary, types, requestable, rows, total, page, loading, people, openId, scope,
  fresh, versionOf, tagOf, route, onAddAsset,
}: Props) {
  const filtered = !!(search.state || search.kind || search.q)
  const counts = summary?.byState ?? {}
  const chips = ASSET_STATES.filter((s) => (counts[s] ?? 0) > 0 || s === search.state)
  const from = (page - 1) * PAGE + 1
  const to = Math.min(page * PAGE, total)
  return (
    <>
      <div className="flex flex-wrap items-center gap-2 px-5 pb-3">
        <SearchField
          label="Tìm tài sản"
          moduleSearch
          value={typed}
          onChange={(e) => onTyped(e.target.value)}
          className="w-full sm:w-56"
          tone="sunk"
        />
        <div className="flex flex-wrap items-center gap-2" role="group" aria-label="Lọc theo trạng thái">
          <FilterChip pressed={!search.state} onClick={() => go((p) => filterSearch(p, { state: undefined }))}>
            Tất cả {summary && <span className="font-normal text-ink-muted tnum">{summary.total}</span>}
          </FilterChip>
          {chips.map((s) => (
            <FilterChip key={s} pressed={search.state === s} onClick={() => go((p) => filterSearch(p, { state: s }))}>
              {stateLabel(s)} <span className="font-normal text-ink-muted tnum">{counts[s] ?? 0}</span>
            </FilterChip>
          ))}
        </div>
        {types.length > 0 && (
          <TypeFilter
            types={types}
            value={search.kind}
            onChange={(kind) => go((p) => filterSearch(p, { kind }))}
          />
        )}
      </div>
      {!loading && rows.length === 0 ? (
        filtered ? (
          <EmptyState
            icon={<SearchX size={24} strokeWidth={1.75} />}
            text={`Không có tài sản nào khớp${search.q ? ` “${search.q.trim()}”` : ' bộ lọc'}. Thử bỏ bộ lọc hoặc tìm theo tên người giữ.`}
            action={<Button variant="soft" size="sm" onClick={() => go((p) => filterSearch(p, { state: undefined, kind: undefined, q: '' }))}>Xoá bộ lọc</Button>}
          />
        ) : (
          <EmptyState
            icon={<Package size={24} strokeWidth={1.75} />}
            text={requestable.length > 0 ? 'Chưa có tài sản nào. Thêm tài sản để theo dõi người giữ và lịch sử của nó.' : 'Chưa có tài sản nào trong phạm vi bạn xem được.'}
            action={requestable.length > 0 ? <Button size="sm" onClick={onAddAsset}>Thêm tài sản</Button> : undefined}
          />
        )
      ) : (
        <motion.div key="list" {...route}>
          <AssetTable
            assets={rows}
            loading={loading}
            people={people}
            openId={openId}
            onOpen={(a) => go((p) => assetSearch(p, a.id))}
            scope={scope}
            fresh={fresh}
            versionOf={versionOf}
            tagOf={tagOf}
          />
          {!loading && total > 0 && (
            <div className="flex items-center justify-between gap-2 px-7 pb-4 text-sm text-ink-muted">
              <span className="tnum">{from} đến {to} trong {total}</span>
              <span className="flex gap-2">
                <Button variant="ghost" size="sm" disabled={page <= 1} onClick={() => go((p) => pageSearch(p, page - 1))}>Trước</Button>
                <Button variant="soft" size="sm" disabled={to >= total} onClick={() => go((p) => pageSearch(p, page + 1))}>Sau</Button>
              </span>
            </div>
          )}
        </motion.div>
      )}
    </>
  )
}
