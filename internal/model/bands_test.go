package model

import "testing"

func TestBands(t *testing.T) {
	cases := []struct {
		name string
		f    func(float64) Band
		in   float64
		want Band
	}{
		{"Severity", Severity, 39.9, BandOK},
		{"Severity", Severity, 40, BandWarn},
		{"Severity", Severity, 70, BandWarn},
		{"Severity", Severity, 70.1, BandDanger},
		{"CPUBand", CPUBand, 59.9, BandOK},
		{"CPUBand", CPUBand, 60, BandWarn},
		{"CPUBand", CPUBand, 89.9, BandWarn},
		{"CPUBand", CPUBand, 90, BandDanger},
		{"ThermalBand", ThermalBand, 69.9, BandOK},
		{"ThermalBand", ThermalBand, 70, BandWarn},
		{"ThermalBand", ThermalBand, 89.9, BandWarn},
		{"ThermalBand", ThermalBand, 90, BandDanger},
		{"MountState", MountState, 79.9, BandOK},
		{"MountState", MountState, 80, BandWarn},
		{"MountState", MountState, 89.9, BandWarn},
		{"MountState", MountState, 90, BandDanger},
	}
	for _, tc := range cases {
		if got := tc.f(tc.in); got != tc.want {
			t.Errorf("%s(%v) = %s, want %s", tc.name, tc.in, got, tc.want)
		}
	}
}

func TestCToF(t *testing.T) {
	cases := map[float64]int{0: 32, 100: 212, -40: -40, 37: 99, 48: 118, 61: 142, 44: 111, 71: 160}
	for c, want := range cases {
		if got := CToF(c); got != want {
			t.Errorf("CToF(%v) = %d, want %d", c, got, want)
		}
	}
}
