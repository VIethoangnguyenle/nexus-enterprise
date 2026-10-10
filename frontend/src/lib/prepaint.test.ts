// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import html from '../../index.html?raw'

/** The inline script of index.html, run the way the browser runs it before the bundle. */
const script = /<script>([\s\S]*?)<\/script>/.exec(html)![1]!
const run = () => new Function(script)()
const root = document.documentElement

beforeEach(() => { localStorage.clear(); root.removeAttribute('data-theme'); root.removeAttribute('data-motion') })
afterEach(() => localStorage.clear())

describe('pre-paint theme script', () => {
  it('does nothing when nothing is stored', () => {
    run()
    expect(root.hasAttribute('data-theme')).toBe(false)
  })

  it('applies the device\'s last choice', () => {
    localStorage.setItem('ngac-prefs:last', JSON.stringify({ theme: 'dark', motion: 'reduce' }))
    run()
    expect(root.getAttribute('data-theme')).toBe('dark')
    expect(root.getAttribute('data-motion')).toBe('reduce')
  })

  it('prefers the signed-in person\'s own choice over the device\'s last', () => {
    localStorage.setItem('ngac-auth', JSON.stringify({ state: { user: { id: 'u1' } } }))
    localStorage.setItem('ngac-prefs:u1', JSON.stringify({ theme: 'light' }))
    localStorage.setItem('ngac-prefs:last', JSON.stringify({ theme: 'dark' }))
    run()
    expect(root.hasAttribute('data-theme')).toBe(false)
  })

  it('survives garbage in storage', () => {
    localStorage.setItem('ngac-prefs:last', '{nope')
    localStorage.setItem('ngac-auth', 'also not json')
    expect(run).not.toThrow()
    expect(root.hasAttribute('data-theme')).toBe(false)
  })
})
