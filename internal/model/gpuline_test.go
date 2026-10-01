package model

import "testing"

func TestGPULine(t *testing.T) {
	ip := func(v int) *int { return &v }
	g := func(temp, util float64, mem int) GPUState {
		return GPUState{TempC: fp(temp), UtilPct: fp(util), MemUsedMiB: ip(mem)}
	}
	cases := []struct {
		name string
		gs   []GPUState
		want string
	}{
		{"stale", []GPUState{{TempC: nil, UtilPct: fp(50)}, g(40, 0, 0)}, "THE BRAINS ARE NOT ANSWERING."},
		{"all null", []GPUState{{}, {}}, "THE BRAINS ARE NOT ANSWERING."},
		{"hot", []GPUState{g(79.4, 88, 10112), g(82.1, 94, 11904)}, "THINKING THIS HARD RUNS AT 82 DEGREES."},
		{"thinking", []GPUState{g(58, 62, 9420), g(61.5, 71, 11800)}, "SOMEONE ASKED IT SOMETHING. NOT ME."},
		{"loaded", []GPUState{g(44, 0, 8099), g(36, 0, 11243)}, "MODEL LOADED. NOBODY ASKS IT ANYTHING."},
		{"empty", []GPUState{g(35, 0, 5), g(33, 1, 5)}, "BOTH BRAINS EMPTY. RESTFUL, NOT HAPPY."},
	}
	for _, tc := range cases {
		if got := GPULine(tc.gs); got != tc.want {
			t.Errorf("%s: GPULine = %q, want %q", tc.name, got, tc.want)
		}
	}
}
