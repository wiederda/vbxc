// Package jit übersetzt Bytecode-Regionen in echten Maschinencode (x86-64),
// führt sie in ausführbarem Speicher aus und kann den adressunabhängigen
// Maschinencode für spätere Programmstarts cachen.
//
// Sicherheitsmodell (wichtig, steht über der Optik):
//
//   - Der generierte Code ruft niemals Go-Funktionen auf (kein cgo, kein
//     Trampoline). Fehlerfälle kehren mit einer Resume-IP zurück; die VM
//     führt den Opcode dann interpretiert aus. Semantik identisch.
//   - Maschinencode schreibt NIEMALS einen lebenden Go-Zeiger in den
//     Speicher. Er manipuliert nur Kind (int), Num (float64) und Bool sowie
//     NULL-Werte in den Zeigerfeldern - das ist ein "Drop", für den keine
//     Write-Barrier nötig ist. Werte mit Zeigern (Strings, Arrays, ...)
//     verlassen den JIT über einen Deopt-Guard in die Interpretation.
//   - Der Goroutine-Stack wächst im JIT nicht: Stack-Slots werden nur
//     beschrieben, wenn sie unterhalb der Eingangstiefe liegen (dann ist der
//     Slot von der GC gescannt und die Zeiger gültig), oder vorher komplett
//     geleert. len(vm.stack) bleibt während eines Fensters konstant und wird
//     beim Zurückkehren exakt gesetzt.
//   - R14 (Go-Goroutine-Zeiger) und alle callee-saved Register werden
//     erhalten. Es werden keine Ausrichtungs-/Stack-Probleme erzeugt, da nur
//     Push/Pop von Registern verwendet wird.
//   - An Rückwärtssprüngen wird der Budget-Zähler dekrementiert; bei 0
//     kehrt der Code in die Interpretation zurück. Damit erreicht die Goroutine
//     regelmäßig echte Go-Safepoints (GC kann sie anhalten).
package jit

import (
	"crypto/sha256"
	"encoding/gob"
	"os"
	"path/filepath"
	"runtime"
	"unsafe"

	"vbxc/bytecode"
	"vbxc/value"
)

// slotSize ist die Größe eines value.Value im Speicher. slotBytes ist die
// gleichwertige Konstante für den Codegenerator (int32, wie alle Offsets).
const (
	slotSize  = int(unsafe.Sizeof(value.Value{}))
	slotBytes = int32(slotSize)
)

// Offsets der Felder von ctx - der Codegenerator in code_amd64.go arbeitet
// mit diesen Zahlen. init() sichert sie ab, damit ein Refactor nicht still
// falschen Code erzeugt.
const (
	offCtxStack  = unsafe.Offsetof(ctx{}.stack)
	offCtxBase   = unsafe.Offsetof(ctx{}.base)
	offCtxDepth  = unsafe.Offsetof(ctx{}.depth)
	offCtxBudget = unsafe.Offsetof(ctx{}.budget)
	offCtxVars   = unsafe.Offsetof(ctx{}.vars)
)

// ctx ist der Maschinen-Kontext, den der generierte Code in Register lädt.
type ctx struct {
	stack  *[]value.Value // +0  Slice-Header der VM (data/len/cap)
	base   int64          // +8  Byte-Offset base*slotSize (R13)
	depth  int64          // +16 Byte-Offset des Stacks nach dem Zurückkehren
	budget int64          // +24 Safepoint-Zähler (an Rückwärtssprüngen)
	vars   *varSlot       // +32 Tabelle der aufgelösten Variablenzellen
}

// varSlot ist eine aufgelöste Variablenzelle. flags sagt dem Code, ob der
// Opcode überhaupt ohne Deopt laufen darf.
type varSlot struct {
	ptr   *value.Value
	flags uint64
}

const (
	flagFound = 1 << 0 // Variable existiert in der Scope-Kette
	flagConst = 1 << 1 // als Const markiert (Speichern verboten)
)

// Env ist die Schnittstelle zur VM-Umgebung. vm.Environment implementiert sie.
type Env interface {
	// VarRef liefert die Zelle über die komplette Scope-Kette.
	VarRef(name string) (ptr *value.Value, found, isConst bool)
	// LocalRef liefert nur die Zelle im aktuellen Scope (GetRef ohne Anlegen).
	LocalRef(name string) (ptr *value.Value, found bool)
}

// Entry ist ein Einstiegspunkt in eine generierte Region. Jeder ip, an dem
// die Interpretation in den Maschinencode springen kann, hat ein Entry.
type Entry struct {
	ip     int32            // Bytecode-IP des Einstiegs
	dEntry int32            // statische Stack-Tiefe beim Einstieg (relativ zum Regionsanfang)
	maxRel int32            // maximale Stack-Tiefe in der Region (relativ)
	names  []string         // Variablennamen der Region (parallel zu slots)
	local  []bool           // true: nur aktueller Scope (For-Variablen)
	slots  []varSlot        // Auflösungspuffer (Ein-Thread-Nutzung, s. Package-Doku)
	budget int64            // Safepoint-Budget (aus Prepare)
	fv     []uintptr        // Funcval: hält den Codezeiger am Leben
	fn     func(*ctx) int32 // generierte Funktion (funcval-Trick, nur amd64)
	code   []bytecode.Instr // nur zur Lebensdauer/Diagnose
}

// Run führt die Region aus und liefert den ip, an dem die Interpretation
// weitermacht. Auflösung der Variablenzellen, Stack-Sicherung und der eigentliche
// Aufruf passieren hier in Go - der Maschinencode selbst bleibt rein.
func (e *Entry) Run(env Env, stk *[]value.Value) int32 {
	if len(e.local) != len(e.slots) || len(e.names) != len(e.slots) {
		// interner Fehler beim Bauen der Tabelle: lieber interpretieren
		return e.ip
	}
	s := *stk
	d0 := int64(len(s))
	base := d0 - int64(e.dEntry)
	if base < 0 {
		// Die tatsächliche Tiefe passt nicht zur Analyse (darf nicht
		// vorkommen): lieber interpretieren als Speicher beschreiben.
		return e.ip
	}
	need := base + int64(e.maxRel)
	if need > int64(cap(s)) {
		// Wachstum IN Go, bevor der Code läuft: der Datenzeiger bleibt danach
		// für die Dauer des Fensters stabil.
		grown := make([]value.Value, len(s), need+need/2)
		copy(grown, s)
		s = grown
		*stk = s
	}

	// Variablenzellen frisch auflösen (env kann sich zwischen zwei Fenstern
	// geändert haben). Fehlschläge erzeugen keinen Abbruch - der Opcode
	// sieht das Flag und deoptet.
	for i := range e.slots {
		var f uint64
		if e.local[i] {
			if ptr, found := env.LocalRef(e.names[i]); found {
				f = flagFound
				e.slots[i].ptr = ptr
			}
		} else if ptr, found, isConst := env.VarRef(e.names[i]); found {
			f = flagFound
			if isConst {
				f |= flagConst
			}
			e.slots[i].ptr = ptr
		}
		e.slots[i].flags = f
	}

	var slotsPtr *varSlot
	if len(e.slots) > 0 {
		slotsPtr = &e.slots[0]
	}
	c := ctx{
		stack:  stk,
		base:   base * int64(slotSize),
		budget: e.budget,
		vars:   slotsPtr,
	}
	ip := e.fn(&c)

	// Fensterende: exakte Tiefe zurückschreiben. Die Slot-Inhalte oberhalb
	// sind entweder leer (Zeigerfelder NULL) oder gültig - daher ist das
	// Verkürzen des Slices ohne Write-Barrier sicher.
	*stk = (*c.stack)[:c.depth/int64(slotSize)]
	return ip
}

func init() {
	if unsafe.Sizeof(ctx{}) != 40 {
		panic("jit: unerwartetes ctx-Layout")
	}
	if unsafe.Sizeof(varSlot{}) != 16 {
		panic("jit: unerwartetes varSlot-Layout")
	}
	if slotSize != 128 {
		panic("jit: value.Value ist nicht 128 Byte")
	}
}

// Program ist die Kompilierung eines Chunks: alle Einstiege plus der
// ausführbare Speicherblock. Es ist NICHT für den gleichzeitigen Gebrauch
// aus mehreren Goroutines ausgelegt (die VM hat genau eine Ausführung).
type Program struct {
	tables       map[uintptr][]*Entry // pro Code-Array: ip -> Entry
	mem          *execMem
	budget       int64
	Stats        Stats
	cacheCode    []byte
	cacheEntries []cacheEntry
}

// Stats beschreibt, was beim Start übersetzt wurde (für --jit-info/-v).
type Stats struct {
	Regions   int
	Entries   int
	CodeBytes int
	Skipped   int // nicht unterstützte Instruktionen (Regionsgrenzen)
}

// TableFor liefert die Einstiegstabelle eines Code-Arrays (ip-Indexiert)
// oder nil, wenn dafür nichts übersetzt wurde.
func (p *Program) TableFor(code []bytecode.Instr) []*Entry {
	if p == nil || len(code) == 0 {
		return nil
	}
	return p.tables[uintptr(unsafe.Pointer(&code[0]))]
}

// Close gibt den ausführbaren Speicher frei (Tests, kein Normalbetrieb -
// vbxr hält das Programm bis zum Prozessende).
func (p *Program) Close() {
	if p != nil && p.mem != nil {
		p.mem.free()
		p.mem = nil
		p.tables = nil
	}
}

// defaultBudget: Anzahl der Rückwärtssprünge, nach denen ein Fenster in die
// Interpretation zurückkehrt. Begrenzt, wie lange eine Goroutine im
// Fremdcode bleiben kann (GC/Scheduler müssen sie anhalten können).
const defaultBudget = 1000000 // Auf 1 Million erhöht, um JIT-Overhead zu reduzieren.

// Prepare übersetzt den Chunk in Maschinencode. Auf Plattformen ohne
// Unterstützung (oder ohne kompilierbaren Code) liefert es nil zurück -
// die VM arbeitet dann weiter interpretiert.
func Prepare(c *bytecode.Chunk, budget int64) *Program {
	if c == nil || len(c.Code) == 0 {
		return nil
	}
	if budget <= 0 {
		budget = defaultBudget
	}
	p := &Program{
		tables: make(map[uintptr][]*Entry),
		budget: budget,
	}
	if err := compile(p, c); err != nil {
		return nil
	}
	return p
}

// PrepareCached lädt den JIT-Maschinencode aus cachePath oder übersetzt den
// Chunk einmalig und speichert das Ergebnis. key muss den Inhalt der Quelle
// bzw. der .vbb-Datei abbilden. Cachefehler sind nicht fatal: dann wird normal
// übersetzt und der Cache neu geschrieben.
func PrepareCached(c *bytecode.Chunk, cachePath string, key [32]byte, budget int64) (*Program, bool) {
	if p, err := readCache(c, cachePath, key, budget); err == nil {
		return p, true
	}
	p := Prepare(c, budget)
	if p != nil {
		_ = writeCache(cachePath, key, p)
	}
	return p, false
}

// CacheKey bildet den Quelltext oder Bytecode für die Cache-Invalidierung ab.
// Der Key enthält zusätzlich den Codegen-Fingerprint (CodegenFingerprint):
// Änderungen am Parser, Compiler oder Codegenerator ändern den Key, sodass
// alte Cache-Dateien nicht mehr geladen werden - veralteter Maschinencode
// (insb. aus Codegen-Fehlern, die inzwischen behoben sind) kann nicht mehr
// zum Laufen kommen.
func CacheKey(data []byte) [32]byte {
	h := sha256.New()
	fp := CodegenFingerprint()
	h.Write([]byte("vbxc-jit-cache-v2/" + string(fp[:]) + "/" + runtime.GOOS + "/" + runtime.GOARCH + "/"))
	h.Write(data)
	var key [32]byte
	copy(key[:], h.Sum(nil))
	return key
}

// CachePath legt den Cachenamen fest: neben der Eingabedatei entsteht
// eine einzige Cache-Datei, die die volle Endung der Eingabe behält
// ("prog.vbb.cache" aus "prog.vbb"). Ein Cache einer fremden Plattform passt
// nicht zum OS-abhängigen Cache-Key (CacheKey) und wird daher beim nächsten
// Lauf ungültig und neu geschrieben.
func CachePath(inputPath string) string {
	return inputPath + ".cache"
}

type cacheEntry = SavedEntry

// SavedEntry ist ein serialisierter Einstiegsinsatz (Cache-Datei und
// Maschinencode-Datei). Die Feldnamen sind gob-vertraglich: ältere Cache-
// Dateien besitzen dieselben Felder unter denselben Namen.
type SavedEntry struct {
	ArrayIndex int
	IP         int32
	StubOffset int
	DEntry     int32
	MaxRel     int32
	Names      []string
	Local      []bool
}

type diskCache = SavedProgram

// SavedProgram ist die serialisierbare Form eines Program: adressunabhängiger
// Maschinencode plus Einstiegstabelle. Die Stub-Offsets gelten innerhalb des
// Code-Blocks (bei der Ladezeit verdrahtet); es stecken keine absoluten
// Adressen des erzeugenden Prozesses im Code.
type SavedProgram struct {
	Magic   string
	Version uint32
	Key     [32]byte
	Code    []byte
	Entries []SavedEntry
	Stats   Stats
}

const cacheMagic = "VBXJ"

// cacheVersion 4: Codegen-Runde 2/3 (natives OpEq/OpNeq, SSE4.1-Rundung).
// Zusätzlich verweigert der Cache-Key über den Codegen-Fingerprint
// (fingerprint.go) selbst die Gültigkeit; die Konstante bleibt als Format-
// Abbruch für Änderungen, die der Kanarien-Region nicht sichtbar sind.
const cacheVersion = 4

func readCache(c *bytecode.Chunk, path string, key [32]byte, budget int64) (*Program, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var d diskCache
	if err := gob.NewDecoder(f).Decode(&d); err != nil {
		return nil, err
	}
	if d.Magic != cacheMagic || d.Version != cacheVersion || d.Key != key || len(d.Code) == 0 {
		return nil, os.ErrInvalid
	}
	return install(c, d, budget)
}

// install baut aus serialisiertem Code + Einstiegstabelle ein lauffähiges
// Programm auf. Die Einstiege werden gegen den Chunk geprüft (Indexgrenzen,
// Stub-Offsets, Namens/Lokal-Parität); der Code braucht keine absoluten
// Adressen des erzeugenden Prozesses, die Stub-Ziele werden hier
// verdrahtet.
func install(c *bytecode.Chunk, sp SavedProgram, budget int64) (*Program, error) {
	if len(sp.Code) == 0 {
		return nil, os.ErrInvalid
	}
	arrays := codeArrays(c)
	for _, e := range sp.Entries {
		if e.ArrayIndex < 0 || e.ArrayIndex >= len(arrays) || e.IP < 0 || int(e.IP) >= len(arrays[e.ArrayIndex]) ||
			e.StubOffset < 0 || e.StubOffset >= len(sp.Code) || len(e.Names) != len(e.Local) {
			return nil, os.ErrInvalid
		}
	}
	mem, err := allocExec(sp.Code)
	if err != nil {
		return nil, err
	}
	if budget <= 0 {
		budget = defaultBudget
	}
	p := &Program{tables: make(map[uintptr][]*Entry), mem: mem, budget: budget, Stats: sp.Stats,
		cacheCode: sp.Code, cacheEntries: sp.Entries}
	for _, ca := range arrays {
		if len(ca) > 0 {
			p.tables[uintptr(unsafe.Pointer(&ca[0]))] = make([]*Entry, len(ca))
		}
	}
	for _, ce := range sp.Entries {
		ca := arrays[ce.ArrayIndex]
		key := uintptr(unsafe.Pointer(&ca[0]))
		e := &Entry{ip: ce.IP, dEntry: ce.DEntry, maxRel: ce.MaxRel,
			names: ce.Names, local: ce.Local, slots: make([]varSlot, len(ce.Names)),
			budget: budget, code: ca}
		fv := []uintptr{mem.ptr + uintptr(ce.StubOffset)}
		e.fv = fv
		e.fn = *(*func(*ctx) int32)(unsafe.Pointer(&fv))
		p.tables[key][e.ip] = e
	}
	return p, nil
}

// Saved liefert die serialisierbare Form des Programms (Cache-Datei oder
// Maschinencode-Datei). Liefert ok=false, wenn nichts übersetzt wurde.
func (p *Program) Saved() (SavedProgram, bool) {
	if p == nil || len(p.cacheCode) == 0 {
		return SavedProgram{}, false
	}
	return SavedProgram{Code: p.cacheCode, Entries: p.cacheEntries, Stats: p.Stats}, true
}

// Install richtet ein gespeichertes Programm für einen Chunk ein (Partorama
// des Ladens: Maschinencode-Datei). Die Einstiege werden gegen den Chunk
// geprüft.
func Install(c *bytecode.Chunk, sp SavedProgram, budget int64) (*Program, error) {
	return install(c, sp, budget)
}

// Translate übersetzt einen Chunk ohne ihn auszuführen und liefert die
// serialisierbare Form (Maschinencode-Datei beim Build). ok=false, wenn
// auf dieser Plattform nichts entstanden ist.
func Translate(c *bytecode.Chunk, budget int64) (SavedProgram, bool) {
	return Prepare(c, budget).Saved()
}

func writeCache(path string, key [32]byte, p *Program) error {
	if path == "" || len(p.cacheCode) == 0 {
		return os.ErrInvalid
	}
	d := diskCache{Magic: cacheMagic, Version: cacheVersion, Key: key,
		Code: p.cacheCode, Entries: p.cacheEntries, Stats: p.Stats}
	f, err := os.CreateTemp(filepath.Dir(path), ".vbb.cache*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err = gob.NewEncoder(f).Encode(d); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func codeArrays(c *bytecode.Chunk) [][]bytecode.Instr {
	var arrays [][]bytecode.Instr
	if c != nil && len(c.Code) > 0 {
		arrays = append(arrays, c.Code)
	}
	if c != nil {
		for i := range c.Funcs {
			if len(c.Funcs[i].Code) > 0 {
				arrays = append(arrays, c.Funcs[i].Code)
			}
		}
	}
	return arrays
}
