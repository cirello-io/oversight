package oversight

import (
	"context"

	"cirello.io/errors"
)

// Worker is a function that can be supervised for restart.
type Worker func(ctx context.Context) error

func safeRun(ctx context.Context, f Worker) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.E(r)
		}
	}()
	err = f(ctx)
	return err
}

// RestartPolicy is a function that decides if a worker has to be restarted or
// not according to its returned error.
type RestartPolicy func(error) bool

// Permanent goroutine is always restarted.
func Permanent(err error) bool { return true }

// Temporary goroutine is never restarted (not even when the supervisor restart
// strategy is rest_for_one or one_for_all and a sibling death causes the
// temporary process to be terminated).
func Temporary(err error) bool { return false }

// Transient goroutine is restarted only if it terminates abnormally, that is,
// with any error.
func Transient(err error) bool { return err != nil }

func restartableRun(ctx context.Context, restart RestartPolicy, f Worker) error {
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
