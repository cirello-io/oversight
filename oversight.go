package oversight

import (
	"context"

	"cirello.io/errors"
)

func safeRun(ctx context.Context, f func(ctx context.Context) error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.E(r)
		}
	}()
	err = f(ctx)
	return err
}
