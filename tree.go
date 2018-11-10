package oversight

import (
	"context"
	"sync"
	"time"
)

// Tree is the supervisor tree proper.
type Tree struct {
	initializeOnce sync.Once

	strategy  func()
	maxR      int
	maxT      time.Duration
	processes []childProcess
}

// Oversight creates and ignites a supervisor tree.
func Oversight(opts ...Option) ChildProcess {
	t := &Tree{}
	t.init()
	for _, opt := range opts {
		opt(t)
	}
	return t.Start
}

func (t *Tree) init() {
	t.initializeOnce.Do(func() {
		if t.maxR == 0 && t.maxT == 0 {
			DefaultRestartIntensity(t)
		}
		if t.strategy == nil {
			DefaultRestartStrategy(t)
		}
	})
}

// Start ignites the supervisor tree.
func (t *Tree) Start(ctx context.Context) error {
	return nil
}

// Option are applied to change the behavior of a Tree.
type Option func(*Tree)

// WithRestart defines a custom restart tolerance and strategy for the instance
// of oversight.
func WithRestart(maxR int, maxT time.Duration, strategy Strategy) Option {
	return func(t *Tree) {
		WithRestartIntensity(maxR, maxT)
		WithRestartStrategy(strategy)
	}
}

// WithRestartIntensity defines a custom tolerance for failures in the
// supervisor tree.
func WithRestartIntensity(maxR int, maxT time.Duration) Option {
	return func(t *Tree) {
		t.maxR, t.maxT = maxR, maxT
	}
}

// Default restart intensity expectations.
const (
	DefaultMaxR = 1
	DefaultMaxT = 5 * time.Second
)

// DefaultRestartIntensity redefines the tolerance for failures in the
// supervisor tree. It defaults to 1 restart (maxR) in the preceding 5 seconds
// (maxT).
func DefaultRestartIntensity(t *Tree) {
	t.maxR, t.maxT = DefaultMaxR, DefaultMaxT
}

// WithRestartStrategy defines a custom restart strategy for the supervisor
// tree.
func WithRestartStrategy(strategy Strategy) Option {
	return func(t *Tree) {
		t.strategy = strategy
	}
}

// DefaultRestartStrategy redefines the supervisor behavior to use OneForOne.
func DefaultRestartStrategy(t *Tree) {
	t.strategy = OneForOne
}

// Processes plugs one or more Permanent child processes to the supervisor tree.
// Processes never reset the child process list.
func Processes(processes ...ChildProcess) Option {
	return func(t *Tree) {
		for _, p := range processes {
			t.processes = append(t.processes, childProcess{
				restart: Permanent,
				f:       p,
			})
		}
	}
}

// Process plugs one child processes to the supervisor tree. Process never reset
// the child process list.
func Process(restart Restart, process ChildProcess) Option {
	return func(t *Tree) {
		t.processes = append(t.processes, childProcess{
			restart: restart,
			f:       process,
		})
	}
}
