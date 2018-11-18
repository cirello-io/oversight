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
		oversight.Add(ctx, func(ctx context.Context) {
			// ...
		})
	}
*/
package easy // import "cirello.io/oversight/easy"

import (
	"context"
	"errors"
	"fmt"
	"log"
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

	mu          sync.Mutex
	supervisors map[string]*oversight.Tree // map of name to oversight.Tree
)

func init() {
	supervisors = make(map[string]*oversight.Tree)
}

// Add inserts a supervised function to the attached tree, it launches
// automatically. If the context is not correctly prepared, it returns an
// ErrNoTreeAttached error. The restart policy is Permanent.
func Add(ctx context.Context, f oversight.ChildProcess) (string, error) {
	name, ok := extractName(ctx)
	if !ok {
		return "", ErrNoTreeAttached
	}
	mu.Lock()
	svr, ok := supervisors[name]
	mu.Unlock()
	if !ok {
		panic("supervisor not found")
	}
	svr.Add(oversight.ChildProcessSpecification{
		Name:    name,
		Restart: oversight.Permanent(),
		Start:   f,
	})
	return name, nil
}

// Delete stops and removes the given service from the attached supervisor. If
// the context is not correctly prepared, it returns an ErrNoTreeAttached
// error
func Delete(ctx context.Context, name string) error {
	name, ok := extractName(ctx)
	if !ok {
		return ErrNoTreeAttached
	}
	mu.Lock()
	svr, ok := supervisors[name]
	mu.Unlock()
	if !ok {
		panic("supervisor not found")
	}
	svr.Delete(name)
	return nil
}

// WithContext takes a context and prepare it to be used by easy supervisor
// package. Internally, it creates a supervisor in OneForAll mode.
func WithContext(ctx context.Context, opts ...oversight.TreeOption) context.Context {
	chosenName := fmt.Sprintf("supervisor-%d", rand.Uint64())
	baseOpts := append([]oversight.TreeOption{
		oversight.WithRestartStrategy(oversight.OneForAll()),
		oversight.NeverHalt(),
	}, opts...)
	tree := oversight.New(baseOpts...)

	mu.Lock()
	supervisors[chosenName] = tree
	mu.Unlock()

	wrapped := context.WithValue(ctx, treeName, chosenName)
	go tree.Start(wrapped)
	return wrapped
}

// WithLogger attaches a log function to the supervisor
func WithLogger(logger *log.Logger) oversight.TreeOption {
	return oversight.WithLogger(logger)
}

func extractName(ctx context.Context) (string, bool) {
	name, ok := ctx.Value(treeName).(string)
	return name, ok
}
