package events

import (
	"testing"

	"ngac-platform/services/workspace/internal/domain"
)

// A service without a broker holds a nil producer; announcing through it must
// cost nothing and never panic.
func TestNilProducerDropsTheNotice(t *testing.T) {
	var p *Producer
	p.InvitationOffered(domain.InvitationNotice{InvitationID: "i1", UserID: "u1", WorkspaceName: "NovaPay"})
	p.Close()
}

func TestProducerWithoutClientDropsTheNotice(t *testing.T) {
	p := &Producer{}
	p.InvitationOffered(domain.InvitationNotice{InvitationID: "i1", UserID: "u1"})
}
