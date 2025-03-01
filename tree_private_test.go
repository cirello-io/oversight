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
	"errors"
	"testing"
)

func Test_uniqueName(t *testing.T) {
	var tree Tree
	f := func(ctx context.Context) error {
		return nil
	}
	if err := tree.Add(f, Permanent(), Natural(), "alpha"); err != nil {
		t.Fatal("unexpected error:", err)
	}
	if err := tree.Add(f, Permanent(), Natural(), "alpha"); !errors.Is(err, ErrNonUniqueProcessName) {
		t.Fatal("unexpected error:", err)
	}
}

func Test_invalidChildProcessSpecification(t *testing.T) {
	var tree Tree
	err := tree.Add(nil, Permanent(), Natural(), "alpha")
	if !errors.Is(err, ErrChildProcessSpecificationMissingStart) {
		t.Error("invalid child process specification should trigger have triggered a panic")
	}
}
