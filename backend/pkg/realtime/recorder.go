package realtime

import "sync"

// Recorder is an Emitter that keeps what it is given. Tests use it to assert
// what a service announced, and when.
type Recorder struct {
	mu     sync.Mutex
	events []Event
}

// Emit records the event.
func (r *Recorder) Emit(e Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, e)
}

// Events returns a copy of everything recorded so far.
func (r *Recorder) Events() []Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Event(nil), r.events...)
}
