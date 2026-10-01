package model

import "testing"

func TestDisplay(t *testing.T) {
	if got := CPUModelDisplay("AMD Ryzen 5 9600X 6-Core Processor"); got != "AMD RYZEN 5 9600X" {
		t.Errorf("CPUModelDisplay = %q", got)
	}
	if got := GPUDisplayName("NVIDIA GeForce RTX 3060"); got != "RTX 3060" {
		t.Errorf("GPUDisplayName = %q", got)
	}
}
