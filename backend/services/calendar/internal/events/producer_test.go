package events

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublishMarksAndKeysTheNotice(t *testing.T) {
	var key string
	var data []byte
	p := &Producer{send: func(k string, d []byte) { key, data = k, d }}
	mins := 10
	p.Publish(Notice{Type: TypeReminder, WorkspaceID: "w1", EventID: "e1", MinutesBefore: &mins,
		Recipients: []Recipient{{NodeID: "n1", UserID: "u1"}}})

	assert.Equal(t, "w1", key, "keyed by workspace")
	var got map[string]any
	require.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, RecordNotice, got["record"], "a notice is told from a realtime frame by its record")
	assert.Equal(t, "event.reminder", got["type"])
	assert.NotZero(t, got["at"])
	assert.NotContains(t, got, "domain", "a notice must never parse as a realtime frame")
}

func TestNilProducerDiscards(t *testing.T) {
	var p *Producer
	p.Publish(Notice{Type: TypeInvited})
}
