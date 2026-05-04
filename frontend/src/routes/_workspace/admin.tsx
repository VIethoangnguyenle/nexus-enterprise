import { createFileRoute, Outlet, Link, useMatches } from '@tanstack/react-router'
import { Building2, Users, Shield } from 'lucide-react'

export const Route = createFileRoute('/_workspace/admin')({
  component: AdminLayout,
})

const adminNavItems = [
  { to: '/admin', label: 'Organization', icon: Building2, exact: true },
  { to: '/admin/users', label: 'Users', icon: Users, exact: false },
  { to: '/admin/roles', label: 'Roles', icon: Shield, exact: false },
] as const

/** Admin layout: sub-sidebar (240px) + content outlet.
 *  This is the content area — AppSidebar/TopBar are provided by _workspace.tsx. */
function AdminLayout() {
  const matches = useMatches()
  const currentPath = matches[matches.length - 1]?.pathname || ''

  return (
    <div className="flex-1 flex min-h-0">
      {/* Admin sub-sidebar — desktop only */}
      <aside
        className="hidden lg:flex w-60 shrink-0 bg-surface-container-low border-r border-outline-variant/30
          flex-col overflow-hidden"
      >
        {/* Section header */}
        <div className="px-4 pt-5 pb-2">
          <span className="text-label-caps text-on-surface-variant uppercase tracking-wider text-[11px] font-semibold">
            Admin
          </span>
        </div>

        {/* Nav items */}
        <nav className="flex flex-col gap-0.5 px-2">
          {adminNavItems.map((item) => {
            const Icon = item.icon
            const isActive = item.exact
              ? currentPath === '/admin' || currentPath === '/admin/'
              : currentPath.startsWith(item.to)

            return (
              <Link
                key={item.to}
                to={item.to}
                className={`flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium
                  transition-all duration-150 no-underline border-none cursor-pointer
                  ${isActive
                    ? 'bg-primary-container text-on-primary-container font-semibold'
                    : 'text-on-surface-variant hover:bg-surface-container hover:text-on-surface'
                  }`}
              >
                <Icon size={18} strokeWidth={isActive ? 2.2 : 1.6} />
                <span>{item.label}</span>
              </Link>
            )
          })}
        </nav>
      </aside>

      {/* Mobile: horizontal tabs */}
      <div className="lg:hidden flex border-b border-outline-variant bg-surface-container-lowest shrink-0
        absolute top-0 left-0 right-0 z-10">
        {adminNavItems.map((item) => {
          const Icon = item.icon
          const isActive = item.exact
            ? currentPath === '/admin' || currentPath === '/admin/'
            : currentPath.startsWith(item.to)

          return (
            <Link
              key={item.to}
              to={item.to}
              className={`flex-1 flex items-center justify-center gap-2 py-3 text-sm font-medium
                no-underline border-b-2 transition-colors
                ${isActive
                  ? 'border-primary text-primary'
                  : 'border-transparent text-on-surface-variant hover:text-on-surface'
                }`}
            >
              <Icon size={16} />
              <span>{item.label}</span>
            </Link>
          )
        })}
      </div>

      {/* Content outlet */}
      <div className="flex-1 flex flex-col min-h-0 min-w-0 overflow-hidden">
        <Outlet />
      </div>
    </div>
  )
}
