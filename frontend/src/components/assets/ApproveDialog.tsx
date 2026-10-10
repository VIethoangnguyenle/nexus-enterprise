import { useMemo, useRef, useState } from 'react'
import { Package } from 'lucide-react'
import type { Asset, AssetRequest } from '../../api/assets'
import { useAssets } from '../../hooks/useAssets'
import { formatFieldValue, type FieldDef } from '../../lib/asset-fields'
import { UNKNOWN_PERSON, type PeopleDirectory } from '../../lib/people'
import { AlertBanner } from '../composites/AlertBanner'
import { Dialog } from '../composites/Dialog'
import { Button, ChoicePicker, type Choice } from '../primitives'

export type ApproveMode = 'approve' | 'assign'

interface ApproveDialogProps {
  /** The request being decided; `null` closes the dialog. */
  request: AssetRequest | null
  /** `approve`: decide and give in one step. `assign`: the request was approved earlier; give it an asset. */
  mode: ApproveMode
  workspaceId: string
  /** The custom fields of the request's type, to tell two assets apart in the list. */
  fields: FieldDef[]
  people: PeopleDirectory
  pending: boolean
  /** Why the last attempt failed, in words; shown inside the dialog. */
  failure?: string
  /** `assetId` is empty only for "Chỉ duyệt", offered when nothing is available to give. */
  onConfirm: (request: AssetRequest, assetId: string) => void
  onClose: () => void
}

/**
 * "Duyệt và giao tài sản" (mockup §3): the approver picks, in this one dialog, a
 * specific available asset of the requested type, and approving hands it over.
 * Nothing is typed as an id: assets are chosen from a list by name.
 */
export function ApproveDialog(props: ApproveDialogProps) {
  const { request, mode, pending, onClose } = props
  // Keep the last request while the dialog fades out, so its text does not blank.
  const last = useRef<AssetRequest | null>(null)
  if (request) last.current = request
  const shown = request ?? last.current

  return (
    <Dialog
      open={!!request}
      onClose={pending ? () => {} : onClose}
      title={mode === 'assign' ? 'Giao tài sản' : 'Duyệt và giao tài sản'}
    >
      {/* Keyed by request: another request starts with nothing chosen, never the last one's pick. */}
      {shown && <AssetChoice key={`${shown.id}:${mode}`} {...props} request={shown} />}
    </Dialog>
  )
}

function AssetChoice({ request, mode, workspaceId, fields, people, pending, failure, onConfirm, onClose }: ApproveDialogProps & { request: AssetRequest }) {
  const available = useAssets(workspaceId, { type_id: request.type_id, state: 'available', limit: 100 })
  const [picked, setPicked] = useState<Choice | null>(null)
  const [error, setError] = useState<string | undefined>()
  const who = request.requester_name || UNKNOWN_PERSON
  const type = request.type_name || 'tài sản'

  const choices: Choice[] = useMemo(
    () => (available.data?.assets ?? []).map((a) => ({ id: a.id, name: a.name, hint: hintOf(a, fields, people) })),
    [available.data, fields, people],
  )
  const none = !!available.data && choices.length === 0

  const submit = () => {
    if (!picked) {
      setError('Chọn tài sản để giao.')
      return
    }
    onConfirm(request, picked.id)
  }

  return (
    <form
      className="grid gap-4.5"
      onSubmit={(e) => {
        e.preventDefault()
        submit()
      }}
    >
      <p className="m-0 text-sm text-ink-muted leading-relaxed">
        <b className="font-semibold text-ink">{who}</b> sẽ nhận tài sản bạn chọn
        {mode === 'approve' ? ' ngay khi yêu cầu được duyệt.' : '.'}
      </p>

      <div className="grid gap-1.5">
        <span id={`pick-${request.id}`} className="text-sm font-semibold text-ink">Tài sản sẽ giao</span>
        <ChoicePicker
          label="Chọn tài sản"
          choices={choices}
          value={picked}
          onChange={(c) => {
            setPicked(c)
            if (error) setError(undefined)
          }}
          icon={<Package size={16} strokeWidth={1.75} />}
          placeholder="Tìm theo tên tài sản"
          emptyText="Không có tài sản sẵn sàng nào khớp."
          loading={available.isLoading}
          invalid={!!error}
        />
        <span role={error ? 'alert' : undefined} className={`text-xs ${error ? 'font-medium text-danger' : 'text-ink-muted'}`}>
          {error ?? `Chỉ liệt kê tài sản Sẵn sàng thuộc loại ${type}.`}
        </span>
      </div>

      {available.isError && <AlertBanner variant="error">Không tải được danh sách tài sản sẵn sàng. Đóng hộp này rồi mở lại.</AlertBanner>}
      {none && (
        <p role="status" className="m-0 px-3 py-2 rounded-surface bg-warning-wash text-sm text-ink">
          Chưa có tài sản nào Sẵn sàng thuộc loại {type}
          {mode === 'approve' ? '. Bạn có thể chỉ duyệt bây giờ và giao khi có tài sản.' : ' để giao.'}
        </p>
      )}
      {failure && <AlertBanner variant="error">{failure}</AlertBanner>}

      <div className="flex justify-end gap-2">
        <Button type="button" variant="soft" onClick={onClose} disabled={pending}>Huỷ</Button>
        {none && mode === 'approve' && (
          <Button type="button" variant="tonal" loading={pending} onClick={() => onConfirm(request, '')}>Chỉ duyệt</Button>
        )}
        <Button type="submit" loading={pending} disabled={none}>{mode === 'assign' ? 'Giao tài sản' : 'Duyệt và giao'}</Button>
      </div>
    </form>
  )
}

/** The first thing about an asset that tells it from the next: its first filled-in field. */
function hintOf(a: Asset, fields: FieldDef[], people: PeopleDirectory): string | undefined {
  for (const f of fields) {
    const v = formatFieldValue(f, a.custom_fields?.[f.key], people)
    if (v) return v
  }
  return undefined
}
