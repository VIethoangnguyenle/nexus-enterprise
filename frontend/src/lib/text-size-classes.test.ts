import { describe, expect, it } from 'vitest'

// Every component and route source, as text.
const sources = import.meta.glob(['../components/**/*.tsx', '../routes/**/*.tsx', '../lib/**/*.ts'], {
  query: '?raw',
  import: 'default',
  eager: true,
}) as Record<string, string>

describe('text size classes', () => {
  it('never uses text-base, which also paints the text in the background colour', () => {
    // `base` is a type size and a surface colour, so Tailwind's text-base sets
    // both; 16px text is text-16.
    const offenders = Object.entries(sources)
      .filter(([path]) => !path.endsWith('text-size-classes.test.ts'))
      .filter(([, src]) => /(^|[^-\w])text-base([^-\w]|$)/m.test(src))
      .map(([path]) => path)
    expect(offenders).toEqual([])
  })
})
