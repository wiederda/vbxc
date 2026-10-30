//go:build !amd64

package jit

// cpuidNative: auf Nicht-amd64 wird ohnehin nichts übersetzt.
func cpuidNative() bool { return false }
