import { useId, useMemo, useRef, useState } from 'react'
import { Package } from 'lucide-react'
import type { AssetType, Urgency } from '../../api/assets'
import { useCreateAssetRequest } from '../../hooks/useAssets'
import { URGENCIES, categoryLabel, urgencyLabel } from '../../lib/asset-model'
import { statusOf } from '../../lib/errors'
import { AlertBanner } from '../composites/AlertBanner'
import { Button, ChoicePicker, Pressable, Textarea, toast, type Choice } from '../primitives'
import { SidePanel } from '../spaces/SidePanel'

const REASON_MAX = 1000
const FORM_ID = 'new-asset-request'

interface NewRequestPanelProps {
  workspaceId: string
  /** The types the caller may ask for (write on the type). */
  types: AssetType[]
  loading: boolean
  /** The request was sent; the screen shows the caller's own requests. */
  onSent: () => void
  onClose: () => void
}

/**
 * "Yêu cầu tài sản" (mockup §4): a form in the detail panel — asset type from a
 * searchable list, urgency as a segmented control, and the reason (required,
 * checked when the user leaves the field or sends). Nothing here asks for an id.
 */
export function NewRequestPanel({ workspaceId, types, loading, onSent, onClose }: NewRequestPanelProps) {
  const create = useCreateAssetRequest(workspaceId)
  const [type, setType] = useState<Choice | null>(null)
  const [urgency, setUrgency] = useState<Urgency>('normal')
  const [reason, setReason] = useState('')
  const [errors, setErrors] = useState<{ type?: string; reason?: string }>({})
  const [failure, setFailure] = useState<string | undefined>()
  const reasonRef = useRef<HTMLTextAreaElement>(null)
  const id = useId()
  const urgencyId = `${id}-urgency`

  const choices: Choice[] = useMemo(
    () => types.map((t) => ({
      id: t.id,
      name: t.name,
      hint: [categoryLabel(t.category), t.available_count !== undefined ? (t.available_count > 0 ? `${t.available_count} sẵn sàng` : 'Hết tài sản trống') : '']
        .filter(Boolean).join(' · '),
    })),
    [types],
  )

  const checkReason = (text: string) => (text.trim() ? undefined : 'Nhập lý do để người duyệt hiểu bạn cần gì.')

  const submit = async () => {
    const next = { type: type ? undefined : 'Chọn loại tài sản bạn cần.', reason: checkReason(reason) }
    setErrors(next)
    if (next.type || next.reason) {
      if (!next.type) reasonRef.current?.focus()
      return
    }
    setFailure(undefined)
    try {
      await create.mutateAsync({ type_id: type!.id, reason: reason.trim(), urgency })
    } catch (err) {
      const s = statusOf(err)
      setFailure(
        s === 403 ? 'Bạn chưa có quyền yêu cầu loại tài sản này. Nhờ quản trị viên cấp quyền.'
        : s === 400 ? 'Thông tin chưa hợp lệ. Kiểm tra lại rồi gửi.'
        : 'Chưa gửi được yêu cầu. Kiểm tra kết nối mạng rồi thử lại.',
      )
      return
    }
    toast('Đã gửi yêu cầu')
    onSent()
  }

  return (
    <SidePanel
      label="Yêu cầu tài sản mới"
      title="Yêu cầu tài sản"
      closeLabel="biểu mẫu"
      onClose={onClose}
      footer={
        <div className="flex justify-end gap-2 px-4 pt-2 pb-4">
          <Button type="button" variant="ghost" onClick={onClose} disabled={create.isPending}>Huỷ</Button>
          <Button type="submit" form={FORM_ID} loading={create.isPending}>Gửi yêu cầu</Button>
        </div>
      }
    >
      <form
        id={FORM_ID}
        className="grid gap-4.5 px-3 pt-1 pb-3"
        noValidate
        onSubmit={(e) => {
          e.preventDefault()
          void submit()
        }}
      >
        <div className="grid gap-1.5">
          <span className="text-sm font-semibold text-ink">Loại tài sản</span>
          {!loading && types.length === 0 ? (
            <p className="m-0 text-sm text-ink-muted">
              Bạn chưa được phép yêu cầu loại tài sản nào. Nhờ quản trị viên cấp quyền hoặc tạo loại tài sản trước.
            </p>
          ) : (
            <ChoicePicker
              label="Chọn loại tài sản"
              choices={choices}
              value={type}
              onChange={(c) => {
                setType(c)
                if (c) setErrors((e) => ({ ...e, type: undefined }))
              }}
              icon={<Package size={16} strokeWidth={1.75} />}
              placeholder="Tìm loại tài sản"
              emptyText="Không có loại nào khớp."
              loading={loading}
              invalid={!!errors.type}
            />
          )}
          {errors.type && <span role="alert" className="text-xs font-medium text-danger">{errors.type}</span>}
        </div>

        <div className="grid gap-1.5">
          <span id={urgencyId} className="text-sm font-semibold text-ink">Mức độ</span>
          <div role="radiogroup" aria-labelledby={urgencyId} className="inline-flex flex-wrap gap-1 p-0.75 rounded-lg bg-sunk justify-self-start">
            {URGENCIES.map((u) => {
              const on = u === urgency
              return (
                <Pressable
                  key={u}
                  role="radio"
                  aria-checked={on}
                  onClick={() => setUrgency(u)}
                  className={`h-9 px-3 rounded-md text-small-ui font-semibold transition-colors duration-quick
                    ${on ? 'bg-raised text-ink' : 'text-ink-muted hover:text-ink'}`}
                >
                  {urgencyLabel(u)}
                </Pressable>
              )
            })}
          </div>
        </div>

        <div className="grid gap-1.5">
          <label htmlFor={`${id}-why`} className="text-sm font-semibold text-ink">Lý do</label>
          <Textarea
            ref={reasonRef}
            id={`${id}-why`}
            rows={4}
            maxLength={REASON_MAX}
            value={reason}
            aria-invalid={!!errors.reason || undefined}
            aria-describedby={`${id}-why-hint`}
            onChange={(e) => {
              setReason(e.target.value)
              if (errors.reason) setErrors((x) => ({ ...x, reason: checkReason(e.target.value) }))
            }}
            onBlur={() => setErrors((x) => ({ ...x, reason: checkReason(reason) }))}
          />
          <span
            id={`${id}-why-hint`}
            role={errors.reason ? 'alert' : undefined}
            className={`text-xs ${errors.reason ? 'font-medium text-danger' : 'text-ink-muted'}`}
          >
            {errors.reason ?? 'Người duyệt thấy nội dung này. Nói rõ dùng cho việc gì.'}
          </span>
        </div>

        {failure && <AlertBanner variant="error">{failure}</AlertBanner>}
      </form>
    </SidePanel>
  )
}
