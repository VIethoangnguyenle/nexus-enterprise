import { useEffect, useRef, useState } from 'react'
import { Link } from '@tanstack/react-router'
import { Building2, Key, User } from 'lucide-react'
import type { WorkspaceDetails } from '../../api/workspaces'
import { useUpdateWorkspaceDetails } from '../../hooks/useWorkspaces'
import { formatDateTime, formatRelative } from '../../lib/format'
import { workspaceDisplayName } from '../../lib/workspace'
import { Avatar, Button, Textarea, TextField, toast } from '../primitives'

// The parts of a workspace's own information that both Cài đặt pages share: the rules for its name and description,
// the form that changes them, and the card that says what kind of workspace it is.

export const NAME_MAX = 100
export const DESCRIPTION_MAX = 500

export const checkName = (value: string) => {
  const v = value.trim()
  if (!v) return 'Nhập tên cho workspace.'
  if (Array.from(v).length > NAME_MAX) return `Tên dài tối đa ${NAME_MAX} ký tự.`
  return undefined
}
export const checkDescription = (value: string) =>
  Array.from(value.trim()).length > DESCRIPTION_MAX ? `Mô tả dài tối đa ${DESCRIPTION_MAX} ký tự.` : undefined

/**
 * The name an edit starts from. A personal workspace created under an internal handle ("user_42's workspace") starts
 * from the phrase shown everywhere else, so the handle never reaches a field; left as it is, nothing is sent.
 */
export const editableName = (name: string | undefined) => (name ? workspaceDisplayName(name) : '')

/** Name and description are the only parts of a workspace that the form can change. */
interface Form { name: string; description: string }

export interface WorkspaceDetailsFormProps {
  wsId: string
  details: WorkspaceDetails
  /** The form's accessible name. */
  label: string
  /** A personal workspace has a name and nothing to describe. */
  withDescription?: boolean
}

/**
 * The workspace's name and description as a form (DESIGN.md §6: text fields are a form with Lưu thay đổi). Only what
 * changed is sent; a refusal is told by the shared handler and the form keeps what was typed. For people the server
 * lets change them (`can_manage`); the caller shows everyone else the values read-only.
 */
export function WorkspaceDetailsForm({ wsId, details, label, withDescription = true }: WorkspaceDetailsFormProps) {
  const save = useUpdateWorkspaceDetails(wsId)
  const initial: Form = { name: editableName(details.name), description: details.description ?? '' }
  const [form, setForm] = useState<Form>(initial)
  const [nameError, setNameError] = useState<string | undefined>()
  const nameRef = useRef<HTMLInputElement>(null)

  // The saved values arrive (or change after a save): a form nobody has edited follows them.
  const seeded = useRef<Form | null>(null)
  useEffect(() => {
    const pristine = !seeded.current || (form.name === seeded.current.name && form.description === seeded.current.description)
    seeded.current = initial
    if (pristine) setForm(initial)
  }, [details])

  const name = form.name.trim()
  const description = form.description.trim()
  const changed = name !== initial.name || (withDescription && description !== initial.description)

  const submit = async () => {
    const err = checkName(form.name)
    setNameError(err)
    if (err) return nameRef.current?.focus()
    if (!changed) return
    const body: { name?: string; description?: string } = {}
    if (name !== initial.name) body.name = name
    if (withDescription && description !== initial.description) body.description = description
    try {
      await save.mutateAsync(body)
    } catch {
      return // told by the shared handler; the form keeps what was typed
    }
    toast('Đã lưu workspace')
  }

  return (
    <form
      aria-label={label}
      noValidate
      onSubmit={(e) => { e.preventDefault(); void submit() }}
      className="grid gap-5 max-w-130"
    >
      <TextField
        ref={nameRef}
        label="Tên workspace"
        value={form.name}
        maxLength={NAME_MAX + 20}
        error={nameError}
        onChange={(e) => {
          setForm((f) => ({ ...f, name: e.target.value }))
          if (nameError) setNameError(checkName(e.target.value))
        }}
        onBlur={() => setNameError(checkName(form.name))}
      />
      {withDescription && (
        <div className="grid gap-1.5">
          <label htmlFor="workspace-description" className="text-sm font-semibold text-ink">
            Mô tả <span className="font-normal text-ink-muted">(không bắt buộc)</span>
          </label>
          <Textarea
            id="workspace-description"
            rows={3}
            maxLength={DESCRIPTION_MAX}
            value={form.description}
            onChange={(e) => setForm((f) => ({ ...f, description: e.target.value }))}
          />
        </div>
      )}

      <div className="flex items-center gap-2">
        <Button type="submit" loading={save.isPending} disabled={!changed}>Lưu thay đổi</Button>
        <Button
          type="button"
          variant="ghost"
          disabled={!changed || save.isPending}
          onClick={() => { setForm(initial); setNameError(undefined) }}
        >
          Huỷ
        </Button>
      </div>
    </form>
  )
}

/** Informational card showing the workspace type. No conversion action. */
export function WorkspaceTypeCard({ details }: { details: WorkspaceDetails }) {
  if (details.type === 'personal') {
    return (
      <section aria-labelledby="workspace-type-title" className="grid gap-3 p-4 rounded-surface bg-raised">
        <div className="flex items-start gap-3">
          <span
            aria-hidden="true"
            className="grid place-items-center w-10 h-10 rounded-lg bg-sunk text-ink-muted shrink-0"
          >
            <User size={20} strokeWidth={1.75} />
          </span>
          <div className="grid gap-1 min-w-0">
            <h3 id="workspace-type-title" className="font-semibold text-16 text-ink m-0">
              Workspace cá nhân
            </h3>
            <span className="text-sm text-ink-muted">
              Có Tin nhắn, Tài liệu và Danh bạ. Không có Tài sản, Phê duyệt và duyệt tài liệu.
            </span>
          </div>
        </div>
      </section>
    )
  }

  // A type this build does not know is not guessed at: the card is left out.
  if (details.type !== 'organization') return null

  return (
    <section aria-labelledby="workspace-type-title" className="grid gap-3 p-4 rounded-surface bg-raised">
      <div className="flex items-start gap-3">
        <span
          aria-hidden="true"
          className="grid place-items-center w-10 h-10 rounded-lg bg-accent-wash text-accent shrink-0"
        >
          <Building2 size={20} strokeWidth={1.75} />
        </span>
        <div className="grid gap-1 min-w-0">
          <h3 id="workspace-type-title" className="font-semibold text-16 text-ink m-0">
            Workspace tổ chức
          </h3>
          <span className="text-sm text-ink-muted">
            Có đủ Tin nhắn, Tài liệu, Phê duyệt, Tài sản và Danh bạ.
          </span>
        </div>
      </div>

      {details.converted_by_name && (
        <div className="flex items-center gap-2 pl-13 max-sm:pl-0 text-sm text-ink-muted flex-wrap">
          <Avatar size={20} name={details.converted_by_name} />
          <span>
            <strong className="text-ink font-semibold">{details.converted_by_name}</strong> đã chuyển sang workspace tổ chức
            {details.converted_at && (
              <>
                {' · '}
                <time dateTime={details.converted_at} title={formatDateTime(details.converted_at)}>
                  {formatRelative(details.converted_at) || 'vừa xong'}
                </time>
              </>
            )}
          </span>
        </div>
      )}

      {details.converted_by_name && (
        <div className="pl-13 max-sm:pl-0">
          <Link
            to="/admin"
            search={(prev: { ws?: string }) => (prev.ws ? { ws: prev.ws } : {})}
            className="inline-flex items-center gap-2 h-8 max-sm:min-h-11 px-3 rounded-md bg-hover text-small-ui font-semibold text-ink no-underline
              hover:bg-line focus-ring transition-colors duration-quick max-sm:w-full max-sm:justify-center"
          >
            <Key size={16} strokeWidth={1.75} aria-hidden="true" className="shrink-0" />
            Đặt quyền Phê duyệt và Tài sản
          </Link>
        </div>
      )}
    </section>
  )
}
