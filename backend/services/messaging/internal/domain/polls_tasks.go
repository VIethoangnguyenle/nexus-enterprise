// Package domain — polls and tasks business logic.
package domain

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "ngac-platform/proto/messaging"
	"ngac-platform/services/messaging/internal/store"
)

// --- Polls ---

// CreatePollInput holds validated parameters for poll creation.
type CreatePollInput struct {
	ChannelID   string
	UserID      string
	UserNodeID  string
	Question    string
	Options     []string
	IsMulti     bool
	IsAnonymous bool
	EndsAt      *time.Time
}

// CreatePoll creates a poll with a linked system message.
func (s *Service) CreatePoll(ctx context.Context, in CreatePollInput) (*pb.Poll, error) {
	if in.Question == "" || len(in.Options) < 2 {
		return nil, ErrInvalidInput
	}

	pollID := uuid.New().String()
	msgID := uuid.New().String()

	// Create a system message announcing the poll
	msg := &store.Message{
		ID:               msgID,
		ChannelID:        in.ChannelID,
		SenderID:         in.UserID,
		Content:          fmt.Sprintf("📊 Poll: %s", in.Question),
		ContentFormat:    "plain",
		MessageType:      "system",
		LinkedEntityType: "poll",
		LinkedEntityID:   pollID,
		CreatedAt:        time.Now(),
	}
	if err := s.store.InsertMessage(ctx, msg); err != nil {
		return nil, fmt.Errorf("create poll message: %w", err)
	}

	poll := &store.Poll{
		ID:          pollID,
		MessageID:   msgID,
		ChannelID:   in.ChannelID,
		Question:    in.Question,
		IsMulti:     in.IsMulti,
		IsAnonymous: in.IsAnonymous,
		CreatedBy:   in.UserID,
		EndsAt:      in.EndsAt,
		CreatedAt:   time.Now(),
	}
	if err := s.store.InsertPoll(ctx, poll); err != nil {
		return nil, fmt.Errorf("create poll: %w", err)
	}

	for i, opt := range in.Options {
		optID := uuid.New().String()
		if err := s.store.InsertPollOption(ctx, optID, pollID, opt, i); err != nil {
			return nil, fmt.Errorf("create poll option: %w", err)
		}
	}

	return s.GetPoll(ctx, pollID)
}

// VotePoll records a user's vote on a poll option.
func (s *Service) VotePoll(ctx context.Context, pollID, optionID, userID string) error {
	if pollID == "" || optionID == "" {
		return ErrInvalidInput
	}
	return s.store.InsertVote(ctx, pollID, optionID, userID)
}

// RemoveVote removes a user's vote from a poll option.
func (s *Service) RemoveVote(ctx context.Context, pollID, optionID, userID string) error {
	if pollID == "" || optionID == "" {
		return ErrInvalidInput
	}
	return s.store.DeleteVote(ctx, pollID, optionID, userID)
}

// GetPoll retrieves a poll with its options and vote counts.
func (s *Service) GetPoll(ctx context.Context, pollID string) (*pb.Poll, error) {
	poll, options, err := s.store.GetPoll(ctx, pollID)
	if err != nil {
		return nil, fmt.Errorf("get poll: %w", err)
	}
	if poll == nil {
		return nil, ErrNotFound
	}
	return pollToProto(poll, options), nil
}

// --- Tasks ---

// CreateTaskInput holds validated parameters for task creation.
type CreateTaskInput struct {
	ChannelID  string
	UserID     string
	UserNodeID string
	Title      string
	AssigneeID string
	DueDate    string // "YYYY-MM-DD"
}

// CreateTask creates a task with a linked system message.
func (s *Service) CreateTask(ctx context.Context, in CreateTaskInput) (*pb.ChatTask, error) {
	if in.Title == "" {
		return nil, ErrInvalidInput
	}

	taskID := uuid.New().String()
	msgID := uuid.New().String()

	msg := &store.Message{
		ID:               msgID,
		ChannelID:        in.ChannelID,
		SenderID:         in.UserID,
		Content:          fmt.Sprintf("📋 Task: %s", in.Title),
		ContentFormat:    "plain",
		MessageType:      "system",
		LinkedEntityType: "task",
		LinkedEntityID:   taskID,
		CreatedAt:        time.Now(),
	}
	if err := s.store.InsertMessage(ctx, msg); err != nil {
		return nil, fmt.Errorf("create task message: %w", err)
	}

	var dueDate *time.Time
	if in.DueDate != "" {
		t, err := time.Parse("2006-01-02", in.DueDate)
		if err == nil {
			dueDate = &t
		}
	}

	task := &store.ChatTask{
		ID:         taskID,
		MessageID:  msgID,
		ChannelID:  in.ChannelID,
		Title:      in.Title,
		AssigneeID: in.AssigneeID,
		Status:     "todo",
		DueDate:    dueDate,
		CreatedBy:  in.UserID,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	if err := s.store.InsertTask(ctx, task); err != nil {
		return nil, fmt.Errorf("create task: %w", err)
	}

	return s.getTaskProto(ctx, taskID)
}

// UpdateTaskInput holds parameters for updating a task.
type UpdateTaskInput struct {
	TaskID     string
	UserID     string
	Status     string
	AssigneeID string
	Title      string
	DueDate    string
}

// UpdateTask updates a task's mutable fields.
func (s *Service) UpdateTask(ctx context.Context, in UpdateTaskInput) (*pb.ChatTask, error) {
	var dueDate *time.Time
	if in.DueDate != "" {
		t, err := time.Parse("2006-01-02", in.DueDate)
		if err == nil {
			dueDate = &t
		}
	}

	if err := s.store.UpdateTask(ctx, in.TaskID, in.Status, in.AssigneeID, in.Title, dueDate); err != nil {
		return nil, fmt.Errorf("update task: %w", err)
	}

	return s.getTaskProto(ctx, in.TaskID)
}

// ListTasks returns tasks for a channel, optionally filtered by status.
func (s *Service) ListTasks(ctx context.Context, channelID, status string) ([]*pb.ChatTask, error) {
	tasks, err := s.store.ListTasksByChannel(ctx, channelID, status)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	var result []*pb.ChatTask
	for i := range tasks {
		result = append(result, taskToProto(&tasks[i]))
	}
	return result, nil
}

// getTaskProto retrieves and converts a single task to proto.
func (s *Service) getTaskProto(ctx context.Context, taskID string) (*pb.ChatTask, error) {
	t, err := s.store.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, ErrNotFound
	}
	return taskToProto(t), nil
}

// --- Proto conversions ---

func pollToProto(p *store.Poll, options []store.PollOption) *pb.Poll {
	var pbOpts []*pb.PollOption
	var totalVotes int32
	for _, o := range options {
		pbOpt := &pb.PollOption{
			Id:        o.ID,
			Text:      o.Text,
			Position:  int32(o.Position),
			VoteCount: o.VoteCount,
		}
		if !p.IsAnonymous {
			pbOpt.VoterIds = o.VoterIDs
		}
		pbOpts = append(pbOpts, pbOpt)
		totalVotes += o.VoteCount
	}

	result := &pb.Poll{
		Id:          p.ID,
		MessageId:   p.MessageID,
		ChannelId:   p.ChannelID,
		Question:    p.Question,
		Options:     pbOpts,
		IsMulti:     p.IsMulti,
		IsAnonymous: p.IsAnonymous,
		CreatedBy:   p.CreatedBy,
		CreatedAt:   timestamppb.New(p.CreatedAt),
		TotalVotes:  totalVotes,
	}
	if p.EndsAt != nil {
		result.EndsAt = timestamppb.New(*p.EndsAt)
	}
	return result
}

func taskToProto(t *store.ChatTask) *pb.ChatTask {
	result := &pb.ChatTask{
		Id:           t.ID,
		MessageId:    t.MessageID,
		ChannelId:    t.ChannelID,
		Title:        t.Title,
		AssigneeId:   t.AssigneeID,
		AssigneeName: t.AssigneeName,
		Status:       t.Status,
		CreatedBy:    t.CreatedBy,
		CreatedAt:    timestamppb.New(t.CreatedAt),
		UpdatedAt:    timestamppb.New(t.UpdatedAt),
	}
	if t.DueDate != nil {
		result.DueDate = t.DueDate.Format("2006-01-02")
	}
	return result
}
