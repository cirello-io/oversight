package easy_test

import (
	"context"
	"testing"

	oversight "cirello.io/oversight/easy"
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
