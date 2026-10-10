import { createRequire } from 'node:module'
import { execFileSync } from 'node:child_process'
import fs from 'node:fs'
const require = createRequire('/home/zane/.claude/skills/gstack/package.json')
export const { chromium, request } = require('playwright')
export const BASE = 'http://localhost:18080'
export const OUT = '/home/zane/Desktop/projects/nexus-enterprise/plans/261010-2205-first-release/reports/e2e'
export const run = process.env.E2E_RUN || Date.now().toString(36)
export const sql = (q) => execFileSync('docker', ['exec', 'nexus-e2e-postgres-1', 'psql', '-U', 'ngac', '-d', 'ngac', '-tAc', q]).toString().trim()
export const sleep = (ms) => new Promise((r) => setTimeout(r, ms))
export const results = []
export function rec(flow, id, ok, detail = '') {
  results.push({ flow, id, ok, detail })
  console.log(`${ok ? 'PASS' : 'FAIL'} [${flow}] ${id}${detail ? ' :: ' + String(detail).slice(0, 300) : ''}`)
}
export function saveResults(name) {
  fs.mkdirSync(OUT, { recursive: true })
  fs.writeFileSync(`${OUT}/${name}.json`, JSON.stringify(results, null, 2))
}
// API session: uses the context's request client, so cookies (refresh) are shared with the page if the page uses the same context.
export async function signInApi(ctx, email, name) {
  const r = await (await ctx.post(`${BASE}/api/auth/otp/request`, { data: { identifier: email, type: 'email' } })).json()
  const vr = await ctx.post(`${BASE}/api/auth/otp/verify`, { data: { session_id: r.session_id, code: '999999' } })
  if (!vr.ok()) throw new Error('verify ' + vr.status() + await vr.text())
  const v = await vr.json()
  const who = { email, user: v.user, token: v.access_token, needsProfile: v.needs_profile }
  if (v.needs_profile) {
    const p = await ctx.patch(`${BASE}/api/me/profile`, { headers: { Authorization: `Bearer ${who.token}` }, data: { display_name: name, username: name.toLowerCase().replace(/\s+/g, '') + run } })
    if (!p.ok()) throw new Error('profile ' + p.status() + await p.text())
  }
  sql(`UPDATE users SET email_verified_at = now() WHERE lower(email) = lower('${email}')`)
  return who
}
export async function api(ctx, who, method, path, data, extraHeaders = {}) {
  const res = await ctx.fetch(`${BASE}/api${path}`, { method, headers: { Authorization: `Bearer ${who.token}`, ...extraHeaders }, data, failOnStatusCode: false })
  const text = await res.text()
  let body; try { body = JSON.parse(text) } catch { body = text }
  return { status: res.status(), body }
}
export async function switchTenant(ctx, who, workspaceId) {
  const r = await api(ctx, who, 'POST', '/auth/switch-tenant', { tenant_id: workspaceId })
  if (r.status !== 200) throw new Error('switch ' + r.status + JSON.stringify(r.body))
  who.token = r.body.access_token
  return r
}
// WORKAROUND (recorded as a finding): approval's /api/admin/tenants/:id/provision is not routed publicly and
// nothing else creates the tenant schema. Called from inside the nexus network with the tenant owner's token.
export function provisionApprovalSchema(who) {
  const out = execFileSync('docker', ['run', '--rm', '--network', 'nexus-e2e_nexus', 'alpine:3.21', 'wget', '-q', '-O', '-', '--post-data=', `--header=Authorization: Bearer ${who.token}`, `http://approval:8080/api/admin/tenants/${who.tenantId}/provision`]).toString()
  return out
}
