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
	"errors"
	"slices"
	"testing"
	"time"
)

func Test_uniqueName(t *testing.T) {
	tree := New(
		Process(ChildProcessSpecification{Name: "alpha", Start: func(ctx context.Context) error { return nil }}),
		Process(ChildProcessSpecification{Name: "alpha", Start: func(ctx context.Context) error { return nil }}),
	)
	seenNames := make(map[string]struct{})
	for _, p := range tree.children {
		if _, ok := seenNames[p.spec.Name]; ok {
			t.Fatal("unique process name logic has failed")
		}
	}
}

func Test_invalidChildProcessSpecification(t *testing.T) {
	var foundPanic bool
	func() {
		defer func() {
			if r := recover(); r != nil {
				foundPanic = true
			}
		}()
		New(Process(ChildProcessSpecification{}))
	}()
	if !foundPanic {
		t.Error("invalid child process specification should trigger have triggered a panic")
	}
}

func Test_automaticPruning(t *testing.T) {
	t.Run("permanent", func(t *testing.T) {
		tree := New(
			NeverHalt(),
			AutomaticPrune(),
		)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		t.Cleanup(cancel)
		go tree.Start(ctx)
		tick := make(chan struct{})
		tree.Add(ChildProcessSpecification{
			Name:    "permanent-always-fail",
			Restart: Permanent(),
			Start: func(ctx context.Context) error {
				<-tick
				return errors.New("always error")
			},
		})
		tick <- struct{}{}
		if len(tree.Children()) != 1 {
			t.Fatal("permanent processes must not be ever pruned")
		}
		tick <- struct{}{}
		if len(tree.Children()) != 1 {
			t.Fatal("permanent processes must not be ever pruned")
		}
	})
	t.Run("temporary/success", func(t *testing.T) {
		tree := New(
			NeverHalt(),
			AutomaticPrune(),
			WithRestartStrategy(OneForAll()),
		)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		t.Cleanup(cancel)
		go tree.Start(ctx)
		cycled := make(chan struct{}, 1)
		tree.Add(ChildProcessSpecification{
			Name:    "permanent-ignore",
			Restart: Permanent(),
			Start: func(ctx context.Context) error {
				<-ctx.Done()
				cycled <- struct{}{}
				return nil
			},
			Shutdown: Infinity(),
		})
		tree.Add(ChildProcessSpecification{
			Name:    "temporary",
			Restart: Temporary(),
			Start: func(ctx context.Context) error {
				return nil
			},
			Shutdown: Infinity(),
		})
		<-cycled
		children := tree.Children()
		idx := slices.IndexFunc(children, func(p State) bool { return p.Name == "temporary" })
		if idx != -1 {
			t.Fatalf("success temporary processes must always be pruned: %#v", children[idx])
		}
	})
	t.Run("temporary/failure", func(t *testing.T) {
		tree := New(
			NeverHalt(),
			AutomaticPrune(),
			WithRestartStrategy(OneForAll()),
		)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		t.Cleanup(cancel)
		go tree.Start(ctx)
		cycled := make(chan struct{}, 1)
		tree.Add(ChildProcessSpecification{
			Name:    "permanent-ignore",
			Restart: Permanent(),
			Start: func(ctx context.Context) error {
				<-ctx.Done()
				cycled <- struct{}{}
				return nil
			},
			Shutdown: Infinity(),
		})
		tree.Add(ChildProcessSpecification{
			Name:    "temporary",
			Restart: Temporary(),
			Start: func(ctx context.Context) error {
				return errors.New("error")
			},
			Shutdown: Infinity(),
		})
		<-cycled
		children := tree.Children()
		idx := slices.IndexFunc(children, func(p State) bool { return p.Name == "temporary" })
		if idx != -1 {
			t.Fatalf("failed temporary processes must always be pruned: %#v", children[idx])
		}
	})
	t.Run("transient", func(t *testing.T) {
		tree := New(
			NeverHalt(),
			AutomaticPrune(),
			WithRestartStrategy(OneForAll()),
		)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		t.Cleanup(cancel)
		go tree.Start(ctx)
		cycled := make(chan struct{}, 1)
		tree.Add(ChildProcessSpecification{
			Name:    "permanent-ignore",
			Restart: Permanent(),
			Start: func(ctx context.Context) error {
				<-ctx.Done()
				cycled <- struct{}{}
				return nil
			},
			Shutdown: Infinity(),
		})
		errs := make(chan error)
		tree.Add(ChildProcessSpecification{
			Name:    "transient",
			Restart: Transient(),
			Start: func(ctx context.Context) error {
				return <-errs
			},
			Shutdown: Infinity(),
		})
		{
			errs <- errors.New("error")
			<-cycled
			children := tree.Children()
			idx := slices.IndexFunc(children, func(p State) bool { return p.Name == "transient" })
			if idx == -1 {
				t.Fatal("failed transient processes must not be pruned")
			}
		}
		{
			errs <- nil
			<-cycled
			children := tree.Children()
			idx := slices.IndexFunc(children, func(p State) bool { return p.Name == "transient" })
			if idx != -1 {
				t.Fatal("successful transient processes must be pruned", children[idx])
			}
		}
	})
}
