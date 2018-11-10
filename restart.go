package oversight

import "time"

// Strategy defines how the supervisor handles individual failures.
type Strategy func()

// OneForOne ensures that if a child process terminates, only that process is
// restarted.
func OneForOne() {}

// OneForAll ensures that if a child process terminates, all other child
// processes are terminated, and then all child processes, including the
// terminated one, are restarted.
func OneForAll() {}

// RestForOne ensures that if a child process terminates, the rest of the child
// processes (that is, the child processes after the terminated process in start
// order) are terminated. Then the terminated child process and the rest of the
// child processes are restarted.
func RestForOne() {}

// TODO(uc): SimpleOneForOne - it would mean handling the child processes as if they
// were dynamic child processes. Leaving this to a later point, when support for
// dynamic child processes is added.

var timeNow = time.Now

type restart struct {
	intensity int
	period    time.Duration
	restarts  []time.Time
}

func (r *restart) terminate(now time.Time) bool {
	restarts := 0
	for i := len(r.restarts) - 1; i >= 0; i-- {
		then := r.restarts[i]
		if then.Add(r.period).After(now) {
			restarts++
		}
	}
	r.restarts = append(r.restarts, now)
	return restarts >= r.intensity
}
