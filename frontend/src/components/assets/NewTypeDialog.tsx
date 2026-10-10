import { useId, useState } from 'react'
import { CATEGORIES, categoryLabel } from '../../lib/asset-model'
import { statusOf } from '../../lib/errors'
import { useCreateAssetType } from '../../hooks/useAssets'
import { AlertBanner } from '../composites/AlertBanner'
import { Dialog } from '../composites/Dialog'
import { Button, Select, TextField, toast } from '../primitives'

const NAME_MAX = 80

interface NewTypeDialogProps {
  open: boolean
  workspaceId: string
  /** The type was made; the screen opens it. */
  onCreated: (typeId: string) => void
  onClose: () => void
}

/** "Thêm loại tài sản": a name and a category. The category's key is sent; only its Vietnamese name is shown. */
export function NewTypeDialog({ open, workspaceId, onCreated, onClose }: NewTypeDialogProps) {
  const create = useCreateAssetType(workspaceId)
  return (
    <Dialog open={open} onClose={create.isPending ? () => {} : onClose} title="Thêm loại tài sản">
      {/* Mounted only while open, so every opening starts from a blank form. */}
      <TypeForm create={create} onCreated={onCreated} onClose={onClose} />
    </Dialog>
  )
}

function TypeForm({ create, onCreated, onClose }: {
  create: ReturnType<typeof useCreateAssetType>
  onCreated: (typeId: string) => void
  onClose: () => void
}) {
  const [name, setName] = useState('')
  const [category, setCategory] = useState('')
  const [errors, setErrors] = useState<{ name?: string; category?: string }>({})
  const [failure, setFailure] = useState<string | undefined>()
  const catId = useId()

  const submit = async () => {
    const trimmed = name.trim()
    const next = { name: trimmed ? undefined : 'Nhập tên loại tài sản.', category: category ? undefined : 'Chọn danh mục.' }
    setErrors(next)
    if (next.name || next.category) return
    setFailure(undefined)
    let created
    try {
      created = await create.mutateAsync({ name: trimmed, category })
    } catch (err) {
      const s = statusOf(err)
      setFailure(
        s === 403 ? 'Bạn chưa có quyền thêm loại tài sản. Cần quyền Quản lý trên vùng tài sản; hỏi quản trị viên.'
        : s === 409 ? 'Đã có loại tài sản tên này. Dùng tên khác.'
        : s === 400 ? 'Thông tin chưa hợp lệ. Kiểm tra lại rồi thử lại.'
        : 'Chưa thêm được loại tài sản. Kiểm tra kết nối mạng rồi thử lại.',
      )
      return
    }
    toast('Đã thêm loại tài sản')
    onCreated(created.id)
    onClose()
  }

  return (
    <form
      className="grid gap-4.5"
      noValidate
      onSubmit={(e) => {
        e.preventDefault()
        void submit()
      }}
    >
      <TextField
        label="Tên loại"
        value={name}
        maxLength={NAME_MAX}
        placeholder="Ví dụ: Laptop, Màn hình"
        error={errors.name}
        onChange={(e) => {
          setName(e.target.value)
          if (errors.name) setErrors((x) => ({ ...x, name: undefined }))
        }}
      />
      <div className="grid gap-1.5">
        <label htmlFor={catId} className="text-sm font-semibold text-ink">Danh mục</label>
        <Select
          id={catId}
          value={category}
          error={errors.category}
          onChange={(e) => {
            setCategory(e.target.value)
            if (errors.category) setErrors((x) => ({ ...x, category: undefined }))
          }}
        >
          <option value="">Chọn danh mục</option>
          {CATEGORIES.map((c) => (
            <option key={c} value={c}>{categoryLabel(c)}</option>
          ))}
        </Select>
      </div>
      {failure && <AlertBanner variant="error">{failure}</AlertBanner>}
      <div className="flex justify-end gap-2">
        <Button type="button" variant="soft" onClick={onClose} disabled={create.isPending}>Huỷ</Button>
        <Button type="submit" loading={create.isPending}>Thêm loại</Button>
      </div>
    </form>
  )
}
