// Package domain — files attached to messages.
package domain

import (
	"context"
	"fmt"

	pb "ngac-platform/proto/messaging"
	"ngac-platform/services/messaging/internal/store"
)

// maxAttachments is how many files one message may carry.
const maxAttachments = 20

// AttachmentInput names a file of the channel's drive to send with a message.
// Width and height are the pixel size the client measured, for images.
type AttachmentInput struct {
	DriveFileID string
	Width       int32
	Height      int32
}

// resolveAttachments turns the requested files into what the message stores.
// Every file must be a confirmed file of this channel's own drive: a file of
// another channel or of the workspace drive would put its contents in front of
// this channel's readers. Any file that fails refuses the whole message.
func (s *Service) resolveAttachments(ctx context.Context, channelID string, in []AttachmentInput) ([]store.Attachment, error) {
	if len(in) == 0 {
		return nil, nil
	}
	if len(in) > maxAttachments {
		return nil, fmt.Errorf("%w: at most %d files per message", ErrInvalidInput, maxAttachments)
	}
	ids := make([]string, 0, len(in))
	seen := make(map[string]bool, len(in))
	for _, a := range in {
		if a.DriveFileID == "" || seen[a.DriveFileID] {
			return nil, fmt.Errorf("%w: each attachment names a different file", ErrInvalidInput)
		}
		seen[a.DriveFileID] = true
		ids = append(ids, a.DriveFileID)
	}
	files, err := s.store.ChannelDriveFiles(ctx, channelID, ids)
	if err != nil {
		return nil, fmt.Errorf("load attachments: %w", err)
	}
	out := make([]store.Attachment, 0, len(in))
	for i, a := range in {
		f, ok := files[a.DriveFileID]
		if !ok {
			return nil, fmt.Errorf("%w: attachment %d is not an uploaded file of this channel", ErrInvalidInput, i+1)
		}
		out = append(out, store.Attachment{
			DriveFileID: f.ID, FileName: f.Name, MimeType: f.MimeType, SizeBytes: f.SizeBytes,
			Width: max(a.Width, 0), Height: max(a.Height, 0),
		})
	}
	return out, nil
}

func attachmentsToProto(in []store.Attachment) []*pb.MessageAttachment {
	if len(in) == 0 {
		return nil
	}
	out := make([]*pb.MessageAttachment, 0, len(in))
	for _, a := range in {
		out = append(out, &pb.MessageAttachment{
			DriveFileId: a.DriveFileID, FileName: a.FileName, MimeType: a.MimeType,
			SizeBytes: a.SizeBytes, Width: a.Width, Height: a.Height,
		})
	}
	return out
}

// chatAttachments is the live-stream form of a message's attachments.
func chatAttachments(in []*pb.MessageAttachment) []*pb.ChatAttachment {
	if len(in) == 0 {
		return nil
	}
	out := make([]*pb.ChatAttachment, 0, len(in))
	for _, a := range in {
		out = append(out, &pb.ChatAttachment{
			DriveFileId: a.DriveFileId, FileName: a.FileName, MimeType: a.MimeType,
			SizeBytes: a.SizeBytes, Width: a.Width, Height: a.Height,
		})
	}
	return out
}
