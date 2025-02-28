// Copyright 2018 cirello.io/oversight/v2 - Ulderico Cirello
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
	"context"
	"testing"
)

func Test_uniqueName(t *testing.T) {
	var tree Tree
	if err := tree.Add(ChildProcessSpecification{Name: "alpha", Start: func(ctx context.Context) error { return nil }}); err != nil {
		t.Fatal("unexpected error:", err)
	}
	if err := tree.Add(ChildProcessSpecification{Name: "alpha", Start: func(ctx context.Context) error { return nil }}); err != nil {
		t.Fatal("unexpected error:", err)
	}

	seenNames := make(map[string]struct{})
	for _, p := range tree.children {
		if _, ok := seenNames[p.spec.Name]; ok {
			t.Fatal("unique process name logic has failed")
		}
	}
}

func Test_invalidChildProcessSpecification(t *testing.T) {
	var foundPanic bool
	func() {
		defer func() {
			if r := recover(); r != nil {
				foundPanic = true
			}
		}()
		var tree Tree
		tree.Add(ChildProcessSpecification{})
	}()
	if !foundPanic {
		t.Error("invalid child process specification should trigger have triggered a panic")
	}
}
