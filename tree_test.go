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

package oversight_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"testing"
	"time"

	"cirello.io/oversight/v2"
)

// ExampleTree_singlePermanent shows how to create a static tree of permanent
// child processes.
func ExampleTree_singlePermanent() {
	var tree oversight.Tree
	err := tree.Add(
		func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(time.Second):
				fmt.Println(1)
			}
			return nil
		},
		oversight.Permanent(),
		oversight.Natural(),
		"childProcess",
	)
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fmt.Println(tree.Start(ctx))

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
		var tree oversight.Tree
		tree.Add(
			func(ctx context.Context) error {
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
			oversight.Permanent(),
			oversight.Timeout(5*time.Second),
			"permanentProc",
		)
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			tree.Start(ctx)
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
		var tree oversight.Tree
		err := tree.Add(
			func(ctx context.Context) error {
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
			oversight.Transient(),
			oversight.Timeout(5*time.Second),
			"transientProc",
		)
		if err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			tree.Start(ctx)
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
		var tree oversight.Tree
		err := tree.Add(
			func(ctx context.Context) error {
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
			oversight.Temporary(),
			oversight.Timeout(5*time.Second),
			"temporaryProc",
		)
		if err != nil {
			t.Fatal(err)
		}
		err = tree.Add(
			func(context.Context) error {
				t.Log("ping...")
				time.Sleep(500 * time.Millisecond)
				return nil
			},
			oversight.Permanent(),
			oversight.Timeout(5*time.Second),
			"pingProc",
		)
		if err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			tree.Start(ctx)
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
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		badSiblingRuns := 0
		goodSiblingRuns := 0
		tree := oversight.New(oversight.WithSpecification(2, 1*time.Second, oversight.OneForOne()))
		err := tree.Add(
			func(ctx context.Context) error {
				t.Log("failed process should always restart...")
				badSiblingRuns++
				if badSiblingRuns >= 2 {
					cancel()
				}
				return errors.New("finished")
			},
			oversight.Permanent(),
			oversight.Natural(),
			"first-child",
		)
		if err != nil {
			t.Fatal(err)
		}
		err = tree.Add(
			func(ctx context.Context) error {
				t.Log("but sibling must stay untouched")
				goodSiblingRuns++
				<-ctx.Done()
				return nil
			},
			oversight.Permanent(),
			oversight.Natural(),
			"second-child",
		)
		if err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			tree.Start(ctx)
		}()
		wg.Wait()
		if ctx.Err() == context.DeadlineExceeded {
			t.Error("should never reach deadline exceeded")
		}
		if badSiblingRuns < 2 {
			t.Error("the test did not run long enough", badSiblingRuns)
		}
		if goodSiblingRuns != 1 {
			t.Error("oneForOne should not terminate siblings")
		}
	})
	t.Run("simpleOneForOne", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		badSiblingRuns := 0
		goodSiblingRuns := 0
		tree := oversight.New(oversight.WithSpecification(2, 1*time.Second, oversight.SimpleOneForOne()))
		err := tree.Add(
			func(ctx context.Context) error {
				t.Log("failed process should always restart...")
				badSiblingRuns++
				if badSiblingRuns >= 2 {
					cancel()
				}
				return errors.New("finished")
			},
			oversight.Permanent(),
			oversight.Natural(),
			"first-child",
		)
		if err != nil {
			t.Fatal(err)
		}
		tree.Add(
			func(ctx context.Context) error {
				t.Log("but sibling must stay untouched")
				goodSiblingRuns++
				<-ctx.Done()
				return nil
			},
			oversight.Permanent(),
			oversight.Natural(),
			"second-child",
		)
		if err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			tree.Start(ctx)
		}()
		wg.Wait()
		if ctx.Err() == context.DeadlineExceeded {
			t.Error("should never reach deadline exceeded")
		}
		if badSiblingRuns < 2 {
			t.Error("the test did not run long enough", badSiblingRuns)
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
		tree := oversight.New(oversight.WithSpecification(2, 1*time.Second, oversight.OneForAll()))
		err := tree.Add(
			func(ctx context.Context) error {
				t.Log("failed process should always restart...")
				badSiblingRuns++
				if badSiblingRuns >= 2 {
					cancel()
				}
				return errors.New("finished")
			},
			oversight.Permanent(),
			oversight.Natural(),
			"oneForAll",
		)
		if err != nil {
			t.Fatal(err)
		}
		err = tree.Add(
			func(ctx context.Context) error {
				t.Log("and the sibling must restart too")
				goodSiblingRuns++
				select {
				case <-ctx.Done():
				}
				return nil
			},
			oversight.Permanent(),
			oversight.Natural(),
			"oneForAll2",
		)
		if err != nil {
			t.Fatal(err)
		}
		if err := tree.Start(ctx); err != nil {
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
		tree := oversight.New(oversight.WithSpecification(2, 1*time.Second, oversight.RestForOne()))
		err := tree.Add(
			func(ctx context.Context) error {
				t.Log("first sibling should never die")
				firstSiblingRuns++
				select {
				case <-ctx.Done():
				}
				return nil
			},
			oversight.Permanent(),
			oversight.Natural(),
			"restForOne",
		)
		if err != nil {
			t.Fatal(err)
		}
		err = tree.Add(
			func(ctx context.Context) error {
				t.Log("failed process should always restart...")
				badSiblingRuns++
				if badSiblingRuns >= 2 {
					cancel()
				}
				return errors.New("finished")
			},
			oversight.Permanent(),
			oversight.Natural(),
			"restForOne2",
		)
		if err != nil {
			t.Fatal(err)
		}
		err = tree.Add(
			func(ctx context.Context) error {
				t.Log("and the younger siblings too")
				goodSiblingRuns++
				select {
				case <-ctx.Done():
				}
				return nil
			},
			oversight.Permanent(),
			oversight.Natural(),
			"restForOne3",
		)
		if err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			tree.Start(ctx)
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
	var leaf oversight.Tree
	leaf.Add(
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
		oversight.Permanent(),
		oversight.Natural(),
		"nestedTree",
	)
	var (
		rootMu    sync.Mutex
		rootCount = 0
	)
	var root oversight.Tree
	root.Add(leaf.Start, oversight.Permanent(), oversight.Natural(), "leaf")
	root.Add(
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
		oversight.Permanent(),
		oversight.Natural(),
		"proc",
	)
	ctx := t.Context()
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
	ctx := t.Context()
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
	o.Add(
		func(context.Context) error {
			tempExecCount++
			return nil
		},
		oversight.Temporary(),
		oversight.Natural(),
		"dynamicChild",
	)
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
	tree := oversight.New(oversight.WithLogger(logger))
	tree.Add(
		func(ctx context.Context) error {
			cancel()
			return nil
		},
		oversight.Permanent(),
		oversight.Natural(),
		"customLogger",
	)
	tree.Start(ctx)
	content := buf.String()
	expectedLog := strings.Contains(content, "child started") && strings.Contains(content, "child done")
	if !expectedLog {
		t.Log(content)
		t.Error("the logger did not log the expected lines")
	}
}

func Test_childProcTimeout(t *testing.T) {
	t.Parallel()
	blockedCtx := t.Context()
	started := make(chan struct{})
	var tree oversight.Tree
	err := tree.Add(
		func(ctx context.Context) error {
			started <- struct{}{}
			t.Log("started")
			<-ctx.Done()
			t.Log("tree stop signal received")
			<-blockedCtx.Done()
			return nil
		},
		oversight.Temporary(),
		oversight.Timeout(2*time.Second),
		"timed out childproc",
	)
	if err != nil {
		t.Fatal(err)
	}
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
		var tree oversight.Tree
		err := tree.Add(
			func(ctx context.Context) error {
				close(processStarted)
				t.Log("started")
				defer t.Log("stopped")
				select {
				case <-ctx.Done():
					processTerminated = true
				}
				return nil
			},
			oversight.Temporary(),
			oversight.Natural(),
			"alpha",
		)
		if err != nil {
			t.Fatal(err)
		}
		ctx := t.Context()
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
		var tree oversight.Tree
		err := tree.Add(
			func(ctx context.Context) error {
				close(processStarted)
				t.Log("started")
				defer t.Log("stopped")
				select {
				case <-ctx.Done():
				}
				return nil
			},
			oversight.Temporary(),
			oversight.Natural(),
			"alpha",
		)
		if err != nil {
			t.Fatal(err)
		}
		err = tree.Add(
			func(ctx context.Context) error {
				<-ctx.Done()
				return nil
			},
			oversight.Permanent(),
			oversight.Natural(),
			"beta",
		)
		if err != nil {
			t.Fatal(err)
		}
		ctx := t.Context()
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
		if err := tree.Terminate("alpha"); !errors.Is(err, oversight.ErrProcessNotRunning) {
			t.Fatalf("termination call should have failed: %v", err)
		} else {
			t.Log("alpha terminated again:", err)
		}
	})
}

func Test_deleteChildProc(t *testing.T) {
	t.Parallel()
	processStarted := make(chan struct{})
	var tree oversight.Tree
	err := tree.Add(
		func(ctx context.Context) error {
			t.Log("alpha started")
			defer t.Log("alpha stopped")
			select {
			case <-ctx.Done():
			}
			return nil
		},
		oversight.Temporary(),
		oversight.Natural(),
		"alpha",
	)
	if err != nil {
		t.Fatal(err)
	}
	err = tree.Add(
		func(ctx context.Context) error {
			close(processStarted)
			t.Log("beta started")
			defer t.Log("beta stopped")
			select {
			case <-ctx.Done():
			}
			return nil
		},
		oversight.Temporary(),
		oversight.Natural(),
		"beta",
	)
	if err != nil {
		t.Fatal(err)
	}
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
	var tree oversight.Tree
	err := tree.Add(
		func(ctx context.Context) error {
			close(childProcStarted)
			select {
			case <-ctx.Done():
			}
			return nil
		},
		oversight.Permanent(),
		oversight.Natural(),
		"alpha",
	)
	if err != nil {
		t.Fatal(err)
	}
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
	var tree oversight.Tree
	err := tree.Add(
		func(ctx context.Context) error { return nil },
		oversight.Permanent(),
		oversight.Natural(),
		"alpha",
	)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	tree.Start(ctx)
	err = tree.Add(
		func(context.Context) error { return nil },
		oversight.Permanent(),
		oversight.Natural(),
		"beta",
	)
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
		tree := oversight.New(oversight.WithMaximumRestartIntensity(spec.maxR, spec.maxT))
		if err := tree.Start(context.Background()); err != oversight.ErrInvalidConfiguration {
			t.Errorf("unexpected error for an invalid configuration: %v", err)
		}
		if err := tree.Add(func(context.Context) error { return nil }, oversight.Permanent(), oversight.Natural(), "proc"); err != oversight.ErrTreeNotRunning {
			t.Errorf("unexpected error for an Add() operations on a badly configured tree: %v", err)
		}
		if err := tree.Delete("404"); err != oversight.ErrTreeNotRunning {
			t.Errorf("unexpected error for a Delete() operations on a badly configured tree: %v", err)
		}
	}
}

func Test_neverHalt(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	restarts := 0
	tree := oversight.New(oversight.NeverHalt())
	err := tree.Add(
		func(ctx context.Context) error {
			if restarts >= 10 {
				cancel()
				return nil
			}
			restarts++
			return nil
		},
		oversight.Permanent(),
		oversight.Natural(),
		"neverHalt",
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := tree.Start(ctx); err == oversight.ErrTooManyFailures {
		t.Fatal("should not see ErrTooManyFailures")
	}
}

func Test_aliases(t *testing.T) {
	t.Parallel()
	t.Run("DefaultMaximumRestartIntensity", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		restarts := 0
		tree := oversight.New(oversight.DefaultMaximumRestartIntensity())
		err := tree.Add(
			func(ctx context.Context) error {
				if restarts >= 10 {
					cancel()
					return nil
				}
				restarts++
				return nil
			},
			oversight.Permanent(),
			oversight.Natural(),
			"DefaultMaximumRestartIntensityProc",
		)
		if err != nil {
			t.Fatal(err)
		}
		if err := tree.Start(ctx); err != oversight.ErrTooManyFailures {
			t.Fatal("expected error missing:", err)
		}
	})
	t.Run("WithRestartIntensity", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		restarts := 0
		tree := oversight.New(oversight.WithMaximumRestartIntensity(oversight.DefaultMaxR, oversight.DefaultMaxT))
		err := tree.Add(
			func(ctx context.Context) error {
				if restarts >= 10 {
					cancel()
					return nil
				}
				restarts++
				return nil
			},
			oversight.Permanent(),
			oversight.Natural(),
			"WithRestartIntensityProc",
		)
		if err != nil {
			t.Fatal(err)
		}
		if err := tree.Start(ctx); err != oversight.ErrTooManyFailures {
			t.Fatal("expected error missing:", err)
		}
	})
}

func TestPanicDoubleStart(t *testing.T) {
	t.Parallel()
	var tree oversight.Tree
	oversight.NeverHalt()(&tree)
	ctx, cancel := context.WithCancel(context.Background())
	err := tree.Add(
		func(context.Context) error {
			cancel()
			return nil
		},
		oversight.Permanent(),
		oversight.Natural(),
		"child",
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(tree.Start(ctx))
	t.Log(tree.Start(ctx))
}

func TestWaitAfterStart(t *testing.T) {
	t.Parallel()
	tree := oversight.New(oversight.NeverHalt())
	var (
		mu    sync.Mutex
		count int
	)
	ctx, cancel := context.WithCancel(context.Background())
	for range 10 {
		mu.Lock()
		count++
		mu.Unlock()
		err := tree.Add(
			func(ctx context.Context) error {
				<-ctx.Done()
				mu.Lock()
				count--
				mu.Unlock()
				return nil
			},
			oversight.Temporary(),
			oversight.Natural(),
			fmt.Sprintf("proc-%d", count),
		)
		if err != nil {
			t.Fatal(err)
		}
	}
	time.AfterFunc(1*time.Second, cancel)
	if err := tree.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("run returned before all children returned")
	}
}

func TestEmptyContextTree(t *testing.T) {
	var tree oversight.Tree
	var nilContext context.Context
	if err := tree.Start(nilContext); !errors.Is(err, oversight.ErrMissingContext) {
		t.Fatal("unexpected error", err)
	}
}

func TestTerminateNoExistingChildProcess(t *testing.T) {
	ctx := t.Context()
	tree := oversight.New(oversight.NeverHalt())
	go tree.Start(ctx)
	if err := tree.Terminate("404"); !errors.Is(err, oversight.ErrUnknownProcess) {
		t.Fatal("unexpected error", err)
	}
}

func Test_deleteFinishedTransient(t *testing.T) {
	tree := oversight.New(
		oversight.NeverHalt(),
		oversight.WithRestartStrategy(oversight.OneForAll()),
	)
	ready := make(chan struct{})
	cycled := make(chan struct{}, 1)
	err := tree.Add(
		func(ctx context.Context) error {
			<-ctx.Done()
			cycled <- struct{}{}
			return nil
		},
		oversight.Permanent(),
		oversight.Natural(),
		"finished-permanent",
	)
	if err != nil {
		t.Fatal(err)
	}
	err = tree.Add(
		func(context.Context) error {
			t.Log("running transient process")
			defer close(ready)
			return nil
		},
		oversight.Transient(),
		oversight.Natural(),
		"failed-transient",
	)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	go tree.Start(ctx)
	<-ready
	<-cycled
	if err := tree.Delete("failed-transient"); err != nil {
		t.Fatal(err)
	}
	<-ctx.Done()
}

func Test_badChildProcessSpecification(t *testing.T) {
	var tree oversight.Tree
	const procName = "alpha"
	if err := tree.Add(nil, nil, nil, procName); !errors.Is(err, oversight.ErrChildProcessSpecificationMissingStart) {
		t.Error("expected error missing:", err)
	}
	fn := func(ctx context.Context) error { return nil }
	if err := tree.Add(fn, nil, nil, procName); !errors.Is(err, oversight.ErrMissingRestartPolicy) {
		t.Error("expected error missing:", err)
	}
	restart := oversight.Permanent()
	if err := tree.Add(fn, restart, nil, procName); !errors.Is(err, oversight.ErrMissingShutdownPolicy) {
		t.Error("expected error missing:", err)
	}
	shutdown := oversight.Natural()
	if err := tree.Add(fn, restart, shutdown, procName); err != nil {
		t.Error("unexpected error:", err)
	}
	if err := tree.Add(fn, restart, shutdown, procName); !errors.Is(err, oversight.ErrNonUniqueProcessName) {
		t.Error("expected error missing:", err)
	}
}
