import { api, rec, saveResults, sleep, sql } from './lib.mjs'
import { buildWorld, ctx } from './world.mjs'
const W = await buildWorld()
const F = 'drive'
const j = (x) => JSON.stringify(x).slice(0, 300)
const WS = W.wsA
// --- upload through the presigned URL, confirm, download
const bytes = Buffer.alloc(2048, 65)
const cf = await api(ctx, W.O, 'POST', `/workspaces/${WS}/drive/files`, { name: 'hop-dong.pdf', mime_type: 'application/pdf', size_bytes: bytes.length, parent_id: '' })
rec(F, 'create file returns presigned upload_url', cf.status === 201 && cf.body.upload_url?.includes('storage.localhost:18080'), `${cf.status}`)
const put = await ctx.fetch(cf.body.upload_url, { method: 'PUT', data: bytes, headers: { 'Content-Type': 'application/pdf' } })
rec(F, 'PUT to presigned URL through edge (storage host) -> 200', put.status() === 200, `${put.status()} ${(await put.text()).slice(0, 200)}`)
const conf = await api(ctx, W.O, 'POST', `/drive/files/${cf.body.file_id}/confirm`, {})
rec(F, 'confirm upload', conf.status === 200, `${conf.status} ${j(conf.body)}`)
const dl = await api(ctx, W.O, 'GET', `/drive/files/${cf.body.file_id}/download`)
rec(F, 'owner gets download URL', dl.status === 200 && !!(dl.body.url || dl.body.download_url), `${dl.status} ${j(dl.body)}`)
const dlUrl = dl.body.url || dl.body.download_url
if (dlUrl) {
  const g = await ctx.fetch(dlUrl)
  const body = await g.body()
  rec(F, 'download bytes match upload (2048)', g.status() === 200 && body.length === 2048, `${g.status()} len=${body.length}`)
}
// --- folders: create, rename, move, move-into-self refused
const fA = await api(ctx, W.O, 'POST', `/workspaces/${WS}/drive/folders`, { name: 'Dự án A', parent_id: '' })
const fB = await api(ctx, W.O, 'POST', `/workspaces/${WS}/drive/folders`, { name: 'Con B', parent_id: fA.body.id || fA.body.folder_id })
rec(F, 'folder create (root and child)', fA.status === 201 && fB.status === 201, `${fA.status}/${fB.status} ${j(fA.body)}`)
const FA = fA.body.id || fA.body.folder_id, FB = fB.body.id || fB.body.folder_id
const ren = await api(ctx, W.O, 'PUT', `/drive/items/${FB}/rename`, { name: 'Con B đổi tên' })
rec(F, 'folder rename', ren.status === 200, `${ren.status} ${j(ren.body)}`)
const selfMove = await api(ctx, W.O, 'POST', `/drive/items/${FA}/move`, { target_folder_id: FA })
rec(F, 'move folder into itself refused', [400, 409, 422].includes(selfMove.status), `${selfMove.status} ${j(selfMove.body)}`)
const descMove = await api(ctx, W.O, 'POST', `/drive/items/${FA}/move`, { target_folder_id: FB })
rec(F, 'move folder into its own descendant refused', [400, 409, 422].includes(descMove.status), `${descMove.status} ${j(descMove.body)}`)
const mv = await api(ctx, W.O, 'POST', `/drive/items/${FB}/move`, { target_folder_id: '' })
rec(F, 'move child folder back to root', mv.status === 200, `${mv.status} ${j(mv.body)}`)
// --- share to a person with "Có thể sửa" (write): grantee reads and downloads
const sh = await api(ctx, W.O, 'POST', `/drive/items/${cf.body.file_id}/share`, { target_node_id: W.M3.user.ngac_node_id, share_type: 'user', permission: 'write' })
rec(F, 'share to M3 with write (Có thể sửa)', sh.status === 201, `${sh.status} ${j(sh.body)}`)
await sleep(800)
const m3get = await api(ctx, W.M3, 'GET', `/drive/items/${cf.body.file_id}`)
rec(F, 'grantee M3 can read the file', m3get.status === 200, `${m3get.status} ${j(m3get.body)}`)
const m3dl = await api(ctx, W.M3, 'GET', `/drive/files/${cf.body.file_id}/download`)
rec(F, 'grantee M3 can download', m3dl.status === 200, `${m3dl.status} ${j(m3dl.body)}`)
const m4get = await api(ctx, W.M4, 'GET', `/drive/items/${cf.body.file_id}`)
rec(F, 'workspace member M4 reads workspace-root file (spec drive-tree-navigation:103 default)', m4get.status === 200, `${m4get.status}`)
const m3shared = await api(ctx, W.M3, 'GET', '/drive/shared-with-me')
rec(F, 'shared-with-me lists the file for M3', m3shared.status === 200 && JSON.stringify(m3shared.body).includes('hop-dong.pdf'), `${m3shared.status} ${j(m3shared.body)}`)
// --- trash and undo
const del = await api(ctx, W.O, 'DELETE', `/drive/items/${FB}`)
rec(F, 'trash folder (soft delete)', del.status < 300, `${del.status} ${j(del.body)}`)
const rst = await api(ctx, W.O, 'POST', `/drive/items/${FB}/restore`, {})
rec(F, 'undo: restore from trash', rst.status < 300, `${rst.status} ${j(rst.body)}`)
// --- quota: a file bigger than the tenant quota is 413
sql(`UPDATE drive_quotas SET max_bytes = 1000 WHERE workspace_id = '${WS}'`)
const big = await api(ctx, W.O, 'POST', `/workspaces/${WS}/drive/files`, { name: 'khong-lo.bin', mime_type: 'application/octet-stream', size_bytes: 5000, parent_id: '' })
let bigStatus = big.status
if (big.status === 201) { const pu = await ctx.fetch(big.body.upload_url, { method: 'PUT', data: Buffer.alloc(5000, 66) }); const cfm = await api(ctx, W.O, 'POST', `/drive/files/${big.body.file_id}/confirm`, {}); bigStatus = cfm.status; big.body = cfm.body }
rec(F, 'file over quota -> 413 (at create or confirm)', bigStatus === 413, `${bigStatus} ${j(big.body)}`)
const quota = await api(ctx, W.O, 'GET', `/workspaces/${WS}/drive/quota`)
rec(F, 'quota endpoint answers', quota.status === 200, `${quota.status} ${j(quota.body)}`)
// --- foreign workspace: tenant B owner asks tenant A folder by URL
const foreign = await api(ctx, W.X, 'GET', `/drive/folders/${FA}`)
rec(F, 'foreign folder by id refused (403)', foreign.status === 403, `${foreign.status} ${j(foreign.body)}`)
const foreignList = await api(ctx, W.X, 'GET', `/workspaces/${WS}/drive`)
const ghost = await api(ctx, W.X, 'GET', `/workspaces/00000000-0000-4000-8000-000000000000/drive`)
rec(F, 'foreign-workspace drive listing refused (403)', foreignList.status === 403, `foreign=${foreignList.status} ${j(foreignList.body)} | nonexistent ws=${ghost.status} ${j(ghost.body)}`)
const xItem = await api(ctx, W.X, 'GET', `/drive/items/${cf.body.file_id}`)
rec(F, 'tenant-B user cannot read tenant-A file by id (403)', xItem.status === 403, `${xItem.status} ${j(xItem.body)}`)
saveResults('drive-api')
console.log('DONE drive')
