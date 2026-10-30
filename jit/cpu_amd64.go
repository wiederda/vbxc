//go:build amd64

// CPU-Erkennung für die Fast-Path-Gates (Klauspost, im Modulgraph vorhanden).
package jit

import "github.com/klauspost/cpuid/v2"

// cpuidNative meldet SSE4.1 (roundsd) — Voraussetzung der Rundungs-Fastpaths.
func cpuidNative() bool {
	return cpuid.CPU.Has(cpuid.SSE4)
}
