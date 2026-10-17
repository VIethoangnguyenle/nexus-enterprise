// Package domain — polls and tasks business logic.
package domain

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	"ngac-platform/ngac"
	pb "ngac-platform/proto/messaging"
	"ngac-platform/services/messaging/internal/store"
)

// announcementError turns the store's refusal of a reply whose parent was
// deleted between the check and the write into the same answer a reply gets.
func announcementError(err error) error {
	if errors.Is(err, store.ErrParentGone) {
		return fmt.Errorf("%w: the message replied to was deleted", ErrInvalidInput)
	}
	return err
}

// announcementProto is the stored announcement as the live stream and the
// caller see it.
func (s *Service) announcementProto(ctx context.Context, msg *store.Message) *pb.Message {
	msg.SenderName = s.lookupUsername(ctx, msg.SenderID)
	return messageToProto(msg)
}

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
	// ParentMessageID, when set, makes the announcement a reply in that
	// message's thread instead of a message of the main stream.
	ParentMessageID string
}

// authorizePoll authorizes an operation on the channel that owns a poll.
// Polls, like messages, are not graph nodes — the check lands on the channel.
func (s *Service) authorizePoll(ctx context.Context, pollID, userNodeID, operation string) (*store.Poll, []store.PollOption, error) {
	poll, options, _, err := s.authorizePollIn(ctx, pollID, userNodeID, operation)
	return poll, options, err
}

// authorizePollIn is authorizePoll that also returns the poll's channel.
func (s *Service) authorizePollIn(ctx context.Context, pollID, userNodeID, operation string) (*store.Poll, []store.PollOption, *store.Channel, error) {
	if pollID == "" {
		return nil, nil, nil, ErrInvalidInput
	}
	poll, options, err := s.store.GetPoll(ctx, pollID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("load poll: %w", err)
	}
	if poll == nil {
		return nil, nil, nil, ErrNotFound
	}
	ch, err := s.authorizeChannel(ctx, poll.ChannelID, userNodeID, operation)
	if err != nil {
		return nil, nil, nil, err
	}
	return poll, options, ch, nil
}

// maxPollOptions is how many choices a poll offers at most.
const maxPollOptions = 10

// pollClosed reports whether a poll takes no more votes: closed early, or past its end.
func pollClosed(p *store.Poll) bool {
	return p.ClosedAt != nil || (p.EndsAt != nil && time.Now().After(*p.EndsAt))
}

// authorizeTask authorizes an operation on the channel that owns a task.
func (s *Service) authorizeTask(ctx context.Context, taskID, userNodeID, operation string) error {
	if taskID == "" {
		return ErrInvalidInput
	}
	task, err := s.store.GetTask(ctx, taskID)
	if err != nil {
		return fmt.Errorf("load task: %w", err)
	}
	if task == nil {
		return ErrNotFound
	}
	_, err = s.authorizeChannel(ctx, task.ChannelID, userNodeID, operation)
	return err
}

// CreatePoll creates a poll with a linked system message.
func (s *Service) CreatePoll(ctx context.Context, in CreatePollInput) (*pb.Poll, error) {
	poll, _, err := s.CreatePollMessage(ctx, in)
	return poll, err
}

// CreatePollMessage creates a poll and also returns the message that announces
// it, so a caller can tell live sessions about it (a reply in a thread needs
// the thread's summary pushed like any reply).
func (s *Service) CreatePollMessage(ctx context.Context, in CreatePollInput) (*pb.Poll, *pb.Message, error) {
	if _, err := s.authorizeChannel(ctx, in.ChannelID, in.UserNodeID, ngac.OpWrite); err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(in.Question) == "" || len(in.Options) < 2 || len(in.Options) > maxPollOptions {
		return nil, nil, fmt.Errorf("%w: a poll asks a question with 2 to %d choices", ErrInvalidInput, maxPollOptions)
	}
	for i, o := range in.Options {
		if in.Options[i] = strings.TrimSpace(o); in.Options[i] == "" {
			return nil, nil, fmt.Errorf("%w: a choice is empty", ErrInvalidInput)
		}
	}
	if in.EndsAt != nil && !in.EndsAt.After(time.Now()) {
		return nil, nil, fmt.Errorf("%w: a poll ends in the future", ErrInvalidInput)
	}

	if in.ParentMessageID != "" {
		if err := s.checkReplyParent(ctx, in.ParentMessageID, in.ChannelID); err != nil {
			return nil, nil, err
		}
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
		ParentMessageID:  in.ParentMessageID,
		LinkedEntityType: "poll",
		LinkedEntityID:   pollID,
		CreatedAt:        time.Now(),
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
	options := make([]store.PollOptionInput, 0, len(in.Options))
	for _, opt := range in.Options {
		options = append(options, store.PollOptionInput{ID: uuid.New().String(), Text: opt})
	}
	// The announcement, the poll and its options stand or fall together.
	if err := s.store.InsertPollWithMessage(ctx, msg, poll, options); err != nil {
		return nil, nil, announcementError(err)
	}

	out, err := s.GetPoll(ctx, pollID, in.UserID, in.UserNodeID)
	if err != nil {
		return nil, nil, err
	}
	return out, s.announcementProto(ctx, msg), nil
}

// VotePoll records a user's vote on a poll option.
func (s *Service) VotePoll(ctx context.Context, pollID, optionID, userNodeID, userID string) error {
	if pollID == "" || optionID == "" {
		return ErrInvalidInput
	}
	// Voting is answering a question put to everyone who reads the channel.
	poll, _, ch, err := s.authorizePollIn(ctx, pollID, userNodeID, ngac.OpRead)
	if err != nil {
		return err
	}
	// Answering changes what the other people of the conversation see, though it
	// takes only read: in a DM it still needs the relationship.
	if ch.ChannelType == ChannelTypeDM {
		if err := s.requireDMRelationship(ctx, ch, userNodeID); err != nil {
			return err
		}
	}
	if pollClosed(poll) {
		return fmt.Errorf("%w: the poll is closed", ErrInvalidInput)
	}
	// The tally of an option is counted by option, so the option must be this
	// poll's: authorising on one poll must not let a vote land in another.
	ok, err := s.store.PollHasOption(ctx, pollID, optionID)
	if err != nil {
		return fmt.Errorf("check poll option: %w", err)
	}
	if !ok {
		return fmt.Errorf("%w: option %s is not part of this poll", ErrInvalidInput, optionID)
	}
	return s.store.InsertVote(ctx, pollID, optionID, userID, !poll.IsMulti)
}

// RemoveVote removes a user's vote from a poll option.
func (s *Service) RemoveVote(ctx context.Context, pollID, optionID, userNodeID, userID string) error {
	if pollID == "" || optionID == "" {
		return ErrInvalidInput
	}
	// Taking a vote back is backing out: it needs only read, with no relationship.
	poll, _, err := s.authorizePoll(ctx, pollID, userNodeID, ngac.OpRead)
	if err != nil {
		return err
	}
	if pollClosed(poll) {
		return fmt.Errorf("%w: the poll is closed", ErrInvalidInput)
	}
	return s.store.DeleteVote(ctx, pollID, optionID, userID)
}

// ClosePoll closes a poll before its end: its creator may (with write on the
// channel), and so may anyone who manages the channel.
func (s *Service) ClosePoll(ctx context.Context, pollID, userID, userNodeID string) (*pb.Poll, error) {
	poll, _, err := s.authorizePoll(ctx, pollID, userNodeID, ngac.OpRead)
	if err != nil {
		return nil, err
	}
	op := ngac.OpManage
	if poll.CreatedBy == userID {
		op = ngac.OpWrite
	}
	if _, err := s.authorizeChannel(ctx, poll.ChannelID, userNodeID, op); err != nil {
		return nil, err
	}
	if err := s.store.ClosePoll(ctx, pollID, userID); err != nil {
		return nil, err
	}
	return s.GetPoll(ctx, pollID, userID, userNodeID)
}

// GetPoll retrieves a poll with its options and vote counts.
func (s *Service) GetPoll(ctx context.Context, pollID, userID, userNodeID string) (*pb.Poll, error) {
	poll, options, err := s.authorizePoll(ctx, pollID, userNodeID, ngac.OpRead)
	if err != nil {
		return nil, err
	}
	return pollToProto(poll, options, userID), nil
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
	// ParentMessageID, when set, makes the announcement a reply in that
	// message's thread instead of a message of the main stream.
	ParentMessageID string
}

// CreateTask creates a task with a linked system message.
func (s *Service) CreateTask(ctx context.Context, in CreateTaskInput) (*pb.ChatTask, error) {
	task, _, err := s.CreateTaskMessage(ctx, in)
	return task, err
}

// CreateTaskMessage creates a task and also returns the message that announces
// it (see CreatePollMessage).
func (s *Service) CreateTaskMessage(ctx context.Context, in CreateTaskInput) (*pb.ChatTask, *pb.Message, error) {
	if in.Title == "" {
		return nil, nil, ErrInvalidInput
	}
	ch, err := s.authorizeChannel(ctx, in.ChannelID, in.UserNodeID, ngac.OpWrite)
	if err != nil {
		return nil, nil, err
	}
	if in.ParentMessageID != "" {
		if err := s.checkReplyParent(ctx, in.ParentMessageID, in.ChannelID); err != nil {
			return nil, nil, err
		}
	}
	// The assignee must read the channel: work handed to someone who cannot
	// see the conversation is work nobody can do.
	if in.AssigneeID != "" {
		node, err := s.store.UserNode(ctx, in.AssigneeID)
		if err != nil {
			return nil, nil, err
		}
		if node == "" || s.checkAccess(ctx, node, ch.NGACOaID, ngac.OpRead) != nil {
			return nil, nil, fmt.Errorf("%w: the assignee is not a member of this conversation", ErrInvalidInput)
		}
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
		ParentMessageID:  in.ParentMessageID,
		LinkedEntityType: "task",
		LinkedEntityID:   taskID,
		CreatedAt:        time.Now(),
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
	// The announcement and the task stand or fall together.
	if err := s.store.InsertTaskWithMessage(ctx, msg, task); err != nil {
		return nil, nil, announcementError(err)
	}

	out, err := s.getTaskProto(ctx, taskID)
	if err != nil {
		return nil, nil, err
	}
	return out, s.announcementProto(ctx, msg), nil
}

// UpdateTaskInput holds parameters for updating a task.
type UpdateTaskInput struct {
	TaskID     string
	UserID     string
	UserNodeID string
	Status     string
	AssigneeID string
	Title      string
	DueDate    string
}

// UpdateTask updates a task's mutable fields.
//
// Who may change what:
//   - the status (done, reopened): the assignee or the creator, who need only
//     read the conversation; anyone else needs manage;
//   - the assignee: the creator or someone with manage, and only to a person
//     who reads the conversation — otherwise a writer could take a task over
//     and then mark it done;
//   - the title and due date: write.
func (s *Service) UpdateTask(ctx context.Context, in UpdateTaskInput) (*pb.ChatTask, error) {
	if in.TaskID == "" {
		return nil, ErrInvalidInput
	}
	t, err := s.store.GetTask(ctx, in.TaskID)
	if err != nil {
		return nil, fmt.Errorf("load task: %w", err)
	}
	if t == nil {
		return nil, ErrNotFound
	}
	ch, err := s.authorizeChannel(ctx, t.ChannelID, in.UserNodeID, ngac.OpRead)
	if err != nil {
		return nil, err
	}
	// Setting the status or handing the task on is a change to what both people see,
	// though its owner needs only read: in a DM it needs the relationship.
	if ch.ChannelType == ChannelTypeDM && (in.Status != "" || (in.AssigneeID != "" && in.AssigneeID != t.AssigneeID)) {
		if err := s.requireDMRelationship(ctx, ch, in.UserNodeID); err != nil {
			return nil, err
		}
	}
	isOwner := in.UserID != "" && (in.UserID == t.AssigneeID || in.UserID == t.CreatedBy)
	manages := func() error { return s.authorizeChannelOp(ctx, ch, in.UserNodeID, ngac.OpManage) }

	if in.Status != "" && !isOwner {
		if err := manages(); err != nil {
			return nil, err
		}
	}
	if in.AssigneeID != "" && in.AssigneeID != t.AssigneeID {
		if in.UserID == "" || in.UserID != t.CreatedBy {
			if err := manages(); err != nil {
				return nil, err
			}
		}
		node, err := s.store.UserNode(ctx, in.AssigneeID)
		if err != nil {
			return nil, err
		}
		if node == "" || s.checkAccess(ctx, node, ch.NGACOaID, ngac.OpRead) != nil {
			return nil, fmt.Errorf("%w: the assignee is not a member of this conversation", ErrInvalidInput)
		}
	}
	if in.Title != "" || in.DueDate != "" {
		if err := s.authorizeChannelOp(ctx, ch, in.UserNodeID, ngac.OpWrite); err != nil {
			return nil, err
		}
	}

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
func (s *Service) ListTasks(ctx context.Context, channelID, userNodeID, status string) ([]*pb.ChatTask, error) {
	if _, err := s.authorizeChannel(ctx, channelID, userNodeID, ngac.OpRead); err != nil {
		return nil, err
	}
	tasks, err := s.store.ListTasksByChannel(ctx, channelID, status)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	// Never nil: an empty list is `[]` on the wire, not `null`.
	result := make([]*pb.ChatTask, 0, len(tasks))
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

func pollToProto(p *store.Poll, options []store.PollOption, viewerID string) *pb.Poll {
	// What the viewer chose, and whether they may see the results: once they
	// voted, or once the poll is closed.
	var mine []string
	for _, o := range options {
		for _, v := range o.VoterIDs {
			if v == viewerID {
				mine = append(mine, o.ID)
			}
		}
	}
	closed := pollClosed(p)
	visible := closed || len(mine) > 0

	var pbOpts []*pb.PollOption
	var totalVotes int32
	for _, o := range options {
		pbOpt := &pb.PollOption{Id: o.ID, Text: o.Text, Position: int32(o.Position)}
		if visible {
			pbOpt.VoteCount = o.VoteCount
			totalVotes += o.VoteCount
			if !p.IsAnonymous {
				pbOpt.VoterIds = o.VoterIDs
			}
		}
		pbOpts = append(pbOpts, pbOpt)
	}

	result := &pb.Poll{
		Id:             p.ID,
		MessageId:      p.MessageID,
		ChannelId:      p.ChannelID,
		Question:       p.Question,
		Options:        pbOpts,
		IsMulti:        p.IsMulti,
		IsAnonymous:    p.IsAnonymous,
		CreatedBy:      p.CreatedBy,
		CreatedAt:      timestamppb.New(p.CreatedAt),
		TotalVotes:     totalVotes,
		ResultsVisible: visible,
		MyOptionIds:    mine,
		Closed:         closed,
	}
	if p.EndsAt != nil {
		result.EndsAt = timestamppb.New(*p.EndsAt)
	}
	if p.ClosedAt != nil {
		result.ClosedAt = timestamppb.New(*p.ClosedAt)
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
