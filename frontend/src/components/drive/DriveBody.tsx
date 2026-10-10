import type { ReactNode } from 'react'
import { Link } from '@tanstack/react-router'
import { motion } from 'motion/react'
import { CircleAlert, FolderOpen, Upload, UserPlus } from 'lucide-react'
import { folderSearch, type DriveSearch } from '../../lib/drive-search'
import type { MotionPresets } from '../../lib/motion'
import { Button } from '../primitives'
import { EmptyState } from '../spaces/EmptyState'

interface Props {
  /** The folder in the link is gone, forbidden or from another workspace. */
  missing: boolean
  isError: boolean
  onRetry: () => void
  loading: boolean
  shared: boolean
  /** Rows the folder holds, before the search filter. */
  total: number
  /** Rows left after the search filter. */
  visible: number
  /** Keeps the table up while the last row folds away. */
  holdTable: boolean
  onClearQuery: () => void
  onPickFiles: () => void
  view: string
  route: MotionPresets['route']
  /** The table itself, rendered by the screen. */
  table: ReactNode
}

/** What fills the scroll area of Tài liệu: an explanation, or the table. */
export function DriveBody({
  missing, isError, onRetry, loading, shared, total, visible, holdTable, onClearQuery, onPickFiles, view, route, table,
}: Props) {
  if (missing) {
    return (
      <EmptyState
        icon={<CircleAlert size={24} strokeWidth={1.75} />}
        text="Không mở được thư mục này. Có thể nó đã bị xoá hoặc bạn chưa có quyền xem."
        action={
          <Link
            to="/drive"
            search={(prev: DriveSearch) => folderSearch(prev)}
            className="inline-flex items-center h-8 px-3 rounded-md bg-hover text-small-ui font-semibold text-ink
              no-underline hover:bg-line focus-ring"
          >
            Về thư mục gốc
          </Link>
        }
      />
    )
  }
  if (isError) {
    return (
      <EmptyState
        icon={<CircleAlert size={24} strokeWidth={1.75} />}
        text="Không tải được danh sách tệp. Kiểm tra kết nối rồi thử lại."
        action={<Button variant="soft" size="sm" onClick={onRetry}>Thử lại</Button>}
      />
    )
  }
  if (!loading && total === 0 && !holdTable) {
    return shared ? (
      <EmptyState
        icon={<UserPlus size={24} strokeWidth={1.75} />}
        text="Chưa có ai chia sẻ tệp với bạn. Tệp được chia sẻ sẽ hiện ở đây."
      />
    ) : (
      <EmptyState
        icon={<FolderOpen size={24} strokeWidth={1.75} />}
        text="Thư mục này chưa có tệp. Kéo tệp vào đây hoặc tải lên từ máy. Người có quyền trong thư mục sẽ thấy ngay."
        action={
          <Button size="sm" onClick={onPickFiles}>
            <Upload size={16} strokeWidth={1.75} aria-hidden="true" />
            Tải lên
          </Button>
        }
      />
    )
  }
  if (!loading && total > 0 && visible === 0) {
    return (
      <EmptyState
        icon={<FolderOpen size={24} strokeWidth={1.75} />}
        text="Không có tệp nào khớp."
        action={
          <Button variant="soft" size="sm" onClick={onClearQuery}>
            Xoá tìm kiếm
          </Button>
        }
      />
    )
  }
  return (
    <motion.div key={view} {...route}>
      {table}
    </motion.div>
  )
}
