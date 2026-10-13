import '@testing-library/jest-dom/vitest'
import { afterAll } from 'vitest'

// jsdom has no layout boxes; the rich-text editor asks for them whenever the caret moves.
if (typeof Range !== 'undefined') {
  const none = () => ({ length: 0, item: () => null, [Symbol.iterator]: function* () {} }) as unknown as DOMRectList
  Range.prototype.getClientRects ??= none
  Range.prototype.getBoundingClientRect ??= () => new DOMRect()
}
if (typeof document !== 'undefined') document.elementFromPoint ??= () => null

// The editor's React binding destroys an unmounted editor on a timer. Let those
// timers run before a test file's environment is torn down: a destroy that
// fires after `window` is gone is reported as an unhandled error and fails the
// run, on a slow machine more than a fast one.
afterAll(() => new Promise<void>((resolve) => setTimeout(resolve, 20)))
