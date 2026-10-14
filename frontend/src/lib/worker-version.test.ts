import { describe, expect, it } from 'vitest'
import { versionWorkerImport } from './worker-version'

const worker = "/* global importScripts */\n// comment\nimportScripts('/sw.js')\n"

describe('versionWorkerImport', () => {
  it('names the build in the app worker import, so the registered script changes with every deploy', () => {
    expect(versionWorkerImport(worker, 'abc123')).toContain("importScripts('/sw.js?v=abc123')")
  })

  it('gives two builds two different scripts', () => {
    expect(versionWorkerImport(worker, 'one')).not.toBe(versionWorkerImport(worker, 'two'))
  })

  it('refuses a worker without the import rather than shipping one that never updates', () => {
    expect(() => versionWorkerImport('importScripts("/other.js")', 'x')).toThrow()
  })
})
