package domain

import (
	"unicode/utf16"

	"ngac-platform/services/calendar/internal/store"
)

// PersonRef is a person as the calendar shows them. NodeID is the key a picker
// sends back and no screen displays; Hue is the person's colour index 0..7.
type PersonRef struct {
	NodeID      string `json:"nodeId"`
	DisplayName string `json:"displayName"`
	AvatarURL   string `json:"avatarUrl,omitempty"`
	Hue         int    `json:"hue"`
}

// personHue is the person hue index 0..7 of a stable key. It is the algorithm of
// personHue() in frontend/src/lib/person-hue.ts (FNV-1a over UTF-16 code units,
// a final avalanche, modulo 8) less one, so a person has the same colour here as
// everywhere else in the app.
func personHue(key string) int {
	h := uint32(0x811c9dc5)
	for _, u := range utf16.Encode([]rune(key)) {
		h ^= uint32(u)
		h *= 0x01000193
	}
	h ^= h >> 16
	h *= 0x45d9f3b
	h ^= h >> 16
	return int(h % 8)
}

// hueKey is what a person's hue is drawn from: their account, as in the rest of
// the app, else their node.
func hueKey(p *store.Person) string {
	if p.UserID != "" {
		return p.UserID
	}
	return p.NodeID
}

// refOf shapes a person for a screen.
func refOf(p *store.Person) *PersonRef {
	if p == nil {
		return nil
	}
	return &PersonRef{NodeID: p.NodeID, DisplayName: p.DisplayName, AvatarURL: p.AvatarURL, Hue: personHue(hueKey(p))}
}

// formerMemberRef stands for a person who has left the workspace.
func formerMemberRef(nodeID string) *PersonRef {
	return &PersonRef{NodeID: nodeID, DisplayName: formerMember, Hue: personHue(nodeID)}
}
