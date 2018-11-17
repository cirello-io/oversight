package oversight

import (
	"context"
	"sync"
	"time"
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

// ChildProcessSpecification provides the complete interface to configure how
// the child process should behave itself in case of failures.
type ChildProcessSpecification struct {
	// Name is the human-friendly reference used for inspecting and
	// terminating child processes.
	Name string

	// Restart must be one of the Restart policies. The each oversight tree
	// implementation is free to interpret the result of this call.
	Restart Restart

	// Start initiates the child process in a panic-trapping cage. It does
	// not circumvent Go's panic-recover semantics. Avoid starting
	// goroutines inside the ChildProcess if they risk panic()'ing.
	Start ChildProcess

	// Shutdown defines the child process timeout. If the process is not
	// stopped within the specified duration, the oversight tree detached
	// the process and moves on. Null values mean wait forever. Unlike
	// Erlang, Infinity is the default both for oversight tree and worker
	// processes.
	Shutdown Shutdown
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

// Shutdown defines how the oversight handles child processes hanging after they
// are signaled to stop.
type Shutdown func() (context.Context, context.CancelFunc)

// Infinity will wait until the process naturally dies.
func Infinity() Shutdown {
	return func() (context.Context, context.CancelFunc) {
		return context.WithCancel(context.Background())
	}
}

// Timeout defines a duration of time that the oversight will wait before
// detaching from the winding process.
func Timeout(d time.Duration) Shutdown {
	return func() (context.Context, context.CancelFunc) {
		return context.WithTimeout(context.Background(), d)
	}
}
