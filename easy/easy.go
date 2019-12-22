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

/*
Package easy is an easier interface to use cirello.io/oversight. Its lifecycle
is managed through context.Context. Stop a given oversight tree by cancelling
its context.


	package main

	import oversight "cirello.io/oversight/easy"

	func main() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		// use cancel() to stop the oversight
		ctx = oversight.WithContext(ctx)
		oversight.Add(ctx, func(ctx context.Context) error {
			// ...
		})
	}

This package is covered by this SLA: https://cirello.io/sla
*/
package easy // import "cirello.io/oversight/easy"

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"

	"cirello.io/oversight"
)

type ctxKey int

const treeName ctxKey = 0

var (
	// ErrNoTreeAttached means that the given context has not been wrapped
	// with WithContext, and thus this package cannot detect which oversight
	// tree you are referring to.
	ErrNoTreeAttached = errors.New("no oversight tree attached to context")

	mu    sync.Mutex
	trees map[string]*oversight.Tree // map of name to oversight.Tree
)

// Add inserts a supervised function to the attached tree, it launches
// automatically. If the context is not correctly prepared, it returns an
// ErrNoTreeAttached error. The restart policy is Permanent.
func Add(ctx context.Context, f oversight.ChildProcess, opts ...Option) (string, error) {
	name, ok := extractName(ctx)
	if !ok {
		return "", ErrNoTreeAttached
	}
	mu.Lock()
	if trees == nil {
		trees = make(map[string]*oversight.Tree)
	}
	svr, ok := trees[name]
	mu.Unlock()
	if !ok {
		panic("oversight tree not found")
	}

	spec := oversight.ChildProcessSpecification{
		Name:    name,
		Restart: oversight.Permanent(),
		Start:   f,
	}
	for _, opt := range opts {
		opt(&spec)
	}
	svr.Add(spec)
	return name, nil
}

// Option reconfigures the attachment of a process to the context tree.
type Option func(*oversight.ChildProcessSpecification)

var (
	// Permanent services are always restarted.
	Permanent = oversight.Permanent

	// Transient services are restarted only when panic.
	Transient = oversight.Transient

	// Temporary services are never restarted.
	Temporary = oversight.Temporary
)

// RestartWith changes the restart policy for the process.
func RestartWith(restart oversight.Restart) Option {
	return func(spec *oversight.ChildProcessSpecification) {
		spec.Restart = restart
	}
}

// Delete stops and removes the given service from the attached tree. If
// the context is not correctly prepared, it returns an ErrNoTreeAttached
// error
func Delete(ctx context.Context, name string) error {
	name, ok := extractName(ctx)
	if !ok {
		return ErrNoTreeAttached
	}
	mu.Lock()
	svr, ok := trees[name]
	mu.Unlock()
	if !ok {
		panic("oversight tree not found")
	}
	svr.Delete(name)
	return nil
}

// WithContext takes a context and prepare it to be used by easy oversight tree
// package. Internally, it creates an oversight tree in OneForAll mode.
func WithContext(ctx context.Context, opts ...TreeOption) context.Context {
	chosenName := fmt.Sprintf("tree-%d", rand.Uint64())
	baseOpts := append([]TreeOption{
		oversight.WithRestartStrategy(oversight.OneForAll()),
		oversight.NeverHalt(),
	}, opts...)
	tree := oversight.New(baseOpts...)

	mu.Lock()
	trees[chosenName] = tree
	mu.Unlock()

	wrapped := context.WithValue(ctx, treeName, chosenName)
	go tree.Start(wrapped)
	return wrapped
}

// WithLogger attaches a log function to the oversight tree.
func WithLogger(logger Logger) TreeOption {
	return oversight.WithLogger(logger)
}

// Logger defines the interface for any logging facility to be compatible with
// oversight trees.
type Logger = oversight.Logger

// TreeOption are applied to change the behavior of a Tree.
type TreeOption = oversight.TreeOption

func extractName(ctx context.Context) (string, bool) {
	name, ok := ctx.Value(treeName).(string)
	return name, ok
}
