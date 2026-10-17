package domain

import (
	"bytes"
	"context"
	"encoding/binary"
	"hash/crc32"
	"time"

	"ngac-platform/services/auth/internal/store"
)

// InitTenantNGAC exposes the tenant graph initialisation to the black-box tests.
func (s *Service) InitTenantNGAC(ctx context.Context, tenantID, pcNodeID, ownersUAID, membersUAID string) error {
	return s.initTenantNGAC(ctx, tenantID, pcNodeID, ownersUAID, membersUAID)
}

// NormalizeAvatar exposes the picture pipeline to the black-box tests.
func NormalizeAvatar(b []byte) (store.AvatarImages, error) { return normalizeAvatar(b) }

// SetAvatarWaitForTest shortens how long an upload waits for the one before it.
func (s *Service) SetAvatarWaitForTest(d time.Duration) { s.avatarWait = d }

// HoldAvatarGateForTest takes the decode slot as if a picture were being
// processed, until the returned function is called.
func (s *Service) HoldAvatarGateForTest() func() {
	s.avatarGate <- struct{}{}
	return func() { <-s.avatarGate }
}

// PNGHeaderForTest is the signature and IHDR chunk of a PNG of the given size,
// bit depth and colour type, with no pixel data: enough for DecodeConfig.
func PNGHeaderForTest(w, h int, bitDepth, colorType byte) []byte {
	var ihdr bytes.Buffer
	ihdr.WriteString("IHDR")
	binary.Write(&ihdr, binary.BigEndian, uint32(w))
	binary.Write(&ihdr, binary.BigEndian, uint32(h))
	ihdr.Write([]byte{bitDepth, colorType, 0, 0, 0})
	var out bytes.Buffer
	out.WriteString("\x89PNG\r\n\x1a\n")
	binary.Write(&out, binary.BigEndian, uint32(ihdr.Len()-4))
	out.Write(ihdr.Bytes())
	binary.Write(&out, binary.BigEndian, crc32.ChecksumIEEE(ihdr.Bytes()))
	return out.Bytes()
}

// SetGraceForTest sets how long after a refresh token is spent presenting it again still
// returns the same successor (see RefreshStore.grace).
func (s *RefreshStore) SetGraceForTest(d time.Duration) { s.grace = d }

// RefreshStoreForTest exposes the service's refresh store to the black-box tests.
func (s *Service) RefreshStoreForTest() *RefreshStore { return s.refresh }

// JoinTenantForTest exposes the graph-first tenant join to the black-box tests.
func (s *Service) JoinTenantForTest(ctx context.Context, tenantID, userID, ngacNodeID, role string) error {
	return s.joinTenant(ctx, tenantID, userID, ngacNodeID, role)
}
