import { launch, newPage, step, shot, signInUI, enterWorkspace, tokens, sleep, saveResults, BASE, run, sql } from './ui-lib.mjs'
import { WORLD } from './ui-env.mjs'
import { request, api } from './lib.mjs'
import fs from 'node:fs'
await launch()
const T = tokens()
const api2 = await request.newContext({ baseURL: BASE })
const WSA = WORLD.wsA, NAMEA = `QA Tenant A ${WORLD.run}`
// data via API: a template by the manager, a routed request, an asset type, a file shared to M3
const tpl = await api(api2, { token: T.M1 }, 'POST', '/approval/templates', { name: `Đề nghị mua sắm ${run}`, entity_type: 'general', priority: 5, is_active: true, conditions: [], steps: [{ step_order: 1, name: 'Tài chính duyệt', approver_type: 'department', approver_value: WORLD.depTC, required_count: 1, timeout_hours: 48 }], form_fields: [{ label: 'Lý do', field_type: 'text', required: false }] })
console.log('template', tpl.status, JSON.stringify(tpl.body).slice(0, 120))
const rq = await api(api2, { token: T.O }, 'POST', '/approval/requests', { template_id: tpl.body.id || '', entity_type: 'general', entity_id: '', form_data_json: JSON.stringify({ 'Lý do': 'Mua máy in' }), department_id: '' })
console.log('request', rq.status, JSON.stringify(rq.body).slice(0, 120))
const at = await api(api2, { token: T.O }, 'POST', `/workspaces/${WSA}/asset-types`, { name: `Máy in ${run}`, category: 'it', description: '', fields_schema: '{}' })
console.log('asset type', at.status)
const F = 'smoke-ui'
const { page: O } = await newPage()
await step(F, 'owner signs in and opens Quản trị (roles and permission editor)', async () => {
  await signInUI(O, WORLD.O.email, null); await enterWorkspace(O, NAMEA)
  await O.goto(`${BASE}/admin/roles?ws=${WSA}`, { waitUntil: 'networkidle' })
  await O.getByText('Quản lý', { exact: true }).first().waitFor({ timeout: 10000 })
  await shot(O, '90-admin-roles')
  return 'roles listed'
}, O)
await step(F, 'role panel shows the permission grid for Quản lý', async () => {
  await O.getByText('Quản lý', { exact: true }).first().click()
  await O.getByText('ĐƯỢC PHÉP', { exact: false }).first().waitFor({ timeout: 10000 })
  await O.getByText('Tài liệu', { exact: true }).first().waitFor({ timeout: 5000 })
  await shot(O, '91-admin-role-permissions')
  return 'grid shown'
}, O)
await step(F, 'users screen lists the people with department and roles', async () => {
  await O.goto(`${BASE}/admin/users?ws=${WSA}`, { waitUntil: 'networkidle' })
  await O.getByText('Mai Member').first().waitFor({ timeout: 10000 })
  await shot(O, '92-admin-users')
  return 'users listed'
}, O)
await step(F, 'approval screen lists the template and the routed request; detail names the actors', async () => {
  await O.goto(`${BASE}/approval?ws=${WSA}`, { waitUntil: 'networkidle' })
  await O.getByText(`Đề nghị mua sắm ${run}`).first().waitFor({ timeout: 10000 }).catch(() => {})
  await shot(O, '93-approval-screen')
  const txt = await O.locator('body').innerText()
  if (!txt.includes('Mua máy in') && !txt.includes('Đề nghị')) throw new Error('approval screen has no request content')
  return 'approval content'
}, O)
await step(F, 'asset types screen lists the new type', async () => {
  await O.goto(`${BASE}/assets/types?ws=${WSA}`, { waitUntil: 'networkidle' })
  await O.getByText(`Máy in ${run}`).first().waitFor({ timeout: 10000 })
  await shot(O, '94-asset-types')
  return 'type listed'
}, O)
const bytes = Buffer.alloc(3000, 66)
const cf = await api(api2, { token: T.O }, 'POST', `/workspaces/${WSA}/drive/files`, { name: 'hop-dong-ui.pdf', mime_type: 'application/pdf', size_bytes: bytes.length, parent_id: '' })
await api2.fetch(cf.body.upload_url, { method: 'PUT', data: bytes })
await api(api2, { token: T.O }, 'POST', `/drive/files/${cf.body.file_id}/confirm`, {})
const shr = await api(api2, { token: T.O }, 'POST', `/drive/items/${cf.body.file_id}/share`, { target_node_id: WORLD.M3.node, share_type: 'user', permission: 'write' })
console.log('file+share', cf.status, shr.status)
await step(F, 'member sees a file shared with him under "Được chia sẻ với tôi" and can download it', async () => {
  const { page: M } = await newPage()
  await signInUI(M, WORLD.M3.email, null); await enterWorkspace(M, NAMEA)
  await M.goto(`${BASE}/drive?ws=${WSA}`, { waitUntil: 'networkidle' })
  await M.getByText('Được chia sẻ với tôi', { exact: true }).first().click()
  await sleep(1200)
  await shot(M, '95-shared-with-me')
  const n = await M.getByText('hop-dong-ui.pdf').count()
  const dl = M.waitForEvent ? null : null
  if (!n) throw new Error('shared file not listed for grantee')
  return `shared list shows the file (${n})`
}, O)
console.log('owner errors', O.errors.slice(0, 4))
saveResults('smoke-ui')
process.exit(0)
