// Copyright 2018 cirello.io/oversight - Ulderico Cirello
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package oversight

import "slices"

// Strategy defines how the supervisor handles individual failures and tree
// shutdowns (best effort). The shutdown is initiated in the reverse order of
// the start of the child processes. The Go scheduler implementation makes it
// impossible to guarantee any order regarding shutdown completion.
type Strategy func(t *Tree, childProcess *childProcess)

// OneForOne ensures that if a child process terminates, only that process is
// restarted.
func OneForOne() Strategy {
	return func(t *Tree, childProcess *childProcess) {
		childProcess.state.setFailed()
		childProcess.state.stop()
	}
}

// OneForAll ensures that if a child process terminates, all other child
// processes are terminated, and then all child processes, including the
// terminated one, are restarted.
func OneForAll() Strategy {
	return func(t *Tree, _ *childProcess) {
		for i := len(t.childrenOrder) - 1; i >= 0; i-- {
			proc := t.childrenOrder[i]
			proc.state.setFailed()
			proc.state.stop()
		}
	}
}

// RestForOne ensures that if a child process terminates, the rest of the child
// processes (that is, the child processes after the terminated process in start
// order) are terminated. Then the terminated child process and the rest of the
// child processes are restarted.
func RestForOne() Strategy {
	return func(t *Tree, childProcess *childProcess) {
		i := len(t.childrenOrder) - 1
		failedChildID := slices.Index(t.childrenOrder, childProcess)
		for i >= failedChildID {
			proc := t.childrenOrder[i]
			proc.state.setFailed()
			proc.state.stop()
			i--
		}
	}
}

// SimpleOneForOne behaves similarly to OneForOne but it runs the stop calls
// asynchronously.
func SimpleOneForOne() Strategy {
	return func(t *Tree, childProcess *childProcess) {
		childProcess.state.setFailed()
		// avoid dereferencing the stop pointer after the t.semaphore is released.
		stop := childProcess.state.stop
		go stop()
	}
}
