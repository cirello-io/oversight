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

import (
	"reflect"
	"testing"
	"time"
)

func Test_restart_terminate(t *testing.T) {
	now := time.Now()
	r := &restart{
		intensity: 1,
		period:    5 * time.Second,
	}
	got := []bool{
		r.terminate(now.Add(1 * time.Second)),
		r.terminate(now.Add(2 * time.Second)),
	}
	expected := []bool{false, true}
	if !reflect.DeepEqual(got, expected) {
		t.Error("intense restarts should have triggered a termination")
	}
}
