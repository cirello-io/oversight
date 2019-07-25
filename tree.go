// Copyright 2018 cirello.io/oversight - Ulderico Cirello
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

// ErrTreeNotRunning is returned to Add, Terminate and Delete calls when the
// oversight tree is initialized but not started yet; or when at that point in
// time is not running anymore.
var ErrTreeNotRunning = errors.New("oversight tree is not running")

// ErrInvalidChildProcessType is returned when caller tries to add an invalid
// child process to the oversight tree. The child process type must always be
// ChildProcessSpecification, ChildProcess, and *Tree.
var ErrInvalidChildProcessType = errors.New("invalid child process type")

// ErrInvalidConfiguration is returned when tree has invalid settings.
var ErrInvalidConfiguration = errors.New("invalid tree configuration")

// Tree is the supervisor tree proper.
type Tree struct {
	initializeOnce sync.Once
	stopped        chan struct{}

	// semaphore must be held when adding/deleting dynamic processes
	semaphore      sync.Mutex
	strategy       Strategy
	maxR           int
	maxT           time.Duration
	processes      []ChildProcessSpecification
	states         []state
	processChanged chan struct{}  // indicates that some change to process slice has been made
	processIndex   map[string]int // map of ChildProcessSpecification.Name to internal ID

	logger Logger

	err error

	// internal loop management variables
	failure               chan int
	anyStartedProcessEver bool
	restarter             *restart
}

// New creates a new oversight (supervisor) tree with the applied options.
func New(opts ...TreeOption) *Tree {
	t := &Tree{}
	for _, opt := range opts {
		opt(t)
	}
	t.init()
	return t
}

func (t *Tree) init() {
	t.initializeOnce.Do(func() {
		t.semaphore.Lock()
		defer t.semaphore.Unlock()
		isValidConfiguration := t.maxR >= -1 && t.maxT >= 0
		if !isValidConfiguration {
			t.err = ErrInvalidConfiguration
			return
		}
		t.processChanged = make(chan struct{}, 1)
		if t.maxR == 0 && t.maxT == 0 {
			DefaultRestartIntensity()(t)
		}
		if t.strategy == nil {
			DefaultRestartStrategy()(t)
		}
		if t.logger == nil {
			t.logger = log.New(ioutil.Discard, "", 0)
		}
		t.processIndex = make(map[string]int)
		t.stopped = make(chan struct{})
		t.failure = make(chan int)
		t.restarter = &restart{
			intensity: t.maxR,
			period:    t.maxT,
		}
	})
}

// Add attaches a new child process to a running oversight tree.  This call must
// be used on running oversight trees. If the tree is halted, it is going to
// fail with ErrTreeNotRunning. The valid types are ChildProcessSpecification,
// ChildProcess, and *Tree. If the added child process is invalid, it is going
// to fail with ErrInvalidChildProcessType.
func (t *Tree) Add(spec interface{}) error {
	t.init()
	if t.err != nil {
		return ErrTreeNotRunning
	}
	select {
	case <-t.stopped:
		return ErrTreeNotRunning
	default:
	}
	var add func()
	switch p := spec.(type) {
	case ChildProcessSpecification:
		add = func() { Process(p)(t) }
	case ChildProcess:
		add = func() { Processes(p)(t) }
	case func(ctx context.Context) error:
		add = func() { Processes(p)(t) }
	case *Tree:
		add = func() { WithTree(p)(t) }
	default:
		return ErrInvalidChildProcessType
	}
	t.semaphore.Lock()
	add()
	t.semaphore.Unlock()
	go func() { t.processChanged <- struct{}{} }()
	return nil
}

// Start ignites the supervisor tree.
func (t *Tree) Start(rootCtx context.Context) error {
	/*
		Theory of operation

		This is not a line-by-line of Erlang's supervisor module because
		functional programming patterns are not the most efficient
		idioms in Go programs. I have referred to Erlang's
		supervisor.erl and its Elixir cousin's supervisor.ex to how this
		implementation should behave. Erlang's design principles
		document outlines a lot of how it works, but leaves significant
		gaps that only the source code can address.

		This supervisor tree has one loop divided in two phases:
		1 - differential processes start according to their restart
		definition.
		2 - capture child processes failures and apply the termination
		strategy.

		The definition of failure and termination strategy will be
		presented shortly.

		1 - Child processes start

		When the oversight tree is configured, it takes each declared
		child process and create a state to represent its lifecyle.

		Using the start definition it decides if the process should be
		either started (when it is the first time), restarted (after
		failure), or ignored.

		Each started process are hold onto a channel to prevent that a
		process that fail on start to automatically trigger a tree wide
		restart. Once all child processes are ready to start, this
		channel signals that they can run and the second phase starts.


		2 - Fail, recovery and termination

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
	if t.err != nil {
		return t.err
	}
	ctx, cancel := context.WithCancel(rootCtx)
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return t.drain(ctx)
		default:
			t.startChildProcesses(ctx, cancel)
			t.handleTreeChanges(ctx, cancel)
		}
	}
}

func (t *Tree) drain(ctx context.Context) error {
	close(t.stopped)
	defer t.logger.Printf("clean up complete")
	t.logger.Printf("context canceled (before start): %v", ctx.Err())
	t.semaphore.Lock()
	for i := len(t.states) - 1; i >= 0; i-- {
		t.strategy(t, i)
	}
	t.semaphore.Unlock()
	for {
		select {
		case <-t.processChanged:
		default:
			return t.err
		}
	}
}

func (t *Tree) startChildProcesses(ctx context.Context, cancel context.CancelFunc) {
	t.semaphore.Lock()
	anyRunningProcess := false
	startSemaphore := make(chan struct{})
	for i, p := range t.processes {
		running := t.states[i].current()
		if running.state == Running {
			anyRunningProcess = true
			continue
		}
		if running.state == Done {
			continue
		}
		anyRunningProcess = true
		t.anyStartedProcessEver = true
		t.logger.Printf("starting %v", p.Name)
		t.startChildProcess(ctx, i, p, startSemaphore)
	}
	close(startSemaphore)
	t.semaphore.Unlock()
	if !anyRunningProcess && t.anyStartedProcessEver {
		t.logger.Printf("no child process left after start")
		t.err = ErrNoChildProcessLeft
		cancel()
	}
}

func (t *Tree) handleTreeChanges(ctx context.Context, cancel context.CancelFunc) {
	select {
	case <-ctx.Done():
	case <-t.processChanged:
		t.logger.Println("detected change in child processes list")
	case failedChild := <-t.failure:
		t.semaphore.Lock()
		t.logger.Printf("child process failure detected (%v)", t.processes[failedChild].Name)
		t.strategy(t, failedChild)
		t.semaphore.Unlock()
		if t.restarter.terminate(time.Now()) {
			t.logger.Printf("too many failures detected:")
			for _, restart := range t.restarter.restarts {
				t.logger.Println("-", restart)
			}
			t.err = ErrTooManyFailures
			cancel()
		}
	}
}

func (t *Tree) startChildProcess(ctx context.Context, processID int,
	p ChildProcessSpecification, startSemaphore <-chan struct{}) {
	childCtx, childWg := t.plugStop(ctx, processID, p)
	go func(processID int, p ChildProcessSpecification) {
		defer childWg.Done()
		<-startSemaphore
		t.logger.Println(p.Name, "child started")
		defer t.logger.Println(p.Name, "child done")
		err := safeRun(childCtx, p.Start)
		if err != nil {
			t.logger.Println(p.Name, "errored:", err)
		}
		restart := p.Restart(err)
		t.setStateError(p.Name, err, restart)
		select {
		case <-childCtx.Done():
		case t.failure <- processID:
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
// ErrNoChildProcessLeft. This call must be used on running oversight trees, if
// the tree is not started yet, it is going to block. If the tree is halted, it
// is going to fail with ErrTreeNotRunning.
func (t *Tree) Terminate(name string) error {
	t.init()
	if t.err != nil {
		return ErrTreeNotRunning
	}
	select {
	case <-t.stopped:
		return ErrTreeNotRunning
	default:
	}
	t.semaphore.Lock()
	id, ok := t.processIndex[name]
	if !ok {
		t.semaphore.Unlock()
		return ErrUnknownProcess
	}
	t.states[id].mu.Lock()
	state := t.states[id].state
	stop := t.states[id].stop
	if state != Running || stop == nil {
		t.states[id].mu.Unlock()
		t.semaphore.Unlock()
		return ErrProcessNotRunning
	}
	t.states[id].state = Done
	t.states[id].mu.Unlock()
	t.semaphore.Unlock()
	stop()
	t.logger.Println("Terminate.processChanged start")
	t.processChanged <- struct{}{}
	t.logger.Println("Terminate.processChanged end")
	return nil
}

func (t *Tree) setStateError(name string, err error, restart bool) {
	processID := t.processIndex[name]
	t.states[processID].setErr(err, restart)
}

// Delete stops the service in the oversight tree and remove from it. If the
// oversight tree runs out of processes to supervise, it will terminate itself
// with ErrNoChildProcessLeft. This call must be used on running oversight
// trees, if the tree is not started yet, it is going to block. If the tree is
// halted, it is going to fail with ErrTreeNotRunning.
func (t *Tree) Delete(name string) error {
	if err := t.Terminate(name); err != nil {
		return err
	}
	t.semaphore.Lock()
	defer t.semaphore.Unlock()
	id := t.processIndex[name]
	t.states = append(t.states[:id], t.states[id+1:]...)
	t.processes = append(t.processes[:id], t.processes[id+1:]...)
	t.processIndex = make(map[string]int)
	for i, p := range t.processes {
		t.processIndex[p.Name] = i
	}
	return nil
}

// Children returns the current set of child processes.
func (t *Tree) Children() []State {
	t.init()
	t.semaphore.Lock()
	defer t.semaphore.Unlock()
	ret := []State{}
	for i := range t.states {
		t.states[i].mu.Lock()
		name := t.processes[i].Name
		ret = append(ret, State{
			Name:  name,
			State: t.states[i].state,
			Stop:  t.states[i].stop,
		})
		t.states[i].mu.Unlock()
	}
	return ret
}
