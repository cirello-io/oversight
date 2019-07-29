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
	"fmt"
	"time"
)

// TreeOption are applied to change the behavior of a Tree.
type TreeOption func(*Tree)

// WithSpecification defines a custom setup to tweak restart tolerance and
// strategy for the instance of oversight.
func WithSpecification(maxR int, maxT time.Duration, strategy Strategy) TreeOption {
	return func(t *Tree) {
		WithMaximumRestartIntensity(maxR, maxT)(t)
		WithRestartStrategy(strategy)(t)
	}
}

// WithMaximumRestartIntensity defines a custom tolerance for failures in the
// supervisor tree.
//
// Refer to http://erlang.org/doc/design_principles/sup_princ.html#maximum-restart-intensity
func WithMaximumRestartIntensity(maxR int, maxT time.Duration) TreeOption {
	return func(t *Tree) {
		t.maxR, t.maxT = maxR, maxT
	}
}

// WithRestartIntensity is an alias for WithMaximumRestartIntensity.
// Deprecated in favor of WithMaximumRestartIntensity.
func WithRestartIntensity(maxR int, maxT time.Duration) TreeOption {
	return WithMaximumRestartIntensity(maxR, maxT)
}

// NeverHalt will configure the oversight tree to never stop in face of failure.
func NeverHalt() TreeOption {
	return func(t *Tree) {
		t.maxR = -1
	}
}

// Default restart intensity expectations.
const (
	DefaultMaxR = 1
	DefaultMaxT = 5 * time.Second
)

// DefaultMaximumRestartIntensity redefines the tolerance for failures in the
// supervisor tree. It defaults to 1 restart (maxR) in the preceding 5 seconds
// (maxT).
//
// Refer to http://erlang.org/doc/design_principles/sup_princ.html#tuning-the-intensity-and-period
func DefaultMaximumRestartIntensity() TreeOption {
	return func(t *Tree) {
		t.maxR, t.maxT = DefaultMaxR, DefaultMaxT
	}
}

// DefaultRestartIntensity is an alias for DefaultMaximumRestartIntensity.
//
// Deprecated in favor of DefaultMaximumRestartIntensity.
func DefaultRestartIntensity() TreeOption {
	return DefaultMaximumRestartIntensity()
}

// WithRestartStrategy defines a custom restart strategy for the supervisor
// tree.
func WithRestartStrategy(strategy Strategy) TreeOption {
	return func(t *Tree) {
		t.strategy = strategy
	}
}

// DefaultRestartStrategy redefines the supervisor behavior to use OneForOne.
func DefaultRestartStrategy() TreeOption {
	return func(t *Tree) {
		t.strategy = OneForOne()
	}
}

// Processes plugs one or more Permanent child processes to the supervisor tree.
// Processes never reset the child process list.
func Processes(processes ...ChildProcess) TreeOption {
	return func(t *Tree) {
		for _, p := range processes {
			Process(ChildProcessSpecification{
				Restart: Permanent(),
				Start:   p,
			})(t)
		}
	}
}

// Process plugs one or more child processes to the supervisor tree. Process
// never reset the child process list.
func Process(specs ...ChildProcessSpecification) TreeOption {
	return func(t *Tree) {
		t.init()
		for _, spec := range specs {
			id := len(t.processes) + 1
			t.states = append(t.states, state{
				stop: func() {
					t.logger.Println("stopped before start")
				},
			})
			if spec.Name == "" {
				spec.Name = fmt.Sprintf("childproc %d", id)
			}
			if _, ok := t.processIndex[spec.Name]; ok {
				spec.Name += fmt.Sprint(" ", id)
			}
			if spec.Restart == nil {
				spec.Restart = Permanent()
			}
			if spec.Shutdown == nil {
				spec.Shutdown = Timeout(DefaultChildProcessTimeout)
			}
			if spec.Start == nil {
				panic("child process must always have a function")
			}
			t.processes = append(t.processes, spec)
			t.processIndex[spec.Name] = id - 1
		}
	}
}

// Logger defines the interface for any logging facility to be compatible with
// oversight trees.
type Logger interface {
	Printf(format string, args ...interface{})
	Println(args ...interface{})
}

// WithLogger plugs a custom logger to the oversight tree.
func WithLogger(logger Logger) TreeOption {
	return func(t *Tree) {
		t.logger = logger
	}
}

// WithTree is a shortcut to add a tree as a child process.
func WithTree(subTree *Tree) TreeOption {
	return func(t *Tree) {
		Process(ChildProcessSpecification{
			Restart:  Permanent(),
			Start:    subTree.Start,
			Shutdown: Infinity(),
		})(t)
	}
}
