//go:build !amd64

package jit

import "crypto/sha256"

// CodegenFingerprint auf Nicht-amd64: es gibt keinen Codegenerator (Prepare
// liefert hier nichts), daher ein stabiler Platzhalter. Cache-Dateien, die
// Key- und Fingerprint-Mechanismus nutzen, entstehen auf solchen Plattformen
// auch nicht.
func CodegenFingerprint() [32]byte {
	return sha256.Sum256([]byte("vbxc-jit: kein Codegen auf dieser Architektur"))
}
