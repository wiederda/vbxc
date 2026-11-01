// Dateiformat der Maschinencode-Datei und die kombinierte Ladefunktion für
// .vbb-Dateien (Bytecode oder Maschinencode).
//
// Beide Formate teilen sich das Layout: 4 Byte Magic (unkomprimiert), danach
// ein gzip-Stream, der per gob die Version und den Inhalt enthält.
//
//	"VBXB": Bytecode-Chunk (portabel, siehe bytecode/file.go)
//	"VBXM": Maschinencode für eine Plattform: Chunk (als Daten) +
//	        adressunabhängige Code-Regionen + Einstiegstabelle
//
// Der Chunk ist auch in der Maschinencode-Datei enthalten - als Daten. Die
// Maschinencode-Regionen greifen ausschließlich auf die Plattform-objektiven
// Instruktionen zu; jeder Einstieg passt auf einen geprüften IP des Chunks.
// Der generierte Code ruft niemals Go-Funktionen auf und enthält keine
// absoluten Adressen des erzeugenden Prozesses (Stub-Offsets gelten
// innerhalb des Code-Blocks und werden beim Install verdrahtet).
package jit

import (
	"compress/gzip"
	"encoding/gob"
	"fmt"
	"io"
	"runtime"
	"strings"

	"vbxc/bytecode"
)

// Magic und Version der Maschinencode-Datei. Version 3 speichert die nötigen
// CPU-Features, damit AVX/FMA- und SSE4.1-Code nicht auf einer inkompatiblen
// amd64-CPU ausgeführt wird. Ältere Versionen werden abgelehnt und müssen mit
// 'vbxc build windows|linux' neu erzeugt werden.
const (
	machineFileMagic   = "VBXM"
	machineFileVersion = 3
)

const (
	cpuFeatureSSE41 uint32 = 1 << iota
	cpuFeatureAVX
	cpuFeatureFMA
)

type machineFile struct {
	Version int    // machineFileVersion
	GOOS    string // Ziel-Betriebssystem
	GOARCH  string // Ziel-Architektur (der Codegenerator erzeugt amd64)
	CPU     uint32 // erforderliche Instruktionsfeatures (cpuFeature*)
	Chunk   *bytecode.Chunk
	Code    []byte
	Entries []SavedEntry
	Stats   Stats
}

// WriteMachineFile übersetzt den Chunk in Maschinencode und schreibt ihn als
// Maschinencode-Datei nach w. goos/goarch ist die Zielplattform (nur als
// Header geführt; der Code selbst ist amd64). ok=false, wenn nichts
// übersetzt wurde.
func WriteMachineFile(w io.Writer, goos, goarch string, c *bytecode.Chunk) (Stats, bool, error) {
	sp, ok := Translate(c, 0)
	if !ok {
		return Stats{}, false, fmt.Errorf("Es konnte kein Maschinencode übersetzt werden (Plattform oder Code nicht unterstützt)")
	}
	if _, err := io.WriteString(w, machineFileMagic); err != nil {
		return Stats{}, false, err
	}
	zw, err := gzip.NewWriterLevel(w, gzip.BestCompression)
	if err != nil {
		return Stats{}, false, err
	}
	enc := gob.NewEncoder(zw)
	m := machineFile{
		Version: machineFileVersion,
		GOOS:    goos,
		GOARCH:  goarch,
		CPU:     requiredCPUFeatures(c),
		Chunk:   c,
		Code:    sp.Code,
		Entries: sp.Entries,
		Stats:   sp.Stats,
	}
	if err := enc.Encode(m); err != nil {
		return Stats{}, false, err
	}
	return sp.Stats, true, zw.Close()
}

// machineHeader prüft die Zielplattform einer Maschinencode-Datei gegen die
// laufende. Der Code läuft nur auf der Plattform, für die er erzeugt wurde
// (die .vbb ist damit pro Ziel-OS/Arch eine eigene Datei).
func machineHeader(m *machineFile) error {
	if m.GOOS == runtime.GOOS && m.GOARCH == runtime.GOARCH {
		const knownCPUFeatures = cpuFeatureSSE41 | cpuFeatureAVX | cpuFeatureFMA
		if m.CPU&^knownCPUFeatures != 0 {
			return fmt.Errorf("Maschinencode-Datei verlangt unbekannte CPU-Features: %#x", m.CPU&^knownCPUFeatures)
		}
		if !hostSupportsCPUFeatures(m.CPU) {
			return fmt.Errorf("Maschinencode-Datei benötigt CPU-Features, die auf diesem Rechner fehlen (Anforderung: %#x)", m.CPU)
		}
		return nil
	}
	return fmt.Errorf(
		"Datei enthält %s/%s-Maschinencode und läuft nicht auf %s/%s",
		m.GOOS, m.GOARCH, runtime.GOOS, runtime.GOARCH,
	)
}

// requiredCPUFeatures ermittelt die Instruktionssätze, die der Generator für
// diesen Chunk tatsächlich freischalten konnte. Maschinencode-Dateien sind
// damit nicht nur an GOOS/GOARCH, sondern auch an ihre CPU-Anforderungen
// gebunden.
func requiredCPUFeatures(c *bytecode.Chunk) uint32 {
	var required uint32
	for _, code := range codeArrays(c) {
		for _, in := range code {
			if in.Op != bytecode.OpCall || in.A < 0 || int(in.A) >= len(c.Names) {
				continue
			}
			name := c.Names[in.A]
			if nativeMathUnaryCall(name, in.B) {
				switch strings.ToLower(name) {
				case "math.floor", "math.ceil", "math.trunc", "math.int":
					required |= cpuFeatureSSE41
				case "math.exp":
					required |= cpuFeatureAVX | cpuFeatureFMA
				}
			}
			if nativeMathRoundCall(name, in.B) && strings.EqualFold(name, "math.roundbank") {
				required |= cpuFeatureSSE41
			}
		}
	}
	return required
}

// LoadRunFile liest eine Maschinencode-Datei. Alte portable VBXB-Dateien
// werden absichtlich abgelehnt: vbxr führt nur vorab erzeugten Maschinencode
// aus und übersetzt keine Dateien mehr beim Start.
func LoadRunFile(r io.Reader) (*bytecode.Chunk, *Program, error) {
	head := make([]byte, 4)
	if _, err := io.ReadFull(r, head); err != nil {
		return nil, nil, fmt.Errorf("keine VBX-Datei (zu kurz): %w", err)
	}
	switch string(head) {
	case machineFileMagic:
		// Das Magic ist hier schon konsumiert: der gzip-Stream beginnt
		// direkt hinter dem Magic-Bereich.
		zr, err := gzip.NewReader(r)
		if err != nil {
			return nil, nil, fmt.Errorf("Maschinencode-Datei hat ein unbekanntes Format, bitte neu mit vbxc build erzeugen")
		}
		defer zr.Close()
		var m machineFile
		if err := gob.NewDecoder(zr).Decode(&m); err != nil {
			return nil, nil, fmt.Errorf("Maschinencode-Datei beschädigt: %w", err)
		}
		if m.Version != machineFileVersion {
			return nil, nil, fmt.Errorf(
				"Maschinencode-Version %d wird nicht unterstützt (erwartet: %d) - bitte neu erzeugen mit 'vbxc build windows|linux <datei.vb>'",
				m.Version, machineFileVersion,
			)
		}
		if m.Chunk == nil || len(m.Code) == 0 {
			return nil, nil, fmt.Errorf("Maschinencode-Datei ungültig: leerer Inhalt")
		}
		if err := m.Chunk.Validate(); err != nil {
			return nil, nil, fmt.Errorf("Maschinencode-Datei ungültig: %w", err)
		}
		if err := machineHeader(&m); err != nil {
			return nil, nil, err
		}
		p, err := Install(m.Chunk, SavedProgram{Code: m.Code, Entries: m.Entries, Stats: m.Stats}, 0)
		if err != nil {
			return nil, nil, fmt.Errorf("Maschinencode-Datei ungültig: %w", err)
		}
		return m.Chunk, p, nil
	case "VBXB":
		return nil, nil, fmt.Errorf("portable Bytecode-Dateien werden nicht mehr unterstützt; bitte mit 'vbxc build windows|linux <datei.vb>' neu bauen")
	default:
		return nil, nil, fmt.Errorf("keine VBX-Maschinencode-Datei (Kennung fehlt)")
	}
}
