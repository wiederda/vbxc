//go:build !amd64

package jit

// cpuidNative: auf Nicht-amd64 wird ohnehin nichts übersetzt.
func cpuidNative() bool { return false }

func hostSupportsCPUFeatures(required uint32) bool { return required == 0 }

var hasFMA = false
