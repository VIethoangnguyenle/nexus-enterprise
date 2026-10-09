import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from '@tanstack/react-router'
import { validateWorkspaceSearch } from '../lib/workspace'

const workspaces = [
  { id: 'ws-1', name: 'Khối Vận hành' },
  { id: 'ws-2', name: 'Khối Kinh doanh' },
]
vi.mock('./useWorkspaces', () => ({
  useWorkspaces: () => ({ data: { workspaces }, isLoading: false, isError: false }),
}))

const { useActiveWorkspace } = await import('./useActiveWorkspace')

function Probe() {
  const { workspaceId, workspaceName } = useActiveWorkspace()
  return <p data-testid="active">{`${workspaceId}|${workspaceName}`}</p>
}

async function activeFor(url: string) {
  const root = createRootRoute({ validateSearch: validateWorkspaceSearch })
  const index = createRoute({ getParentRoute: () => root, path: '/', component: Probe })
  const router = createRouter({
    routeTree: root.addChildren([index]),
    history: createMemoryHistory({ initialEntries: [url] }),
  })
  render(
    <QueryClientProvider client={new QueryClient()}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  )
  return (await screen.findByTestId('active')).textContent
}

beforeEach(() => {
  document.body.innerHTML = ''
})

describe('useActiveWorkspace', () => {
  it('honours the workspace in the URL', async () => {
    expect(await activeFor('/?ws=ws-2')).toBe('ws-2|Khối Kinh doanh')
  })

  it('falls back to the first workspace when the URL names none', async () => {
    expect(await activeFor('/')).toBe('ws-1|Khối Vận hành')
  })

  it('ignores a workspace the user does not belong to', async () => {
    expect(await activeFor('/?ws=someone-elses')).toBe('ws-1|Khối Vận hành')
  })
})

describe('validateWorkspaceSearch', () => {
  it('keeps a string id', () => {
    expect(validateWorkspaceSearch({ ws: 'ws-2' })).toEqual({ ws: 'ws-2' })
  })

  it('keeps an id the URL parser turned into a number as text', () => {
    expect(validateWorkspaceSearch({ ws: 42 })).toEqual({ ws: '42' })
  })

  it('drops anything that is not an id, and leaves other params alone', () => {
    expect(validateWorkspaceSearch({ ws: { a: 1 } })).toEqual({})
    expect(validateWorkspaceSearch({ ws: '' })).toEqual({})
    expect(validateWorkspaceSearch({})).toEqual({})
  })
})
