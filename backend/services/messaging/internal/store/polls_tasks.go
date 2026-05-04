// Package store — poll and task queries.
package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// --- Poll Models ---

// Poll represents a row in the polls table.
type Poll struct {
	ID          string
	MessageID   string
	ChannelID   string
	Question    string
	IsMulti     bool
	IsAnonymous bool
	CreatedBy   string
	EndsAt      *time.Time
	CreatedAt   time.Time
}

// PollOption represents a row in the poll_options table.
type PollOption struct {
	ID        string
	PollID    string
	Text      string
	Position  int
	VoteCount int32
	VoterIDs  []string
}

// --- Task Models ---

// ChatTask represents a row in the chat_tasks table.
type ChatTask struct {
	ID           string
	MessageID    string
	ChannelID    string
	Title        string
	AssigneeID   string
	AssigneeName string
	Status       string
	DueDate      *time.Time
	CreatedBy    string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// --- Polls Store ---

// InsertPoll creates a new poll.
func (s *Store) InsertPoll(ctx context.Context, p *Poll) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO polls (id, message_id, channel_id, question, is_multi, is_anonymous, created_by, ends_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		p.ID, p.MessageID, p.ChannelID, p.Question, p.IsMulti, p.IsAnonymous, p.CreatedBy, p.EndsAt)
	if err != nil {
		return fmt.Errorf("insert poll: %w", err)
	}
	return nil
}

// InsertPollOption adds an option to a poll.
func (s *Store) InsertPollOption(ctx context.Context, id, pollID, text string, position int) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO poll_options (id, poll_id, text, position) VALUES ($1, $2, $3, $4)`,
		id, pollID, text, position)
	if err != nil {
		return fmt.Errorf("insert poll option: %w", err)
	}
	return nil
}

// InsertVote records a user's vote. Idempotent via UNIQUE constraint.
func (s *Store) InsertVote(ctx context.Context, pollID, optionID, userID string) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO poll_votes (poll_id, option_id, user_id) VALUES ($1, $2, $3)
		 ON CONFLICT DO NOTHING`,
		pollID, optionID, userID)
	if err != nil {
		return fmt.Errorf("insert vote: %w", err)
	}
	return nil
}

// DeleteVote removes a user's vote from a poll option.
func (s *Store) DeleteVote(ctx context.Context, pollID, optionID, userID string) error {
	_, err := s.db.Exec(ctx,
		`DELETE FROM poll_votes WHERE poll_id = $1 AND option_id = $2 AND user_id = $3`,
		pollID, optionID, userID)
	if err != nil {
		return fmt.Errorf("delete vote: %w", err)
	}
	return nil
}

// GetPoll retrieves a poll with its options and vote counts.
func (s *Store) GetPoll(ctx context.Context, pollID string) (*Poll, []PollOption, error) {
	var p Poll
	var endsAt *time.Time
	err := s.db.QueryRow(ctx,
		`SELECT id, message_id, channel_id, question, is_multi, is_anonymous, created_by, ends_at, created_at
		 FROM polls WHERE id = $1`, pollID).
		Scan(&p.ID, &p.MessageID, &p.ChannelID, &p.Question,
			&p.IsMulti, &p.IsAnonymous, &p.CreatedBy, &endsAt, &p.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("get poll: %w", err)
	}
	p.EndsAt = endsAt

	rows, err := s.db.Query(ctx,
		`SELECT po.id, po.poll_id, po.text, po.position,
		        COUNT(pv.id) AS vote_count,
		        COALESCE(ARRAY_AGG(pv.user_id) FILTER (WHERE pv.user_id IS NOT NULL), '{}')
		 FROM poll_options po
		 LEFT JOIN poll_votes pv ON pv.option_id = po.id
		 WHERE po.poll_id = $1
		 GROUP BY po.id, po.poll_id, po.text, po.position
		 ORDER BY po.position`, pollID)
	if err != nil {
		return nil, nil, fmt.Errorf("get poll options: %w", err)
	}
	defer rows.Close()

	var options []PollOption
	for rows.Next() {
		var o PollOption
		if err := rows.Scan(&o.ID, &o.PollID, &o.Text, &o.Position, &o.VoteCount, &o.VoterIDs); err != nil {
			return nil, nil, err
		}
		options = append(options, o)
	}

	return &p, options, nil
}

// GetPollByMessage retrieves a poll by its linked message ID.
func (s *Store) GetPollByMessage(ctx context.Context, messageID string) (*Poll, error) {
	var p Poll
	var endsAt *time.Time
	err := s.db.QueryRow(ctx,
		`SELECT id, message_id, channel_id, question, is_multi, is_anonymous, created_by, ends_at, created_at
		 FROM polls WHERE message_id = $1`, messageID).
		Scan(&p.ID, &p.MessageID, &p.ChannelID, &p.Question,
			&p.IsMulti, &p.IsAnonymous, &p.CreatedBy, &endsAt, &p.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get poll by message: %w", err)
	}
	p.EndsAt = endsAt
	return &p, nil
}

// --- Tasks Store ---

// InsertTask creates a new chat task.
func (s *Store) InsertTask(ctx context.Context, t *ChatTask) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO chat_tasks (id, message_id, channel_id, title, assignee_id, status, due_date, created_by)
		 VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6, $7, $8)`,
		t.ID, t.MessageID, t.ChannelID, t.Title, t.AssigneeID, t.Status, t.DueDate, t.CreatedBy)
	if err != nil {
		return fmt.Errorf("insert task: %w", err)
	}
	return nil
}

// UpdateTask updates a chat task's mutable fields.
func (s *Store) UpdateTask(ctx context.Context, id, status, assigneeID, title string, dueDate *time.Time) error {
	_, err := s.db.Exec(ctx,
		`UPDATE chat_tasks SET
		   status = COALESCE(NULLIF($2, ''), status),
		   assignee_id = CASE WHEN $3 = '' THEN assignee_id ELSE $3 END,
		   title = COALESCE(NULLIF($4, ''), title),
		   due_date = COALESCE($5, due_date),
		   updated_at = NOW()
		 WHERE id = $1`,
		id, status, assigneeID, title, dueDate)
	if err != nil {
		return fmt.Errorf("update task: %w", err)
	}
	return nil
}

// GetTask retrieves a single task by ID.
func (s *Store) GetTask(ctx context.Context, id string) (*ChatTask, error) {
	var t ChatTask
	var dueDate *time.Time
	err := s.db.QueryRow(ctx,
		`SELECT t.id, t.message_id, t.channel_id, t.title,
		        COALESCE(t.assignee_id, ''), COALESCE(u.username, ''),
		        t.status, t.due_date, t.created_by, t.created_at, t.updated_at
		 FROM chat_tasks t LEFT JOIN users u ON t.assignee_id = u.id
		 WHERE t.id = $1`, id).
		Scan(&t.ID, &t.MessageID, &t.ChannelID, &t.Title,
			&t.AssigneeID, &t.AssigneeName, &t.Status, &dueDate,
			&t.CreatedBy, &t.CreatedAt, &t.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get task: %w", err)
	}
	t.DueDate = dueDate
	return &t, nil
}

// ListTasksByChannel returns tasks for a channel, optionally filtered by status.
func (s *Store) ListTasksByChannel(ctx context.Context, channelID, status string) ([]ChatTask, error) {
	query := `SELECT t.id, t.message_id, t.channel_id, t.title,
	                 COALESCE(t.assignee_id, ''), COALESCE(u.username, ''),
	                 t.status, t.due_date, t.created_by, t.created_at, t.updated_at
	          FROM chat_tasks t LEFT JOIN users u ON t.assignee_id = u.id
	          WHERE t.channel_id = $1`
	args := []interface{}{channelID}

	if status != "" {
		query += ` AND t.status = $2`
		args = append(args, status)
	}
	query += ` ORDER BY t.created_at DESC`

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	var tasks []ChatTask
	for rows.Next() {
		var t ChatTask
		var dueDate *time.Time
		if err := rows.Scan(&t.ID, &t.MessageID, &t.ChannelID, &t.Title,
			&t.AssigneeID, &t.AssigneeName, &t.Status, &dueDate,
			&t.CreatedBy, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		t.DueDate = dueDate
		tasks = append(tasks, t)
	}
	return tasks, nil
}
