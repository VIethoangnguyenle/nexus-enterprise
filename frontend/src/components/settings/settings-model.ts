import type { Contact } from '../../hooks/useContacts'
import type { ProfileChanges } from '../../api/profile'
import { ApiError } from '../../api/client'

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


/** The largest picture the server takes (auth service, domain.AvatarMaxBytes). */
export const AVATAR_MAX_BYTES = 5 * 1024 * 1024

/** The picture types the server takes; also the picker's `accept`. */
export const AVATAR_TYPES = ['image/jpeg', 'image/png']

const AVATAR_NOT_IMAGE = 'Chọn ảnh JPG hoặc PNG.'
const AVATAR_TOO_LARGE = 'Ảnh lớn hơn 5 MB. Chọn ảnh nhỏ hơn.'

/**
 * What is wrong with a chosen picture, before anything is sent; null when it
 * can be uploaded. The server checks again, by the file's content.
 */
export function validateAvatarFile(file: File): string | null {
  if (!AVATAR_TYPES.includes(file.type)) return AVATAR_NOT_IMAGE
  if (file.size > AVATAR_MAX_BYTES) return AVATAR_TOO_LARGE
  return null
}

/** A failed upload, in words that say what to do. Server text is never shown. */
export function avatarErrorMessage(err: unknown): string {
  if (!(err instanceof ApiError)) return 'Chưa đổi được ảnh. Kiểm tra kết nối mạng rồi thử lại.'
  const code = (err.body as { code?: unknown } | undefined)?.code
  switch (code) {
    case 'avatar_not_image': return AVATAR_NOT_IMAGE
    case 'avatar_too_large': return AVATAR_TOO_LARGE
    case 'avatar_too_many_pixels': return 'Ảnh quá lớn về kích thước. Chọn ảnh nhỏ hơn hoặc chụp lại.'
    case 'avatar_busy': return 'Máy chủ đang bận xử lý ảnh khác. Thử lại sau vài giây.'
    case 'rate_limited': return 'Bạn đã đổi ảnh nhiều lần trong giờ qua. Thử lại sau.'
  }
  return 'Chưa đổi được ảnh. Thử lại sau ít phút.'
}
