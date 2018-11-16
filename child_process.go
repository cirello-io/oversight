package oversight

import (
	"context"
	"sync"
)

type state struct {
	mu    sync.Mutex
	state string // "" | "running" | "failed" | "done" --- maybe "shutdown"
	err   error
	stop  func()
}

func (r *state) current() state {
	r.mu.Lock()
	defer r.mu.Unlock()
	return state{
		state: r.state,
		err:   r.err,
		stop:  r.stop,
	}
}

func (r *state) setRunning(stop func()) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.state = "running"
	r.stop = stop
}

func (r *state) setErr(err error, restart bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.err = err
	if !restart {
		r.state = "done"
	}
}

func (r *state) setFailed() {
	r.mu.Lock()
	if r.state == "done" {
		r.mu.Unlock()
		return
	}
	r.state = "failed"
	r.mu.Unlock()
	r.stop()
}

type childProcess struct {
	restart Restart
	f       ChildProcess
}

// ChildProcess is a function that can be supervised for restart.
type ChildProcess func(ctx context.Context) error

// Restart is a function that decides if a worker has to be restarted or not
// according to its returned error.
type Restart func(error) bool

// Permanent goroutine is always restarted.
func Permanent() Restart { return func(err error) bool { return true } }

// Temporary goroutine is never restarted (not even when the supervisor restart
// strategy is rest_for_one or one_for_all and a sibling death causes the
// temporary process to be terminated).
func Temporary() Restart { return func(err error) bool { return false } }

// Transient goroutine is restarted only if it terminates abnormally, that is,
// with any error.
func Transient() Restart { return func(err error) bool { return err != nil } }
