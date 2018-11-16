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

	"cirello.io/oversight"
)

// ExampleTree_singlePermanent shows how to create a static tree of permanent
// child processes.
func ExampleTree_singlePermanent() {
	supervise := oversight.Oversight(
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
	err := supervise(ctx)
	if err != nil {
		fmt.Println(err)
	}

	// Output:
	// 1
	// 1
	// too many failures
}

func TestTree_childProcessRestarts(t *testing.T) {
	t.Run("permanent", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		const expectedRuns = 2
		totalRuns := 0
		supervise := oversight.Oversight(
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
			supervise(ctx)
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
		supervise := oversight.Oversight(
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
			supervise(ctx)
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
		supervise := oversight.Oversight(
			oversight.Process(oversight.ChildProcessSpecification{
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
			}),
			oversight.Process(oversight.ChildProcessSpecification{
				Restart: oversight.Permanent(),
				Start: func(context.Context) error {
					t.Log("ping...")
					time.Sleep(500 * time.Millisecond)
					return nil
				},
			}),
		)
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			supervise(ctx)
		}()
		wg.Wait()
		if totalRuns >= unexpectedRuns || totalRuns == 0 {
			t.Log(totalRuns, unexpectedRuns)
			t.Error("oversight did not restart temporary service the right amount of times")
		}
	})
}

func TestTree_treeRestarts(t *testing.T) {
	t.Run("oneForOne", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		badSiblingRuns := 0
		goodSiblingRuns := 0
		supervise := oversight.Oversight(
			oversight.WithSpecification(2, 1*time.Second, oversight.OneForOne()),
			oversight.Process(oversight.ChildProcessSpecification{
				Restart: oversight.Permanent(),
				Start: func(ctx context.Context) error {
					t.Log("failed process should always restart...")
					badSiblingRuns++
					if badSiblingRuns >= 2 {
						cancel()
					}
					return errors.New("finished")
				},
			}),
			oversight.Process(oversight.ChildProcessSpecification{
				Restart: oversight.Permanent(),
				Start: func(ctx context.Context) error {
					t.Log("but sibling must stay untouched")
					goodSiblingRuns++
					select {
					case <-ctx.Done():
					}
					return nil
				},
			}),
		)
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			supervise(ctx)
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
		supervise := oversight.Oversight(
			oversight.WithSpecification(2, 1*time.Second, oversight.OneForAll()),
			oversight.Process(oversight.ChildProcessSpecification{
				Restart: oversight.Permanent(),
				Start: func(ctx context.Context) error {
					t.Log("failed process should always restart...")
					badSiblingRuns++
					if badSiblingRuns >= 2 {
						cancel()
					}
					return errors.New("finished")
				},
			}),
			oversight.Process(oversight.ChildProcessSpecification{
				Restart: oversight.Permanent(),
				Start: func(ctx context.Context) error {
					t.Log("and the sibling must restart too")
					goodSiblingRuns++
					select {
					case <-ctx.Done():
					}
					return nil
				},
			}),
		)
		err := supervise(ctx)
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
		supervise := oversight.Oversight(
			oversight.WithSpecification(2, 1*time.Second, oversight.RestForOne()),
			oversight.Process(oversight.ChildProcessSpecification{
				Restart: oversight.Permanent(),
				Start: func(ctx context.Context) error {
					t.Log("first sibling should never die")
					firstSiblingRuns++
					select {
					case <-ctx.Done():
					}
					return nil
				}}),
			oversight.Process(oversight.ChildProcessSpecification{
				Restart: oversight.Permanent(),
				Start: func(ctx context.Context) error {
					t.Log("failed process should always restart...")
					badSiblingRuns++
					if badSiblingRuns >= 2 {
						cancel()
					}
					return errors.New("finished")
				}}),
			oversight.Process(oversight.ChildProcessSpecification{
				Restart: oversight.Permanent(),
				Start: func(ctx context.Context) error {
					t.Log("and the younger siblings too")
					goodSiblingRuns++
					select {
					case <-ctx.Done():
					}
					return nil
				}}),
		)
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			supervise(ctx)
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
	leafCount := 0
	leaf := oversight.Oversight(oversight.Processes(
		func(ctx context.Context) error {
			for {
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(500 * time.Millisecond):
					leafCount++
				}
			}
		},
	))
	rootCount := 0
	root := oversight.Oversight(
		oversight.Processes(
			leaf,
			func(ctx context.Context) error {
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(1 * time.Second):
					rootCount++
				}
				return nil
			},
		),
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	root(ctx)
	if leafCount == 0 || rootCount == 0 {
		t.Error("tree did not run")
	} else if leafCount == 0 {
		t.Error("subtree did not run")
	}
}

func Test_dynamicChild(t *testing.T) {
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	oversight.Oversight(
		oversight.WithLogger(logger),
		oversight.Processes(
			func(ctx context.Context) error {
				cancel()
				return nil
			},
		),
	)(ctx)
	content := buf.String()
	expectedLog := strings.Contains(content, "child started") && strings.Contains(content, "child done")
	if !expectedLog {
		t.Log(content)
		t.Error("the logger did not log the expected lines")
	}
}
