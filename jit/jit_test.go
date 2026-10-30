//go:build amd64

package jit

import (
	"testing"
	"unsafe"
)

// TestAllocExec prüft den ausführbaren Speicherblock (W^X): Code hinein-
// schreiben, Schutz auf RX stellen, ausführen. Es wird dabei keine Datei
// angelegt.
func TestAllocExec(t *testing.T) {
	// mov eax, 42 ; ret
	code := []byte{0xB8, 0x2A, 0x00, 0x00, 0x00, 0xC3}
	mem, err := allocExec(code)
	if err != nil {
		t.Fatalf("allocExec: %v", err)
	}
	defer mem.free()

	if mem.ptr == 0 || mem.size == 0 {
		t.Fatalf("leerer Speicherblock: ptr=%#x size=%d", mem.ptr, mem.size)
	}

	// Funcval-Trick wie in Entry: erstes Wort des Slice-Headers ist der Code.
	fv := make([]uintptr, 1)
	fv[0] = mem.ptr
	fn := *(*func() int32)(unsafe.Pointer(&fv))
	if got := fn(); got != 42 {
		t.Errorf("Maschinencode lieferte %d, erwartet 42", got)
	}
}

// TestAllocExecLeer stellt sicher, dass ein leerer Block abgelehnt wird
// (statt eine Seite ausführbaren Speichers zu verschwenden).
func TestAllocExecLeer(t *testing.T) {
	if _, err := allocExec(nil); err == nil {
		t.Error("leerer Codeblock wurde akzeptiert")
	}
}
