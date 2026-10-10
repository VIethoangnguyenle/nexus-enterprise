import { request, signInApi, api, BASE, run, sql, rec, provisionApprovalSchema } from './lib.mjs'
export const ctx = await request.newContext({ baseURL: BASE })
export async function asTenant(who, wsId) {
  const r = await api(ctx, who, 'POST', '/auth/switch-tenant', { tenant_id: wsId })
  if (r.status !== 200) throw new Error('switch ' + r.status + JSON.stringify(r.body))
  who.token = r.body.access_token
  who.tenantId = wsId
  return who
}
export async function buildWorld() {
  const W = { run }
  W.O = await signInApi(ctx, `e2e-o-${run}@example.test`, 'Olivia Owner')
  W.X = await signInApi(ctx, `e2e-x-${run}@example.test`, 'Xuan Outsider')
  const wsA = (await api(ctx, W.O, 'POST', '/me/workspaces', { name: `QA Tenant A ${run}` })).body
  const wsB = (await api(ctx, W.X, 'POST', '/me/workspaces', { name: `QA Tenant B ${run}` })).body
  W.wsA = wsA.id; W.wsB = wsB.id
  await asTenant(W.O, W.wsA)
  await asTenant(W.X, W.wsB)
  // approval tenant schema: not created by workspace creation (see finding); provisioned here via the internal route
  W.provA = provisionApprovalSchema(W.O)
  W.provB = provisionApprovalSchema(W.X)
  // roles and departments (owner of A)
  W.roleMgr = (await api(ctx, W.O, 'POST', `/workspaces/${W.wsA}/roles`, { name: 'Quản lý', description: 'quản lý', can_manage: true })).body.id
  W.roleThamdinh = (await api(ctx, W.O, 'POST', `/workspaces/${W.wsA}/roles`, { name: 'Thẩm định', description: 'thẩm định', can_manage: false })).body.id
  W.depKD = (await api(ctx, W.O, 'POST', `/workspaces/${W.wsA}/departments`, { name: 'Kinh doanh', parent_oa_id: '' })).body.id
  W.depTC = (await api(ctx, W.O, 'POST', `/workspaces/${W.wsA}/departments`, { name: 'Tài chính', parent_oa_id: '' })).body.id
  const ops = [
    ['management', ['manage', 'invite']],
    ['documents', ['read', 'write', 'share']],
    ['channels', ['read', 'write', 'manage', 'invite', 'create_channel']],
  ]
  for (const [area, operations] of ops) {
    const r = await api(ctx, W.O, 'PUT', `/workspaces/${W.wsA}/roles/${W.roleMgr}/permissions/${area}`, { operations })
    if (r.status >= 300) throw new Error('perm ' + area + r.status + JSON.stringify(r.body))
  }
  // invitations, then acceptance by each invitee (email verified via the SQL path)
  const invitees = [
    ['M1', `e2e-m1-${run}@example.test`, 'Minh Manager', { role_id: W.roleMgr, department_id: W.depKD }],
    ['M2', `e2e-m2-${run}@example.test`, 'Mai Member', { role_id: W.roleThamdinh, department_id: W.depTC }],
    ['M3', `e2e-m3-${run}@example.test`, 'Hoa Member', { role_id: '', department_id: W.depTC }],
    ['M4', `e2e-m4-${run}@example.test`, 'Khang Member', { role_id: '', department_id: W.depTC }],
  ]
  for (const [key, email, name, extra] of invitees) {
    const inv = await api(ctx, W.O, 'POST', `/workspaces/${W.wsA}/members`, { email, ...extra })
    if (inv.status >= 300) throw new Error('invite ' + key + ' ' + inv.status + JSON.stringify(inv.body))
    W[key] = await signInApi(ctx, email, name)
    const list = await api(ctx, W[key], 'GET', '/invitations')
    const mine = list.body.invitations?.find((i) => i.workspace_name === `QA Tenant A ${run}`)
    if (!mine) throw new Error('no invitation for ' + key + JSON.stringify(list.body).slice(0, 200))
    const acc = await api(ctx, W[key], 'POST', `/invitations/${mine.id}/accept`, {})
    if (acc.status >= 300) throw new Error('accept ' + key + ' ' + acc.status + JSON.stringify(acc.body))
    await asTenant(W[key], W.wsA)
  }
  return W
}
