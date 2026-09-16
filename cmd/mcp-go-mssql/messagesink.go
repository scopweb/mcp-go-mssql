package main

import (
	"context"
	"sync"
	"time"

	"github.com/golang-sql/sqlexp"
)

// messageSink captures SQL Server informational messages emitted during
// a single query execution. It is created per query, attached to the
// query as a *sqlexp.ReturnMessage argument, and drained by a goroutine
// keyed off the request context.
//
// The driver enqueues messages via sqlexp.ReturnMessageEnqueue. We
// drain them with Message(ctx) on a goroutine, append their textual
// representation to an internal slice, and stop when:
//   - the context is done (request cancelled or timed out), or
//   - the driver signals end-of-batch (MsgNextResultSet + no more
//     rows to process), or
//   - any other RawMessage type arrives (defensive: stop on unknown).
type messageSink struct {
	rm    *sqlexp.ReturnMessage
	done  chan struct{}
	mu    sync.Mutex
	lines []string
	// ctxCancel cancels the goroutine's drain context. It is called by
	// stop() after the query rows are drained.
	ctxCancel context.CancelFunc
}

func newMessageSink() *messageSink {
	rm := &sqlexp.ReturnMessage{}
	sqlexp.ReturnMessageInit(rm)
	return &messageSink{
		rm:   rm,
		done: make(chan struct{}),
	}
}

// start launches a goroutine that drains rm.Message(ctx) until the
// context is done or the driver signals end-of-batch. The goroutine
// appends MsgNotice lines to the internal slice in arrival order.
func (s *messageSink) start(ctx context.Context) {
	drainCtx, cancel := context.WithCancel(ctx)
	s.ctxCancel = cancel
	go func() {
		defer close(s.done)
		for {
			raw := s.rm.Message(drainCtx)
			switch m := raw.(type) {
			case sqlexp.MsgNotice:
				if m.Message != nil {
					s.append(m.Message.String())
				}
			case sqlexp.MsgNextResultSet:
				// Driver signals "another result set is starting" —
				// keep draining; messages may still arrive.
				continue
			default:
				// End of messages (or unsupported type). Stop.
				return
			}
		}
	}()
}

// append adds a captured line to the internal slice. Safe for
// concurrent use (drained by the goroutine, read by the caller).
func (s *messageSink) append(line string) {
	s.mu.Lock()
	s.lines = append(s.lines, line)
	s.mu.Unlock()
}

// linesSnapshot returns a copy of the captured lines.
func (s *messageSink) linesSnapshot() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.lines))
	copy(out, s.lines)
	return out
}

// stop cancels the drain goroutine's context and waits up to timeout
// for it to exit. If the timeout elapses the goroutine is left to
// exit on its own (it will eventually notice the cancelled context);
// the worst case is a brief leak until the surrounding *sql.DB
// connection is recycled.
func (s *messageSink) stop(timeout time.Duration) {
	if s.ctxCancel != nil {
		s.ctxCancel()
	}
	select {
	case <-s.done:
	case <-time.After(timeout):
		// Give up waiting; the drain goroutine will exit when the
		// driver closes the underlying channel or the process ends.
	}
}
