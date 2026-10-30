//go:build !windows && !linux

package stdlib

func processCPUTime() float64 { return 0 }
