package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"ngac-platform/services/calendar/internal/store"
)

// The expected values come from personHue() in frontend/src/lib/person-hue.ts
// (hue 1..8) less one; they pin the two implementations together.
func TestPersonHueMatchesTheFrontend(t *testing.T) {
	for key, want := range map[string]int{"user-1": 5, "n-alice": 6, "Nguyễn 😀": 2, "a": 2, "abc-123-def": 6} {
		assert.Equal(t, want, personHue(key), key)
	}
}

func TestPersonRefHueIsKeyedByAccountElseNode(t *testing.T) {
	assert.Equal(t, personHue("user-1"), refOf(&store.Person{NodeID: "n-alice", UserID: "user-1", DisplayName: "A"}).Hue)
	assert.Equal(t, personHue("n-alice"), refOf(&store.Person{NodeID: "n-alice", DisplayName: "A"}).Hue)
	assert.Nil(t, refOf(nil))
	for i := 0; i < 50; i++ {
		h := personHue(string(rune('a'+i)) + "x")
		assert.True(t, h >= 0 && h <= 7)
	}
}
