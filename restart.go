package oversight

import "time"

var timeNow = time.Now

type restart struct {
	intensity int
	period    time.Duration
	restarts  []time.Time
}

func (r *restart) terminate(now time.Time) bool {
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
