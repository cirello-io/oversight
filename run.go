package oversight

import (
	"context"

	"cirello.io/errors"
)

func safeRun(ctx context.Context, f ChildProcess) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.E(r)
		}
	}()
	err = f(ctx)
	return err
}

func restartableRun(ctx context.Context, restart Restart, f ChildProcess) error {
	var err error
	for {
		err = f(ctx)
		select {
		case <-ctx.Done():
			return err
		default:
			if !restart(err) {
				return err
			}
		}
	}
}
