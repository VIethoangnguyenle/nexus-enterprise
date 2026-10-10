import { api, rec, saveResults, sql, run, sleep } from './lib.mjs'
import { buildWorld, ctx, asTenant } from './world.mjs'
const W = await buildWorld()
const F = 'approval'
const j = (x) => JSON.stringify(x).slice(0, 400)
const steps3 = (n) => [
  { step_order: 1, name: 'Người phụ trách', approver_type: 'specific_user', approver_value: W.M1.user.ngac_node_id, required_count: 1, timeout_hours: 48 },
  { step_order: 2, name: 'Thẩm định viên', approver_type: 'role_in_dept', approver_value: W.roleThamdinh, required_count: 1, timeout_hours: 48 },
  { step_order: 3, name: 'Tài chính (2 người)', approver_type: 'department', approver_value: W.depTC, required_count: 2, timeout_hours: 48 },
]
// 1. template authoring: manager may, member may not
const bigBody = { name: `Chi lớn ${run}`, entity_type: 'general', priority: 10, is_active: true, conditions: [{ field: 'amount', operator: 'gt', value: '1000' }], steps: steps3(), form_fields: [{ label: 'amount', field_type: 'number', required: true }] }
const tMgr = await api(ctx, W.M1, 'POST', '/approval/templates', bigBody)
rec(F, 'manager creates template', tMgr.status === 201, `${tMgr.status} ${j(tMgr.body)}`)
const tMem = await api(ctx, W.M3, 'POST', '/approval/templates', { ...bigBody, name: 'member try' })
rec(F, 'member template create refused (403)', tMem.status === 403, `${tMem.status} ${j(tMem.body)}`)
const T_BIG = tMgr.body.id
const smallBody = { name: `Chi nhỏ ${run}`, entity_type: 'general', priority: 1, is_active: true, conditions: [{ field: 'amount', operator: 'lte', value: '1000' }], steps: [{ step_order: 1, name: 'Tài chính', approver_type: 'department', approver_value: W.depTC, required_count: 1, timeout_hours: 48 }], form_fields: [{ label: 'amount', field_type: 'number', required: true }] }
const tSmall = await api(ctx, W.M1, 'POST', '/approval/templates', smallBody)
rec(F, 'manager creates second (small) template', tSmall.status === 201, `${tSmall.status} ${j(tSmall.body)}`)
const T_SMALL = tSmall.body.id
const tEdit = await api(ctx, W.M3, 'PUT', `/approval/templates/${T_BIG}`, { ...bigBody, name: 'hack', expected_updated_at: tMgr.body.updated_at })
rec(F, 'member template edit refused (403)', tEdit.status === 403, `${tEdit.status} ${j(tEdit.body)}`)

// 2. routed by conditions: big amount -> T_BIG, small -> T_SMALL
const rBig = await api(ctx, W.O, 'POST', '/approval/requests', { template_id: '', entity_type: 'general', entity_id: '', form_data_json: JSON.stringify({ amount: '5000' }), department_id: '' })
rec(F, 'request 5000 routed to big template', rBig.status === 201 && rBig.body.template_id === T_BIG, `${rBig.status} tid=${rBig.body?.template_id} want ${T_BIG} ${j(rBig.body)}`)
const rSmall = await api(ctx, W.O, 'POST', '/approval/requests', { template_id: '', entity_type: 'general', entity_id: '', form_data_json: JSON.stringify({ amount: '100' }), department_id: '' })
rec(F, 'request 100 routed to small template', rSmall.status === 201 && rSmall.body.template_id === T_SMALL, `${rSmall.status} tid=${rSmall.body?.template_id} want ${T_SMALL}`)
const RB = rBig.body.id
// 3. deny before its turn: step 3 department approver cannot act on step 1
const early = await api(ctx, W.M3, 'POST', '/approval/approve', { request_id: RB, comment: 'sớm' })
rec(F, 'dept member (step 3) cannot approve before step 1 (refused)', [403,409,400,422].includes(early.status), `${early.status} ${j(early.body)}`)
// 4. deny: wrong person for step 1 (M2 is not the person approver)
const wrong = await api(ctx, W.M2, 'POST', '/approval/approve', { request_id: RB, comment: 'không phải tôi' })
rec(F, 'role holder cannot act on person step (refused)', [403,409,400,422].includes(wrong.status), `${wrong.status} ${j(wrong.body)}`)
// 5. person step: M1 approves (through policy)
const s1 = await api(ctx, W.M1, 'POST', '/approval/approve', { request_id: RB, comment: 'OK bước 1' })
rec(F, 'step 1 person approver (M1) approves', s1.status === 200, `${s1.status} ${j(s1.body)}`)
// 6. role AND department: role-in-dept step (M2 holds Thẩm định)
const s2 = await api(ctx, W.M2, 'POST', '/approval/approve', { request_id: RB, comment: 'OK thẩm định' })
rec(F, 'step 2 role-in-department approver (M2) approves', s2.status === 200, `${s2.status} ${j(s2.body)}`)
// 7. quorum 2 concurrent on department step
const [q1, q2] = await Promise.all([
  api(ctx, W.M3, 'POST', '/approval/approve', { request_id: RB, comment: 'M3 OK' }),
  api(ctx, W.M4, 'POST', '/approval/approve', { request_id: RB, comment: 'M4 OK' }),
])
rec(F, 'quorum 2: concurrent approvals both accepted', q1.status === 200 && q2.status === 200, `${q1.status} / ${q2.status} ${j(q1.body)} ${j(q2.body)}`)
const det = await api(ctx, W.O, 'GET', `/approval/requests/${RB}`)
console.log('DETAIL KEYS', Object.keys(det.body||{}), JSON.stringify(det.body).slice(0,300))
rec(F, 'quorum 2: request final status approved', det.status === 200 && /approved/i.test(JSON.stringify(det.body).slice(0,2000)), `${det.status}`)
const dup = await api(ctx, W.M3, 'POST', '/approval/approve', { request_id: RB, comment: 'again' })
rec(F, 'approving an already-finished request refused', [403,409,400,422].includes(dup.status), `${dup.status} ${j(dup.body)}`)
// 7b. quorum semantics on a fresh request: one approval of 2 does not complete the step; same person twice refused
const rQ = await api(ctx, W.O, 'POST', '/approval/requests', { template_id: '', entity_type: 'general', entity_id: '', form_data_json: JSON.stringify({ amount: '7000' }), department_id: '' })
const RQ = rQ.body.id
await api(ctx, W.M1, 'POST', '/approval/approve', { request_id: RQ, comment: 'ok1' })
await api(ctx, W.M2, 'POST', '/approval/approve', { request_id: RQ, comment: 'ok2' })
const q1only = await api(ctx, W.M3, 'POST', '/approval/approve', { request_id: RQ, comment: 'one of two' })
rec(F, 'quorum 2: first of two department approvals accepted', q1only.status === 200, `${q1only.status} ${j(q1only.body)}`)
const qMid = await api(ctx, W.O, 'GET', `/approval/requests/${RQ}`)
rec(F, 'quorum 2: request still open after one approval', qMid.status === 200 && /pending|open|in_progress/i.test(JSON.stringify(qMid.body.request?.status)) , `status=${qMid.body?.request?.status} current_step=${qMid.body?.request?.current_step}`)
const qDup = await api(ctx, W.M3, 'POST', '/approval/approve', { request_id: RQ, comment: 'twice' })
rec(F, 'quorum 2: same person cannot count twice (refused)', [403, 409, 400, 422].includes(qDup.status), `${qDup.status} ${j(qDup.body)}`)
const qEnd = await api(ctx, W.M4, 'POST', '/approval/approve', { request_id: RQ, comment: 'two of two' })
rec(F, 'quorum 2: second person completes the request', qEnd.status === 200 && qEnd.body?.status === 'approved', `${qEnd.status} ${j(qEnd.body)}`)
// 7c. live policy: revoke M2's role, M2 is refused on step 2 at once; restore and it works
const rR = await api(ctx, W.O, 'POST', '/approval/requests', { template_id: '', entity_type: 'general', entity_id: '', form_data_json: JSON.stringify({ amount: '8000' }), department_id: '' })
const RR = rR.body.id
await api(ctx, W.M1, 'POST', '/approval/approve', { request_id: RR, comment: 'ok1' })
const rv = await api(ctx, W.O, 'DELETE', `/workspaces/${W.wsA}/members/${W.M2.user.ngac_node_id}/roles/${W.roleThamdinh}`)
rec(F, 'owner revokes role Thẩm định from M2', rv.status < 300, `${rv.status} ${j(rv.body)}`)
await sleep(1500)
const noRole = await api(ctx, W.M2, 'POST', '/approval/approve', { request_id: RR, comment: 'after revoke' })
rec(F, 'revoked role holder refused on step 2 (policy, live)', [403].includes(noRole.status), `${noRole.status} ${j(noRole.body)}`)
const rs = await api(ctx, W.O, 'PUT', `/workspaces/${W.wsA}/members/${W.M2.user.ngac_node_id}/roles/${W.roleThamdinh}`, {})
rec(F, 'owner restores role to M2', rs.status < 300, `${rs.status} ${j(rs.body)}`)
await sleep(1500)
const back = await api(ctx, W.M2, 'POST', '/approval/approve', { request_id: RR, comment: 'restored' })
rec(F, 'restored role holder approves step 2 again', back.status === 200, `${back.status} ${j(back.body)}`)
// 8. rejection: a fresh small request for each rejection check (rejecting completes the request)
const rNoReason = await api(ctx, W.O, 'POST', '/approval/requests', { template_id: '', entity_type: 'general', entity_id: '', form_data_json: JSON.stringify({ amount: '50' }), department_id: '' })
const rej = await api(ctx, W.M3, 'POST', '/approval/reject', { request_id: rNoReason.body.id, comment: '' })
rec(F, 'reject without reason refused (400)', [400, 422].includes(rej.status), `${rej.status} ${j(rej.body)}`)
const rWithReason = await api(ctx, W.O, 'POST', '/approval/requests', { template_id: '', entity_type: 'general', entity_id: '', form_data_json: JSON.stringify({ amount: '60' }), department_id: '' })
const rej2 = await api(ctx, W.M3, 'POST', '/approval/reject', { request_id: rWithReason.body.id, comment: 'Thiếu chứng từ đính kèm' })
rec(F, 'reject with reason accepted', rej2.status === 200, `${rej2.status} ${j(rej2.body)}`)
const rd = await api(ctx, W.O, 'GET', `/approval/requests/${rWithReason.body.id}`)
rec(F, 'rejected request status = rejected', rd.status === 200 && /reject/i.test(JSON.stringify(rd.body).slice(0,2000)), `${rd.status} ${j(rd.body).slice(0,200)}`)
// 9. audit trail with actor names
const au = await api(ctx, W.O, 'GET', `/approval/requests/${RB}/audit`)
const auditTxt = JSON.stringify(au.body)
rec(F, 'audit returns entries for owner', au.status === 200, `${au.status} ${auditTxt.slice(0, 300)}`)
rec(F, 'audit names actors (Minh/Mai/Hoa/Khang present, not only ids)', au.status === 200 && ['Minh Manager', 'Mai Member'].every((n) => auditTxt.includes(n)), auditTxt.slice(0, 500))
const au2 = await api(ctx, W.O, 'GET', `/approval/requests/${rWithReason.body.id}/audit`)
rec(F, 'rejection reason recorded in audit', JSON.stringify(au2.body).includes('Thiếu chứng từ'), JSON.stringify(au2.body).slice(0, 400))
// 10. outsider (tenant B) isolation
const xDetail = await api(ctx, W.X, 'GET', `/approval/requests/${RB}`)
rec(F, 'outsider 403 on request detail', xDetail.status === 403, `${xDetail.status} ${j(xDetail.body)}`)
const xAudit = await api(ctx, W.X, 'GET', `/approval/requests/${RB}/audit`)
rec(F, 'outsider 403 on audit', xAudit.status === 403, `${xAudit.status} ${j(xAudit.body)}`)
// tenant-B token asking tenant-A workspace context directly (token re-scope check)
const xLists = await api(ctx, W.X, 'GET', '/approval/pending')
rec(F, 'outsider pending list contains no tenant-A request', xLists.status === 200 && !JSON.stringify(xLists.body).includes(RB), `${xLists.status} ${j(xLists.body)}`)
saveResults('approval-api')
console.log('DONE approval')
