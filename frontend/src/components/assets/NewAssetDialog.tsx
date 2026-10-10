import { useMemo, useState } from 'react'
import { Package } from 'lucide-react'
import type { AssetType } from '../../api/assets'
import { useCreateAsset } from '../../hooks/useAssets'
import { parseFields, validateFieldValues, valuesToPayload } from '../../lib/asset-fields'
import { categoryLabel } from '../../lib/asset-model'
import { statusOf } from '../../lib/errors'
import type { PeopleDirectory } from '../../lib/people'
import { AlertBanner } from '../composites/AlertBanner'
import { Dialog } from '../composites/Dialog'
import { Button, ChoicePicker, TextField, toast, type Choice } from '../primitives'
import { FieldInput } from './FieldInput'

const NAME_MAX = 120

interface NewAssetDialogProps {
  open: boolean
  workspaceId: string
  /** The types the caller may add assets to (write on the type). */
  types: AssetType[]
  people: PeopleDirectory
  /** The asset was added; the screen opens it. */
  onCreated: (assetId: string) => void
  onClose: () => void
}

/** "Thêm tài sản": the type from a list, a name, and the type's own fields. */
export function NewAssetDialog({ open, workspaceId, types, people, onCreated, onClose }: NewAssetDialogProps) {
  const create = useCreateAsset(workspaceId)
  return (
    <Dialog open={open} onClose={create.isPending ? () => {} : onClose} title="Thêm tài sản">
      {/* Mounted only while open, so every opening starts from a blank form. */}
      <AssetForm create={create} types={types} people={people} onCreated={onCreated} onClose={onClose} />
    </Dialog>
  )
}

function AssetForm({ create, types, people, onCreated, onClose }: Omit<NewAssetDialogProps, 'open' | 'workspaceId'> & {
  create: ReturnType<typeof useCreateAsset>
}) {
  const [type, setType] = useState<Choice | null>(null)
  const [name, setName] = useState('')
  const [values, setValues] = useState<Record<string, string>>({})
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [failure, setFailure] = useState<string | undefined>()

  const choices: Choice[] = useMemo(() => types.map((t) => ({ id: t.id, name: t.name, hint: categoryLabel(t.category) })), [types])
  const chosen = types.find((t) => t.id === type?.id)
  const fields = useMemo(() => parseFields(chosen?.fields_schema), [chosen])

  const submit = async () => {
    const found: Record<string, string> = {
      ...(type ? {} : { type: 'Chọn loại tài sản.' }),
      ...(name.trim() ? {} : { name: 'Nhập tên tài sản.' }),
      ...validateFieldValues(fields, values),
    }
    setErrors(found)
    if (Object.keys(found).length > 0) return
    setFailure(undefined)
    let created
    try {
      created = await create.mutateAsync({ type_id: type!.id, name: name.trim(), custom_fields: valuesToPayload(fields, values) })
    } catch (err) {
      const s = statusOf(err)
      setFailure(
        s === 403 ? 'Bạn chưa có quyền thêm tài sản thuộc loại này. Nhờ quản trị viên cấp quyền.'
        : s === 400 ? 'Thông tin chưa hợp lệ. Kiểm tra các trường rồi thử lại.'
        : 'Chưa thêm được tài sản. Kiểm tra kết nối mạng rồi thử lại.',
      )
      return
    }
    toast('Đã thêm tài sản')
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
      <div className="grid gap-1.5">
        <span className="text-sm font-semibold text-ink">Loại tài sản</span>
        <ChoicePicker
          label="Chọn loại tài sản"
          choices={choices}
          value={type}
          onChange={(c) => {
            setType(c)
            setValues({})
            setErrors({})
          }}
          icon={<Package size={16} strokeWidth={1.75} />}
          placeholder="Tìm loại tài sản"
          emptyText="Không có loại nào khớp."
          invalid={!!errors.type}
        />
        {errors.type && <span role="alert" className="text-xs font-medium text-danger">{errors.type}</span>}
      </div>
      <TextField
        label="Tên tài sản"
        value={name}
        maxLength={NAME_MAX}
        placeholder="Ví dụ: MacBook Pro 14 inch, máy số 8"
        error={errors.name}
        onChange={(e) => {
          setName(e.target.value)
          if (errors.name) setErrors(({ name: _gone, ...rest }) => rest)
        }}
      />
      {fields.map((f) => (
        <FieldInput
          key={f.key}
          field={f}
          value={values[f.key] ?? ''}
          error={errors[f.key]}
          people={people}
          disabled={create.isPending}
          onChange={(v) => {
            setValues((all) => ({ ...all, [f.key]: v }))
            if (errors[f.key]) setErrors(({ [f.key]: _gone, ...rest }) => rest)
          }}
        />
      ))}
      {failure && <AlertBanner variant="error">{failure}</AlertBanner>}
      <div className="flex justify-end gap-2">
        <Button type="button" variant="soft" onClick={onClose} disabled={create.isPending}>Huỷ</Button>
        <Button type="submit" loading={create.isPending}>Thêm tài sản</Button>
      </div>
    </form>
  )
}
