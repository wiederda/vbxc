# Bytecode-Design (Phase 1 bis 3)

Pakete: `value` (Value/Kind + Operator-Semantik), `bytecode` (Op, Instr, Chunk,
Disassembler), `compiler` (AST -> Chunk), `vm` (Environment + Run-Schleife),
`stdlib` (Builtin-Registry und portable Basisfunktionen), `evaluator`
(Ast-Walk für direktes Ausführen von `.vb`).

CLI: `vbxc dis datei.vb` zeigt den Bytecode, `vbxc datei.vb` führt es aus
(`.vb` über den Evaluator, `.vbb` über die VM).

## Zwei Ausführungswäge

```
datei.vb   -> lexer -> parser -> evaluator (Ast-Walk)    kein Schreibzugriff
datei.vb   -> lexer -> parser -> compiler -> vm          nur mit `vbxc build`
datei.vbb  -> bytecode laden -> vm                       (oder direkt starten)
```

`vbxc <datei.vb>` läuft über den Evaluator, also über denselben Ast-Walk wie
der Interpreter `vbx`: der Quelltext wird geparst und sofort ausgewertet,
ohne Umweg über Bytecode und ohne eine Datei zu schreiben. Erst `vbxc build`
schreibt `.vbb`. Ein eigener Ausführungs-Befehl (`run`) gibt es nicht mehr.

## Format
- `Instr{Op, A, B, C int32}`, feste Breite. Namen (`Chunk.Names`) und Literale
  (`Chunk.Consts`) stehen in eigenen Tabellen.
- `Chunk.Pos[i]` gehört zu `Code[i]` (Datei-Index + Zeile), Fehler melden Zeile
  und Dateiname, auch aus Include-Dateien.

## Fehlermodell (wie evalExpr/evalSingleStatement)
- Ausdrücke liefern Fehler als WERT. Erst Anweisungen machen daraus einen
  Skript-Fehler: `OpCheck` bricht `Run` ab (Phase 4: springt zum Catch).
- `OpJumpIfError A,B`: liegt oben ein Fehlerwert, werden B Werte darunter
  verworfen und nach A gesprungen. Damit wird die rechte Seite eines Ausdrucks
  (und spätere Argumente) nicht mehr ausgewertet, wenn links schon ein Fehler
  steckt. Das entspricht dem frühen `return` im Tree-Walker.
- `IsError`/`ErrorText` sind Compiler-Sonderfälle (nur erstes Argument wird
  ausgewertet), `Cls` ein eigener Opcode.
- Unäres `Not`/`-` prüfen, wie im Original, nicht auf Fehlerwerte.

## Namensbasierte Variablen
`Load/Store/Declare` arbeiten über `Environment` (Name -> *Value), genau wie
der Interpreter. Slots kommen erst, wenn Messungen es rechtfertigen.

## For-Schleife
Während der Schleife liegen `[aktuell, ende]` auf dem Stack. Das Ziel von
`Exit For` und das normale Ende ist dasselbe `POPN 2`. `compiler.depth` zählt
solche Stack-Temporaries mit, damit `Exit`/`Continue` aus verschachtelten
Konstrukten (später: Select) sauber aufräumen.

## Bytecode-Dateien (.vbb)
`vbxc build datei.vb [-o ziel.vbb]` schreibt den Chunk (Magic "VBXB", Version,
dann gob). Ohne Befehl und `dis` nehmen `.vb` oder `.vbb`. Includes sind beim
Kompilieren eingebettet, die Datei läuft ohne die Quelldateien. Beim Laden und
am Ende jedes Kompilierens prüft `Chunk.Validate`, dass alle Operanden und
Sprünge auf existierende Ziele zeigen. Die Endung steht an einer Stelle
(`bytecode.Extension`). `#use`-Module stehen in `Chunk.Modules`, die VM lädt sie
noch nicht.

## Eigenständige Programme
`vbxc build` erzeugt ausschließlich Bytecode-Dateien (`.vbb`). Die Runtime
`vbxr` führt diese Dateien aus. Ein Packaging des Bytecodes als EXE wird nicht
unterstützt.

## Trennung Compiler / Runtime
Abhängigkeiten laufen in eine Richtung: `compiler -> bytecode <- vm`. Das
Paket `vm` importiert weder Lexer noch Parser noch Compiler
(`go list -deps ./vm | grep vbxc`). `cmd/vbxr` ist die reine Runtime (nur VM +
stdlib) und kann als Stub dienen. Das ist eine zusätzliche `main`, kein Umbau.

## Absicherung
`OpHalt` prüft, dass der Stack leer ist. Ein Rest ist ein Compiler-Fehler und
wird als "interner Fehler" gemeldet.

## Phase 2: Arrays, For Each, Select Case

**Arrays.** `Dim`/`Public` mit Größe: `OpDimArray`/`OpPublicArray` (Größen vorher
auf dem Stack). Zuweisung `a(i) = x` hält die Reihenfolge des Interpreters ein:
`ARRCHECK` (deklariert? Const?) -> Wert -> Index (`TOINDEX`) -> zweiter Index
nur, wenn die Variable ein 2D-Array ist (`ARRGUARD2D` überspringt ihn sonst) ->
`ARRSTORE`. 1D-Arrays wachsen beim Schreiben über das Ende hinaus.

**For Each.** Auf dem Stack liegen `[iterierbar, index, schlüssel-liste]`. Bei
Maps ist die Liste die sortierte Schlüsselliste (Snapshot), den Wert liest
`EACHNEXT` bei jedem Durchlauf frisch aus der Map. Schleifenvariablen werden
schon vor der Typprüfung angelegt, wie im Interpreter.

**Select Case.** Zwei Wege wie im Interpreter:
- Sind ALLE Bedingungen reine Zahl-/String-Literale, baut der Compiler eine
  `SelectTable` (Chunk.Selects). `SELECT` schlägt den Zweig nach; Bool-Werte
  fallen davon aus und laufen über den Scan.
- Sonst (Range, Is, Variablen, Ausdrücke, Bool/Null-Literale, `-1`) läuft der
  lineare Scan: `DUP`, Bedingung, `CASEEQ`/`CASERANGE`/`CASEIS`.
Der Prüfwert liegt nur während des Scans auf dem Stack und wird vor dem Sprung
in einen Körper entfernt.

## Phase 3: Sub und Function

Jede Sub/Function ist eine eigene Einheit (`Chunk.Funcs[i]` mit eigenem Code);
Namen, Konstanten und Dateien teilt sie sich mit dem Chunk. Ein Aufruf ist
`OpCall` wie bei Builtins, die VM löst den Namen zur Laufzeit auf, in der
Reihenfolge des Interpreters: Variable (Array/Map/Objekt) -> Builtin -> Function
-> Sub. Ein Builtin gleichen Namens verdeckt also eine eigene Function.

- **Scope:** Der Körper läuft in einer neuen `Environment`, deren Parent das
  Root ist (nicht der Aufrufer). `fnReturn` und `currentFuncName` liegen
  wie im Interpreter in der Environment.
- **Rückgabewert:** `Name = x` in einer Function ist `OpSetReturn` (der Compiler
  kennt den Namen, auch vor `Dim Name = x`), `Return x` ist `OpReturn 1`.
  Ohne Zuweisung liefert eine Function 0, eine Sub immer 0.
- **Optionale Parameter:** Standardwerte stehen als Code im Prolog
  (`JMPARG`, Ausdruck, `PARAMDEF`) und werden in der Umgebung der Funktion
  ausgewertet. Ein Fehlerwert beendet den Aufruf mit genau diesem Fehlerwert.
- **Fehler im Körper:** kommen als Fehlerwert "Fehler in Funktion 'x': ..." beim
  Aufrufer an (Zeilen im Körper gehen dabei wie im Interpreter verloren, der
  Aufrufer meldet seine eigene Zeile).
- **Registrierung:** `Chunk.Preregister` (oberste Ebene, wie `registerFuncsAndSubs`)
  UND `OpDefine` an der Definitionsstelle (wie das Ausführen von `Function ...`).

## Bewusste Abweichungen vom Interpreter
- `Dim a(-5)` und absurd große Arrays (> 2^28 Zellen) geben einen Skript-Fehler
  "Ungültige Array-Größe" statt einer Go-Panic.
- Aufruftiefe auf 10000 begrenzt (`maxCallDepth`): Endlosrekursion endet mit
  einem Skript-Fehler statt im Go-Stack-Overflow.
- Pflichtparameter-Anzahl gilt auch für Definitionen in Blöcken (im Interpreter
  nur für vorab registrierte).

## Noch nicht umgesetzt
- Phase 4: `Try/Catch/Finally`

## Paket `evaluator` (Direktes Ausführen einer .vb-Datei)

Das Paket ist 1:1 aus dem Interpreter übernommen (`evaluator.go` aus `vbx`,
`runfile.go`, `debug.go`), angebunden über `evaluator/bridge.go`.

**Pipeline** (`runfile.go`): Pfad absichern -> Endung prüfen -> lesen ->
`isBinary` (Null-Byte in den ersten 512 Bytes) -> `looksLikeScript`
(UTF-8, HTML/XML/PDF-Präfixe, GitHub-Contents-API-Antwort) -> `#requires`
prüfen -> `#use` rausschneiden -> parsen (`parser.ParseSource` inkl. Includes)
-> `registerFuncsAndSubs` -> `evalStatements`. Ausführung unter
`stdlib.RunWithShutdownHook` (Ctrl+C stoppt OBS-Aufnahmen wie im Interpreter).

**`#requires`** wird nur geprüft, wenn die vbxc-Version numerisch ist
(`isNumericVersion`); Dev-Bauten tragen "dev" und überspringen den Check.

**Bridge** (`bridge.go`): In `vbx` lagen AST, Token, Werte und Builtins in
einem Package, in vbxc in `ast`, `token`, `value` und `stdlib`. Aliase
(`Expr`, `AssignNode`, `Kind`, ...) und Wrapper (`NumVal`, `toNumVal`,
`isTruthy`, `requiredParams`, ...) halten `evaluator.go` im Interpreter-Stil.

**AST bleibt zustandsfrei.** Die Cache-Felder des Interpreters (Ziel-Environment
und Value-Pointer an Assign/CompoundAssign/VarNode, Constant-Folding an BinOp,
Dispatch-Tabellen am SelectNode) liegen in Neben-Maps (`nodeCaches`,
`selectDispatches`, Schlüssel ist der Ast-Knoten). Semantik identisch, der
Ast aus `ast/` bleibt reine Datenstruktur.

**Abweichungen vom Interpreter-Code (nur dort, wo das AST es verlangt):**
- `case Value:` durch `*BoolNode`/`*NullNode` ersetzt (Bool/Null-Literale sind
  eigene Knoten, kein Wert im Ast).
- Tote Cases `*CallNode`/`*MultiStmtNode` in `evalExpr` entfernt (beide sind
  Anweisungen, keine Ausdrücke).
- `VarNode.Index1/Index2` entfallen: der Parser setzt sie nie, Array-/Map-
  Zugriffe laufen über `CallExprNode`; der tote Block und `checkArrayIndex`
  sind raus.
- `fn.RequiredParams` -> `requiredParams(fn.Params)` (Wert wird aus der
  Parameterliste berechnet statt am Knoten gespeichert).

**Logging:** `Print` schreibt bei offenem Debug-Log über `stdlib.ActiveLog()`
mit Zeitstempel in die Datei (wie `logFile` im Interpreter); `Dump()` liefert
den Scope-Stapel (`dumpEnvironment`).

## Bytecode-Dateien (.vbb)
`vbxc build datei.vb [-o ziel.vbb]` schreibt den Chunk (Magic "VBXB", Version,
dann gob). Ohne Befehl und `dis` nehmen `.vb` oder `.vbb`. Includes sind beim
Kompilieren eingebettet, die Datei läuft ohne die Quelldateien. Beim Laden und
am Ende jedes Kompilierens prüft `Chunk.Validate`, dass alle Operanden und
Sprünge auf existierende Ziele zeigen. Die Endung steht an einer Stelle
(`bytecode.Extension`). `#use`-Module stehen in `Chunk.Modules`, die VM lädt sie
noch nicht.

## Absicherung
`OpHalt` prüft, dass der Stack leer ist. Ein Rest ist ein Compiler-Fehler und
wird als "interner Fehler" gemeldet.

## Phase 2: Arrays, For Each, Select Case

**Arrays.** `Dim`/`Public` mit Größe: `OpDimArray`/`OpPublicArray` (Größen vorher
auf dem Stack). Zuweisung `a(i) = x` hält die Reihenfolge des Interpreters ein:
`ARRCHECK` (deklariert? Const?) -> Wert -> Index (`TOINDEX`) -> zweiter Index
nur, wenn die Variable ein 2D-Array ist (`ARRGUARD2D` überspringt ihn sonst) ->
`ARRSTORE`. 1D-Arrays wachsen beim Schreiben über das Ende hinaus.

**For Each.** Auf dem Stack liegen `[iterierbar, index, schlüssel-liste]`. Bei
Maps ist die Liste die sortierte Schlüsselliste (Snapshot), den Wert liest
`EACHNEXT` bei jedem Durchlauf frisch aus der Map. Schleifenvariablen werden
schon vor der Typprüfung angelegt, wie im Interpreter.

**Select Case.** Zwei Wege wie im Interpreter:
- Sind ALLE Bedingungen reine Zahl-/String-Literale, baut der Compiler eine
  `SelectTable` (Chunk.Selects). `SELECT` schlägt den Zweig nach; Bool-Werte
  fallen davon aus und laufen über den Scan.
- Sonst (Range, Is, Variablen, Ausdrücke, Bool/Null-Literale, `-1`) läuft der
  lineare Scan: `DUP`, Bedingung, `CASEEQ`/`CASERANGE`/`CASEIS`.
Der Prüfwert liegt nur während des Scans auf dem Stack und wird vor dem Sprung
in einen Körper entfernt.

## Bewusste Abweichungen vom Interpreter
- `Dim a(-5)` und absurd große Arrays (> 2^28 Zellen) geben einen Skript-Fehler
  "Ungültige Array-Größe" statt einer Go-Panic.

## Noch nicht umgesetzt
- Phase 3: `Sub`/`Function`/`Return`/`Exit Sub|Function`, Call-Frames, optionale Parameter
- Phase 4: `Try/Catch/Finally`
- `Dump` und die Log-Datei bei `Print` liegen inzwischen im Paket `evaluator`
  (dort laufen beide Features, die VM kennt sie weiterhin nicht)

## Portierte Builtins
- `value/helpers.go`: nur `GetKindName` (liefert `Kind.String()`; betrifft nur die
  Meldung von `RequireNumber`). Alle anderen Helfer sind die Originale aus dem Interpreter.
- `stdlib/global.go`: globale Funktionen, darunter Hashes, URL/Base64, Prüfungen,
  Systeminformationen, Zwischenablage und Worker.
- `Build` und `IsCompiled` arbeiten mit dem Bytecodeformat `.vbb` (`vbx` verwendete
  dafür `.vbc`, das gibt es hier nicht). `CanRun` kompiliert und prüft Syntax, führt
  aber keine zusätzliche Sicherheitsanalyse unbekannter Aufrufe durch.
- `stdlib/core.go`: häufige Text-, Konvertierungs- und Mathematikfunktionen.
- `stdlib/array.go`: Funktionen des `array`-Namespaces. Da Builtins in der VM
  Wertkopien erhalten, liefern Add/Insert/Remove/Clear das aktualisierte Array
  zurück; Skripte müssen das Ergebnis zuweisen (z. B. `a = array.Add(a, x)`).
- `stdlib/string.go`: Funktionen des `string`-Namespaces, einschließlich Regex,
  Textformatierung, Vergleichen, Base32 und SQL-Insert-Generierung.
- `stdlib/math.go`: Funktionen des `math`-Namespaces, einschließlich Rundung,
  Wurzeln, Logarithmen, Winkelumrechnung und Prozentrechnung.
- `stdlib/app.go`: alle sieben Funktionen des `app`-Namespaces.
- Die übrigen Namespaces aus dem Interpreter sind noch nicht portiert.

## JIT: Übersetzung in Maschinencode (x86-64)

Paket `jit` (nur `//go:build amd64`), angebunden über `vm.SetJIT`. `vbxr`
übersetzt beim Start einmalig; `vbxc <datei.vb>` (Evaluator) und `vbx`
(Interpreter) kennen den JIT nicht.

```
chunk.Code  -> analyze (Dataflow/Regionen) -> code (x86-64) -> allocExec (W^X)
                                                      |
vm.exec:  entry ip -> Entry.Run -> Maschinencode -> Resume-ip -> weiter interpretieren
```

### Architektur des Pakets

| Datei | Aufgabe |
|-------|---------|
| `analyze.go` | Dataflow: Stack-Tiefe `d[ip]` relativ zum Code-Array; Sprungziele; `supported()` (übersetzbare Opcodes inkl. Übersetzungszeit-Bedingungen wie STEP 0, gültige Namen); `minDepth()` (Operanden dürfen nie unterhalb der Basis liegen); Regionen, Einstiege, Variablen-Slots |
| `asm_amd64.go` | kleiner x86-64-Encoder: rel32-Sprünge, Labels mit Referenzzähler (`refs`, damit ungenutzte Blöcke wegfallen), SSE2 für floats |
| `code_amd64.go` | Codegenerator: Stub (Registerrahmen, Kontext laden), Regionstext pro Opcode, Deopt-Blöcke, Epilog |
| `jit.go` | `ctx`, `Entry`, `Program`, `Prepare`, `Entry.Run` (Aufruf aus Go) |
| `mem_windows.go` / `mem_unix.go` | `allocExec`: RW allokieren, Code kopieren, RX umschalten, Cache flushen – nie eine Datei |
| `mem_stub.go`, `code_other.go` | Fallback (andere Plattformen): kein JIT, identische Semantik |

### Registerbelegung (im generierten Code)

`RBX` = `&ctx`, `R12` = Datenzeiger des Stack-Slices, `R13` = virtuelle Basis
(Basis-Offset aus `ctx`), `R15` = Eingangstiefe × 128, `RBP` = Auflösungs-
tabelle, `R14` = Go-Register `g` (wird gerettet/gepatcht, nie überschrieben).
Scratch: `RAX/RCX/RDX/RSI/RDI`, `XMM0/1` (Operanden), `XMM2` (Null),
`XMM3` (Löschmaske). Slotgröße = `sizeof(value.Value)` = 128 Bytes
(`slotBytes`; `init()` sichert die Größe ab).

### Aufruf (Go-ABI)

Der Maschinencode ist eine `func(*ctx) int32` – erzeugt per Funcval-Trick
(`fv[0] = codeAddr`, `Program` hält `fv` am Leben). `Entry.Run` ruft in Go auf:
Stack-Wachstum **vor** dem Code (danach bleibt der Datenzeiger für das Fenster
stabil), Variablenzellen frisch auflösen, dann CALL. Rückgabe ist der ip, an
dem die Interpretation weitermacht. Der Epilog rettet 8 Register (rbx, rbp,
rdi, rsi, r12, r13, r14, r15) und in umgekehrter Reihenfolge zurück.

### Sicherheitsmodell

1. **Keine lebenden Go-Zeiger.** Der Code schreibt nur `Kind`, `Num` und `Bool`
   (sowie Null-Kind) – das sind keine Zeiger, es braucht keine Write-Barrier.
   Jeder Opcode, der einen Wert mit Zeiger (Str/Arr/Obj/Map/Fehler/Nil) lesen
   oder schreiben müsste, geht vorher über einen **Deopt-Guard** in die
   Interpretation. Der Guard prüft VOR jeder Schreibung, der Resume-ip ist der
   Opcode selbst – die Interpretation führt ihn also vollständig aus.
2. **Stack-Sicherheit.** `d[ip]` ist statisch bekannt; `Entry.Run` leitet
   `base = tatsächliche Tiefe - d[einstieg]` ab. `base < 0` oder eine Längen-
   Diskrepanz → sofort interpretieren. Slots oberhalb der Eingangstiefe werden
   vor dem Push gelöscht (7× `movdqu` nach `pxor`), damit keine alten Zeiger
   außerhalb des gescannten Bereichs liegen.
3. **Kein asynchrones Preemption im JIT-Code** (Go findet über `findfunc(pc)`
   keine gültige Funktion, ein Stack-Shrink wäre unzulässig). Deshalb
   **Pflicht: Budget-Safepoints an jedem Rückwärtssprung** – `defaultBudget`
   = 100 000, heruntergezählt im Speicher, beim Erreichen Rückgabe an die
   Interpretation, die das Fenster neu startet. Lange Schleifen bleiben so
   immer steuerbar.
4. **Skip-once in `vm.exec`.** Nach einem Fenster wird genau EINE Instruktion
   interpretiert, bevor erneut eingestiegen wird – sonst entstünde eine
   Endlosschleife, wenn der Resume-ip selbst ein Einstieg der Region ist.
5. **Ein Eintrag pro ip:** `TableFor` liefert `map`-artig die Einstiege eines
   Code-Arrays; die Tabelle wird pro `exec`-Call einmalig geholt.

### Grenzen (bewusst)

- allgemeine Funktionsaufrufe, Maps und Try/Catch laufen weiter über die VM; `math.Abs`, `math.Sign`, `math.Sqrt`, `math.Min`, `math.Max`, `math.Floor`, `math.Ceil`, `math.Trunc`, `math.Int`, `math.Mod`, `math.Clamp`, `math.DegToRad`, `math.RadToDeg`, `math.Percent`, `math.PercentOf`, `math.Round` und `math.RoundBank` werden für numerische Argumente nativ ausgeführt (Rundung ab SSE4.1 bitidentisch über `roundsd`, sonst Deopt; `Mod` über `cvttsd2si`+`idiv` mit Deopt der Grenzfälle; `Clamp` als Komposition `Max(lo, Min(x, hi))` aus denselben Min/Max-Entscheidungen mit Deopt vor jedem Slot-Write; Winkel/Prozent mit exakt der stdlib-Operationsreihenfolge, `PercentOf` deoptiert bei Nenner 0/NaN; `Round` als Bit-Spiegelung des Go-Algorithmus, `RoundBank` über `roundsd` Ties-Even - beide nur für Trunc(d)==0, sonst Deopt). `math.Pi()` und `math.E()` pushen ihre Konstanten nativ (überschattete Namen deoptieren)
- **For Each nativ** (`EachInit`/`EachNext`): iterierbar muss ein 1D-Array sein (Maps bauen die Schlüsselliste in Go, 2D kopiert Rows - beides bleibt in der VM), die Loop-Variablen müssen bereits existieren (Anlegen via `GetRef` wäre ein Go-Aufruf - fehlende deoptieren). Pro Element wird nur Num/Bool/Null in die Variable geschrieben (Kind+Num/Bool/Null-Feld); Str-/Arr-/Map-Elemente deoptieren, bevor *etwas* geschrieben wird. Der Zähler (float64(i+1)) wird erst **nach** dem Element-Dispatch fortgeschrieben: jeder Deopt-Pfad kehrt in einen unberührten Zustand zurück, die VM setzt vor `EachNext` fort und würde eine vorgezogene Erhöhung als übersprungenes Element wiederholen
- keine Erzeugung von Strings/Arrays (nur Konkatenations-lose Pfade zählen)
- nicht goroutine-sicher: eine VM pro Prozess (dieselbe Annahme wie bisher)
- `--nojit` / `VBX_JIT=0` schalten ab, `--jit-stats` / `VBX_JIT=stats` zeigen
  Regionen, Einstiege, Code-Bytes und übersprungene Instruktionen

Die geplanten Erweiterungen, Aufwandseinschätzungen und bewussten VM-Fallbacks
stehen in [JIT_ROADMAP.md](JIT_ROADMAP.md).
