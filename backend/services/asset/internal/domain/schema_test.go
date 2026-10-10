package domain_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/asset/internal/domain"
)

const laptopSchema = `{
  "type": "object",
  "properties": {
    "cfg":   {"title": "Cấu hình",     "type": "string", "x-kind": "text"},
    "ram":   {"title": "RAM (GB)",     "type": "number", "x-kind": "number"},
    "exp":   {"title": "Hết bảo hành", "type": "string", "x-kind": "date"},
    "owner": {"title": "Người phụ trách", "type": "string", "x-kind": "person"},
    "color": {"title": "Màu",          "type": "string", "x-kind": "choice", "enum": ["Bạc", "Đen"]}
  },
  "required": ["cfg"]
}`

func TestValidateSchema_AcceptsTheFieldKindsTheScreenOffers(t *testing.T) {
	require.NoError(t, domain.ValidateSchema(json.RawMessage(laptopSchema)))
	require.NoError(t, domain.ValidateSchema(json.RawMessage(`{}`)))
	require.NoError(t, domain.ValidateSchema(nil))
	// A schema written before field kinds existed keeps working.
	require.NoError(t, domain.ValidateSchema(json.RawMessage(`{"type":"object","properties":{"serial":{"type":"string"}}}`)))
}

func TestValidateSchema_Refusals(t *testing.T) {
	cases := map[string]string{
		"not json":                 `{`,
		"not an object":            `[1]`,
		"properties not an object": `{"properties": 3}`,
		"property not an object":   `{"properties": {"a": 3}}`,
		"unknown kind":             `{"properties": {"a": {"title": "A", "x-kind": "formula"}}}`,
		"title not text":           `{"properties": {"a": {"title": 7, "x-kind": "text"}}}`,
		"title too long":           `{"properties": {"a": {"title": "` + strings.Repeat("x", 81) + `", "x-kind": "text"}}}`,
		"choice with no options":   `{"properties": {"a": {"title": "A", "x-kind": "choice"}}}`,
		"choice with empty option": `{"properties": {"a": {"title": "A", "x-kind": "choice", "enum": ["x", ""]}}}`,
		"choice with repeats":      `{"properties": {"a": {"title": "A", "x-kind": "choice", "enum": ["x", "x"]}}}`,
		"required names no field":  `{"properties": {"a": {"title": "A", "x-kind": "text"}}, "required": ["b"]}`,
		"required not a list":      `{"properties": {"a": {"title": "A", "x-kind": "text"}}, "required": "a"}`,
		"too many fields":          manyFields(21),
	}
	for name, schema := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Error(t, domain.ValidateSchema(json.RawMessage(schema)))
		})
	}
}

func manyFields(n int) string {
	var b strings.Builder
	b.WriteString(`{"properties": {`)
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(`"f` + string(rune('a'+i)) + `": {"title": "F", "x-kind": "text"}`)
	}
	b.WriteString(`}}`)
	return b.String()
}

func TestValidateCustomFields_ChecksEachValueAgainstItsKind(t *testing.T) {
	schema := json.RawMessage(laptopSchema)
	ok := func(v string) { assert.NoError(t, domain.ValidateCustomFields(schema, json.RawMessage(v)), v) }
	bad := func(v string) { assert.Error(t, domain.ValidateCustomFields(schema, json.RawMessage(v)), v) }

	ok(`{"cfg": "M3 Pro, 18 GB"}`)
	ok(`{"cfg": "x", "ram": 18, "exp": "2027-03-14", "owner": "someone", "color": "Bạc"}`)

	bad(`{}`)                                // required
	bad(`{"cfg": "x", "ram": "18"}`)         // number as text
	bad(`{"cfg": 5}`)                        // text as number
	bad(`{"cfg": "x", "exp": "14/03/2027"}`) // date in another shape
	bad(`{"cfg": "x", "exp": "2027-13-40"}`) // not a day
	bad(`{"cfg": "x", "color": "Đỏ"}`)       // not an option
	bad(`{"cfg": "x", "extra": "?"}`)        // not a field of the type
	bad(`{"cfg": "` + strings.Repeat("x", 501) + `"}`)
	bad(`not json`)
}

func TestValidateCustomFields_RequiredTextMayNotBeBlank(t *testing.T) {
	schema := json.RawMessage(laptopSchema)
	assert.Error(t, domain.ValidateCustomFields(schema, json.RawMessage(`{"cfg": ""}`)))
	assert.Error(t, domain.ValidateCustomFields(schema, json.RawMessage(`{"cfg": "   "}`)))
	assert.NoError(t, domain.ValidateCustomFields(schema, json.RawMessage(`{"cfg": "M3"}`)))
}

func TestValidateCustomFields_ATypeWithNoFieldsHoldsNoValues(t *testing.T) {
	for _, schema := range []json.RawMessage{nil, json.RawMessage(`{}`), json.RawMessage(`{"type":"object"}`)} {
		assert.Error(t, domain.ValidateCustomFields(schema, json.RawMessage(`{"anything": "x"}`)), string(schema))
		assert.NoError(t, domain.ValidateCustomFields(schema, json.RawMessage(`{}`)))
		assert.NoError(t, domain.ValidateCustomFields(schema, nil))
	}
}

func TestPersonValues_CollectsOnlyPersonKindFields(t *testing.T) {
	got := domain.PersonValues(json.RawMessage(laptopSchema), json.RawMessage(`{"cfg":"x","owner":"u-1","color":"Bạc"}`))
	assert.Equal(t, []string{"u-1"}, got)
	assert.Empty(t, domain.PersonValues(json.RawMessage(laptopSchema), json.RawMessage(`{"cfg":"x"}`)))
	assert.Empty(t, domain.PersonValues(nil, json.RawMessage(`{"owner":"u"}`)))
}
