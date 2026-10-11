import { describe, expect, it } from 'vitest'
import {
  MAX_BYTES, MAX_LINES, officeLabel, parseCsv, readCappedText, splitMiddle, takeLines, textFlavor,
} from './viewer-text'

const response = (body: string) => new Response(body)

describe('readCappedText', () => {
  it('returns a small body whole', async () => {
    expect(await readCappedText(response('xin chào'), MAX_BYTES)).toEqual({ text: 'xin chào', cut: false })
  })

  it('stops at the byte cap and says it cut', async () => {
    const { text, cut } = await readCappedText(response('a'.repeat(100)), 10)
    expect(cut).toBe(true)
    expect(text.length).toBeLessThanOrEqual(10)
  })

  it('does not break a multi-byte character at the cap', async () => {
    const { text } = await readCappedText(response('ệ'.repeat(10)), 4)
    expect(text).not.toContain('�')
  })
})

describe('takeLines', () => {
  it('keeps everything under the cap and knows the total', () => {
    expect(takeLines('a\nb\nc', 5)).toEqual({ lines: ['a', 'b', 'c'], total: 3 })
  })

  it('cuts at the cap and still counts the whole when the text is whole', () => {
    const text = Array.from({ length: MAX_LINES + 20 }, (_, i) => String(i)).join('\n')
    const r = takeLines(text, MAX_LINES)
    expect(r.lines).toHaveLength(MAX_LINES)
    expect(r.total).toBe(MAX_LINES + 20)
  })

  it('ignores one trailing newline', () => {
    expect(takeLines('a\nb\n', 5).total).toBe(2)
  })
})

describe('parseCsv', () => {
  it('splits rows and cells', () => {
    expect(parseCsv('a,b\n1,2\r\n3,4')).toEqual([['a', 'b'], ['1', '2'], ['3', '4']])
  })

  it('reads quoted cells with commas, quotes and newlines', () => {
    expect(parseCsv('a,"b,1","say ""hi""","x\ny"')).toEqual([['a', 'b,1', 'say "hi"', 'x\ny']])
  })

  it('detects a semicolon delimiter', () => {
    expect(parseCsv('a;b\n1;2')).toEqual([['a', 'b'], ['1', '2']])
  })

  it('drops a trailing blank line', () => {
    expect(parseCsv('a,b\n')).toEqual([['a', 'b']])
  })
})

describe('textFlavor', () => {
  it.each([
    ['d.csv', 'text/csv', 'csv'],
    ['d.csv', 'application/vnd.ms-excel', 'csv'],
    ['a.py', 'text/plain', 'code'],
    ['a.json', 'application/json', 'code'],
    ['index.html', 'text/plain', 'code'],
    ['ghi-chu.txt', 'text/plain', 'text'],
    ['README', 'text/plain', 'text'],
  ])('%s (%s) is %s', (name, mime, flavor) => {
    expect(textFlavor(name, mime)).toBe(flavor)
  })
})

describe('splitMiddle', () => {
  it('keeps the tail with the extension', () => {
    const [head, tail] = splitMiddle('Biên bản đối soát tháng 10 - bản cuối cùng.docx')
    expect(head + tail).toBe('Biên bản đối soát tháng 10 - bản cuối cùng.docx')
    expect(tail.endsWith('.docx')).toBe(true)
  })

  it('leaves a short name in one piece', () => {
    expect(splitMiddle('a.pdf')).toEqual(['a.pdf', ''])
  })
})

describe('officeLabel', () => {
  it.each([['a.docx', 'Word'], ['a.rtf', 'Word'], ['a.pptx', 'PowerPoint'], ['a.odt', 'OpenDocument'], ['a.pdf', 'tài liệu']])(
    '%s is %s', (name, label) => expect(officeLabel(name)).toBe(label))
})
