package oversight

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
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
func Oversight(opts ...TreeOption) ChildProcess {
	t := New(opts...)
	return t.Start
}

// Tree is the supervisor tree proper.
type Tree struct {
	initializeOnce sync.Once

	// semaphore must be held when adding/deleting dynamic processes
	semaphore  sync.Mutex
	strategy   Strategy
	maxR       int
	maxT       time.Duration
	processes  []ChildProcessSpecification
	states     []state
	newProcess chan struct{} // indicates a new dynamic process is present

	rootErrMu sync.Mutex
	rootErr   error

	logger *log.Logger
}

// New creates a new oversight (supervisor) tree with the applied options.
func New(opts ...TreeOption) *Tree {
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
		t.semaphore.Lock()
		defer t.semaphore.Unlock()
		t.newProcess = make(chan struct{})
		if t.maxR == 0 && t.maxT == 0 {
			DefaultRestartIntensity()(t)
		}
		if t.strategy == nil {
			DefaultRestartStrategy()(t)
		}
		t.logger = log.New(ioutil.Discard, "", 0)
	})
}

// Add attaches a new child process to a running oversight tree.
func (t *Tree) Add(restart Restart, f ChildProcess) {
	t.init()
	t.semaphore.Lock()
	t.states = append(t.states, state{})
	name := fmt.Sprintf("childproc %d", len(t.states))
	t.processes = append(t.processes, ChildProcessSpecification{
		Name:    name,
		Restart: restart,
		Start:   f,
	})
	t.semaphore.Unlock()
	t.newProcess <- struct{}{}
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
	var anyStartedProcessEver bool
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			t.semaphore.Lock()
			anyNewStartedProcess := false
			startSemaphore := make(chan struct{})
			for i, p := range t.processes {
				running := t.states[i].current()
				if running.state == "running" {
					anyNewStartedProcess = true
					continue
				}
				if running.state == "done" {
					continue
				}
				if running.state == "failed" &&
					!p.Restart(running.err) {
					continue
				}
				anyNewStartedProcess = true
				anyStartedProcessEver = true
				t.startChildProcess(ctx, &wg, i, p,
					startSemaphore, failure)
			}
			close(startSemaphore)
			t.semaphore.Unlock()
			if !anyNewStartedProcess && anyStartedProcessEver {
				t.setErr(ErrNoChildProcessLeft)
				cancel()
				return
			}

			select {
			case <-ctx.Done():
				return
			case <-t.newProcess:
			case failedChild := <-failure:
				t.semaphore.Lock()
				t.strategy(t, failedChild)
				t.semaphore.Unlock()
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

func (t *Tree) startChildProcess(ctx context.Context, wg *sync.WaitGroup, i int,
	p ChildProcessSpecification, startSemaphore <-chan struct{}, failure chan int) {
	childCtx, childCancel := context.WithCancel(ctx)
	var childWg sync.WaitGroup
	wg.Add(1)
	childWg.Add(1)
	t.states[i].setRunning(func() {
		childCancel()
		childWg.Wait()
		t.logger.Println("child stopped")
	})
	go func(i int, p ChildProcessSpecification) {
		defer wg.Done()
		defer childWg.Done()
		<-startSemaphore
		t.logger.Println("child started")
		defer t.logger.Println("child done")
		err := safeRun(childCtx, p.Start)
		restart := p.Restart(err)
		t.states[i].setErr(err, restart)
		// TODO(uc): add support for timeout detach
		select {
		case <-childCtx.Done():
		case failure <- i:
		}

	}(i, p)
}
