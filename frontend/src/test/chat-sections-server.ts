import { vi } from 'vitest'
import { ApiError } from '../api/client'
import { fixtureApi, WS_ID } from './chat-fixtures'
import { withMoved, withPlaced, withRenamed, withRestored, withUnplaced, withoutSection, type ChatSection } from '../lib/chat-sections'

/**
 * A stateful stand-in for the chat-sections endpoints (docs/specs/chat-sections)
 * on top of `fixtureApi`: it keeps the user's sections, applies each call the
 * way the server does, and records the calls. `fail` makes the next call to a
 * path fragment answer with a status. `gate` holds the next call to a path
 * fragment until released, so a test can look at the optimistic state.
 */
export function createSectionsServer(initial: ChatSection[] = []) {
  let sections: ChatSection[] = structuredClone(initial)
  let seq = 100
  const calls: { method: string; path: string; body: unknown }[] = []
  const failures: { fragment: string; status: number }[] = []
  const gates: { fragment: string; release: () => void; wait: Promise<void> }[] = []
  const base = `/workspaces/${WS_ID}/chat-sections`

  const api = vi.fn(async (path: string, init?: RequestInit): Promise<unknown> => {
    if (!path.startsWith(base)) return fixtureApi(path, init)
    const method = (init?.method || 'GET').toUpperCase()
    const rest = path.slice(base.length)
    const body = init?.body ? JSON.parse(init.body as string) : undefined
    calls.push({ method, path: rest, body })

    const gate = gates.findIndex((g) => rest.includes(g.fragment))
    if (gate >= 0) await gates.splice(gate, 1)[0]!.wait
    const fail = failures.findIndex((f) => rest.includes(f.fragment))
    if (fail >= 0) throw new ApiError('failed', failures.splice(fail, 1)[0]!.status, {})

    const parts = rest.split('/').filter(Boolean)
    if (method === 'GET' && parts.length === 0) return structuredClone({ sections })
    if (method === 'POST' && parts.length === 0) {
      const made: ChatSection = {
        id: `sec-${++seq}`,
        name: String(body.name).trim(),
        position: body.position ?? sections.length,
        channel_ids: body.channel_ids ?? [],
      }
      sections = withRestored(sections, made)
      return structuredClone(made)
    }
    if (method === 'PATCH' && parts.length === 1) {
      sections = body.name !== undefined ? withRenamed(sections, parts[0]!, body.name) : withMoved(sections, parts[0]!, body.move)
      return { status: 'ok' }
    }
    if (method === 'DELETE' && parts.length === 1) {
      const gone = sections.find((s) => s.id === parts[0])!
      sections = withoutSection(sections, parts[0]!)
      return structuredClone(gone)
    }
    if (method === 'PUT' && parts[1] === 'channels') {
      sections = withPlaced(sections, parts[0]!, parts[2]!)
      return { status: 'ok' }
    }
    if (method === 'DELETE' && parts[0] === 'channels') {
      sections = withUnplaced(sections, parts[1]!)
      return { status: 'ok' }
    }
    throw new Error(`sections server: unexpected ${method} ${path}`)
  })

  return {
    api,
    calls,
    get sections() { return sections },
    failNext: (fragment: string, status: number) => void failures.push({ fragment, status }),
    holdNext: (fragment: string) => {
      let release!: () => void
      const wait = new Promise<void>((r) => { release = r })
      gates.push({ fragment, release, wait })
      return release
    },
  }
}
