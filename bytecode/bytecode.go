// Package bytecode definiert das Chunk-Format der VM.
//
// Eine Instruktion hat feste Breite (Op + drei int32-Operanden). Das ist
// bewusst einfach gehalten: gut lesbar, gut zu debuggen, später immer noch
// in ein kompakteres Format überführbar.
package bytecode

import "vbxc/value"

type Op uint8

const (
	OpHalt Op = iota

	// Konstanten und Stack
	OpConst // A = Konstante
	OpNull
	OpTrue
	OpFalse
	OpPop
	OpPopN // A = Anzahl

	// Variablen (namensbasiert, Semantik wie Environment im Interpreter)
	OpLoad          // A = Name; legt Wert ODER Fehlerwert auf den Stack
	OpLoadField     // A = Objektname, B = Feldname ("obj.feld")
	OpLoadForUpdate // A = Name; für "x += ..." (bricht bei undefiniert/Const ab)
	OpDeclare       // A = Name, B = Flags; nimmt Wert vom Stack (Dim/Const)
	OpStore         // A = Name; nimmt Wert (Zuweisung, prüft deklariert/Const)
	OpStoreRef      // A = Name; nimmt Wert (ohne Prüfung, nach OpLoadForUpdate)
	OpSetGlobal     // A = Name; nimmt Wert (Public)

	// Rechnen / Vergleichen: nehmen zwei Werte, legen eins zurück
	OpAdd
	OpSub
	OpMul
	OpDiv
	OpConcat
	OpEq
	OpNeq
	OpLt
	OpGt
	OpLe
	OpGe
	OpCalc // A = token.Type; calculateBinaryOp (für +=, -=, ...)

	// Logik
	OpNot
	OpNeg
	OpTruthy       // Wert -> Bool (Fehlerwerte bleiben Fehlerwerte)
	OpShortCircuit // A = Ziel, B = 1 für Or; And/Or-Kurzschluss

	// Sprünge
	OpJump
	OpJumpIfFalse // pop
	OpJumpIfTrue  // pop
	OpJumpIfError // A = Ziel, B = Anzahl Werte UNTER dem Fehlerwert, die verworfen werden
	OpCheck       // Anweisungsebene: Fehlerwert oben -> Skript-Fehler auslösen

	// Aufrufe
	OpCall      // A = Name, B = Argumentanzahl
	OpIsError   // Sonderfall wie im Interpreter
	OpErrorText // Sonderfall wie im Interpreter

	// Arrays / Maps
	OpMakeArray // A = Anzahl Elemente
	OpMapBase   // A = Ziel; Basis muss eine Map sein, sonst Fehlerwert + Sprung
	OpMapIndex  // pop Key, pop Basis

	// For-Schleife: auf dem Stack liegen [aktuell, ende]
	OpForInit // A = Variablenname, B = Schritt-Konstante
	OpForTest // A = Ziel (Schleifenende), B = Variablenname, C = 1 aufsteigend
	OpForStep // A = Schritt-Konstante

	// Arrays
	OpDimArray    // A = Name, B = Dimensionen (0/1/2), C = 1 wenn in Schleife
	OpPublicArray // A = Name, B = Dimensionen (0/1/2)
	OpArrCheck    // A = Name; Variable muss existieren und darf keine Const sein
	OpToIndex     // Wert -> Index (Zahl) oder Skript-Fehler
	OpArrGuard2D  // A = Name, B = Ziel; kein 2D-Array: Platzhalter pushen + springen
	OpArrStore    // A = Name, B = 1 wenn zweiter Index auf dem Stack liegt

	// For Each: auf dem Stack liegen [iterierbar, index, schlüssel-liste]
	OpEachInit // A = Schlüsselvariable, B = Wertvariable+1 (0 = keine)
	OpEachNext // A = Ziel (Ende), B = Schlüsselvariable, C = Wertvariable+1

	// Select Case
	OpDup            // oberstes Element kopieren
	OpSelectDispatch // A = Tabelle; Wert wird verbraucht (außer bei Bool: dann weiter zum Scan)
	OpCaseEq         // [wert, bedingung] -> Bool (valuesAreEqual)
	OpCaseRange      // [wert, low, high] -> Bool
	OpCaseIs         // A = Operator; [wert, vergleichswert] -> Bool

	// Ein-/Ausgabe
	OpPrint // A = 1 wenn Farbe oben auf dem Stack liegt
	OpCls

	// Sub/Function
	OpDefine       // A = Funktions-Index; registriert die Definition (wie das Ausführen von "Function ..." im Interpreter)
	OpReturn       // A = 1: Wert vom Stack wird Rückgabewert; beendet Function/Sub bzw. das Programm
	OpSetReturn    // Wert vom Stack wird Rückgabewert (Zuweisung "FunktionsName = x")
	OpJumpIfArg    // A = Ziel, B = Parameterindex; springt, wenn dieses Argument übergeben wurde
	OpParamDefault // A = Name; Standardwert vom Stack an den Parameter (ein Fehlerwert beendet den Aufruf)
)

// Flags für OpDeclare
const (
	FlagConst  = 1
	FlagInLoop = 2
)

type Instr struct {
	Op      Op
	A, B, C int32
}

// Pos verweist über den Dateinamen-Index in Chunk.Files auf die Quelle.
type Pos struct {
	File int32
	Line int32
}

// Func ist eine übersetzte Sub oder Function mit eigenem Code. Namen,
// Konstanten und Dateien teilt sie sich mit dem Chunk.
type Func struct {
	Name     string
	IsFunc   bool     // Function (mit Rückgabewert) oder Sub
	Params   []string // Parameternamen; Standardwerte stehen als Code im Prolog
	Required int32    // Anzahl Pflichtparameter
	Code     []Instr
	Pos      []Pos
}

type Chunk struct {
	Name   string
	Code   []Instr
	Pos    []Pos // parallel zu Code
	Consts []value.Value
	Names  []string
	Files  []string

	Selects []SelectTable // Dispatch-Tabellen für Select Case

	Funcs       []Func
	Preregister []int32 // Funktionen, die vor dem Start registriert werden (registerFuncsAndSubs)

	// Module aus "#use": werden im Chunk festgehalten und von vm.Run beim
	// Ausführen nachgeladen (eingebaute Namespaces übersprungen, der Rest
	// muss als WASM-Plugin plugins/<name>.wasm vorliegen).
	Modules []string
}
