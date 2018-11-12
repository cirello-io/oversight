package oversight_test

import (
	"context"
	"errors"
	"fmt"
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
			oversight.Process(oversight.Permanent, func(ctx context.Context) error {
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
			oversight.Process(oversight.Transient, func(ctx context.Context) error {
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

	// t.Run("temporary", func(t *testing.T) {
	// 	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	// 	defer cancel()
	// 	const unexpectedRuns = 2
	// 	totalRuns := 0
	// 	supervise := oversight.Oversight(
	// 		oversight.Process(oversight.Temporary, func(ctx context.Context) error {
	// 			var err error
	// 			if totalRuns == 0 {
	// 				err = errors.New("finished")
	// 			}
	// 			select {
	// 			case <-ctx.Done():
	// 				return nil
	// 			default:
	// 				totalRuns++
	// 				t.Log("run:", totalRuns)
	// 				cancel()
	// 			}
	// 			return err
	// 		}),
	// 		oversight.Process(oversight.Permanent, func(context.Context) error {
	// 			t.Log("ping...")
	// 			time.Sleep(500 * time.Millisecond)
	// 			return nil
	// 		}),
	// 	)
	// 	var wg sync.WaitGroup
	// 	wg.Add(1)
	// 	go func() {
	// 		defer wg.Done()
	// 		supervise(ctx)
	// 	}()
	// 	wg.Wait()
	// 	if totalRuns >= unexpectedRuns || totalRuns == 0 {
	// 		t.Log(totalRuns, unexpectedRuns)
	// 		t.Error("oversight did not restart temporary service the right amount of times")
	// 	}
	// })
}

func TestTree_treeRestarts(t *testing.T) {
	t.Run("oneForOne", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		badSiblingRuns := 0
		goodSiblingRuns := 0
		supervise := oversight.Oversight(
			oversight.WithRestart(2, 1*time.Second, oversight.OneForOne),
			oversight.Process(oversight.Permanent, func(ctx context.Context) error {
				t.Log("failed process should always restart...")
				badSiblingRuns++
				if badSiblingRuns >= 2 {
					cancel()
				}
				return errors.New("finished")
			}),
			oversight.Process(oversight.Permanent, func(ctx context.Context) error {
				t.Log("but sibling must stay untouched")
				goodSiblingRuns++
				select {
				case <-ctx.Done():
				}
				return nil
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
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		badSiblingRuns := 0
		goodSiblingRuns := 0
		supervise := oversight.Oversight(
			oversight.WithRestart(2, 1*time.Second, oversight.OneForAll),
			oversight.Process(oversight.Permanent, func(ctx context.Context) error {
				t.Log("failed process should always restart...")
				badSiblingRuns++
				if badSiblingRuns >= 2 {
					cancel()
				}
				return errors.New("finished")
			}),
			oversight.Process(oversight.Permanent, func(ctx context.Context) error {
				t.Log("and the sibling must restart too")
				goodSiblingRuns++
				select {
				case <-ctx.Done():
				}
				return nil
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
			oversight.WithRestart(2, 1*time.Second, oversight.RestForOne),
			oversight.Process(oversight.Permanent, func(ctx context.Context) error {
				t.Log("first sibling should never die")
				firstSiblingRuns++
				select {
				case <-ctx.Done():
				}
				return nil
			}),
			oversight.Process(oversight.Permanent, func(ctx context.Context) error {
				t.Log("failed process should always restart...")
				badSiblingRuns++
				if badSiblingRuns >= 2 {
					cancel()
				}
				return errors.New("finished")
			}),
			oversight.Process(oversight.Permanent, func(ctx context.Context) error {
				t.Log("and the younger siblings too")
				goodSiblingRuns++
				return nil
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
		if goodSiblingRuns != badSiblingRuns {
			t.Error("restForOne should always terminate younger siblings")
		}
		if firstSiblingRuns != 1 {
			t.Error("restForOne should never terminate older siblings")
		}
	})
}
