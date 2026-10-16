package domain

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"time"
)

// HoldAvatarGate takes the one decode slot until the returned func is called,
// and shortens how long an upload waits for it.
func (s *Service) HoldAvatarGate(wait time.Duration) (release func()) {
	s.avatarWait = wait
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
