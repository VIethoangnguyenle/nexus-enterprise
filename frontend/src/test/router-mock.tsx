import type { ReactNode } from 'react'
import { vi } from 'vitest'

/**
 * Minimal stand-in for @tanstack/react-router in component tests: links are
 * plain anchors, navigation is a spy, and params/path/search come from `routerState`.
 */
export const routerState = {
  params: {} as Record<string, string>,
  pathname: '/channels',
  search: {} as Record<string, unknown>,
  navigate: vi.fn(),
  /** Whether the app's own history has an entry before this one. */
  canGoBack: false,
  /** The history state of the current entry. */
  historyState: {} as Record<string, unknown>,
  back: vi.fn(),
}

export function routerMockFactory() {
  return {
    Link: ({ to, params, search, children, className, ...rest }: {
      to: string; params?: Record<string, string>; search?: unknown; children?: ReactNode; className?: string
      [k: string]: unknown
    }) => {
      const href = params ? Object.entries(params).reduce((h, [k, v]) => h.replace(`$${k}`, v), to) : to
      const { activeProps: _a, preload: _p, ...anchorProps } = rest as Record<string, unknown>
      return (
        // The search the real link would put after the address, for tests that open a thread or a tab.
        <a href={href} className={className} data-search={search ? JSON.stringify(search) : undefined} {...(anchorProps as object)}>
          {children}
        </a>
      )
    },
    useNavigate: () => routerState.navigate,
    useCanGoBack: () => routerState.canGoBack,
    useRouter: () => ({ history: { back: routerState.back } }),
    useParams: () => routerState.params,
    useSearch: () => routerState.search,
    useRouterState: ({ select }: { select: (s: { location: { pathname: string; state: Record<string, unknown> } }) => unknown }) =>
      select({ location: { pathname: routerState.pathname, state: routerState.historyState } }),
  }
}
