package model

import "testing"

func ip(v int) *int { return &v }

func TestFanVerdict(t *testing.T) {
	ok0, warn0 := BandOK, BandWarn
	ok, warn := &ok0, &warn0
	cases := []struct {
		rpm, max *int
		band     *Band
		want     string
	}{
		{nil, nil, ok, "unknown"},
		{nil, nil, nil, "unknown"},
		{ip(0), ip(2900), ok, "stalled"},
		{ip(2900), ip(2900), warn, "not_cooling"},
		{ip(2900), ip(2900), nil, "ok"},
		{ip(2900), ip(2900), ok, "ok"},
		{ip(1500), ip(2900), warn, "ok"},
	}
	for _, tc := range cases {
		if got := FanVerdict(tc.rpm, tc.max, tc.band); got != tc.want {
			t.Errorf("FanVerdict(%v, %v, %v) = %s, want %s", tc.rpm, tc.max, tc.band, got, tc.want)
		}
	}
}
