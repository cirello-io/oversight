// Copyright 2019 cirello.io/oversight - Ulderico Cirello
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

package easy

import (
	"context"
	"fmt"
	"testing"
)

func TestCorruptedState(t *testing.T) {
	ctx := context.WithValue(context.Background(), treeName, "404")
	var msgAdd string
	func() {
		defer func() {
			if r := recover(); r != nil {
				msgAdd = fmt.Sprint(r)
			}
		}()
		Add(ctx, func(context.Context) error { return nil })
	}()
	var msgDelete string
	func() {
		defer func() {
			if r := recover(); r != nil {
				msgDelete = fmt.Sprint(r)
			}
		}()
		Delete(ctx, "fake name")
	}()
	if msgAdd != "oversight tree not found" || msgDelete != "oversight tree not found" {
		t.Errorf("does not handle corrupted global state")
	}
}
