package model

import "strings"

// CPUModelDisplay shortens the raw /proc/cpuinfo model name for the panel (D-020).
func CPUModelDisplay(raw string) string {
	return strings.ToUpper(strings.TrimSuffix(raw, " 6-Core Processor"))
}

// GPUDisplayName shortens the raw nvidia-smi name for the panel (D-020).
func GPUDisplayName(raw string) string {
	return strings.TrimPrefix(raw, "NVIDIA GeForce ")
}
