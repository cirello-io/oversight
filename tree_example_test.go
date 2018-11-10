package oversight_test

import (
	"context"
	"fmt"
	"time"

	"cirello.io/oversight"
)

func ExampleOversight() {
	f := func(id int) oversight.ChildProcess {
		return func(context.Context) error { fmt.Println(id); return nil }
	}
	supervise := oversight.Oversight(
		oversight.WithRestart(10, 10*time.Second, oversight.OneForAll),
		oversight.Processes(f(1), f(2)),
		oversight.Process(oversight.Temporary, f(3)),
	)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	supervise(ctx)
	// Output:
}
