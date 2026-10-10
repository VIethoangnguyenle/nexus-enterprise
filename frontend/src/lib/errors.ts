import { ApiError } from '../api/client'

/** HTTP status of a failed request, when there is one. */
export function statusOf(err: unknown): number | undefined {
  return err instanceof ApiError ? err.status : undefined
}

/** The machine-readable `reason` a 409 carries, when the server gave one. */
export function reasonOf(err: unknown): string | undefined {
  const body = err instanceof ApiError ? (err.body as { reason?: unknown } | undefined) : undefined
  return typeof body?.reason === 'string' ? body.reason : undefined
}

/** Drive refuses to delete a folder that still holds text documents. */
export const FOLDER_HAS_DOCUMENTS = 'folder_has_documents'

/** The workspace's drive is full: the server answers 413 with this reason. */
export const QUOTA_EXCEEDED = 'quota_exceeded'

export const isForbidden = (err: unknown) => statusOf(err) === 403

/**
 * A sentence for a failed mutation: what went wrong and what to do about it.
 * Server error strings are not shown: they are English and can carry ids.
 */
export function explain(err: unknown, action: string): string {
  const status = statusOf(err)
  if (status === 403) return `Bạn chưa có quyền ${action}. Nhờ quản trị viên hoặc quản lý nhóm cấp quyền.`
  if (status === 404) return `Không ${action} được vì mục này không còn nữa. Tải lại trang rồi thử lại.`
  if (status === 409 && reasonOf(err) === FOLDER_HAS_DOCUMENTS) {
    return `Chưa ${action} được vì trong thư mục còn văn bản. Chuyển hoặc xoá các văn bản trước, rồi xoá thư mục.`
  }
  if (status === 409) return `Không ${action} được vì mục này vừa được người khác thay đổi. Tải lại trang để lấy bản mới nhất rồi làm lại.`
  if (status === 413) return `Chưa ${action} được vì kho tài liệu của workspace đã đầy. Xoá bớt tệp hoặc nhờ quản trị viên tăng dung lượng.`
  if (status === 400) return `Không ${action} được vì thông tin chưa hợp lệ. Kiểm tra lại rồi thử lại.`
  if (status && status >= 500) return `Máy chủ đang gặp sự cố nên chưa ${action} được. Thử lại sau ít phút.`
  return `Chưa ${action} được. Kiểm tra kết nối mạng rồi thử lại.`
}
