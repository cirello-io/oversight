// Copyright 2024 cirello.io/oversight/v2 - Ulderico Cirello
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

import (
	"slices"
	"sync"
	"testing"
	"time"
)

func TestStrategy_OneForOne(t *testing.T) {
	t.Parallel()
	var called bool
	OneForOne()(nil, &childProcess{
		state: &state{
			stop: func() {
				called = true
			},
		},
	})
	if !called {
		t.Error("stop method was not called")
	}
}

func TestStrategy_OneForAll(t *testing.T) {
	t.Parallel()
	test := func(t *testing.T, procs []string, stoppedProc string, expectedSeq []string) {
		var (
			mu         sync.Mutex
			stoppedSeq []string
		)
		tree := &Tree{}
		tree.init()
		for _, n := range procs {
			cp := &childProcess{
				state: &state{
					state: Running,
					stop: func() {
						mu.Lock()
						defer mu.Unlock()
						stoppedSeq = append(stoppedSeq, n)
					},
				},
			}
			tree.children[n] = cp
			tree.childrenOrder = append(tree.childrenOrder, cp)
		}
		OneForAll()(tree, tree.children[stoppedProc])
		mu.Lock()
		defer mu.Unlock()
		if got, want := stoppedSeq, expectedSeq; !slices.Equal(got, want) {
			t.Errorf("stopped sequence: got %v, want %v", got, want)
		}
	}
	t.Run("ok", func(t *testing.T) {
		test(t, []string{"a", "b", "c"}, "b", []string{"c", "b", "a"})
	})
	t.Run("unknown", func(t *testing.T) {
		test(t, []string{"a", "b", "c"}, "unknown", []string{"c", "b", "a"})
	})
}

func TestStrategy_RestForOne(t *testing.T) {
	t.Parallel()
	test := func(t *testing.T, procs []string, stoppedProc string, expectedSeq []string) {
		var (
			mu         sync.Mutex
			stoppedSeq []string
		)
		tree := &Tree{}
		tree.init()
		for _, n := range procs {
			cp := &childProcess{
				state: &state{
					state: Running,
					stop: func() {
						mu.Lock()
						defer mu.Unlock()
						stoppedSeq = append(stoppedSeq, n)
					},
				},
			}
			tree.children[n] = cp
			tree.childrenOrder = append(tree.childrenOrder, cp)
		}
		RestForOne()(tree, tree.children[stoppedProc])
		mu.Lock()
		defer mu.Unlock()
		if got, want := stoppedSeq, expectedSeq; !slices.Equal(got, want) {
			t.Errorf("stopped sequence: got %v, want %v", got, want)
		}
	}
	t.Run("ok", func(t *testing.T) {
		test(t, []string{"a", "b", "c"}, "b", []string{"c", "b"})
	})
	t.Run("all", func(t *testing.T) {
		test(t, []string{"a", "b", "c"}, "a", []string{"c", "b", "a"})
	})
	t.Run("unknown", func(t *testing.T) {
		test(t, []string{"a", "b", "c"}, "unknown", []string{})
	})
}

func TestStrategy_SimpleOneForOne(t *testing.T) {
	t.Parallel()
	called := make(chan struct{})
	SimpleOneForOne()(nil, &childProcess{
		state: &state{
			stop: func() {
				close(called)
			},
		},
	})
	select {
	case <-called:
	case <-time.After(1 * time.Second):
		t.Error("stop method was not called")
	}
}
