// Package livekit talks to the LiveKit server that carries call media: it
// creates and closes rooms, removes a participant, signs the ticket a browser
// joins a room with, and verifies the webhooks LiveKit sends back. Nothing here
// decides who may join; the domain does that before asking for a ticket.
package livekit

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/livekit/protocol/auth"
	lkproto "github.com/livekit/protocol/livekit"
	"github.com/livekit/protocol/webhook"
	lksdk "github.com/livekit/server-sdk-go/v2"
	"github.com/twitchtv/twirp"
)

// Config is where the LiveKit server is and the key pair it trusts.
type Config struct {
	// URL is how this service reaches LiveKit's API (inside the network).
	URL string
	// PublicURL is the address a browser opens for signalling.
	PublicURL string
	APIKey    string
	APISecret string
}

// Enabled reports whether calls are configured at all.
func (c Config) Enabled() bool {
	return c.URL != "" && c.PublicURL != "" && c.APIKey != "" && c.APISecret != ""
}

// Client is a LiveKit server seen from the messaging service.
type Client struct {
	cfg   Config
	rooms *lksdk.RoomServiceClient
	keys  auth.KeyProvider
}

// New returns a client for cfg.
func New(cfg Config) *Client {
	return &Client{
		cfg:   cfg,
		rooms: lksdk.NewRoomServiceClient(cfg.URL, cfg.APIKey, cfg.APISecret),
		keys:  auth.NewSimpleKeyProvider(cfg.APIKey, cfg.APISecret),
	}
}

// PublicURL is the signalling address handed to browsers with a ticket.
func (c *Client) PublicURL() string { return c.cfg.PublicURL }

// CreateRoom opens a room ahead of the first participant. Creating a room that
// exists is not an error.
func (c *Client) CreateRoom(ctx context.Context, name string, maxParticipants uint32, emptyTimeout time.Duration) error {
	_, err := c.rooms.CreateRoom(ctx, &lkproto.CreateRoomRequest{
		Name:            name,
		MaxParticipants: maxParticipants,
		EmptyTimeout:    uint32(emptyTimeout / time.Second),
	})
	if err != nil {
		return fmt.Errorf("livekit create room: %w", err)
	}
	return nil
}

// DeleteRoom closes a room and disconnects everyone in it.
func (c *Client) DeleteRoom(ctx context.Context, name string) error {
	if _, err := c.rooms.DeleteRoom(ctx, &lkproto.DeleteRoomRequest{Room: name}); err != nil {
		return fmt.Errorf("livekit delete room: %w", err)
	}
	return nil
}

// RemoveParticipant disconnects one person from a room.
func (c *Client) RemoveParticipant(ctx context.Context, room, identity string) error {
	_, err := c.rooms.RemoveParticipant(ctx, &lkproto.RoomParticipantIdentity{Room: room, Identity: identity})
	var terr twirp.Error
	if errors.As(err, &terr) && terr.Code() == twirp.NotFound {
		return nil // not in the room (or no room): already out
	}
	if err != nil {
		return fmt.Errorf("livekit remove participant: %w", err)
	}
	return nil
}

// Participants returns the identities connected to a room now.
func (c *Client) Participants(ctx context.Context, room string) ([]string, error) {
	res, err := c.rooms.ListParticipants(ctx, &lkproto.ListParticipantsRequest{Room: room})
	var terr twirp.Error
	if errors.As(err, &terr) && terr.Code() == twirp.NotFound {
		return nil, nil // a room that does not exist has nobody in it
	}
	if err != nil {
		return nil, fmt.Errorf("livekit list participants: %w", err)
	}
	out := make([]string, 0, len(res.GetParticipants()))
	for _, p := range res.GetParticipants() {
		out = append(out, p.GetIdentity())
	}
	return out, nil
}

// JoinToken signs a ticket into one room for one identity, shown under name. The
// holder may publish camera, microphone and screen, send data (chat reactions,
// raised hands) and subscribe; it cannot administer the room.
func (c *Client) JoinToken(room, identity, name string, ttl time.Duration) (string, error) {
	grant := &auth.VideoGrant{RoomJoin: true, Room: room}
	grant.SetCanPublish(true)
	grant.SetCanPublishData(true)
	grant.SetCanSubscribe(true)
	token, err := auth.NewAccessToken(c.cfg.APIKey, c.cfg.APISecret).
		SetVideoGrant(grant).
		SetIdentity(identity).
		SetName(name).
		SetValidFor(ttl).
		ToJWT()
	if err != nil {
		return "", fmt.Errorf("livekit join token: %w", err)
	}
	return token, nil
}

// Event is a webhook from LiveKit, reduced to what the call lifecycle reads.
type Event struct {
	Kind     string // participant_joined | participant_left | room_finished | …
	Room     string
	Identity string // set for participant events
	// Participant is the connection's sid, for participant events.
	Participant string
}

// ReceiveWebhook verifies a webhook's signature (a token signed with the API
// secret, carrying the body's hash) and decodes it. A request that is not
// signed by this server's key is an error.
func (c *Client) ReceiveWebhook(r *http.Request) (Event, error) {
	ev, err := webhook.ReceiveWebhookEvent(r, c.keys)
	if err != nil {
		return Event{}, fmt.Errorf("livekit webhook: %w", err)
	}
	return Event{
		Kind:     ev.GetEvent(),
		Room:     ev.GetRoom().GetName(),
		Identity:    ev.GetParticipant().GetIdentity(),
		Participant: ev.GetParticipant().GetSid(),
	}, nil
}
