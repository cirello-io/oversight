package oversight

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrTooManyFailures means that the supervisor detected that one of the child
// processes has failed too much and that it decided to fully stop.
var ErrTooManyFailures = errors.New("too many failures")

// ErrNoChildProcessLeft means that all processes in the supervisor are done,
// and there is no one left to restart.
var ErrNoChildProcessLeft = errors.New("no child process left")

// Oversight creates and ignites a supervisor tree.
func Oversight(opts ...Option) ChildProcess {
	t := New(opts...)
	return t.Start
}

// Tree is the supervisor tree proper.
type Tree struct {
	initializeOnce sync.Once

	strategy  Strategy
	maxR      int
	maxT      time.Duration
	processes []childProcess
	states    []state

	rootErrMu sync.Mutex
	rootErr   error
}

// New creates a new oversight (supervisor) tree with the applied options.
func New(opts ...Option) *Tree {
	t := &Tree{}
	t.init()
	for _, opt := range opts {
		opt(t)
	}
	return t
}

func (t *Tree) setErr(err error) {
	t.rootErrMu.Lock()
	defer t.rootErrMu.Unlock()
	if t.rootErr == nil {
		t.rootErr = err
	}
}

func (t *Tree) err() error {
	t.rootErrMu.Lock()
	defer t.rootErrMu.Unlock()
	return t.rootErr
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
func (t *Tree) Start(rootCtx context.Context) error {
	t.init()
	ctx, cancel := context.WithCancel(rootCtx)
	defer cancel()
	var wg sync.WaitGroup
	defer wg.Wait()
	restarter := &restart{
		intensity: t.maxR,
		period:    t.maxT,
	}
	failure := make(chan int)
	t.states = make([]state, len(t.processes))
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			anyStartedProcess := false
			startSemaphore := make(chan struct{})
			for i, p := range t.processes {
				running := t.states[i].current()
				if running.state == "running" {
					anyStartedProcess = true
					continue
				}
				if running.state == "done" {
					continue
				}
				if running.state == "failed" && !p.restart(running.err) {
					continue
				}
				anyStartedProcess = true
				childCtx, childCancel := context.WithCancel(ctx)
				var childWg sync.WaitGroup
				wg.Add(1)
				childWg.Add(1)
				t.states[i].setRunning(func() {
					childCancel()
					childWg.Wait()
				})
				go func(i int, p childProcess) {
					defer wg.Done()
					defer childWg.Done()
					<-startSemaphore
					err := safeRun(childCtx, p.f)
					restart := p.restart(err)
					t.states[i].setErr(err, restart)
					// TODO(uc): add support for timeout detach
					select {
					case <-childCtx.Done():
					case failure <- i:
					}

				}(i, p)
			}
			close(startSemaphore)
			if !anyStartedProcess {
				t.setErr(ErrNoChildProcessLeft)
				cancel()
				return
			}

			select {
			case <-ctx.Done():
				return
			case failedChild := <-failure:
				t.strategy(t, failedChild)
				if restarter.terminate(time.Now()) {
					t.setErr(ErrTooManyFailures)
					cancel()
					return
				}
			}
		}
	}()

	wg.Wait()
	return t.err()
}
