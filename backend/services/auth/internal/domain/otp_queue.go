package domain

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultQueueSize    = 256
	defaultQueueRetries = 3
	defaultQueueBackoff = time.Second
)

// QueueOptions tunes a QueuedSender. Zero values take the defaults.
type QueueOptions struct {
	// Size bounds the jobs waiting for the worker (default 256).
	Size int
	// Retries is how many times a failed delivery is tried again (default 3),
	// waiting Backoff, 2*Backoff, 4*Backoff... between tries.
	Retries int
	// Backoff is the first wait before a retry (default 1s).
	Backoff time.Duration
}

// QueueStats counts what the worker has done since start. None of it carries a
// code or an address.
type QueueStats struct {
	Sent     int64 // delivered
	Failed   int64 // gave up after the retries
	Rejected int64 // refused at the door: the queue was full or closed
	Dropped  int64 // accepted but abandoned at shutdown, or too old to be useful
}

type queuedCode struct {
	identifier, identType, code string
	at                          time.Time
}

// QueuedSender delivers codes in the background so a sign-in request answers as
// soon as its code is stored, not when the mail server does. One worker owns the
// inner sender, which lets an SMTP connection be reused between codes.
//
// A code that cannot be delivered after the retries is logged and counted, never
// reported to the caller (who would otherwise learn whether an address exists);
// the person asks for a new code. Codes in the queue are lost if the process
// dies, which the resend button covers.
type QueuedSender struct {
	inner   CodeSender
	opts    QueueOptions
	jobs    chan queuedCode
	ctx     context.Context
	cancel  context.CancelFunc
	done    chan struct{}
	mu      sync.RWMutex // guards closed against sends racing Close
	closed  bool
	sent    atomic.Int64
	failed  atomic.Int64
	reject  atomic.Int64
	dropped atomic.Int64
}

var (
	_ CodeSender      = (*QueuedSender)(nil)
	_ OwnershipProver = (*QueuedSender)(nil)
)

// NewQueuedSender starts the worker. Call Close at shutdown.
func NewQueuedSender(inner CodeSender, opts QueueOptions) *QueuedSender {
	if opts.Size <= 0 {
		opts.Size = defaultQueueSize
	}
	if opts.Retries < 0 {
		opts.Retries = 0
	} else if opts.Retries == 0 {
		opts.Retries = defaultQueueRetries
	}
	if opts.Backoff <= 0 {
		opts.Backoff = defaultQueueBackoff
	}
	ctx, cancel := context.WithCancel(context.Background())
	q := &QueuedSender{
		inner: inner, opts: opts,
		jobs: make(chan queuedCode, opts.Size),
		ctx:  ctx, cancel: cancel,
		done: make(chan struct{}),
	}
	go q.run()
	return q
}

// DeliversToOwner repeats what the wrapped sender says.
func (q *QueuedSender) DeliversToOwner() bool {
	p, ok := q.inner.(OwnershipProver)
	return ok && p.DeliversToOwner()
}

// SendCode queues the code and returns at once. It returns ErrOTPBusy, without
// waiting, when the queue is full or closed.
func (q *QueuedSender) SendCode(_ context.Context, identifier, identType, code string) error {
	q.mu.RLock()
	defer q.mu.RUnlock()
	if q.closed {
		q.reject.Add(1)
		return ErrOTPBusy
	}
	select {
	case q.jobs <- queuedCode{identifier: identifier, identType: identType, code: code, at: time.Now()}:
		return nil
	default:
		n := q.reject.Add(1)
		slog.Warn("OTP delivery queue is full, request refused", "queue_size", q.opts.Size, "rejected_total", n)
		return ErrOTPBusy
	}
}

// Stats returns the counters.
func (q *QueuedSender) Stats() QueueStats {
	return QueueStats{Sent: q.sent.Load(), Failed: q.failed.Load(), Rejected: q.reject.Load(), Dropped: q.dropped.Load()}
}

func (q *QueuedSender) run() {
	defer close(q.done)
	for job := range q.jobs {
		if q.ctx.Err() != nil {
			q.dropped.Add(1)
			continue
		}
		q.deliver(job)
	}
}

// deliver tries one job, with retries.
func (q *QueuedSender) deliver(job queuedCode) {
	wait := q.opts.Backoff
	var err error
	for attempt := 0; attempt <= q.opts.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(wait):
				wait *= 2
			case <-q.ctx.Done():
				q.dropped.Add(1)
				return
			}
		}
		// A code older than its own lifetime opens nothing.
		if time.Since(job.at) >= otpTTL {
			q.dropped.Add(1)
			slog.Warn("OTP email dropped: code expired before it could be sent")
			return
		}
		if err = q.inner.SendCode(q.ctx, job.identifier, job.identType, job.code); err == nil {
			q.sent.Add(1)
			return
		}
		if q.ctx.Err() != nil {
			q.dropped.Add(1)
			return
		}
	}
	// The error comes from the mailer, which never puts a body or password in it.
	// Logged before it is counted, so a reader of the counter finds the line.
	slog.Error("OTP email delivery failed", "attempts", q.opts.Retries+1, "error", err, "failed_total", q.failed.Load()+1)
	q.failed.Add(1)
}

// Close stops taking codes, gives the worker up to grace to send what is queued,
// then abandons the rest. It returns how many codes were abandoned and closes the
// wrapped sender if it can be closed.
func (q *QueuedSender) Close(grace time.Duration) int {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return int(q.dropped.Load())
	}
	q.closed = true
	close(q.jobs)
	q.mu.Unlock()

	select {
	case <-q.done:
	case <-time.After(grace):
		q.cancel()
		// A sender that ignores cancellation must not hold shutdown either.
		select {
		case <-q.done:
		case <-time.After(time.Second):
		}
	}
	q.cancel()
	dropped := int(q.dropped.Load())
	if dropped > 0 {
		slog.Warn("OTP delivery queue closed with unsent codes", "dropped", dropped,
			"sent_total", q.sent.Load(), "failed_total", q.failed.Load())
	}
	if c, ok := q.inner.(io.Closer); ok {
		_ = c.Close()
	}
	return dropped
}
