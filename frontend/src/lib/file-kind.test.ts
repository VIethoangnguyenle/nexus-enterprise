import { describe, expect, it } from 'vitest'
import { isImageAttachment } from './file-kind'

const att = (file_name: string, mime_type: string) => ({ drive_file_id: 'f', file_name, mime_type, size_bytes: 1 })

describe('isImageAttachment', () => {
  it('goes by the mime type, then by the extension', () => {
    expect(isImageAttachment(att('a.bin', 'image/png'))).toBe(true)
    expect(isImageAttachment(att('Ảnh.JPG', ''))).toBe(true)
    expect(isImageAttachment(att('bao-cao.pdf', 'application/pdf'))).toBe(false)
  })
})
