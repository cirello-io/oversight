package oversight

import "context"

type childProcess struct {
	restart Restart
	f       ChildProcess
}

// ChildProcess is a function that can be supervised for restart.
type ChildProcess func(ctx context.Context) error

// Restart is a function that decides if a worker has to be restarted or not
// according to its returned error.
type Restart func(error) bool

// Permanent goroutine is always restarted.
func Permanent(err error) bool { return true }

// Temporary goroutine is never restarted (not even when the supervisor restart
// strategy is rest_for_one or one_for_all and a sibling death causes the
// temporary process to be terminated).
func Temporary(err error) bool { return false }

// Transient goroutine is restarted only if it terminates abnormally, that is,
// with any error.
func Transient(err error) bool { return err != nil }
