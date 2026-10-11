import '@testing-library/jest-dom/vitest'

// jsdom has no layout boxes; the rich-text editor asks for them whenever the caret moves.
if (typeof Range !== 'undefined') {
  const none = () => ({ length: 0, item: () => null, [Symbol.iterator]: function* () {} }) as unknown as DOMRectList
  Range.prototype.getClientRects ??= none
  Range.prototype.getBoundingClientRect ??= () => new DOMRect()
}
if (typeof document !== 'undefined') document.elementFromPoint ??= () => null
