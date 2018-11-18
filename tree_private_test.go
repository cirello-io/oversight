package oversight

import (
	"context"
	"testing"
)

func Test_uniqueName(t *testing.T) {
	tree := New(
		Process(ChildProcessSpecification{Name: "alpha", Start: func(ctx context.Context) error { return nil }}),
		Process(ChildProcessSpecification{Name: "alpha", Start: func(ctx context.Context) error { return nil }}),
	)
	seenNames := make(map[string]struct{})
	for _, p := range tree.processes {
		if _, ok := seenNames[p.Name]; ok {
			t.Fatal("unique process name logic has failed")
		}
	}
}
