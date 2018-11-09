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
