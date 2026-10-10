import { api, rec, saveResults, sleep, sql, run } from './lib.mjs'
import { buildWorld, ctx } from './world.mjs'
const W = await buildWorld()
const WS = W.wsA
const j = (x) => JSON.stringify(x).slice(0, 300)
const F = 'documents'
// ---- documents: create, autosave (PATCH with base_version), conflict across two tabs
const cd = await api(ctx, W.O, 'POST', `/workspaces/${WS}/documents/texts`, { title: 'Báo cáo tuần', folder_id: '', content: '<p>Bản nháp 1</p>' })
rec(F, 'create text document', cd.status === 201, `${cd.status} ${j(cd.body)}`)
const DOC = cd.body.id
const v1 = await api(ctx, W.O, 'GET', `/documents/texts/${DOC}`)
rec(F, 'owner reads document with version', v1.status === 200 && v1.body.version >= 1, `${v1.status} ${j(v1.body)}`)
const base = v1.body.version
const save1 = await api(ctx, W.O, 'PATCH', `/documents/texts/${DOC}`, { base_version: base, content: '<p>Tab 1 lưu</p>' })
rec(F, 'autosave from tab 1 accepted', save1.status === 200, `${save1.status} ${j(save1.body)}`)
const stale = await api(ctx, W.M3, 'PATCH', `/documents/texts/${DOC}`, { base_version: base, content: '<p>Tab 2 cũ</p>' })
rec(F, 'stale save from tab 2 -> 409 with current content', stale.status === 409 && JSON.stringify(stale.body).includes('Tab 1 lưu'), `${stale.status} ${j(stale.body)}`)
const fresh = await api(ctx, W.M3, 'PATCH', `/documents/texts/${DOC}`, { base_version: (save1.body.version ?? base + 1), content: '<p>Tab 2 sau khi tải lại</p>' })
rec(F, 'member (write on Documents) saves after reload', fresh.status === 200, `${fresh.status} ${j(fresh.body)}`)
// sanitisation is a render concern: the stored text is raw, the UI must neutralise it (checked in the UI flow)
const xss = await api(ctx, W.O, 'POST', `/workspaces/${WS}/documents/texts`, { title: 'XSS', folder_id: '', content: '<img src=x onerror="window.__pwned=1"><a href="javascript:window.__pwned=2">x</a><script>window.__pwned=3</script>' })
rec(F, 'raw script content stored (render must neutralise; see UI flow)', xss.status === 201, `${xss.status}`)
// outsider cannot read it
const xr = await api(ctx, W.X, 'GET', `/documents/texts/${DOC}`)
rec(F, 'outsider (tenant B) cannot read tenant-A document (403)', xr.status === 403, `${xr.status} ${j(xr.body)}`)
const xl = await api(ctx, W.X, 'GET', `/workspaces/${WS}/documents/texts`)
rec(F, 'outsider document list for tenant A is refused or empty', [403].includes(xl.status) || (xl.status === 200 && !JSON.stringify(xl.body).includes('Báo cáo')), `${xl.status} ${j(xl.body)}`)
// ---- assets: type with custom fields, asset, request, approve-and-assign, return, lifecycle, 409 races
const schema = JSON.stringify({ type: 'object', properties: { serial: { type: 'string', title: 'Số serial', 'x-kind': 'text' }, owner_person: { type: 'string', title: 'Người giữ', 'x-kind': 'person' } }, required: ['serial'] })
const at = await api(ctx, W.O, 'POST', `/workspaces/${WS}/asset-types`, { name: `Laptop ${run}`, category: 'it', description: '', fields_schema: schema })
rec('assets', 'create asset type with custom fields', at.status === 201, `${at.status} ${j(at.body)}`)
const TYPE = at.body.id
const badAsset = await api(ctx, W.O, 'POST', `/workspaces/${WS}/assets`, { type_id: TYPE, name: 'Máy A', custom_fields: {} })
rec('assets', 'required custom field blank refused (400)', [400, 422].includes(badAsset.status), `${badAsset.status} ${j(badAsset.body)}`)
const a1 = await api(ctx, W.O, 'POST', `/workspaces/${WS}/assets`, { type_id: TYPE, name: 'Máy A', custom_fields: { serial: 'SN-1' } })
const a2 = await api(ctx, W.O, 'POST', `/workspaces/${WS}/assets`, { type_id: TYPE, name: 'Máy B', custom_fields: { serial: 'SN-2' } })
rec('assets', 'add two assets', a1.status === 201 && a2.status === 201, `${a1.status}/${a2.status} ${j(a1.body)}`)
const A1 = a1.body.id, A2 = a2.body.id
// Requests: a member cannot file one (write on the type is needed, and no product path grants it), and the owner
// cannot decide his own, so the request path is checked for refusal only (see finding).
const rq = await api(ctx, W.M3, 'POST', `/workspaces/${WS}/asset-requests`, { type_id: TYPE, reason: 'Làm việc từ xa', urgency: 'normal', quantity: 1 })
rec('assets', 'member files asset request: refused (403) because write on type is not grantable to members', rq.status === 403, `${rq.status} ${j(rq.body)}`)
const rqSelf = await api(ctx, W.O, 'POST', `/workspaces/${WS}/asset-requests`, { type_id: TYPE, reason: 'Tự yêu cầu', urgency: 'normal', quantity: 1 })
const selfDecide = rqSelf.body?.id ? await api(ctx, W.O, 'POST', `/asset-requests/${rqSelf.body.id}/approve`, { asset_id: A2, comment: 'tự duyệt' }) : { status: 0, body: 'no request' }
rec('assets', 'owner cannot approve own request (refused)', [403, 409].includes(selfDecide.status), `req=${rqSelf.status} decide=${selfDecide.status} ${j(selfDecide.body)}`)
// Lifecycle with the owner: requested -> available (approve), assign (hand-over), return
const ap1 = await api(ctx, W.O, 'POST', `/assets/${A1}/transition`, { action: 'approve', comment: 'nhập kho' })
rec('assets', 'lifecycle approve: requested -> available', ap1.status === 200 && ap1.body.state === 'available', `${ap1.status} state=${ap1.body?.state}`)
const as1 = await api(ctx, W.O, 'POST', `/assets/${A1}/assign`, { assignee_id: W.M3.user.id, comment: 'bàn giao' })
rec('assets', 'hand over (assign) to member M3', as1.status === 200, `${as1.status} ${j(as1.body)}`)
const hist = await api(ctx, W.O, 'GET', `/assets/${A1}/history`)
rec('assets', 'history names the actor (Olivia Owner) and the holder', hist.status === 200 && JSON.stringify(hist.body).includes('Olivia Owner'), `${hist.status} ${j(hist.body).slice(0, 240)}`)
const ret = await api(ctx, W.O, 'POST', `/assets/${A1}/return`, { comment: 'trả máy' })
rec('assets', 'return asset (assigned -> available)', ret.status === 200, `${ret.status} ${j(ret.body)}`)
const tm = await api(ctx, W.O, 'POST', `/assets/${A1}/transition`, { action: 'flag_maintenance', comment: 'hỏng màn hình' })
rec('assets', 'lifecycle: flag_maintenance', tm.status === 200, `${tm.status} ${j(tm.body)}`)
const tm2 = await api(ctx, W.O, 'POST', `/assets/${A1}/transition`, { action: 'retire', comment: 'thanh lý' })
rec('assets', 'lifecycle: retire from maintenance', tm2.status === 200, `${tm2.status} ${j(tm2.body)}`)
const bad = await api(ctx, W.O, 'POST', `/assets/${A1}/transition`, { action: 'approve', comment: '' })
rec('assets', 'invalid transition from retired refused (409)', [409, 400, 422].includes(bad.status), `${bad.status} ${j(bad.body)}`)
const mem = await api(ctx, W.M3, 'POST', `/assets/${A2}/transition`, { action: 'approve', comment: '' })
rec('assets', 'member cannot run a lifecycle step on a type he cannot manage (403)', mem.status === 403, `${mem.status} ${j(mem.body)}`)
// 409 races: two concurrent steps on one asset; two concurrent hand-overs of one asset
const [t1, t2] = await Promise.all([
  api(ctx, W.O, 'POST', `/assets/${A2}/transition`, { action: 'approve', comment: 'a' }),
  api(ctx, W.O, 'POST', `/assets/${A2}/transition`, { action: 'approve', comment: 'b' }),
])
rec('assets', 'race: two concurrent approve steps -> one 200, one 409 state_changed', [t1.status, t2.status].sort().join() === '200,409', `${t1.status}/${t2.status} ${j(t1.body)} | ${j(t2.body)}`)
const [h1, h2] = await Promise.all([
  api(ctx, W.O, 'POST', `/assets/${A2}/assign`, { assignee_id: W.M3.user.id, comment: 'x' }),
  api(ctx, W.O, 'POST', `/assets/${A2}/assign`, { assignee_id: W.M4.user.id, comment: 'y' }),
])
rec('assets', 'race: two concurrent hand-overs of one asset -> one 200, one 409', [h1.status, h2.status].sort().join() === '200,409', `${h1.status}/${h2.status} ${j(h1.body)} | ${j(h2.body)}`)
const xa = await api(ctx, W.X, 'GET', `/assets/${A2}`)
rec('assets', 'outsider cannot read tenant-A asset (403)', xa.status === 403, `${xa.status} ${j(xa.body)}`)
saveResults('docs-assets-api')
console.log('DONE docs/assets')
