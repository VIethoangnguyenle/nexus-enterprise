/**
 * Lifecycle state → color mapping for asset state badges.
 *
 * Đọc từ token chức năng của design system thay vì nướng hex. Ánh xạ theo ngữ
 * nghĩa chứ không theo sắc độ gần nhất: `in_use` là trạng thái hoạt động nhất
 * nên lấy `primary`, `assigned` là thông tin trung tính nên lấy `info`.
 * Giá trị dùng trong `style` inline nên phải là biến CSS, không phải tên class.
 */
export const ASSET_STATE_COLORS: Record<string, string> = {
  requested: 'var(--color-warning)',
  approved: 'var(--color-success)',
  assigned: 'var(--color-info)',
  in_use: 'var(--color-primary)',
  returned: 'var(--color-outline)',
  disposed: 'var(--color-error)',
}

/** Request status → color mapping for asset request badges. */
export const REQUEST_STATUS_COLORS: Record<string, string> = {
  pending: 'var(--color-warning)',
  approved: 'var(--color-success)',
  rejected: 'var(--color-error)',
}
