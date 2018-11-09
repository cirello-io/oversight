package oversight

import (
	"context"
	"testing"

	"cirello.io/errors"
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
		{"errored case", args{context.Background(), func(context.Context) error { return errors.E("error") }}, true},
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

func Test_restartableRun_permanent(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"normal case", nil},
		{"errored case", errors.E("error")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const expectedCount = 2
			var gotCount int
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			f := func(ctx context.Context) error {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
					gotCount++
					if gotCount == expectedCount {
						cancel()
					}
				}
				return tt.err
			}
			go restartableRun(ctx, Permanent, f)
			<-ctx.Done()
			if gotCount != expectedCount {
				t.Error("permanent policy should always restart")
			}
		})
	}
}

func Test_restartableRun_temporary(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ticks := make(chan struct{})
	f := func(ctx context.Context) error {
		<-ticks
		return errors.E("errored")
	}
	go restartableRun(ctx, Temporary, f)
	ticks <- struct{}{}
	select {
	case ticks <- struct{}{}:
		t.Error("temporary policy should never restart")
	default:
	}
}

func Test_restartableRun_transient(t *testing.T) {
	// success - should tick once, and only once.
	t.Run("worker ended without error", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		ticks := make(chan struct{})
		f := func(ctx context.Context) error {
			<-ticks
			return nil
		}
		go restartableRun(ctx, Transient, f)

		ticks <- struct{}{}
		select {
		case ticks <- struct{}{}:
			t.Error("transient policy should never restart on success")
		default:
		}
	})

	// failure - should tick at least twice.
	t.Run("worker must restart on error", func(t *testing.T) {
		expectedErr := errors.E("error")
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		ticks := make(chan chan struct{})
		f := func(ctx context.Context) error {
			pong := <-ticks
			pong <- struct{}{}
			return expectedErr
		}

		var err error
		go func() {
			err = restartableRun(ctx, Transient, f)
		}()

		for i := 0; i < 2; i++ {
			ping := make(chan struct{})
			ticks <- ping
			select {
			case <-ping:
			default:
				t.Error("transient policy should always restart on failure")
			}
		}

		if err != nil && !errors.Match(expectedErr, err) {
			t.Error("not matched error", expectedErr, err)
		}
	})

}
