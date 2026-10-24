package simulator

import (
	"math"
	"testing"
	"time"
)

func TestHeldButtonHeartbeatAndLease(t *testing.T) {
	var i Input
	now := time.Now()
	edges, _, err := i.Apply(Snapshot{1, []string{"up", "b"}, 0}, now)
	if err != nil || len(edges) != 2 {
		t.Fatal(edges, err)
	}
	edges, delta, err := i.Apply(Snapshot{2, []string{"up", "b"}, 20}, now)
	if err != nil || len(edges) != 0 || delta != 20 {
		t.Fatal("heartbeat retriggered buttons", edges, delta, err)
	}
	if len(i.Expire(now.Add(999*time.Millisecond))) != 0 {
		t.Fatal("lease expired early")
	}
	if len(i.Expire(now.Add(time.Second))) != 2 {
		t.Fatal("held keys not released")
	}
	if len(i.Release()) != 0 {
		t.Fatal("duplicate release")
	}
}
func TestInvalidSnapshotDoesNotMutateState(t *testing.T) {
	var i Input
	now := time.Now()
	i.Apply(Snapshot{1, []string{"left"}, 0}, now)
	for _, s := range []Snapshot{{1, nil, 0}, {2, []string{"escape"}, 0}, {2, nil, math.NaN()}, {2, nil, 721}} {
		if _, _, err := i.Apply(s, now); err == nil {
			t.Fatal("accepted", s)
		}
	}
	edges, delta, err := i.Apply(Snapshot{2, []string{"left", "a"}, -30}, now)
	if err != nil || len(edges) != 1 || edges[0] != (Edge{"a", true}) || delta != -30 {
		t.Fatal(edges, delta, err)
	}
}
