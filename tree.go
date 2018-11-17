package oversight

import (
	"context"
	"errors"
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

// ErrUnknownProcess is returned when runtime operations (like delete or
// terminate) failed because the process is not present.
var ErrUnknownProcess = errors.New("unknown process")

// ErrProcessNotRunning is returned when caller tries to terminated processes
// that are not running.
var ErrProcessNotRunning = errors.New("process not running")

// Oversight creates and ignites a supervisor tree.
func Oversight(opts ...TreeOption) ChildProcess {
	t := New(opts...)
	return t.Start
}

// Tree is the supervisor tree proper.
type Tree struct {
	initializeOnce sync.Once

	// semaphore must be held when adding/deleting dynamic processes
	semaphore      sync.Mutex
	strategy       Strategy
	maxR           int
	maxT           time.Duration
	processes      []ChildProcessSpecification
	states         []state
	processChanged chan struct{}  // indicates that some change to process slice has been made
	processIndex   map[string]int // map of ChildProcessSpecification.Name to internal ID

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
		t.processChanged = make(chan struct{})
		if t.maxR == 0 && t.maxT == 0 {
			DefaultRestartIntensity()(t)
		}
		if t.strategy == nil {
			DefaultRestartStrategy()(t)
		}
		t.logger = log.New(ioutil.Discard, "", 0)
		t.processIndex = make(map[string]int)
	})
}

// Add attaches a new child process to a running oversight tree.
func (t *Tree) Add(spec ChildProcessSpecification) {
	t.init()
	t.semaphore.Lock()
	Process(spec)(t)
	t.semaphore.Unlock()
	t.processChanged <- struct{}{}
}

// Start ignites the supervisor tree.
func (t *Tree) Start(rootCtx context.Context) error {
	/*
		Theory of operation

		This is not a line-by-line of Erlang's supervisor module because
		functional programming patterns are not the most efficient
		idioms for Go programs. I have referred to Erlang's
		supervisor.erl and its Elixir cousin's supervisor.ex to how this
		implementation should behave. The design principles document for
		Erlang outlines a lot of how it works, but leaves significant
		gaps that only the source code answers.

		This supervisor tree has one loop divided in two phases:
		1 - differential process start according to the restart definition.
		2 - failure capture with application of termination strategy.

		The definition of failure and termination strategy will be
		presented shortly.

		1 - Differential process start

		When the oversight tree is configured, it takes each declared
		child process and create a state to represent its lifecyle.

		Using the start definition it decides if the process should be
		either started (when it is the first time), restarted (after
		failure), or ignored.

		Each started process are hold onto a channel to prevent that a
		process that fail on start to automatically trigger a tree wide
		restart. Once all child processes are ready to start, this
		channel signals that they can run and the second phase starts.


		2 - Capture failure and apply termination strategy.

		Each child process is given the access to a channel to notify
		failures. When one of the child processes fails, the oversight
		tree applies a failure strategy (one_for_one, one_for_all,
		rest_for_one, and simple_one_for_one) - that is it terminates
		all other child processes affected by the strategy.

		It records the termination in the restarter bookkeeper, that
		decides if the tree has failed too much too soon; if that is the
		case, the tree terminates its alive child processes and then
		itself.


		Definition of failure (Permanent, Temporary and Transient)

		The definition of failure determines whether the process needs
		to be restarted once it reached the "failed" state. It is
		particularly sensitive for Temporary processes, because even
		when they do fail, the net result is always success. I checked
		Elixir's implementation and in fact, Temporary child processes
		are always considered successful whether they fail or not.

		Thus, only Permanent and Transient can fail. Permanent
		terminations are always considered failure. Transient successes
		are considered normal terminations and Transient failures are
		considered failures. Failures triggers tree restarts.


		Definition of termination strategy (OneForOne, OneForAll, RestForOne, SimpleOneForOne)

		Termination strategies handle how the oversight tree handle
		failures. They have the same as they do in Erlang. The
		difference is that in Erlang you can use brutalKill to terminate
		a child process. That's not possible in Go. In this
		implementation, when the child process does not terminate on
		time, the oversight tree simply detaches the offending goroutine
		and moves on.

		Blind Spots:
		- due to panic/recover semantics, child processes that spawn
		panicky goroutines will never be able to trap these events; it
		is up to the programmer to make sure that goroutines inside of
		child processes to never panic.
		- Goroutines cannot be killed - this implementation relies on
		contexts cancelations to propagate termination calls.
	*/
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
				t.logger.Printf("context canceled (before start): %v", ctx.Err())
				t.semaphore.Lock()
				OneForAll()(t, 0)
				t.semaphore.Unlock()
				t.logger.Printf("clean up complete")
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
				t.logger.Printf("starting %v", p.Name)
				t.startChildProcess(ctx, i, p, startSemaphore,
					failure)
			}
			close(startSemaphore)
			t.semaphore.Unlock()
			if !anyNewStartedProcess && anyStartedProcessEver {
				t.logger.Printf("no child process left after start")
				t.setErr(ErrNoChildProcessLeft)
				cancel()
				return
			}

			select {
			case <-ctx.Done():
				t.logger.Printf("context canceled (after start): %v", ctx.Err())
				t.semaphore.Lock()
				OneForAll()(t, 0)
				t.semaphore.Unlock()
				t.logger.Printf("clean up complete")
				return
			case <-t.processChanged:
				t.logger.Println("detected change in child processes list")
			case failedChild := <-failure:
				t.semaphore.Lock()
				t.logger.Printf("child process failure detected (%v)", t.processes[failedChild].Name)
				t.strategy(t, failedChild)
				t.semaphore.Unlock()
				if restarter.terminate(time.Now()) {
					t.logger.Printf("too many failures detected:")
					for _, restart := range restarter.restarts {
						t.logger.Println("-", restart)
					}
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

func (t *Tree) startChildProcess(ctx context.Context, processID int,
	p ChildProcessSpecification, startSemaphore <-chan struct{}, failure chan int) {
	childCtx, childWg := t.plugStop(ctx, processID, p)
	go func(processID int, p ChildProcessSpecification) {
		defer childWg.Done()
		<-startSemaphore
		t.logger.Println(p.Name, "child started")
		defer t.logger.Println(p.Name, "child done")
		err := safeRun(childCtx, p.Start)
		restart := p.Restart(err)
		t.states[processID].setErr(err, restart)
		select {
		case <-childCtx.Done():
		case failure <- processID:
		}
	}(processID, p)
}

func (t *Tree) plugStop(ctx context.Context, processID int, p ChildProcessSpecification) (context.Context, *sync.WaitGroup) {
	childCtx, childCancel := context.WithCancel(ctx)
	var childWg sync.WaitGroup
	childWg.Add(1)
	t.states[processID].setRunning(func() {
		t.logger.Println(p.Name, "stopping")
		stopCtx, stopCancel := p.Shutdown()
		defer stopCancel()
		wgComplete := make(chan struct{})
		childCancel()
		go func() {
			childWg.Wait()
			close(wgComplete)
		}()
		select {
		case <-wgComplete:
			t.logger.Println(p.Name, "stopped")
		case <-stopCtx.Done():
			t.logger.Println(p.Name, "timeout")
		}
	})
	return childCtx, &childWg
}

// Terminate stop the named process. Terminated child processes do not count
// as failures in the oversight tree restart policy. If the oversight tree runs
// out of processes to supervise, it will terminate itself with
// ErrNoChildProcessLeft.
func (t *Tree) Terminate(name string) error {
	t.init()
	t.semaphore.Lock()
	defer t.semaphore.Unlock()
	id, ok := t.processIndex[name]
	if !ok {
		return ErrUnknownProcess
	}
	t.states[id].mu.Lock()
	state := t.states[id].state
	stop := t.states[id].stop
	if state != "running" || stop == nil {
		t.states[id].mu.Unlock()
		return ErrProcessNotRunning
	}
	t.states[id].state = "done"
	t.states[id].mu.Unlock()
	stop()
	go func() { t.processChanged <- struct{}{} }()
	return nil
}
