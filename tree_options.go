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

// Logger defines the interface for any logging facility to be compatible with
// oversight trees.
type Logger interface {
	Printf(format string, args ...any)
	Println(args ...any)
}

// WithLogger plugs a custom logger to the oversight tree. It assumes the logger
// is thread-safe.
func WithLogger(logger Logger) TreeOption {
	return func(t *Tree) {
		t.logger = logger
	}
}
