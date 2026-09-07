package window

import "testing"

func TestJobsQueuedInsideJobSurviveNextFrame(t *testing.T) {
	laterJobs = nil
	calls := 0
	RunLater(func() { calls++; RunLater(func() { calls++ }) })
	runLaterJobs()
	if calls != 1 {
		t.Fatal(calls)
	}
	runLaterJobs()
	if calls != 2 {
		t.Fatal("nested commit job lost")
	}
}
