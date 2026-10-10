import type { Contact } from '../../hooks/useContacts'
import type { ProfileChanges } from '../../api/profile'

/** The limits the server enforces (auth service), counted in characters. */
export const NAME_MAX = 80
export const FIELD_MAX = 120

export interface ProfileForm {
  displayName: string
  title: string
  location: string
}

export const profileOf = (c: Pick<Contact, 'display_name' | 'title' | 'location'> | undefined): ProfileForm => ({
  displayName: c?.display_name ?? '',
  title: c?.title ?? '',
  location: c?.location ?? '',
})

export type ProfileErrors = Partial<Record<keyof ProfileForm, string>>

const length = (s: string) => Array.from(s.trim()).length
 
const CONTROL = /[\u0000-\u001f\u007f]/

/** What is wrong with a form, in words that say how to fix it. Empty when it can be saved. */
export function validateProfile(form: ProfileForm): ProfileErrors {
  const errors: ProfileErrors = {}
  if (!form.displayName.trim()) errors.displayName = 'Nhập tên để mọi người nhận ra bạn.'
  else if (length(form.displayName) > NAME_MAX) errors.displayName = `Tên dài tối đa ${NAME_MAX} ký tự.`
  else if (CONTROL.test(form.displayName)) errors.displayName = 'Tên có ký tự không dùng được. Gõ lại bằng chữ thường.'
  for (const key of ['title', 'location'] as const) {
    if (length(form[key]) > FIELD_MAX) errors[key] = `Tối đa ${FIELD_MAX} ký tự.`
    else if (CONTROL.test(form[key])) errors[key] = 'Có ký tự không dùng được. Gõ lại bằng chữ thường.'
  }
  return errors
}

/**
 * Only the fields that differ from what is stored. Sending a field that did not
 * change would overwrite whatever someone else (an administrator) set in the
 * meantime. Null when nothing changed.
 */
export function profileChanges(initial: ProfileForm, form: ProfileForm): ProfileChanges | null {
  const out: ProfileChanges = {}
  if (form.displayName.trim() !== initial.displayName.trim()) out.display_name = form.displayName.trim()
  if (form.title.trim() !== initial.title.trim()) out.title = form.title.trim()
  if (form.location.trim() !== initial.location.trim()) out.location = form.location.trim()
  return Object.keys(out).length ? out : null
}
