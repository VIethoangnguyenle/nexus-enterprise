/**
 * A scripted stand-in for the auth, workspace and invitation REST endpoints,
 * installed in place of `apiFetch`. It checks request bodies field by field
 * against what the Go handlers read (`display_name`, `session_id`, ...) and
 * records every call, so a screen that sends a field the server ignores fails
 * its test instead of failing in production. Every id is a real-looking UUID so
 * tests can assert that none reaches the screen.
 */
import { ApiError } from '../api/client'
import type { AuthProviders, MeResponse, MyWorkspace, OTPVerifyResponse } from '../api/auth'
import type { MyInvitation } from '../api/invitations'
import { UUID_RE } from './chat-fixtures'

export { UUID_RE }

export const ID = {
  me: '5b7a1c20-0000-4a00-8000-00000000a001',
  node: '5b7a1c20-0000-4a00-8000-00000000a002',
  wsOps: '7c1d0000-1111-4a00-8000-00000000b001',
  wsProject: '7c1d0000-1111-4a00-8000-00000000b002',
  wsControl: '7c1d0000-1111-4a00-8000-00000000b003',
  invControl: '9e000000-2222-4a00-8000-00000000c001',
  invOther: '9e000000-2222-4a00-8000-00000000c002',
}

/** A signed token for a workspace; only its payload is read, by `tenantIdFromToken`. */
export const jwtFor = (tenantId: string) => `h.${btoa(JSON.stringify({ tenant_id: tenantId }))}.s`

export interface Call { method: string; path: string; body?: Record<string, unknown> }
export const calls: Call[] = []

export interface World {
  providers: AuthProviders | Error | 'pending'
  me: MeResponse['user']
  workspaces: { id: string; name: string }[]
  summaries: MyWorkspace[] | Error
  invitations: MyInvitation[]
  invitationsError?: Error
  /** The code the server will accept. */
  goodCode: string
  /** Tries left before the session is dead. */
  attempts: number
  verifyResult: Partial<OTPVerifyResponse>
  requestError?: Error
  verifyError?: Error
  profileError?: Error
  createError?: Error
  acceptError?: Error
  declineError?: Error
  switchError?: Error
  workspacesError?: Error
  /** The code the server will accept for "prove my address" (a real sender delivered it). */
  emailCode: string
  emailAttempts: number
  emailRequestError?: Error
  emailConfirmError?: Error
  googleVerifyError?: Error
  /** Counts how many OTP requests were made, to number sessions. */
  sessions: number
}

const baseMe = (): MeResponse['user'] => ({
  id: ID.me, username: 'hoa.le', ngac_node_id: ID.node, email: 'hoa.le@novapay.vn', union_id: 'u',
  display_name: 'Lê Thị Hoa', title: '', location: '', avatar_url: '', email_verified: true, needs_profile: false,
})

export function freshWorld(): World {
  return {
    providers: { google: true, otp: true, otp_fixed_code: true, otp_proves_email: false },
    me: baseMe(),
    workspaces: [
      { id: ID.wsOps, name: 'Khối Vận hành' },
      { id: ID.wsProject, name: 'Dự án Ví điện tử 2026' },
    ],
    summaries: [
      { id: ID.wsOps, name: 'Khối Vận hành', role: 'member', member_count: 64, domain: '' },
      { id: ID.wsProject, name: 'Dự án Ví điện tử 2026', role: 'owner', member_count: 18, domain: '' },
    ],
    invitations: [],
    goodCode: '481209',
    attempts: 5,
    emailCode: '246810',
    emailAttempts: 5,
    verifyResult: {},
    sessions: 0,
  }
}

export let world: World = freshWorld()
export function resetFixtures() {
  world = freshWorld()
  calls.length = 0
}

export const invitation = (over: Partial<MyInvitation> = {}): MyInvitation => ({
  id: ID.invControl,
  workspace_name: 'Kiểm soát nội bộ',
  inviter_name: 'Trần Minh Đức',
  role_name: 'Kế toán',
  department_name: '',
  created_at: '2026-10-09T08:00:00Z',
  expires_at: new Date(Date.now() + 5 * 86_400_000 + 3_600_000).toISOString(),
  ...over,
})

function expectKeys(path: string, body: Record<string, unknown> | undefined, allowed: string[]) {
  const extra = Object.keys(body ?? {}).filter((k) => !allowed.includes(k))
  if (extra.length) throw new Error(`${path}: the handler does not read ${extra.join(', ')}`)
}

const api = (status: number, body: Record<string, unknown> = {}) => new ApiError('server words', status, body)

/**
 * Drop-in for `apiFetch`. Answers are copies, as a network would give: handing
 * out the fixture's own objects would let a test mutate what the query cache
 * already holds and hide the refetch it is checking for.
 */
export async function authFixtureApi(path: string, options: RequestInit = {}): Promise<unknown> {
  return structuredClone(await answer(path, options))
}

async function answer(path: string, options: RequestInit): Promise<unknown> {
  const method = (options.method ?? 'GET').toUpperCase()
  const body = options.body ? (JSON.parse(String(options.body)) as Record<string, unknown>) : undefined
  calls.push({ method, path, body })

  if (method === 'GET' && path === '/auth/providers') {
    if (world.providers === 'pending') return new Promise(() => {})
    if (world.providers instanceof Error) throw world.providers
    return world.providers
  }

  if (method === 'POST' && path === '/auth/otp/request') {
    expectKeys(path, body, ['identifier', 'type'])
    if (world.requestError) throw world.requestError
    world.sessions += 1
    world.attempts = 5
    return { session_id: `session-${world.sessions}`, expires_in: 300 }
  }

  if (method === 'POST' && path === '/auth/otp/verify') {
    expectKeys(path, body, ['session_id', 'code'])
    if (world.verifyError) throw world.verifyError
    if (world.attempts <= 0) throw api(429, { code: 'otp_too_many_attempts' })
    if (body?.code !== world.goodCode) {
      world.attempts -= 1
      throw api(401, { code: 'otp_invalid', attempts_left: world.attempts })
    }
    return {
      access_token: jwtFor(ID.wsOps),
      user: {
        id: world.me.id, username: world.me.username, ngac_node_id: world.me.ngac_node_id,
        email: world.me.email, union_id: 'u', email_verified: world.me.email_verified,
      },
      is_new_user: false,
      needs_profile: world.me.needs_profile,
      ...world.verifyResult,
    } satisfies OTPVerifyResponse
  }

  if (method === 'GET' && (path === '/me' || path.startsWith('/me?workspace='))) {
    return { user: world.me, current_tenant: { id: ID.wsOps, name: 'Khối Vận hành', role: 'member', open_id: 'o', department: 'Đối soát' } }
  }

  if (method === 'PATCH' && path === '/me/profile') {
    expectKeys(path, body, ['display_name', 'title', 'department', 'location', 'avatar_url'])
    if (world.profileError) throw world.profileError
    if (typeof body?.display_name === 'string') world.me.display_name = body.display_name
    world.me.needs_profile = false
    return { status: 'updated' }
  }

  if (method === 'GET' && path === '/workspaces') {
    if (world.workspacesError) throw world.workspacesError
    return { workspaces: world.workspaces }
  }

  if (method === 'GET' && path === '/me/workspaces') {
    if (world.summaries instanceof Error) throw world.summaries
    return { workspaces: world.summaries }
  }

  if (method === 'POST' && path === '/me/workspaces') {
    expectKeys(path, body, ['name'])
    if (world.createError) throw world.createError
    const id = `7c1d0000-1111-4a00-8000-0000000fff${world.workspaces.length}`
    // The server refuses an account whose address nobody has proved.
    if (!world.me.email_verified) throw api(403, { code: 'email_unverified' })
    world.workspaces.push({ id, name: String(body?.name) })
    if (Array.isArray(world.summaries)) {
      world.summaries.push({ id, name: String(body?.name), role: 'owner', member_count: 1, domain: '' })
    }
    return { id, name: body?.name, role: 'owner', member_count: 1 }
  }

  if (method === 'GET' && path === '/invitations') {
    if (world.invitationsError) throw world.invitationsError
    // The server lists offers only for an account whose address is verified.
    return { invitations: world.me.email_verified ? world.invitations : [] }
  }

  const accept = /^\/invitations\/([^/]+)\/accept$/.exec(path)
  if (method === 'POST' && accept) {
    expectKeys(path, body, [])
    if (world.acceptError) throw world.acceptError
    const inv = world.invitations.find((i) => i.id === accept[1])
    if (!inv) throw api(404)
    world.invitations = world.invitations.filter((i) => i.id !== inv.id)
    const wsId = `7c1d0000-1111-4a00-8000-0000000eee${world.workspaces.length}`
    world.workspaces.push({ id: wsId, name: inv.workspace_name })
    if (Array.isArray(world.summaries)) {
      world.summaries.push({ id: wsId, name: inv.workspace_name, role: 'member', member_count: 2, domain: '' })
    }
    return { workspace_id: wsId, workspace_name: inv.workspace_name, role_applied: true, department_applied: true }
  }

  const decline = /^\/invitations\/([^/]+)\/decline$/.exec(path)
  if (method === 'POST' && decline) {
    if (world.declineError) throw world.declineError
    world.invitations = world.invitations.filter((i) => i.id !== decline[1])
    return {}
  }

  if (method === 'POST' && path === '/me/email/verify') {
    expectKeys(path, body, ['code'])
    if (body?.code === undefined) {
      if (world.emailRequestError) throw world.emailRequestError
      world.emailAttempts = 5
      return { expires_in: 300 }
    }
    if (world.emailConfirmError) throw world.emailConfirmError
    if (world.emailAttempts <= 0) throw api(429, { code: 'otp_too_many_attempts' })
    if (body.code !== world.emailCode) {
      world.emailAttempts -= 1
      throw api(401, { code: 'otp_invalid', attempts_left: world.emailAttempts })
    }
    world.me.email_verified = true
    return { email_verified: true }
  }

  if (method === 'POST' && path === '/me/email/verify/google') {
    expectKeys(path, body, [])
    if (world.googleVerifyError) throw world.googleVerifyError
    return { url: 'https://accounts.example/auth?state=s' }
  }

  if (method === 'POST' && path === '/auth/switch-tenant') {
    expectKeys(path, body, ['tenant_id'])
    if (world.switchError) throw world.switchError
    return { access_token: jwtFor(String(body?.tenant_id)) }
  }

  throw new Error(`unscripted request: ${method} ${path}`)
}

export { api as apiError }
