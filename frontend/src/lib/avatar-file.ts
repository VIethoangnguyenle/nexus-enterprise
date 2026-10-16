import { ApiError } from '../api/client'

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
  if (err.status === 403) return 'Bạn không có quyền đổi ảnh này.'
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
