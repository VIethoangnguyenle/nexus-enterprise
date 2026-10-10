import { createRequire } from 'node:module'
import { execSync } from 'node:child_process'
const require = createRequire('/home/zane/.claude/skills/gstack/package.json')
export const { chromium } = require('playwright')
export const BASE = 'http://localhost:5173'
export const run = Date.now().toString(36)
const psql = (sql) => execSync(`docker exec nexus-enterprise-postgres-1 psql -U ngac -d ngac -tAc "${sql}"`).toString().trim()

export async function signIn(ctx, email, name) {
  const r = ctx.request
  const req = await (await r.post(`${BASE}/api/auth/otp/request`, { data: { identifier: email, type: 'email' } })).json()
  const vr = await r.post(`${BASE}/api/auth/otp/verify`, { data: { session_id: req.session_id, code: '999999' } })
  if (!vr.ok()) throw new Error('verify ' + vr.status() + await vr.text())
  const v = await vr.json()
  let token = v.access_token
  const auth = () => ({ Authorization: `Bearer ${token}` })
  if (v.needs_profile) {
    const p = await r.patch(`${BASE}/api/me/profile`, { headers: auth(), data: { display_name: name, username: name } })
    if (!p.ok()) console.log('profile', p.status(), await p.text())
  }
  psql(`UPDATE users SET email_verified_at = now() WHERE lower(email) = '${email}'`)
  return { user: v.user, get token() { return token }, set token(t) { token = t }, auth }
}
export async function api(ctx, who, method, path, data) {
  const res = await ctx.request.fetch(`${BASE}/api${path}`, { method, headers: who.auth(), data })
  const text = await res.text()
  let body; try { body = JSON.parse(text) } catch { body = text }
  return { status: res.status(), body }
}
export async function switchTenant(ctx, who, tenantId) {
  const r = await api(ctx, who, 'POST', '/auth/switch-tenant', { tenant_id: tenantId })
  if (r.status !== 200) throw new Error('switch ' + r.status + JSON.stringify(r.body))
  who.token = r.body.access_token
}
