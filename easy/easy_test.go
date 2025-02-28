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

package easy_test

import (
	"context"
	"testing"

	oversight "cirello.io/oversight/v2/easy"
)

func TestInvalidContext(t *testing.T) {
	ctx := context.Background()
	_, err := oversight.Add(ctx, func(context.Context) error { return nil })
	if err != oversight.ErrNoTreeAttached {
		t.Errorf("ErrNoTreeAttached not found: %v", err)
	}

	if err := oversight.Delete(ctx, "fake name"); err != oversight.ErrNoTreeAttached {
		t.Errorf("ErrNoTreeAttached not found: %v", err)
	}
}
