package oversight

// Strategy defines how the supervisor handles individual failures.
type Strategy func(t *Tree, failedChildID int)

// OneForOne ensures that if a child process terminates, only that process is
// restarted.
func OneForOne() Strategy {
	return func(t *Tree, failedChildID int) {
		t.states[failedChildID].setFailed()
	}
}

// OneForAll ensures that if a child process terminates, all other child
// processes are terminated, and then all child processes, including the
// terminated one, are restarted.
func OneForAll() Strategy {
	return func(t *Tree, failedChildID int) {
		for i := len(t.states) - 1; i >= 0; i-- {
			t.states[i].setFailed()
		}
	}
}

// RestForOne ensures that if a child process terminates, the rest of the child
// processes (that is, the child processes after the terminated process in start
// order) are terminated. Then the terminated child process and the rest of the
// child processes are restarted.
func RestForOne() Strategy {
	return func(t *Tree, failedChildID int) {
		for i := len(t.states) - 1; i >= failedChildID; i-- {
			t.states[i].setFailed()
		}
	}
}

// TODO(uc): SimpleOneForOne - it would mean handling the child processes as if they
// were dynamic child processes. Leaving this to a later point, when support for
// dynamic child processes is added.
