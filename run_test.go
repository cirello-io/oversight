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
	"context"
	"errors"
	"testing"
)

func Test_safeRun(t *testing.T) {
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	type args struct {
		ctx context.Context
		f   func(ctx context.Context) error
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{"happy case", args{context.Background(), func(context.Context) error { return nil }}, false},
		{"errored case", args{context.Background(), func(context.Context) error { return errors.New("error") }}, true},
		{"canceledContext case", args{canceledCtx, func(ctx context.Context) error { return ctx.Err() }}, true},
		{"panic case", args{canceledCtx, func(ctx context.Context) error { panic("panic") }}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := safeRun(tt.args.ctx, tt.args.f); (err != nil) != tt.wantErr {
				t.Errorf("safeRun() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
