import { givenNameOf } from './contacts-model'

/** A list this long or longer gets the A–Z column on a phone (DESIGN.md "Danh bạ" › Điện thoại). */
export const ALPHABET_FROM = 30

/** Letters of the index; Đ is a letter of its own. Any other mark (Ă, Â, Ê, Ô, Ơ, Ư) files under its base letter. */
const LETTERS = new Set('ABCDĐEFGHIJKLMNOPQRSTUVWXYZ')

/**
 * The letter a person is filed under: the first letter of their given name (the last word, which the list is
 * sorted by), "#" when it is not a letter.
 */
export function letterOf(fullName: string): string {
  const first = givenNameOf(fullName).charAt(0).toLocaleUpperCase('vi')
  if (first === 'Đ') return 'Đ'
  const base = first.normalize('NFD').replace(/[̀-ͯ]/g, '')
  return LETTERS.has(base) ? base : '#'
}

/** The letters of a sorted list, each once, in the order its rows come: the list's own order is the index's. */
export function indexLettersOf(names: string[]): string[] {
  const out: string[] = []
  for (const name of names) {
    const letter = letterOf(name)
    if (!out.includes(letter)) out.push(letter)
  }
  return out
}
