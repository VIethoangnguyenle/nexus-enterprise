import { type ButtonHTMLAttributes, forwardRef } from 'react'

type NavRowKind = 'groupHeader' | 'subItem' | 'navItem'

interface NavRowProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  kind: NavRowKind
  active?: boolean
}

/**
 * Dòng điều hướng bấm được — sidebar item và node cây.
 * Không phải `Button`: đây là phần tử điều hướng, không phải nút hành động.
 * Mọi trạng thái lấy từ `.stitch/DESIGN.md` §Navigation Sidebar.
 */
/**
 * `gap` và bo góc nằm ở đây chứ KHÔNG ở chuỗi base. Nếu để `gap-2 rounded-md` ở
 * base, chúng sẽ chọi với giá trị của `navItem` (`gap-3 rounded-lg`), hoà nhau về
 * độ đặc hiệu, và Tailwind xử theo thứ tự stylesheet — kind mới sẽ thua âm thầm.
 * Đúng cơ chế đã làm variant `secondary` của `Button` mất viền trên toàn app.
 * Xem spec 2026-08-01 §7.5.
 */
const layoutStyles: Record<NavRowKind, string> = {
  groupHeader: 'gap-2 rounded-md px-3 py-2 text-body-ui',
  subItem: 'gap-2 rounded-md pl-9 pr-2 py-1.5 text-small-ui',
  navItem: 'gap-3 rounded-lg px-3 py-2.5 text-sm font-semibold',
}

const activeStyles: Record<NavRowKind, string> = {
  groupHeader: 'bg-surface-container text-primary font-medium',
  subItem: 'bg-primary-fixed text-on-primary-fixed-variant font-medium',
  navItem: 'bg-surface-container-highest text-primary shadow-sm',
}

const inactiveStyles: Record<NavRowKind, string> = {
  groupHeader: 'text-on-surface-variant hover:bg-surface-container-low',
  subItem: 'text-on-surface-variant hover:bg-surface-container-low',
  navItem: 'bg-transparent text-on-surface-variant hover:bg-surface-container hover:text-on-surface',
}

export const NavRow = forwardRef<HTMLButtonElement, NavRowProps>(
  ({ kind, active = false, className = '', children, ...props }, ref) => (
    <button
      ref={ref}
      aria-current={active ? 'page' : undefined}
      className={`w-full flex items-center text-left
        transition-colors duration-fast cursor-pointer border-none focus-ring
        ${layoutStyles[kind]}
        ${active ? activeStyles[kind] : inactiveStyles[kind]}
        ${className}`}
      {...props}
    >
      {children}
    </button>
  ),
)

NavRow.displayName = 'NavRow'
