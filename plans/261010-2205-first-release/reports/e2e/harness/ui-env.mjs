import fs from 'node:fs'
import { OUT } from './lib.mjs'
export const WORLD = JSON.parse(fs.readFileSync(`${OUT}/world.json`, 'utf8'))
