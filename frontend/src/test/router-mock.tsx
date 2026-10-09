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
}

export function routerMockFactory() {
  return {
    Link: ({ to, params, children, className, ...rest }: {
      to: string; params?: Record<string, string>; children?: ReactNode; className?: string
      [k: string]: unknown
    }) => {
      const href = params ? Object.entries(params).reduce((h, [k, v]) => h.replace(`$${k}`, v), to) : to
      const { activeProps: _a, search: _s, preload: _p, ...anchorProps } = rest as Record<string, unknown>
      return (
        <a href={href} className={className} {...(anchorProps as object)}>
          {children}
        </a>
      )
    },
    useNavigate: () => routerState.navigate,
    useParams: () => routerState.params,
    useSearch: () => routerState.search,
    useRouterState: ({ select }: { select: (s: { location: { pathname: string } }) => unknown }) =>
      select({ location: { pathname: routerState.pathname } }),
  }
}
