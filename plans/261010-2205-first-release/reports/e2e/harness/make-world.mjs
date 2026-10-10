import fs from 'node:fs'
import { buildWorld } from './world.mjs'
import { OUT, run } from './lib.mjs'
const W = await buildWorld()
const pick = (p) => ({ email: p.email, name: p.user.display_name || p.user.username, id: p.user.id, node: p.user.ngac_node_id })
const out = { run, wsA: W.wsA, wsB: W.wsB, depKD: W.depKD, depTC: W.depTC, roleMgr: W.roleMgr, roleThamdinh: W.roleThamdinh,
  O: { email: W.O.email || `e2e-o-${run}@example.test`, id: W.O.user.id, node: W.O.user.ngac_node_id },
  X: { email: `e2e-x-${run}@example.test`, id: W.X.user.id, node: W.X.user.ngac_node_id },
  M1: { email: `e2e-m1-${run}@example.test`, id: W.M1.user.id, node: W.M1.user.ngac_node_id },
  M2: { email: `e2e-m2-${run}@example.test`, id: W.M2.user.id, node: W.M2.user.ngac_node_id },
  M3: { email: `e2e-m3-${run}@example.test`, id: W.M3.user.id, node: W.M3.user.ngac_node_id },
  M4: { email: `e2e-m4-${run}@example.test`, id: W.M4.user.id, node: W.M4.user.ngac_node_id } }
fs.writeFileSync(`${OUT}/world.json`, JSON.stringify(out, null, 2))
fs.writeFileSync(`${process.env.SP_DIR}/world-tokens.json`, JSON.stringify({ O: W.O.token, M1: W.M1.token, M2: W.M2.token, M3: W.M3.token, M4: W.M4.token, X: W.X.token }))
console.log('world written', run, out.wsA)
process.exit(0)
