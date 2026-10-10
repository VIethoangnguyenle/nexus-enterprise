import { expect } from 'vitest'
import { UUID_RE } from './chat-fixtures'

/** No identifier in anything a person reads: text, names, tooltips, placeholders, field values. */
export function expectNoIds(root: HTMLElement = document.body) {
  expect(root.textContent).not.toMatch(UUID_RE)
  for (const el of root.querySelectorAll('[aria-label],[title],[alt],[placeholder],[href]')) {
    for (const attr of ['aria-label', 'title', 'alt', 'placeholder']) {
      expect(el.getAttribute(attr) ?? '').not.toMatch(UUID_RE)
    }
  }
  for (const el of root.querySelectorAll('input,textarea')) {
    expect((el as HTMLInputElement).value).not.toMatch(UUID_RE)
  }
}
