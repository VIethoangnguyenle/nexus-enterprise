import { type ButtonHTMLAttributes, forwardRef } from 'react'

type NavRowKind = 'groupHeader' | 'subItem'

interface NavRowProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  kind: NavRowKind
  active?: boolean
}

/**
 * Dòng điều hướng bấm được — sidebar item và node cây.
 * Không phải `Button`: đây là phần tử điều hướng, không phải nút hành động.
 * Mọi trạng thái lấy từ `.stitch/DESIGN.md` §Navigation Sidebar.
 */
const layoutStyles: Record<NavRowKind, string> = {
  groupHeader: 'px-3 py-2 text-body-ui',
  subItem: 'pl-9 pr-2 py-1.5 text-small-ui',
}

const activeStyles: Record<NavRowKind, string> = {
  groupHeader: 'bg-surface-container text-primary font-medium',
  subItem: 'bg-primary-fixed text-on-primary-fixed-variant font-medium',
}

const inactiveStyles = 'text-on-surface-variant hover:bg-surface-container-low'

export const NavRow = forwardRef<HTMLButtonElement, NavRowProps>(
  ({ kind, active = false, className = '', children, ...props }, ref) => (
    <button
      ref={ref}
      aria-current={active ? 'page' : undefined}
      className={`w-full flex items-center gap-2 rounded-md text-left
        transition-colors duration-fast cursor-pointer border-none focus-ring
        ${layoutStyles[kind]}
        ${active ? activeStyles[kind] : inactiveStyles}
        ${className}`}
      {...props}
    >
      {children}
    </button>
  ),
)

NavRow.displayName = 'NavRow'
