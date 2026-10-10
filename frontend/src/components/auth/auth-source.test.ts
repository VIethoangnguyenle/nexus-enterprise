import { describe, expect, it } from 'vitest'

/**
 * Guards that read the source the sign-in screens ship, so a regression cannot
 * hide behind a test that happens to mock the one place it lives.
 */

// Raw text of every file the guards cover, keyed by path relative to src/.
const RAW = {
  ...import.meta.glob('../../components/auth/*.{ts,tsx}', { query: '?raw', import: 'default', eager: true }),
  ...import.meta.glob('../../routes/_auth/*.{ts,tsx}', { query: '?raw', import: 'default', eager: true }),
  ...import.meta.glob('../../routes/_auth.tsx', { query: '?raw', import: 'default', eager: true }),
  ...import.meta.glob('../../routes/auth.google.done.tsx', { query: '?raw', import: 'default', eager: true }),
  ...import.meta.glob('../../components/primitives/OtpInput.tsx', { query: '?raw', import: 'default', eager: true }),
  ...import.meta.glob('../../hooks/useAuth.ts', { query: '?raw', import: 'default', eager: true }),
  ...import.meta.glob('../../hooks/useInvitations.ts', { query: '?raw', import: 'default', eager: true }),
  ...import.meta.glob('../../lib/auth-flow.ts', { query: '?raw', import: 'default', eager: true }),
} as Record<string, string>

/** Comments explain what the code avoids ("no slug"); only code can break a rule. */
function stripComments(text: string): string {
  return text.replace(/\/\*[\s\S]*?\*\//g, '').replace(/(^|[^:'"`])\/\/.*$/gm, '$1')
}

/** Every non-test source file of the sign-in group. */
function sources(): { file: string; text: string }[] {
  return Object.entries(RAW)
    .map(([path, text]) => ({ file: new URL(path, 'file:///src/components/auth/x.test.ts').pathname.slice('/src/'.length), text }))
    .filter(({ file }) => !/\.test\.|\/-/.test(file))
    .map(({ file, text }) => ({ file, text: stripComments(text) }))
}

describe('sign-in sources', () => {
  const files = sources()

  it('reads the files it is meant to guard', () => {
    expect(files.length).toBeGreaterThan(10)
    expect(files.map((f) => f.file)).toEqual(expect.arrayContaining([
      'components/auth/LoginScreen.tsx', 'components/auth/CodeEntry.tsx', 'components/primitives/OtpInput.tsx',
    ]))
  })

  // The fixed test code is a server setting. Printing it on a screen, in any
  // build, publishes a master key to the sign-in of every account.
  it('never contains the fixed test code or a "test mode" line, in any build', () => {
    for (const { file, text } of files) {
      expect(text, file).not.toMatch(/999999/)
      expect(text, file).not.toMatch(/test mode|OTP code is|import\.meta\.env\.(DEV|PROD)/i)
    }
  })

  it('uses no off-system motion, colour, layer or class (DESIGN.md §7, §2, §4)', () => {
    for (const { file, text } of files) {
      expect(text, file).not.toMatch(/transition-all|animate-bounce|animate-pulse|\bz-\[|rgba?\(|\bduration-\d/)
      expect(text, file).not.toMatch(/\b(bg|text|border|ring|fill|stroke)-(slate|gray|zinc|neutral|stone|red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose)-\d/)
      expect(text, file).not.toMatch(/shake/i)
    }
  })

  it('has no hex colour but Google’s own logo', () => {
    for (const { file, text } of files.filter((f) => !f.file.endsWith('GoogleLogo.tsx'))) {
      expect(text, file).not.toMatch(/#[0-9a-fA-F]{6}\b/)
    }
  })

  it('has no raw heading or paragraph tag outside Heading and Text', () => {
    for (const { file, text } of files.filter((f) => f.file.startsWith('components/auth') || f.file.startsWith('routes'))) {
      expect(text, file).not.toMatch(/<(h[1-6]|p)[\s>]/)
    }
  })

  it('never asks for or builds a workspace URL or slug', () => {
    for (const { file, text } of files) {
      expect(text, file).not.toMatch(/slug|enterpriseflow|workspace-url/i)
    }
  })
})
