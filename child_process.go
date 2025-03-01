// Copyright 2018 cirello.io/oversight/v2 - Ulderico Cirello
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
	"sync"
	"time"
)

// ChildProcessState represents the current lifecycle step of the child process.
type ChildProcessState string

// Child processes navigate through a sequence of states, that are atomically
// managed by the oversight tree to decide if child process needs to be started
// or not.
//
//	                         ┌─────────────────────┐
//	                         │                     │
//	                         │              ┌────────────┐
//	                         ▼         ┌───▶│   Failed   │
//	┌────────────┐    ┌────────────┐   │    └────────────┘
//	│  Starting  │───▶│  Running   │───┤
//	└────────────┘    └────────────┘   │    ┌────────────┐
//	                                   └───▶│    Done    │
//	                                        └────────────┘
const (
	Starting ChildProcessState = ""
	Running  ChildProcessState = "running"
	Failed   ChildProcessState = "failed"
	Done     ChildProcessState = "done"
)

// State is a snapshot of the child process current state.
type State struct {
	Name  string
	State ChildProcessState
	Stop  func()
}

type state struct {
	mu    sync.Mutex
	state ChildProcessState
	err   error
	stop  func()
}

func (r *state) currentChildProcessState() ChildProcessState {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.state
}

func (r *state) setRunning(stop func()) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.state = Running
	r.stop = stop
}

func (r *state) setErr(err error, restart bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.err = err
	if !restart {
		r.state = Done
	}
}

func (r *state) setFailed() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.state == Done {
		return
	}
	r.state = Failed
}

// childProcessSpecification provides the complete interface to configure how
// the child process should behave itself in case of failures.
type childProcessSpecification struct {
	// name is the human-friendly reference used for inspecting and
	// terminating child processes. The name must be unique within the
	// oversight tree. If left empty, the oversight tree will generate a
	// unique name.
	name string

	// restart must be one of the [restart] policies.
	restart Restart

	// fn initiates the child process in a panic-trapping cage. It does not
	// circumvent Go's panic-recover semantics. Avoid starting goroutines
	// inside the ChildProcess if they risk panic()'ing.
	fn ChildProcess

	// shutdown must be one of the [shutdown] policies.
	shutdown Shutdown
}

// ChildProcess is a function that can be restarted
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

type shutdownContextValue string

var detachableContext = shutdownContextValue("detachable")

// Natural will wait until the process naturally dies.
func Natural() Shutdown {
	return func() (context.Context, context.CancelFunc) {
		ctx := context.WithValue(context.Background(), detachableContext, false)
		return context.WithCancel(ctx)
	}
}

// Timeout defines a duration of time that the oversight will wait before
// detaching from the winding process.
func Timeout(d time.Duration) Shutdown {
	return func() (context.Context, context.CancelFunc) {
		ctx := context.WithValue(context.Background(), detachableContext, true)
		return context.WithTimeout(ctx, d)
	}
}
