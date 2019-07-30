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

package oversight_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"cirello.io/oversight"
)

// ExampleTree_singlePermanent shows how to create a static tree of permanent
// child processes.
func ExampleTree_singlePermanent() {
	supervise := oversight.New(
		oversight.Processes(func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(time.Second):
				fmt.Println(1)
			}
			return nil
		}),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := supervise.Start(ctx)
	if err != nil {
		fmt.Println(err)
	}

	// Output:
	// 1
	// 1
	// too many failures
}

func TestTree_childProcessRestarts(t *testing.T) {
	t.Parallel()
	t.Run("permanent", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		const expectedRuns = 2
		totalRuns := 0
		supervise := oversight.New(
			oversight.Process(oversight.ChildProcessSpecification{
				Restart: oversight.Permanent(),
				Start: func(ctx context.Context) error {
					select {
					case <-ctx.Done():
						return nil
					default:
						totalRuns++
						if totalRuns == expectedRuns {
							cancel()
						}
					}
					return errors.New("finished")
				},
			}),
		)
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			supervise.Start(ctx)
		}()
		wg.Wait()
		if ctx.Err() == context.DeadlineExceeded {
			t.Error("should never reach deadline exceeded")
		}
		if totalRuns != expectedRuns {
			t.Log(totalRuns, expectedRuns)
			t.Error("oversight did not restart permanent service the right amount of times")
		}
	})

	t.Run("transient", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		const expectedRuns = 2
		totalRuns := 0
		supervise := oversight.New(
			oversight.Process(oversight.ChildProcessSpecification{
				Restart: oversight.Transient(),
				Start: func(ctx context.Context) error {
					var err error
					if totalRuns == 0 {
						err = errors.New("finished")
					}
					select {
					case <-ctx.Done():
						return nil
					default:
						totalRuns++
						if totalRuns == expectedRuns {
							cancel()
						}
					}
					return err
				},
			}),
		)
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			supervise.Start(ctx)
		}()
		wg.Wait()
		if ctx.Err() == context.DeadlineExceeded {
			t.Error("should never reach deadline exceeded")
		}
		if totalRuns != expectedRuns {
			t.Log(totalRuns, expectedRuns)
			t.Error("oversight did not restart transient service the right amount of times")
		}
	})

	t.Run("temporary", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		const unexpectedRuns = 2
		totalRuns := 0
		supervise := oversight.New(
			oversight.Process(
				oversight.ChildProcessSpecification{
					Restart: oversight.Temporary(),
					Start: func(ctx context.Context) error {
						var err error
						if totalRuns == 0 {
							err = errors.New("finished")
						}
						select {
						case <-ctx.Done():
							return nil
						default:
							totalRuns++
							t.Log("run:", totalRuns)
							cancel()
						}
						return err
					},
				},
				oversight.ChildProcessSpecification{
					Restart: oversight.Permanent(),
					Start: func(context.Context) error {
						t.Log("ping...")
						time.Sleep(500 * time.Millisecond)
						return nil
					},
				},
			),
		)
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			supervise.Start(ctx)
		}()
		wg.Wait()
		if totalRuns >= unexpectedRuns || totalRuns == 0 {
			t.Log(totalRuns, unexpectedRuns)
			t.Error("oversight did not restart temporary service the right amount of times")
		}
	})
}

func TestTree_treeRestarts(t *testing.T) {
	t.Parallel()
	t.Run("oneForOne", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		badSiblingRuns := 0
		goodSiblingRuns := 0
		supervise := oversight.New(
			oversight.WithSpecification(2, 1*time.Second, oversight.OneForOne()),
			oversight.Process(
				oversight.ChildProcessSpecification{
					Restart: oversight.Permanent(),
					Start: func(ctx context.Context) error {
						t.Log("failed process should always restart...")
						badSiblingRuns++
						if badSiblingRuns >= 2 {
							cancel()
						}
						return errors.New("finished")
					},
				},
				oversight.ChildProcessSpecification{
					Restart: oversight.Permanent(),
					Start: func(ctx context.Context) error {
						t.Log("but sibling must stay untouched")
						goodSiblingRuns++
						select {
						case <-ctx.Done():
						}
						return nil
					},
				},
			),
		)
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			supervise.Start(ctx)
		}()
		wg.Wait()
		if ctx.Err() == context.DeadlineExceeded {
			t.Error("should never reach deadline exceeded")
		}
		if badSiblingRuns < 2 {
			t.Error("the test did not run long enough")
		}
		if goodSiblingRuns != 1 {
			t.Error("oneForOne should not terminate siblings")
		}
	})
	t.Run("oneForAll", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		badSiblingRuns := 0
		goodSiblingRuns := 0
		supervise := oversight.New(
			oversight.WithSpecification(2, 1*time.Second, oversight.OneForAll()),
			oversight.Process(
				oversight.ChildProcessSpecification{
					Restart: oversight.Permanent(),
					Start: func(ctx context.Context) error {
						t.Log("failed process should always restart...")
						badSiblingRuns++
						if badSiblingRuns >= 2 {
							cancel()
						}
						return errors.New("finished")
					},
				},
				oversight.ChildProcessSpecification{
					Restart: oversight.Permanent(),
					Start: func(ctx context.Context) error {
						t.Log("and the sibling must restart too")
						goodSiblingRuns++
						select {
						case <-ctx.Done():
						}
						return nil
					},
				},
			),
		)
		err := supervise.Start(ctx)
		if err != nil {
			t.Error(err)
		}
		if ctx.Err() == context.DeadlineExceeded {
			t.Error("should never reach deadline exceeded")
		}
		if badSiblingRuns < 2 {
			t.Error("the test did not run long enough")
		}
		if goodSiblingRuns != badSiblingRuns {
			t.Error("oneForAll should always terminate siblings")
		}
	})
	t.Run("restForOne", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		firstSiblingRuns := 0
		badSiblingRuns := 0
		goodSiblingRuns := 0
		supervise := oversight.New(
			oversight.WithSpecification(2, 1*time.Second, oversight.RestForOne()),
			oversight.Process(
				oversight.ChildProcessSpecification{
					Restart: oversight.Permanent(),
					Start: func(ctx context.Context) error {
						t.Log("first sibling should never die")
						firstSiblingRuns++
						select {
						case <-ctx.Done():
						}
						return nil
					},
				},
				oversight.ChildProcessSpecification{
					Restart: oversight.Permanent(),
					Start: func(ctx context.Context) error {
						t.Log("failed process should always restart...")
						badSiblingRuns++
						if badSiblingRuns >= 2 {
							cancel()
						}
						return errors.New("finished")
					},
				},
				oversight.ChildProcessSpecification{
					Restart: oversight.Permanent(),
					Start: func(ctx context.Context) error {
						t.Log("and the younger siblings too")
						goodSiblingRuns++
						select {
						case <-ctx.Done():
						}
						return nil
					},
				},
			),
		)
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			supervise.Start(ctx)
		}()
		wg.Wait()
		if ctx.Err() == context.DeadlineExceeded {
			t.Error("should never reach deadline exceeded")
		}
		if badSiblingRuns < 2 {
			t.Error("the test did not run long enough")
		}
		if goodSiblingRuns != badSiblingRuns {
			t.Error("restForOne should always terminate younger siblings")
		}
		if firstSiblingRuns != 1 {
			t.Error("restForOne should never terminate older siblings")
		}
	})
}

func Test_nestedTree(t *testing.T) {
	t.Parallel()
	var (
		leafMu    sync.Mutex
		leafCount = 0
	)
	leaf := oversight.New(oversight.Processes(
		func(ctx context.Context) error {
			for {
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(500 * time.Millisecond):
					leafMu.Lock()
					leafCount++
					leafMu.Unlock()
				}
			}
		},
	))
	var (
		rootMu    sync.Mutex
		rootCount = 0
	)
	root := oversight.New(
		oversight.WithTree(leaf),
		oversight.Processes(
			func(ctx context.Context) error {
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(1 * time.Second):
					rootMu.Lock()
					rootCount++
					rootMu.Unlock()
				}
				return nil
			},
		),
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	root.Start(ctx)
	leafMu.Lock()
	lc := leafCount
	leafMu.Unlock()
	rootMu.Lock()
	rc := rootCount
	rootMu.Unlock()
	if lc == 0 || rc == 0 {
		t.Error("tree did not run")
	} else if leafCount == 0 {
		t.Error("subtree did not run")
	}
}

func Test_dynamicChild(t *testing.T) {
	t.Parallel()
	tempExecCount := 0
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var o oversight.Tree
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := o.Start(ctx); err != nil {
			t.Log(err)
		}
	}()
	// proves that the clockwork waits for the first child process.
	time.Sleep(1 * time.Second)
	o.Add(oversight.ChildProcessSpecification{
		Restart: oversight.Temporary(),
		Start: func(context.Context) error {
			tempExecCount++
			return nil
		},
	})
	wg.Wait()
	if tempExecCount == 0 {
		t.Error("dynamic child process did not start")
	}
}

func Test_customLogger(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	supervise := oversight.New(
		oversight.WithLogger(logger),
		oversight.Processes(
			func(ctx context.Context) error {
				cancel()
				return nil
			},
		),
	)
	supervise.Start(ctx)
	content := buf.String()
	expectedLog := strings.Contains(content, "child started") && strings.Contains(content, "child done")
	if !expectedLog {
		t.Log(content)
		t.Error("the logger did not log the expected lines")
	}
}

func Test_childProcTimeout(t *testing.T) {
	t.Parallel()
	blockedCtx, blockedCancel := context.WithCancel(context.Background())
	defer blockedCancel()
	started := make(chan struct{})
	supervise := oversight.New(
		oversight.Process(oversight.ChildProcessSpecification{
			Name:    "timed out childproc",
			Restart: oversight.Temporary(),
			Start: func(ctx context.Context) error {
				started <- struct{}{}
				t.Log("started")
				select {
				case <-ctx.Done():
				}
				t.Log("tree stop signal received")
				select {
				case <-blockedCtx.Done():
				}
				return nil
			},
			Shutdown: oversight.Timeout(2 * time.Second),
		}),
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer t.Log("tree stopped")
		err := supervise.Start(ctx)
		if err != nil {
			t.Log(err)
		}
	}()
	go func() {
		<-started
		t.Log("stopping oversight tree")
		cancel()
	}()

	completed := make(chan struct{})
	go func() {
		wg.Wait()
		close(completed)
	}()

	select {
	case <-completed:
		t.Log("tree has completed")
	case <-time.After(10 * time.Second):
		t.Error("tree is not honoring detach timeout")
	}
}

func Test_terminateChildProc(t *testing.T) {
	t.Parallel()
	t.Run("simple terminate", func(t *testing.T) {
		var processTerminated bool
		processStarted := make(chan struct{})
		tree := oversight.New(
			oversight.Process(
				oversight.ChildProcessSpecification{
					Restart: oversight.Temporary(),
					Name:    "alpha",
					Start: func(ctx context.Context) error {
						close(processStarted)
						t.Log("started")
						defer t.Log("stopped")
						select {
						case <-ctx.Done():
							processTerminated = true
						}
						return nil
					},
				},
			),
		)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		var expectedError error
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer t.Log("tree stopped")
			err := tree.Start(ctx)
			if err != nil {
				expectedError = err
				t.Log(err)
			}
		}()
		<-processStarted
		if err := tree.Terminate("alpha"); err != nil {
			t.Fatalf("termination call failed: %v", err)
		}
		t.Log("alpha terminated")
		wg.Wait()

		if !processTerminated {
			t.Error("terminated command did not run")
		}
		if expectedError != oversight.ErrNoChildProcessLeft {
			t.Error("tree should have acknowledged that there was nothing else running")
		}
	})
	t.Run("duplicate terminate", func(t *testing.T) {
		processStarted := make(chan struct{})
		tree := oversight.New(
			oversight.Process(
				oversight.ChildProcessSpecification{
					Restart: oversight.Temporary(),
					Name:    "alpha",
					Start: func(ctx context.Context) error {
						close(processStarted)
						t.Log("started")
						defer t.Log("stopped")
						select {
						case <-ctx.Done():
						}
						return nil
					},
				},
				oversight.ChildProcessSpecification{
					Restart: oversight.Permanent(),
					Name:    "beta",
					Start: func(ctx context.Context) error {
						<-ctx.Done()
						return nil
					},
				},
			),
		)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			tree.Start(ctx)
		}()
		<-processStarted
		if err := tree.Terminate("alpha"); err != nil {
			t.Fatalf("termination call failed: %v", err)
		}
		t.Log("alpha terminated")
		if err := tree.Terminate("alpha"); err == nil {
			t.Fatalf("termination call should have failed: %v", err)
		} else {
			t.Log("alpha terminated again:", err)
		}
	})
}

func Test_deleteChildProc(t *testing.T) {
	t.Parallel()
	processStarted := make(chan struct{})
	tree := oversight.New(
		oversight.Process(
			oversight.ChildProcessSpecification{
				Restart: oversight.Temporary(),
				Name:    "alpha",
				Start: func(ctx context.Context) error {
					t.Log("alpha started")
					defer t.Log("alpha stopped")
					select {
					case <-ctx.Done():
					}
					return nil
				},
			},
			oversight.ChildProcessSpecification{
				Restart: oversight.Temporary(),
				Name:    "beta",
				Start: func(ctx context.Context) error {
					close(processStarted)
					t.Log("beta started")
					defer t.Log("beta stopped")
					select {
					case <-ctx.Done():
					}
					return nil
				},
			},
		),
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer t.Log("tree stopped")
		err := tree.Start(ctx)
		if err != nil {
			t.Log(err)
		}
	}()
	<-processStarted
	if err := tree.Delete("alpha"); err != nil {
		t.Fatalf("deletion call failed: %v", err)
	}
	t.Log("alpha deleted")
	if err := tree.Delete("alpha"); err == nil {
		t.Error("deletion call should have failed")
	}
	cancel()
	wg.Wait()
}

func Test_currentChildren(t *testing.T) {
	t.Parallel()
	childProcStarted := make(chan struct{})
	tree := oversight.New(
		oversight.Process(oversight.ChildProcessSpecification{
			Restart: oversight.Permanent(),
			Name:    "alpha",
			Start: func(ctx context.Context) error {
				close(childProcStarted)
				select {
				case <-ctx.Done():
				}
				return nil
			},
		}),
	)
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wg.Add(1)
	go func() {
		defer wg.Done()
		tree.Start(ctx)
	}()
	<-childProcStarted
	var foundAlphaRunning bool
	for _, childproc := range tree.Children() {
		if childproc.Name == "alpha" && childproc.State == "running" {
			foundAlphaRunning = true
			break
		}
	}
	if !foundAlphaRunning {
		t.Error("did not find alpha running")
	}
	cancel()
	wg.Wait()
	var foundAlphaFailed bool
	for _, childproc := range tree.Children() {
		if childproc.Name == "alpha" && childproc.State == "failed" {
			foundAlphaFailed = true
			break
		}
	}
	if !foundAlphaFailed {
		t.Error("did not find alpha failed")
	}
}

func Test_operationsOnDeadTree(t *testing.T) {
	t.Parallel()
	tree := oversight.New(
		oversight.Process(oversight.ChildProcessSpecification{
			Restart: oversight.Permanent(),
			Name:    "alpha",
			Start:   func(ctx context.Context) error { return nil },
		}),
	)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	tree.Start(ctx)
	err := tree.Add(oversight.ChildProcessSpecification{
		Name:  "beta",
		Start: func(context.Context) error { return nil },
	})
	if err != oversight.ErrTreeNotRunning {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := tree.Delete("alpha"); err != oversight.ErrTreeNotRunning {
		t.Fatalf("unexpected error: %v", err)
	}
}

func Test_invalidTreeConfiguration(t *testing.T) {
	specs := []struct {
		maxR int
		maxT time.Duration
	}{
		{maxR: -2, maxT: -1},
		{maxR: -1, maxT: -1},
		{maxR: 0, maxT: -1},
	}
	for _, spec := range specs {
		tree := oversight.New(oversight.WithRestartIntensity(spec.maxR, spec.maxT))
		if err := tree.Start(context.Background()); err != oversight.ErrInvalidConfiguration {
			t.Errorf("unexpected error for an invalid configuration: %v", err)
		}
		if err := tree.Add(func(context.Context) error { return nil }); err != oversight.ErrTreeNotRunning {
			t.Errorf("unexpected error for an Add() operations on a badly configured tree: %v", err)
		}
		if err := tree.Delete("404"); err != oversight.ErrTreeNotRunning {
			t.Errorf("unexpected error for a Delete() operations on a badly configured tree: %v", err)
		}
	}
}

func Test_multipleAdd(t *testing.T) {
	t.Parallel()
	var tree oversight.Tree
	var f oversight.ChildProcess = func(context.Context) error { return nil }
	children := []struct {
		name          string
		f             interface{}
		expectedError error
	}{
		{"childProcessSpecification", oversight.ChildProcessSpecification{Start: f}, nil},
		{"abstract childProcess", f, nil},
		{"concrete childProcess", func(context.Context) error { return nil }, nil},
		{"tree", &oversight.Tree{}, nil},
		{"invalid", func() {}, oversight.ErrInvalidChildProcessType},
	}
	for _, child := range children {
		err := tree.Add(child.f)
		if err != child.expectedError {
			t.Errorf("unexpected error found for %v: %v (expected: %v)",
				child.name, err, child.expectedError)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	tree.Start(ctx)
}

func TestTree_shutdownOrder(t *testing.T) {
	t.Parallel()
	t.Run("oneForOne", func(t *testing.T) {
		var (
			mu     sync.Mutex
			order  []int
			wgFunc sync.WaitGroup
		)
		wgFunc.Add(4)
		f := func(i int) oversight.ChildProcess {
			return func(ctx context.Context) error {
				wgFunc.Done()
				<-ctx.Done()
				time.Sleep(time.Duration(i) * 100 * time.Millisecond)
				mu.Lock()
				order = append(order, i)
				mu.Unlock()
				return nil
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var buf bytes.Buffer
		logger := log.New(&buf, "", 0)
		supervise := oversight.New(
			oversight.WithLogger(logger),
			oversight.NeverHalt(),
			oversight.WithRestartStrategy(oversight.OneForOne()),
			oversight.Processes(
				f(1), f(2), f(3), f(4),
				func(ctx context.Context) error {
					wgFunc.Wait()
					<-ctx.Done()
					return nil
				},
			),
		)
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			supervise.Start(ctx)
		}()
		wgFunc.Wait()
		cancel()
		wg.Wait()
		if ctx.Err() == context.DeadlineExceeded {
			t.Error("should never reach deadline exceeded")
		}
		if !sort.IntsAreSorted(order) {
			t.Errorf("OneForOne not shutting down in reverse start order: %v", order)
			t.Log("log:", buf.String())
		}
	})
	t.Run("oneForAll", func(t *testing.T) {
		var (
			mu     sync.Mutex
			order  []int
			wgFunc sync.WaitGroup
		)
		wgFunc.Add(4)
		f := func(i int) oversight.ChildProcess {
			return func(ctx context.Context) error {
				wgFunc.Done()
				<-ctx.Done()
				time.Sleep(time.Duration(i) * 100 * time.Millisecond)
				mu.Lock()
				order = append(order, i)
				mu.Unlock()
				return nil
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var buf bytes.Buffer
		logger := log.New(&buf, "", 0)
		supervise := oversight.New(
			oversight.WithLogger(logger),
			oversight.NeverHalt(),
			oversight.WithRestartStrategy(oversight.OneForAll()),
			oversight.Processes(
				f(1), f(2), f(3), f(4),
				func(ctx context.Context) error {
					wgFunc.Wait()
					<-ctx.Done()
					return nil
				},
			),
		)
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			supervise.Start(ctx)
		}()
		wgFunc.Wait()
		cancel()
		wg.Wait()
		if ctx.Err() == context.DeadlineExceeded {
			t.Error("should never reach deadline exceeded")
		}
		if !sort.IntsAreSorted(order) {
			t.Errorf("OneForAll not shutting down in reverse start order: %v", order)
			t.Log("log:", buf.String())
		}
	})
	t.Run("restForOne", func(t *testing.T) {
		var (
			mu     sync.Mutex
			order  []int
			wgFunc sync.WaitGroup
		)
		wgFunc.Add(4)
		f := func(i int) oversight.ChildProcess {
			return func(ctx context.Context) error {
				wgFunc.Done()
				<-ctx.Done()
				time.Sleep(time.Duration(i) * 100 * time.Millisecond)
				mu.Lock()
				order = append(order, i)
				mu.Unlock()
				return nil
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var buf bytes.Buffer
		logger := log.New(&buf, "", 0)
		supervise := oversight.New(
			oversight.WithLogger(logger),
			oversight.NeverHalt(),
			oversight.WithRestartStrategy(oversight.RestForOne()),
			oversight.Processes(
				f(1), f(2), f(3), f(4),
				func(ctx context.Context) error {
					wgFunc.Wait()
					<-ctx.Done()
					return nil
				},
			),
		)
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			supervise.Start(ctx)
		}()
		wgFunc.Wait()
		cancel()
		wg.Wait()
		if ctx.Err() == context.DeadlineExceeded {
			t.Error("should never reach deadline exceeded")
		}
		if !sort.IntsAreSorted(order) {
			t.Errorf("RestForOne not shutting down in reverse start order: %v", order)
			t.Log("log:", buf.String())
		}
	})
	t.Run("simpleOneForOne", func(t *testing.T) {
		var (
			mu     sync.Mutex
			order  []int
			wgFunc sync.WaitGroup
		)
		wgFunc.Add(4)
		f := func(i int) oversight.ChildProcess {
			return func(ctx context.Context) error {
				wgFunc.Done()
				<-ctx.Done()
				time.Sleep(time.Duration(i) * 100 * time.Millisecond)
				mu.Lock()
				order = append(order, i)
				mu.Unlock()
				return nil
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var buf bytes.Buffer
		logger := log.New(&buf, "", 0)
		supervise := oversight.New(
			oversight.WithLogger(logger),
			oversight.NeverHalt(),
			oversight.WithRestartStrategy(oversight.SimpleOneForOne()),
			oversight.Processes(
				f(1), f(2), f(3), f(4),
				func(ctx context.Context) error {
					wgFunc.Wait()
					<-ctx.Done()
					return nil
				},
			),
		)
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			supervise.Start(ctx)
		}()
		wgFunc.Wait()
		cancel()
		wg.Wait()
		if ctx.Err() == context.DeadlineExceeded {
			t.Error("should never reach deadline exceeded")
		}
		mu.Lock()
		lenOrder := len(order)
		mu.Unlock()
		if lenOrder == 4 {
			t.Errorf("SimpleOneForOne should not wait for goroutines: %v", order)
			t.Log("log:", buf.String())
		}
	})
}

func Test_neverHalt(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	restarts := 0
	supervise := oversight.New(
		oversight.NeverHalt(),
		oversight.Processes(
			func(ctx context.Context) error {
				if restarts >= 10 {
					cancel()
					return nil
				}
				restarts++
				return nil
			},
		),
	)
	err := supervise.Start(ctx)
	if err == oversight.ErrTooManyFailures {
		t.Fatal("should not see ErrTooManyFailures")
	}
}

func Test_aliases(t *testing.T) {
	t.Parallel()
	t.Run("DefaultRestartIntensity", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		restarts := 0
		supervise := oversight.New(
			oversight.DefaultRestartIntensity(),
			oversight.Processes(
				func(ctx context.Context) error {
					if restarts >= 10 {
						cancel()
						return nil
					}
					restarts++
					return nil
				},
			),
		)
		err := supervise.Start(ctx)
		if err != oversight.ErrTooManyFailures {
			t.Fatal("expected error missing:", err)
		}
	})
	t.Run("WithRestartIntensity", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		restarts := 0
		supervise := oversight.New(
			oversight.WithRestartIntensity(oversight.DefaultMaxR, oversight.DefaultMaxT),
			oversight.Processes(
				func(ctx context.Context) error {
					if restarts >= 10 {
						cancel()
						return nil
					}
					restarts++
					return nil
				},
			),
		)
		err := supervise.Start(ctx)
		if err != oversight.ErrTooManyFailures {
			t.Fatal("expected error missing:", err)
		}
	})
}
