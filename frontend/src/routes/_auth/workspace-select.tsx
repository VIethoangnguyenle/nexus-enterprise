import { useEffect } from 'react'
import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useWorkspaces, useCreateWorkspace } from '../../hooks/useWorkspaces'
import { useAuthStore } from '../../stores/auth.store'
import { Spinner } from '../../components/primitives'
import { Briefcase, ChevronRight, Plus, Users } from 'lucide-react'

export const Route = createFileRoute('/_auth/workspace-select')({
  component: WorkspaceSelectPage,
})

/** Workspace selection page matching Stitch source:
 *  Desktop: .stitch/designs/workspace-selection.html — max-w-4xl bento grid, decorative blur BG
 *  Mobile: .stitch/designs/workspace-mobile.html — max-w-[400px] card container, fixed footer
 *  Key tokens: grid-cols-1 md:grid-cols-2, p-space-lg rounded-xl cards, group hover animations. */
function WorkspaceSelectPage() {
  const navigate = useNavigate()
  const token = useAuthStore((s) => s.token)
  const { data, isLoading } = useWorkspaces()
  const createWorkspace = useCreateWorkspace()

  const incomingSearch = Object.fromEntries(new URLSearchParams(window.location.search))

  const workspaces = data?.workspaces || []
  const personal = workspaces.filter((w: any) => w.type === 'personal' || workspaces.length <= 1)
  const organizations = workspaces.filter((w: any) => w.type === 'organization' && workspaces.length > 1)

  // Auto-skip: 0 workspaces → onboarding, 1 workspace → straight to app
  useEffect(() => {
    if (!token || isLoading || !data) return
    const ws = data.workspaces || []
    if (ws.length === 0) {
      navigate({ to: '/onboarding' as any })
    } else if (ws.length === 1) {
      navigate({ to: '/documents', search: { ws: ws[0].id, ...incomingSearch } })
    }
  }, [token, data, isLoading, navigate, incomingSearch])

  if (!token) {
    navigate({ to: '/login' })
    return null
  }

  const selectWorkspace = (wsId: string) => {
    navigate({ to: '/documents', search: { ws: wsId, ...incomingSearch } })
  }

  return (
    <div className="bg-background text-on-background min-h-screen flex items-center justify-center relative overflow-hidden">
      {/* Decorative Background — Stitch: blur circles */}
      <div className="absolute inset-0 z-0 pointer-events-none">
        <div className="absolute top-[-10%] left-[-10%] w-[50%] h-[50%] bg-surface-container rounded-full blur-[120px] opacity-70" />
        <div className="absolute bottom-[-10%] right-[-10%] w-[60%] h-[60%] bg-surface-variant rounded-full blur-[150px] opacity-60" />
      </div>

      {/* Main Container — Desktop: max-w-4xl, Mobile: max-w-[400px] */}
      <main className="relative z-10 w-full max-w-4xl px-4 md:px-8 py-10 flex flex-col gap-10">
        {/* Header — Stitch: centered, w-16 h-16 brand icon */}
        <header className="text-center space-y-2 flex flex-col items-center">
          <div className="w-16 h-16 bg-primary-container text-on-primary rounded-xl md:rounded-xl rounded-full
            flex items-center justify-center mb-2 shadow-[0_4px_16px_rgba(37,99,235,0.2)]">
            <Briefcase size={32} />
          </div>
          <h1 className="text-h2 md:text-h1 text-on-surface">Select your workspace</h1>
          <p className="text-body text-on-surface-variant max-w-md mx-auto">
            Choose where you want to go. You can switch between workspaces at any time.
          </p>
        </header>

        {isLoading ? (
          <div className="flex justify-center py-12">
            <Spinner size="lg" />
          </div>
        ) : (
          <>
            {/* Bento Grid — Stitch: grid-cols-1 md:grid-cols-2 gap-gutter */}
            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
              {/* Organizations Section */}
              {(organizations.length > 0 || workspaces.length > 0) && (
                <section className="flex flex-col gap-4">
                  <div className="flex items-center gap-1 px-1">
                    <Users size={16} className="text-outline" />
                    <h2 className="text-h3 text-on-surface">Organizations</h2>
                  </div>
                  <div className="flex flex-col gap-4 md:gap-4">
                    {(organizations.length > 0 ? organizations : workspaces).map((ws: any) => (
                      <WorkspaceCard
                        key={ws.id}
                        name={ws.name}
                        subtitle={[
                          ws.plan || 'Free Tier',
                          ws.member_count ? `${ws.member_count} Members` : '1 Member',
                        ].join(' • ')}
                        initial={ws.name?.slice(0, 2).toUpperCase() || 'WS'}
                        onClick={() => selectWorkspace(ws.id)}
                      />
                    ))}
                  </div>
                </section>
              )}

              {/* Personal Section */}
              {personal.length > 0 && organizations.length > 0 && (
                <section className="flex flex-col gap-4">
                  <div className="flex items-center gap-1 px-1">
                    <Briefcase size={16} className="text-outline" />
                    <h2 className="text-h3 text-on-surface">Personal</h2>
                  </div>
                  <div className="flex flex-col gap-4">
                    {personal.map((ws: any) => (
                      <WorkspaceCard
                        key={ws.id}
                        name={ws.name}
                        subtitle="Free Tier • 1 Member"
                        initial={ws.name?.charAt(0).toUpperCase() || 'P'}
                        onClick={() => selectWorkspace(ws.id)}
                      />
                    ))}
                  </div>
                </section>
              )}
            </div>

            {/* Footer Actions — Stitch: border-t, flex sm:flex-row */}
            <footer className="pt-6 border-t border-surface-variant flex flex-col sm:flex-row items-center justify-center gap-4">
              <button
                className="w-full sm:w-auto px-6 py-3 bg-surface-container-lowest border border-outline-variant
                  text-on-surface rounded-lg font-semibold text-small hover:bg-surface-container-low
                  transition-colors flex items-center justify-center gap-2 shadow-sm cursor-pointer"
              >
                <Users size={18} />
                Join an Organization
              </button>
              <button
                onClick={() => navigate({ to: '/onboarding' as any })}
                className="w-full sm:w-auto px-6 py-3 bg-primary-container text-on-primary rounded-lg
                  font-semibold text-small hover:bg-primary transition-colors flex items-center justify-center
                  gap-2 shadow-sm cursor-pointer border-none"
              >
                <Plus size={18} />
                Create Workspace
              </button>
            </footer>
          </>
        )}
      </main>
    </div>
  )
}

/** Individual workspace card matching Stitch desktop source:
 *  p-space-lg rounded-xl, shadow-[0_4px_16px], group hover with primary border,
 *  w-12 h-12 avatar with group-hover:bg-primary-container. */
function WorkspaceCard({
  name,
  subtitle,
  initial,
  onClick,
}: {
  name: string
  subtitle: string
  initial: string
  onClick: () => void
}) {
  return (
    <button
      onClick={onClick}
      className="w-full bg-surface-container-lowest p-6 rounded-xl flex items-center gap-4
        shadow-[0_4px_16px_rgba(0,0,0,0.04)]
        hover:shadow-[0_8px_24px_rgba(37,99,235,0.08)] hover:bg-surface-bright
        transition-all duration-300
        border border-transparent hover:border-primary-container
        group text-left cursor-pointer"
    >
      {/* Avatar — Stitch: w-12 h-12 rounded-lg, group hover to primary-container */}
      <div className="w-12 h-12 rounded-lg bg-surface-container-high text-primary
        flex items-center justify-center shrink-0 font-semibold text-small
        group-hover:bg-primary-container group-hover:text-on-primary transition-colors">
        {initial}
      </div>

      {/* Content */}
      <div className="flex-1 min-w-0">
        <h3 className="text-h3 text-on-surface truncate">{name}</h3>
        <p className="text-small text-on-surface-variant truncate">{subtitle}</p>
      </div>

      {/* Chevron — Stitch: text-outline-variant, hover:text-primary-container */}
      <ChevronRight
        size={20}
        className="text-outline-variant group-hover:text-primary-container transition-colors shrink-0"
      />
    </button>
  )
}
