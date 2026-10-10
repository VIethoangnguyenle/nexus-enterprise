import { formatDate } from './format'
import { UNKNOWN_PERSON, type PeopleDirectory } from './people'

/**
 * Custom fields of an asset type. The server stores them as a JSON-Schema-shaped
 * object (`fields_schema`) and validates asset values against it
 * (backend/services/asset/internal/domain/schema.go):
 *
 *   { type: 'object',
 *     properties: { <key>: { title, type, 'x-kind', enum? } },
 *     required: [<key>] }
 *
 * `key` is an opaque name made here and never shown; `title` is what people read.
 */

export type FieldKind = 'text' | 'number' | 'date' | 'person' | 'choice'
export const FIELD_KINDS: FieldKind[] = ['text', 'number', 'date', 'person', 'choice']

const KIND_LABEL: Record<FieldKind, string> = {
  text: 'Văn bản',
  number: 'Số',
  date: 'Ngày',
  person: 'Người',
  choice: 'Danh sách chọn',
}
export const fieldKindLabel = (k: FieldKind) => KIND_LABEL[k]

export interface FieldDef {
  key: string
  title: string
  kind: FieldKind
  required: boolean
  /** The choices of a `choice` field. */
  options?: string[]
}

export const MAX_FIELDS = 20
export const TITLE_MAX = 80
const UNTITLED = 'Trường thông tin'

const isKind = (k: unknown): k is FieldKind => FIELD_KINDS.includes(k as FieldKind)
const isObject = (v: unknown): v is Record<string, unknown> => !!v && typeof v === 'object' && !Array.isArray(v)

/** The fields a schema string defines. No schema, an empty one or a damaged one defines none. */
export function parseFields(schema: string | undefined): FieldDef[] {
  if (!schema) return []
  let raw: unknown
  try {
    raw = JSON.parse(schema)
  } catch {
    return []
  }
  if (!isObject(raw) || !isObject(raw.properties)) return []
  const required = new Set(Array.isArray(raw.required) ? raw.required.filter((r): r is string => typeof r === 'string') : [])
  return Object.entries(raw.properties).flatMap(([key, def]) => {
    if (!isObject(def)) return []
    // A schema from before kinds existed: the JSON type says number or text.
    const kind: FieldKind = isKind(def['x-kind']) ? def['x-kind'] : def.type === 'number' ? 'number' : 'text'
    const title = typeof def.title === 'string' && def.title.trim() ? def.title : UNTITLED
    const field: FieldDef = { key, title, kind, required: required.has(key) }
    if (kind === 'choice') field.options = Array.isArray(def.enum) ? def.enum.filter((o): o is string => typeof o === 'string') : []
    return [field]
  })
}

/** The schema for a list of fields; no fields is `{}`, which the server reads as "no custom fields". */
export function fieldsToSchema(fields: FieldDef[]): object {
  if (fields.length === 0) return {}
  const properties: Record<string, object> = {}
  for (const f of fields) {
    properties[f.key] = {
      title: f.title,
      type: f.kind === 'number' ? 'number' : 'string',
      'x-kind': f.kind,
      ...(f.kind === 'choice' ? { enum: f.options ?? [] } : {}),
    }
  }
  const required = fields.filter((f) => f.required).map((f) => f.key)
  return { type: 'object', properties, ...(required.length ? { required } : {}) }
}

let counter = 0
/** A fresh, unused key for a field. Letters of the title help a human reading the database; uniqueness comes from the suffix. */
export function newFieldKey(title: string, taken: string[]): string {
  const stem = title
    .normalize('NFD').replace(/\p{Diacritic}/gu, '').replace(/đ/gi, 'd')
    .toLowerCase().replace(/[^a-z0-9]+/g, '_').replace(/^_+|_+$/g, '').slice(0, 24)
  for (;;) {
    counter += 1
    const key = `f${stem ? `_${stem}` : ''}_${counter.toString(36)}`
    if (!taken.includes(key)) return key
  }
}

const numberFormat = new Intl.NumberFormat('vi-VN', { maximumFractionDigits: 6 })

/** A stored value the way a person reads it. A person's value is a user id, shown as their name. */
export function formatFieldValue(field: FieldDef, value: unknown, people: PeopleDirectory): string {
  if (value === undefined || value === null || value === '') return ''
  switch (field.kind) {
    case 'date':
      return formatDate(value) || String(value)
    case 'number':
      return typeof value === 'number' ? numberFormat.format(value) : String(value)
    case 'person':
      return people.byUserId.get(String(value))?.name ?? UNKNOWN_PERSON
    default:
      return String(value)
  }
}

/** Parses what was typed into a number field ("18,5" or "18.5"); null when it is not one. */
export function parseNumber(text: string): number | null {
  const n = Number(text.trim().replace(',', '.'))
  return text.trim() !== '' && Number.isFinite(n) ? n : null
}

/** Field errors for the values typed, keyed by field key. Empty means they can be sent. */
export function validateFieldValues(fields: FieldDef[], values: Record<string, string>): Record<string, string> {
  const errors: Record<string, string> = {}
  for (const f of fields) {
    const v = (values[f.key] ?? '').trim()
    if (!v) {
      if (f.required) errors[f.key] = `Nhập ${f.title.toLocaleLowerCase('vi')}.`
    } else if (f.kind === 'number' && parseNumber(v) === null) {
      errors[f.key] = `${f.title} phải là một số.`
    }
  }
  return errors
}

/** The values to send: empty ones left out, numbers as numbers. */
export function valuesToPayload(fields: FieldDef[], values: Record<string, string>): Record<string, unknown> {
  const out: Record<string, unknown> = {}
  for (const f of fields) {
    const v = (values[f.key] ?? '').trim()
    if (!v) continue
    out[f.key] = f.kind === 'number' ? parseNumber(v) : v
  }
  return out
}
