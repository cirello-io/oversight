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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := safeRun(tt.args.ctx, tt.args.f); (err != nil) != tt.wantErr {
				t.Errorf("safeRun() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
