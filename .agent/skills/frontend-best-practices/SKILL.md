---
name: frontend-best-practices
description: Frontend best practices for Vite + TanStack Router + TanStack Query + Zustand SPA — file conventions, routing patterns, data fetching, state management, error handling, component design, performance, and accessibility.
user-invocable: false
---

# Frontend Best Practices — NGAC Platform

Apply these rules when writing or reviewing frontend code in the NGAC Platform.
This project uses **Vite + TanStack Router + TanStack Query + Zustand** (NOT Next.js).

## File Conventions

See [file-conventions.md](./references/file-conventions.md) for:
- Route file naming (TanStack Router file-based routing)
- Layout routes (`_auth.tsx`, `_workspace.tsx`)
- Dynamic params (`$paramName`)
- Route grouping and nesting

## TanStack Router Patterns

See [tanstack-router.md](./references/tanstack-router.md) for:
- `createFileRoute` and `createRootRoute`
- Auth guards in layout routes
- Navigate vs useNavigate
- Route params and search params
- Loader functions and route context

## TanStack Query Patterns

See [tanstack-query.md](./references/tanstack-query.md) for:
- `queryOptions` factory pattern (REQUIRED)
- Query key conventions
- Mutation + invalidation patterns
- Optimistic updates
- Stale/cache time defaults
- Error/loading state handling

## State Management

See [state-management.md](./references/state-management.md) for:
- Zustand is UI-ONLY (sidebar, modals, WebSocket connection)
- TanStack Query is ALL server state
- WebSocket → Query invalidation bridge
- NEVER duplicate server data in Zustand

## Component Design

See [component-design.md](./references/component-design.md) for:
- Component file structure
- Props interface conventions
- Composition over configuration
- Presentational vs container split
- Reusable component patterns

## Error Handling

See [error-handling.md](./references/error-handling.md) for:
- API error handling in `apiFetch`
- Mutation error display
- Loading/error/empty state patterns
- Toast notifications for mutations

## Performance

See [performance.md](./references/performance.md) for:
- React.lazy and code splitting
- Avoiding unnecessary re-renders
- Memoization guidelines
- Bundle analysis with Vite

## Responsive Design

See [responsive.md](./references/responsive.md) for:
- Mobile-first breakpoint system (`md:`, `lg:`, `xl:`)
- Layout patterns: shell+drawer, cards vs tables, panels
- Per-module responsive guides (Chat, Drive, Assets)
- Touch target requirements (≥ 44px)
- Responsive testing checklist

## CSS & Styling (Tailwind v4)

See [styling.md](./references/styling.md) for:
- Tailwind v4 CSS-first configuration
- Design tokens via `@theme {}` in `index.css`
- Custom `@utility` directives for typography
- When to use Tailwind utilities vs custom CSS
- Dark mode (dark-first palette)
- Animation guidelines

## Security

See [security.md](./references/security.md) for:
- XSS prevention (no dangerouslySetInnerHTML)
- Token storage (localStorage risks)
- Input sanitization
- CORS and proxy configuration
