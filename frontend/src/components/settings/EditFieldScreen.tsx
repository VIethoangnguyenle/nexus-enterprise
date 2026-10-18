import { useEffect, useId, useRef, useState } from 'react'
import { Button, TextField, Textarea } from '../primitives'
import { SettingsPage } from './SettingsGroup'

export interface EditFieldScreenProps {
  /** Page title: the field's name ("Tên hiển thị"). */
  title: string
  initial: string
  /** A line under the field saying where the value shows. */
  hint?: string
  /** Characters the field takes; the server's own limit is checked by `validate`. */
  maxLength?: number
  /** What is wrong with a value, in words that say how to fix it; undefined when it can be saved. */
  validate?: (value: string) => string | undefined
  /** Several lines (Mô tả). */
  multiline?: boolean
  /** Saves the trimmed value. True closes the page; false keeps what was typed (the error is already told). */
  onSave: (value: string) => Promise<boolean>
  onBack: () => void
}

/**
 * Sửa một trường (DESIGN.md §6 "Cài đặt"): a page with the one field, Lưu in the top bar, on only when the value
 * changed and can be saved. Going back (the arrow or Esc) cancels; nothing is sent until Lưu.
 */
export function EditFieldScreen({ title, initial, hint, maxLength, validate, multiline, onSave, onBack }: EditFieldScreenProps) {
  const [value, setValue] = useState(initial)
  const [error, setError] = useState<string | undefined>()
  const [saving, setSaving] = useState(false)
  const ref = useRef<HTMLInputElement & HTMLTextAreaElement>(null)
  const id = useId()

  useEffect(() => { ref.current?.focus() }, [])
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape' && !e.defaultPrevented) onBack() }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onBack])

  const changed = value.trim() !== initial.trim()
  const submit = async () => {
    const problem = validate?.(value)
    setError(problem)
    if (problem || !changed || saving) return
    setSaving(true)
    const ok = await onSave(value.trim())
    setSaving(false)
    if (ok) onBack()
  }

  const onChange = (next: string) => {
    setValue(next)
    if (error) setError(validate?.(next))
  }

  return (
    <SettingsPage
      title={title}
      onBack={onBack}
      backLabel="Huỷ"
      action={
        <Button size="sm" loading={saving} disabled={!changed} onClick={() => void submit()}>
          Lưu
        </Button>
      }
    >
      <form noValidate aria-label={title} onSubmit={(e) => { e.preventDefault(); void submit() }} className="grid gap-2">
        {multiline ? (
          <>
            <label htmlFor={`${id}-field`} className="text-label text-ink-muted">{title}</label>
            <Textarea
              ref={ref}
              id={`${id}-field`}
              rows={4}
              value={value}
              maxLength={maxLength}
              error={error}
              aria-describedby={hint ? `${id}-hint` : undefined}
              onChange={(e) => onChange(e.target.value)}
            />
            {hint && <p id={`${id}-hint`} className="m-0 text-[0.8125rem] text-ink-muted">{hint}</p>}
          </>
        ) : (
          <TextField ref={ref} label={title} value={value} maxLength={maxLength} error={error} hint={hint} onChange={(e) => onChange(e.target.value)} />
        )}
      </form>
    </SettingsPage>
  )
}
