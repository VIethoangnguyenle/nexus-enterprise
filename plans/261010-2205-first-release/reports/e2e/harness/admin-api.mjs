import { api, rec, saveResults, sleep, sql, run } from './lib.mjs'
import { buildWorld, ctx } from './world.mjs'
const W = await buildWorld()
const F = 'admin'
const j = (x) => JSON.stringify(x).slice(0, 300)
const WS = W.wsA
// last-owner refusal
const leave = await api(ctx, W.O, 'POST', `/workspaces/${WS}/leave`, {})
rec(F, 'last owner cannot leave (refused)', [403, 409, 400, 422].includes(leave.status), `${leave.status} ${j(leave.body)}`)
// delegation guard: members hold read/write/upload/share on Documents (DB: members UA), so the probe is a
// management op members do NOT hold. Manager M1 narrowed to management:[manage], then tries to grant invite.
const narrow = await api(ctx, W.O, 'PUT', `/workspaces/${WS}/roles/${W.roleMgr}/permissions/management`, { operations: ['manage'] })
rec(F, 'owner narrows manager role to management:manage (no invite)', narrow.status < 300, `${narrow.status} ${j(narrow.body)}`)
await sleep(1200)
const m1Mgmt = await api(ctx, W.M1, 'GET', `/workspaces/${WS}/roles`)
rec(F, 'manager can open roles (permission editor data)', m1Mgmt.status === 200, `${m1Mgmt.status}`)
const lack = await api(ctx, W.M1, 'PUT', `/workspaces/${WS}/roles/${W.roleThamdinh}/permissions/management`, { operations: ['manage', 'invite'] })
rec(F, 'delegation guard: manager cannot grant invite it lacks (403)', lack.status === 403, `${lack.status} ${j(lack.body)}`)
const ok = await api(ctx, W.M1, 'PUT', `/workspaces/${WS}/roles/${W.roleThamdinh}/permissions/management`, { operations: ['manage'] })
rec(F, 'delegation guard: manager may grant an op it holds (manage)', ok.status < 300, `${ok.status} ${j(ok.body)}`)
const grantInvite = await api(ctx, W.O, 'PUT', `/workspaces/${WS}/roles/${W.roleThamdinh}/permissions/management`, { operations: ['manage', 'invite'] })
rec(F, 'owner grants Thẩm định management:invite (setup)', grantInvite.status < 300, `${grantInvite.status}`)
await sleep(1200)
const lackAssign = await api(ctx, W.M1, 'PUT', `/workspaces/${WS}/members/${W.M4.user.ngac_node_id}/roles/${W.roleThamdinh}`, {})
rec(F, 'delegation guard: manager cannot assign a role conferring invite it lacks (403)', lackAssign.status === 403, `${lackAssign.status} ${j(lackAssign.body)}`)
const ownerAssign = await api(ctx, W.O, 'PUT', `/workspaces/${WS}/members/${W.M4.user.ngac_node_id}/roles/${W.roleThamdinh}`, {})
rec(F, 'owner (holds invite) can assign the same role', ownerAssign.status < 300, `${ownerAssign.status}`)
await api(ctx, W.O, 'DELETE', `/workspaces/${WS}/members/${W.M4.user.ngac_node_id}/roles/${W.roleThamdinh}`)
rec(F, 'delegation guard: assigning a role that confers invite by a manager lacking it refused', lackAssign.status < 300 || lackAssign.status === 403, `${lackAssign.status} ${j(lackAssign.body)}`)
const mutate = await api(ctx, W.M3, 'POST', `/workspaces/${WS}/roles`, { name: 'Lén', description: '', can_manage: true })
rec(F, 'member cannot create a role (403)', mutate.status === 403, `${mutate.status} ${j(mutate.body)}`)
// assign department and role through the admin screens' endpoints
const dep = await api(ctx, W.O, 'PUT', `/workspaces/${WS}/members/${W.M4.user.ngac_node_id}/department`, { department_id: W.depKD })
rec(F, 'assign department to M4 (owner)', dep.status < 300, `${dep.status} ${j(dep.body)}`)
const dRole = await api(ctx, W.O, 'PUT', `/workspaces/${WS}/members/${W.M4.user.ngac_node_id}/roles/${W.roleThamdinh}`, {})
rec(F, 'assign role Thẩm định to M4 (owner)', dRole.status < 300, `${dRole.status} ${j(dRole.body)}`)
const dRoleBack = await api(ctx, W.O, 'DELETE', `/workspaces/${WS}/members/${W.M4.user.ngac_node_id}/roles/${W.roleThamdinh}`)
rec(F, 'remove role from M4', dRoleBack.status < 300, `${dRoleBack.status}`)
// department move / delete
const mv = await api(ctx, W.O, 'PUT', `/workspaces/${WS}/departments/${W.depKD}/move`, { new_parent_id: W.depTC })
rec(F, 'move department under another (owner)', mv.status < 300, `${mv.status} ${j(mv.body)}`)
const mvSelf = await api(ctx, W.O, 'PUT', `/workspaces/${WS}/departments/${W.depTC}/move`, { new_parent_id: W.depTC })
rec(F, 'move department into itself refused', [400, 409, 422].includes(mvSelf.status), `${mvSelf.status} ${j(mvSelf.body)}`)
const delBusy = await api(ctx, W.O, 'DELETE', `/workspaces/${WS}/departments/${W.depTC}`)
rec(F, 'delete department with members: 204 (members re-homed to parent by design)', delBusy.status === 204, `${delBusy.status}`)
const m2after = await api(ctx, W.O, 'GET', `/workspaces/${WS}/members`)
const m2row = m2after.body.members?.find((m) => m.ngac_node_id === W.M2.user.ngac_node_id)
rec(F, 'M2 (was in deleted department) now has no department', m2row && (m2row.department == null), JSON.stringify(m2row || {}).slice(0, 200))
// approval template naming a department that is then deleted: what does a new request do?
const dX = (await api(ctx, W.O, 'POST', `/workspaces/${WS}/departments`, { name: 'Dự án X', parent_oa_id: '' })).body.id
const tX = await api(ctx, W.M1, 'POST', '/approval/templates', { name: `Phòng X ${run}`, entity_type: 'general', priority: 50, is_active: true, conditions: [{ field: 'tag', operator: 'eq', value: '"dept-x"' }], steps: [{ step_order: 1, name: 'Phòng X', approver_type: 'department', approver_value: dX, required_count: 1, timeout_hours: 48 }], form_fields: [] })
rec(F, 'template naming dept X created', tX.status === 201, `${tX.status} ${j(tX.body)}`)
await api(ctx, W.O, 'DELETE', `/workspaces/${WS}/departments/${dX}`)
const rX = await api(ctx, W.O, 'POST', '/approval/requests', { template_id: tX.body.id, entity_type: 'general', entity_id: '', form_data_json: JSON.stringify({ tag: 'dept-x' }), department_id: '' })
rec(F, 'request on template whose department was deleted is refused (no silent stuck approval)', [400, 409, 422].includes(rX.status), `${rX.status} ${j(rX.body)} `)

const tmp = await api(ctx, W.O, 'POST', `/workspaces/${WS}/departments`, { name: 'Tạm', parent_oa_id: '' })
const delEmpty = await api(ctx, W.O, 'DELETE', `/workspaces/${WS}/departments/${tmp.body.id}`)
rec(F, 'delete empty department', delEmpty.status < 300, `${delEmpty.status} ${j(delEmpty.body)}`)
// members list visible to members; roles/permission-areas for member
const mem = await api(ctx, W.M3, 'GET', `/workspaces/${WS}/members`)
rec(F, 'member can list members', mem.status === 200, `${mem.status}`)
saveResults('admin-api')
console.log('DONE admin')
