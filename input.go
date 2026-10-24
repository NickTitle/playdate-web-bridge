package simulator

import (
	"errors"
	"math"
	"time"
)

type Snapshot struct {
	Sequence uint64   `json:"sequence"`
	Buttons  []string `json:"buttons"`
	Crank    float64  `json:"crank"`
}
type Edge struct {
	Button string
	Down   bool
}

// Input is owned by one connection, on one goroutine. Absolute crank totals
// and button snapshots avoid replaying B presses during heartbeat updates.
type Input struct {
	sequence uint64
	held     map[string]bool
	crank    float64
	last     time.Time
}

var buttons = []string{"up", "down", "left", "right", "a", "b", "menu"}

func (i *Input) Apply(s Snapshot, now time.Time) ([]Edge, float64, error) {
	if s.Sequence <= i.sequence {
		return nil, 0, errors.New("stale sequence")
	}
	if math.IsNaN(s.Crank) || math.IsInf(s.Crank, 0) || math.Abs(s.Crank-i.crank) > 720 {
		return nil, 0, errors.New("invalid crank delta")
	}
	next := map[string]bool{}
	for _, b := range s.Buttons {
		valid := false
		for _, known := range buttons {
			if b == known {
				valid = true
				break
			}
		}
		if !valid {
			return nil, 0, errors.New("unknown button")
		}
		next[b] = true
	}
	var edges []Edge
	for _, b := range buttons {
		if next[b] != i.held[b] {
			edges = append(edges, Edge{b, next[b]})
		}
	}
	delta := s.Crank - i.crank
	i.sequence = s.Sequence
	i.held = next
	i.crank = s.Crank
	i.last = now
	return edges, delta, nil
}
func (i *Input) Release() []Edge {
	var edges []Edge
	for _, b := range buttons {
		if i.held[b] {
			edges = append(edges, Edge{b, false})
		}
	}
	i.held = nil
	return edges
}
func (i *Input) Expire(now time.Time) []Edge {
	if now.Sub(i.last) >= time.Second {
		return i.Release()
	}
	return nil
}
