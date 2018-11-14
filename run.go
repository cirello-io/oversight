package oversight

import (
	"context"
	"fmt"
)

func safeRun(ctx context.Context, f ChildProcess) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("recovered panic: %v", r)
		}
	}()
	err = f(ctx)
	return err
}
