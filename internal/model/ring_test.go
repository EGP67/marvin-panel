package model

import (
	"slices"
	"testing"
)

func TestRingEmpty(t *testing.T) {
	r := NewRing[float64](120)
	if v := r.Values(); len(v) != 0 || r.Len() != 0 || r.Cap() != 120 {
		t.Errorf("empty ring: Values len %d, Len %d, Cap %d", len(v), r.Len(), r.Cap())
	}
}

func TestRingPartial(t *testing.T) {
	r := NewRing[int](5)
	for i := 1; i <= 3; i++ {
		r.Push(i)
	}
	if got := r.Values(); !slices.Equal(got, []int{1, 2, 3}) || r.Len() != 3 {
		t.Errorf("partial = %v (Len %d), want [1 2 3]", got, r.Len())
	}
}

func TestRingWrap(t *testing.T) {
	r := NewRing[int](120)
	for i := 1; i <= 125; i++ {
		r.Push(i)
	}
	got := r.Values()
	if len(got) != 120 || r.Len() != 120 {
		t.Fatalf("len = %d (Len %d), want 120", len(got), r.Len())
	}
	if got[0] != 6 || got[119] != 125 {
		t.Errorf("first/last = %d/%d, want 6/125", got[0], got[119])
	}
	for i := 1; i < len(got); i++ {
		if got[i] != got[i-1]+1 {
			t.Fatalf("not oldest-first at %d: %v", i, got[i-1:i+1])
		}
	}
}

func TestRingValuesIsCopy(t *testing.T) {
	r := NewRing[int](3)
	r.Push(1)
	r.Push(2)
	v := r.Values()
	v[0] = 99
	if got := r.Values(); got[0] != 1 {
		t.Errorf("mutating Values() changed the ring: %v", got)
	}
}

func TestRingZeroCapacityPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("NewRing(0) did not panic")
		}
	}()
	NewRing[int](0)
}
