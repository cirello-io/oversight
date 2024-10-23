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
	"time"
)

var timeNow = time.Now

type restart struct {
	intensity int
	period    time.Duration
	restarts  []time.Time
}

func (r *restart) shouldTerminate(now time.Time) bool {
	if r.intensity == -1 {
		return false
	}
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
