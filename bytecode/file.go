package bytecode

import (
	"compress/gzip"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
)

// Dateiformat: 4 Byte Magic (unkomprimiert), danach ein gzip-Stream, der per
// gob die Formatversion und den Chunk enthält. Includes sind beim Kompilieren
// schon eingebettet, die Datei ist in sich vollständig.
const (
	Extension     = ".vbb" // eine Stelle für die Dateiendung
	magic         = "VBXB"
	formatVersion = 3 // 3: gzip-komprimiert
)

// Write schreibt den Chunk im Dateiformat nach w.
func Write(w io.Writer, c *Chunk) error {
	if _, err := io.WriteString(w, magic); err != nil {
		return err
	}
	zw, err := gzip.NewWriterLevel(w, gzip.BestCompression)
	if err != nil {
		return err
	}
	enc := gob.NewEncoder(zw)
	if err := enc.Encode(formatVersion); err != nil {
		return err
	}
	if err := enc.Encode(c); err != nil {
		return err
	}
	return zw.Close() // schreibt den gzip-Abschluss, nicht weglassen
}

// Read liest einen Chunk und prüft ihn, bevor er an die VM geht.
func Read(r io.Reader) (*Chunk, error) {
	head := make([]byte, len(magic))
	if _, err := io.ReadFull(r, head); err != nil || string(head) != magic {
		return nil, errors.New("keine VBX-Bytecode-Datei (Kennung fehlt)")
	}

	zr, err := gzip.NewReader(r)
	if err != nil {
		return nil, errors.New("Bytecode-Datei hat ein unbekanntes oder altes Format, bitte neu mit vbxc build erzeugen")
	}
	defer zr.Close()

	dec := gob.NewDecoder(zr)
	var ver int
	if err := dec.Decode(&ver); err != nil {
		return nil, fmt.Errorf("Bytecode-Datei beschädigt: %w", err)
	}
	if ver != formatVersion {
		return nil, fmt.Errorf("Bytecode-Version %d wird nicht unterstützt (erwartet: %d)", ver, formatVersion)
	}

	var c Chunk
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("Bytecode-Datei beschädigt: %w", err)
	}
	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("Bytecode-Datei ungültig: %w", err)
	}
	return &c, nil
}

// Validate prüft, dass alle Operanden auf existierende Einträge zeigen und
// jeder Sprung ein Ziel hat. Das fängt beschädigte Dateien ab und dient dem
// Compiler als Selbsttest (z.B. nicht gepatchte Sprünge).
func (c *Chunk) Validate() error {
	if err := c.validateUnit("Hauptprogramm", c.Code, c.Pos, OpHalt); err != nil {
		return err
	}
	for _, f := range c.Funcs {
		if f.Required < 0 || int(f.Required) > len(f.Params) {
			return fmt.Errorf("%s: ungültige Parameterzahl", f.Name)
		}
		if err := c.validateUnit(f.Name, f.Code, f.Pos, OpReturn); err != nil {
			return err
		}
	}
	for _, idx := range c.Preregister {
		if idx < 0 || int(idx) >= len(c.Funcs) {
			return fmt.Errorf("Vorab-Registrierung verweist auf unbekannte Funktion %d", idx)
		}
	}
	return nil
}

func (c *Chunk) validateUnit(name string, code []Instr, pos []Pos, lastOp Op) error {
	n := len(code)
	if n == 0 || code[n-1].Op != lastOp {
		return fmt.Errorf("%s: Programm endet nicht mit %s", name, lastOp)
	}
	if len(pos) != n {
		return fmt.Errorf("%s: Positionstabelle passt nicht zum Code", name)
	}
	for _, p := range pos {
		if p.File < 0 || int(p.File) >= len(c.Files) {
			return fmt.Errorf("%s: Position verweist auf unbekannte Datei", name)
		}
	}

	for i, in := range code {
		m, ok := meta[in.Op]
		if !ok {
			return fmt.Errorf("%s, Instruktion %d: unbekannter Opcode %d", name, i, in.Op)
		}
		ops := [3]int32{in.A, in.B, in.C}
		for k := 0; k < 3; k++ {
			x := ops[k]
			bad := false
			switch m.operands[k] {
			case 'k':
				bad = x < 0 || int(x) >= len(c.Consts)
			case 'n':
				bad = x < 0 || int(x) >= len(c.Names)
			case 'v':
				bad = x < 0 || int(x) > len(c.Names)
			case 'F':
				bad = x < 0 || int(x) >= len(c.Funcs)
			case 'j':
				bad = x < 0 || int(x) >= n
			}
			if bad {
				return fmt.Errorf("%s, Instruktion %d (%s): Operand %d (%d) ungültig", name, i, m.name, k+1, x)
			}
		}

		if in.Op == OpSelectDispatch {
			if in.A < 0 || int(in.A) >= len(c.Selects) {
				return fmt.Errorf("%s, Instruktion %d: unbekannte Select-Tabelle %d", name, i, in.A)
			}
			tbl := c.Selects[in.A]
			for _, target := range append([]int32{tbl.Default}, tbl.Targets...) {
				if target < 0 || int(target) >= n {
					return fmt.Errorf("%s, Instruktion %d: ungültiges Sprungziel %d in der Select-Tabelle", name, i, target)
				}
			}
		}
	}
	return nil
}
