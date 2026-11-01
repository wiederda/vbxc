//go:build amd64

// CPU-Erkennung für die Fast-Path-Gates (Klauspost, im Modulgraph vorhanden).
package jit

import (
	"os"
	"strings"

	"github.com/klauspost/cpuid/v2"
)

// cpuidNative meldet SSE4.1 (roundsd) — Voraussetzung der Rundungs-Fastpaths.
func cpuidNative() bool {
	return cpuid.CPU.Has(cpuid.SSE4)
}

func hostSupportsCPUFeatures(required uint32) bool {
	if required&cpuFeatureSSE41 != 0 && !cpuid.CPU.Has(cpuid.SSE4) {
		return false
	}
	if required&cpuFeatureAVX != 0 && !cpuid.CPU.Has(cpuid.AVX) {
		return false
	}
	if required&cpuFeatureFMA != 0 && !cpuid.CPU.Has(cpuid.FMA3) {
		return false
	}
	return true
}

// hasFMA spiegelt Go math.useFMA (AVX && FMA), einschließlich der
// GODEBUG-CPU-Schalter, die Go beim Prozessstart auf dieselben Features
// anwendet. Nur dann kann der Native-Pfad denselben archExp-Zweig spiegeln.
var hasFMA = goMathUsesFMA()

func goMathUsesFMA() bool {
	avxAvailable := cpuid.CPU.Has(cpuid.AVX)
	fmaAvailable := cpuid.CPU.Has(cpuid.FMA3)
	avxEnabled, fmaEnabled := true, true
	for _, field := range strings.Split(os.Getenv("GODEBUG"), ",") {
		key, val, ok := strings.Cut(field, "=")
		if !ok {
			continue
		}
		switch key {
		case "cpu.all":
			if val == "on" || val == "off" {
				avxEnabled = val == "on"
				fmaEnabled = val == "on"
			}
		case "cpu.avx":
			if val == "on" || val == "off" {
				avxEnabled = val == "on"
			}
		case "cpu.fma":
			if val == "on" || val == "off" {
				fmaEnabled = val == "on"
			}
		}
	}
	return avxAvailable && avxEnabled && fmaAvailable && fmaEnabled
}
