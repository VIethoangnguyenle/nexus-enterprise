import { describe, expect, it } from 'vitest'
import { buildDirectory } from './people'
import {
  FIELD_KINDS, fieldKindLabel, fieldsToSchema, formatFieldValue, newFieldKey, parseFields, validateFieldValues,
  type FieldDef,
} from './asset-fields'
import { CONTACTS, U } from '../test/chat-fixtures'

const people = buildDirectory(CONTACTS)

const SCHEMA = JSON.stringify({
  type: 'object',
  properties: {
    cfg: { title: 'Cấu hình', type: 'string', 'x-kind': 'text' },
    exp: { title: 'Hết bảo hành', type: 'string', 'x-kind': 'date' },
    ram: { title: 'RAM (GB)', type: 'number', 'x-kind': 'number' },
    owner: { title: 'Người phụ trách', type: 'string', 'x-kind': 'person' },
    color: { title: 'Màu', type: 'string', 'x-kind': 'choice', enum: ['Bạc', 'Đen'] },
  },
  required: ['cfg'],
})

describe('reading a type\'s fields', () => {
  it('turns the schema into fields in the order they were written', () => {
    const fields = parseFields(SCHEMA)
    expect(fields.map((f) => [f.title, f.kind, f.required])).toEqual([
      ['Cấu hình', 'text', true], ['Hết bảo hành', 'date', false], ['RAM (GB)', 'number', false],
      ['Người phụ trách', 'person', false], ['Màu', 'choice', false],
    ])
    expect(fields[4]!.options).toEqual(['Bạc', 'Đen'])
  })

  it('reads nothing from no schema, an empty one, or a damaged one', () => {
    expect(parseFields(undefined)).toEqual([])
    expect(parseFields('{}')).toEqual([])
    expect(parseFields('{not json')).toEqual([])
    expect(parseFields('[1,2]')).toEqual([])
  })

  it('reads a schema written before kinds existed as text, titled by nothing the screen would print as a code', () => {
    const [f] = parseFields('{"properties":{"serial":{"type":"string"}}}')
    expect(f!.kind).toBe('text')
    expect(f!.title).toBe('Trường thông tin')
    const [n] = parseFields('{"properties":{"qty":{"type":"number"}}}')
    expect(n!.kind).toBe('number')
  })

  it('names the five kinds in words', () => {
    expect(FIELD_KINDS).toEqual(['text', 'number', 'date', 'person', 'choice'])
    expect(FIELD_KINDS.map(fieldKindLabel)).toEqual(['Văn bản', 'Số', 'Ngày', 'Người', 'Danh sách chọn'])
  })
})

describe('writing them back', () => {
  it('round-trips, so editing a type does not change the fields it did not touch', () => {
    const fields = parseFields(SCHEMA)
    expect(parseFields(JSON.stringify(fieldsToSchema(fields)))).toEqual(fields)
  })

  it('writes the shape the server validates', () => {
    const schema = fieldsToSchema([
      { key: 'a', title: 'Cấu hình', kind: 'text', required: true },
      { key: 'b', title: 'Màu', kind: 'choice', required: false, options: ['Bạc', 'Đen'] },
      { key: 'c', title: 'RAM', kind: 'number', required: false },
    ])
    expect(schema).toEqual({
      type: 'object',
      properties: {
        a: { title: 'Cấu hình', type: 'string', 'x-kind': 'text' },
        b: { title: 'Màu', type: 'string', 'x-kind': 'choice', enum: ['Bạc', 'Đen'] },
        c: { title: 'RAM', type: 'number', 'x-kind': 'number' },
      },
      required: ['a'],
    })
    expect(fieldsToSchema([])).toEqual({})
  })

  it('makes a key that is not on screen and is not taken, even for a title with no letters', () => {
    expect(newFieldKey('Cấu hình', [])).toMatch(/^[a-z0-9_]+$/)
    const k1 = newFieldKey('Cấu hình', [])
    expect(newFieldKey('Cấu hình', [k1])).not.toBe(k1)
    expect(newFieldKey('???', [])).toMatch(/^f/)
  })
})

describe('showing and checking values', () => {
  const [cfg, exp, ram, owner, color] = parseFields(SCHEMA) as [FieldDef, FieldDef, FieldDef, FieldDef, FieldDef]

  it('shows each value the way a person reads it', () => {
    expect(formatFieldValue(cfg, 'M3 Pro, 18 GB', people)).toBe('M3 Pro, 18 GB')
    expect(formatFieldValue(exp, '2027-03-14', people)).toBe('14/03/2027')
    expect(formatFieldValue(ram, 18, people)).toBe('18')
    expect(formatFieldValue(color, 'Bạc', people)).toBe('Bạc')
  })

  it('shows a person by name, and a stranger as a neutral word, never the id', () => {
    expect(formatFieldValue(owner, U.lan.id, people)).toBe('Nguyễn Thu Lan')
    expect(formatFieldValue(owner, 'a-user-who-left', people)).toBe('Thành viên')
  })

  it('shows nothing for a value that is not there', () => {
    expect(formatFieldValue(cfg, undefined, people)).toBe('')
    expect(formatFieldValue(cfg, '', people)).toBe('')
  })

  it('asks for what is required and for dates and numbers that are real', () => {
    expect(validateFieldValues([cfg, exp, ram], { cfg: '', exp: '', ram: '' })).toEqual({ cfg: 'Nhập cấu hình.' })
    expect(validateFieldValues([cfg, exp, ram], { cfg: 'x', exp: '', ram: '' })).toEqual({})
    expect(validateFieldValues([cfg, ram], { cfg: 'x', ram: 'abc' })).toEqual({ ram: 'RAM (GB) phải là một số.' })
    expect(validateFieldValues([cfg, ram], { cfg: 'x', ram: '18,5' })).toEqual({})
  })
})
