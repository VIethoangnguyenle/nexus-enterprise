import { useMemo, useState } from 'react'
import { useQueries, useQuery } from '@tanstack/react-query'
import { ChevronDown, X } from 'lucide-react'
import type { DriveItem } from '../../api/drive'
import { approvalTemplatesOptions } from '../../hooks/useApproval'
import { assetTypesQueryOptions } from '../../hooks/useAssets'
import { calendarsQueryOptions } from '../../hooks/useCalendar'
import { driveItemQueryOptions } from '../../hooks/useDrive'
import { ASSET_STATES, stateLabel } from '../../lib/asset-model'
import type { PeopleDirectory } from '../../lib/people'
import { IconButton, Pressable } from '../primitives'
import { NavSheet, SheetItem, SheetSeparator } from '../spaces/NavSheet'
import type { SearchScope } from './search-scopes'
import {
  APPROVAL_STATUSES, DRIVE_KINDS, NEXT_RANGES, PAST_RANGES, driveKindOf, type SearchFilters,
} from './search-filters'

/** The name of the top of the drive, as the Thư mục chip offers it. */
const DRIVE_TOP = 'Tài liệu (ngoài thư mục)'
/** Folders the Thư mục chip names at most; each is one cached lookup. */
const MAX_FOLDERS = 8

interface Option {
  /** Opaque: handed back, never shown. */
  id: string
  label: string
}

/** One chip: its name, the options it offers and the one chosen. */
interface ChipSpec {
  key: string
  name: string
  options: Option[]
  value?: string
  set: (id: string | undefined) => void
}

interface Props {
  scope: SearchScope
  filters: SearchFilters
  onChange: (next: SearchFilters) => void
  workspaceId: string
  people: PeopleDirectory
  /** The documents found before the chips narrowed them; Người sửa and Thư mục offer what is in them. */
  documents: DriveItem[]
  isPersonal: boolean
}

/**
 * The filter chips after the scope chips (DESIGN.md §6 "Tìm cho mọi tác vụ"): Tài liệu (Loại · Người sửa · Ngày ·
 * Thư mục), Phê duyệt (Trạng thái · Loại · Người gửi · Ngày), Lịch (Lịch · Khoảng ngày), Tài sản (Loại · Trạng thái
 * · Người giữ). Each opens a sheet of names to choose from; a chip that is set shows its value and a ✕. Every
 * choice is a picked name, never an id typed in.
 */
export function SearchFilterChips(props: Props) {
  const chips = useChipSpecs(props)
  const [open, setOpen] = useState<string | null>(null)
  const shown = chips.find((c) => c.key === open)
  if (chips.length === 0) return null
  return (
    <>
      {chips.map((c) => <Chip key={c.key} chip={c} onOpen={() => setOpen(c.key)} />)}
      {shown && (
        <NavSheet label={shown.name} onClose={() => setOpen(null)} roomy>
          <div role="radiogroup" aria-label={shown.name} className="grid gap-1">
            {shown.options.map((o) => (
              <SheetItem
                key={o.id}
                checked={shown.value === o.id}
                onClick={() => {
                  shown.set(o.id)
                  setOpen(null)
                }}
              >
                {o.label}
              </SheetItem>
            ))}
            {shown.options.length === 0 && <p className="m-0 px-3.5 py-3 text-sm text-ink-muted">Chưa có gì để chọn.</p>}
          </div>
          {shown.value !== undefined && (
            <>
              <SheetSeparator />
              <SheetItem onClick={() => { shown.set(undefined); setOpen(null) }}>Bỏ lọc</SheetItem>
            </>
          )}
        </NavSheet>
      )}
    </>
  )
}

function Chip({ chip, onOpen }: { chip: ChipSpec; onOpen: () => void }) {
  const chosen = chip.value !== undefined ? chip.options.find((o) => o.id === chip.value)?.label : undefined
  if (chosen === undefined) {
    return (
      <Pressable
        onClick={onOpen}
        aria-haspopup="dialog"
        className="shrink-0 inline-flex items-center gap-1 h-9 pl-3 pr-2 rounded-full text-sm font-medium text-ink
          ring-1 ring-inset ring-line active:bg-hover"
      >
        {chip.name}
        <ChevronDown size={16} strokeWidth={1.75} aria-hidden="true" className="text-ink-muted" />
      </Pressable>
    )
  }
  return (
    <span className="shrink-0 inline-flex items-center h-9 rounded-full bg-accent-wash ring-1 ring-inset ring-accent">
      <Pressable
        onClick={onOpen}
        aria-haspopup="dialog"
        aria-label={`${chip.name}: ${chosen}`}
        className="h-9 pl-3 pr-1 rounded-l-full text-sm font-semibold text-ink"
      >
        {chosen}
      </Pressable>
      <IconButton
        type="button"
        shape="round"
        aria-label={`Bỏ lọc ${chip.name}: ${chosen}`}
        className="w-9 h-9"
        onClick={() => chip.set(undefined)}
      >
        <X size={16} strokeWidth={1.75} aria-hidden="true" />
      </IconButton>
    </span>
  )
}

const named = (list: { id: string; name: string }[]): Option[] => list.map((x) => ({ id: x.id, label: x.name }))

function useChipSpecs({ scope, filters, onChange, workspaceId, people, documents, isPersonal }: Props): ChipSpec[] {
  const org = !isPersonal
  const templates = useQuery({ ...approvalTemplatesOptions(), enabled: scope === 'approval' && org })
  const calendars = useQuery({ ...calendarsQueryOptions(workspaceId), enabled: scope === 'calendar' && !!workspaceId })
  const types = useQuery({ ...assetTypesQueryOptions(workspaceId), enabled: scope === 'assets' && org && !!workspaceId })

  // The folders holding what was found, named by the drive (which checks read on each); one it refuses is left out.
  const parentIds = useMemo(
    () => [...new Set(documents.map((d) => d.parent_id).filter((id): id is string => !!id))].slice(0, MAX_FOLDERS),
    [documents],
  )
  const parents = useQueries({
    queries: parentIds.map((id) => ({ ...driveItemQueryOptions(id), enabled: scope === 'drive', retry: false })),
  })

  const everyone: Option[] = people.list.map((p) => ({ id: p.userId, label: p.name }))
  const set = <S extends keyof SearchFilters>(s: S, key: keyof SearchFilters[S]) => (id: string | undefined) =>
    onChange({ ...filters, [s]: { ...filters[s], [key]: id } })

  switch (scope) {
    case 'drive': {
      const f = filters.drive
      const editors = new Map<string, string>()
      for (const d of documents) if (d.owner_name) editors.set(d.owner_id, d.owner_name)
      const folders: Option[] = parents.flatMap((q, i) => (q.data ? [{ id: parentIds[i]!, label: q.data.name }] : []))
      if (documents.some((d) => !d.parent_id)) folders.unshift({ id: '', label: DRIVE_TOP })
      const kinds = new Set(documents.map(driveKindOf))
      return [
        { key: 'kind', name: 'Loại', value: f.kind, set: set('drive', 'kind'),
          options: DRIVE_KINDS.filter((k) => kinds.has(k) || k === f.kind).map((k) => ({ id: k, label: k })) },
        { key: 'editor', name: 'Người sửa', value: f.editor, set: set('drive', 'editor'),
          options: [...editors].map(([id, label]) => ({ id, label })) },
        { key: 'date', name: 'Ngày', value: f.date, set: set('drive', 'date'), options: PAST_RANGES.map((r) => ({ id: r.id, label: r.label })) },
        { key: 'folder', name: 'Thư mục', value: f.folder, set: set('drive', 'folder'), options: folders },
      ]
    }
    case 'approval': {
      if (!org) return []
      const f = filters.approval
      return [
        { key: 'status', name: 'Trạng thái', value: f.status, set: set('approval', 'status'), options: APPROVAL_STATUSES.map((s) => ({ id: s.id, label: s.label })) },
        { key: 'template', name: 'Loại', value: f.template, set: set('approval', 'template'), options: named(templates.data?.templates ?? []) },
        { key: 'sender', name: 'Người gửi', value: f.sender, set: set('approval', 'sender'), options: everyone },
        { key: 'date', name: 'Ngày', value: f.date, set: set('approval', 'date'), options: PAST_RANGES.map((r) => ({ id: r.id, label: r.label })) },
      ]
    }
    case 'calendar': {
      const f = filters.calendar
      return [
        { key: 'calendar', name: 'Lịch', value: f.calendar, set: set('calendar', 'calendar'), options: named(calendars.data ?? []) },
        { key: 'range', name: 'Khoảng ngày', value: f.range, set: set('calendar', 'range'), options: NEXT_RANGES.map((r) => ({ id: r.id, label: r.label })) },
      ]
    }
    case 'assets': {
      if (!org) return []
      const f = filters.assets
      return [
        { key: 'type', name: 'Loại', value: f.type, set: set('assets', 'type'), options: named(types.data?.types ?? []) },
        { key: 'state', name: 'Trạng thái', value: f.state, set: set('assets', 'state'), options: ASSET_STATES.map((s) => ({ id: s, label: stateLabel(s) })) },
        { key: 'holder', name: 'Người giữ', value: f.holder, set: set('assets', 'holder'), options: everyone },
      ]
    }
    default:
      return []
  }
}
