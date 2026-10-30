# Änderungen gegenüber dem Interpreter-Frontend

Checkliste für das spätere Zusammenführen. Alles hier steht im Interpreter noch
in der alten Form.

## Phase B: 2D-Matrix-Zugriffe nativ (neu in vbxc)
- Der JIT führt jetzt auch `m(i, j)`-Zugriffe nativ aus, wenn sie numerisch
  bleiben: Lesen (`OpCall` mit B==2) und Schreiben (`OpArrStore` mit B==1)
  erzeugen Maschinencode; alles andere kehrt mit demselben Ergebniswert an
  dieselbe Instruktion zur Interpretation zurück (Trunc der Indizes wie
  `int(...)` in der VM, Grenzprüfungen gegen die gewählte Zeile, Element
  muss bereits Zahl sein - Ersetzung durch Zeiger-haltige Werte bleibt in
  der VM mit Write-Barrier).
- Ein Dataflow-Bug in `jit/analyze.go` wurde nebenbei behoben: `OpEachNext`
  reichte sein Sprungziel (Schleifenende) nicht in die Depth-Arbeitliste -
  danach übersetzte der Analyzer nichts mehr hinter jeder
  `For Each`-Schleife. Jetzt werden beide Nachfolger (Fall-Through und
  Ziel) mit derselben Tiefe eingetragen; alle vm/jit-Paritätstests bleiben
  grün.
- Paritätstests: 2D-Programme über `TestJITParitaet` (schreiben+lesen in
  verschachtelten Schleifen, fraktionale Indizes) und Fehlerfälle über
  `TestJITParitaet2DFehler` (Grenzverletzungen, negative Indizes,
  Nicht-Zahl-Indizes, String-Werte in Zellen, Fehlerwert-Indizes) -
  interpretiert und JIT liefern identische Ausgaben und identische
  Fehlermeldungen.
- Benchmarks (i7-8750H): 1D-Schleifenkranz 0,64 ms vs. 7,4 ms interpretiert
  (~12x); 2D-Matrix 100×100 (schreiben+lesen in verschachtelten Schleifen)
  ~1,5 ms vs. ~13 ms interpretiert (~10x). Vor der nächsten Änderung waren
  es 9,0 ms - der Gewinn kommt aus dem nativen `OpArrGuard2D`.
- Neu nativ: `OpArrGuard2D`. Der Guard vor dem 2D-Store war vorher eine
  Regionsgrenze mitten in jede Write-Schleife (2 Region-Crossings pro
  Inneriteration). Er prüft nur den Kind des Arrays: bei Arr2D läuft es
  ohne Wirkung weiter, sonst schreibt der Maschinencode einen NIL-Platz-
  halter in einen neuen Stack-Slot (GC-sicher, Zeigerfeld korrekt geleert)
  und springt zum `OpArrStore`. Beide Pfade haben dieselbe Tiefen-Semantik
  wie in der VM (Dataflow kennt beides: Fall-Through mit d, Sprung mit d+1).
- Nach den Fixes bleibt in der Coverage-Probe nur noch echte
  Regionsgrenzen (PRINT/HALT/DECLARE/DEFINE/EACH*/DIMARRAY/CONCAT/
  str-CONST) - die Schleifen selbst laufen vollständig nativ.

## Runde 2: native Wert-Gleichheit (OpEq/OpNeq)
- `OpEq`/`OpNeq` werden maschinell entschieden, wenn beide Operanden
  denselben Kind haben: Num exakt (NaN ist ungleich - kein Deopt mehr
  am ucomisd-Parity-Flag), Bool-Feld nur als Byte verglichen (Padding
  bleibt unberührt), Str exakt case-sensitiv wie Go `==`: erst die
  Längen, dann 8-Byte-Chunk-Vergleich, dann Reste-Bytes (0..7) über
  movzx (0F B6 neu im Encoder für die Byte-Loads). Gleiche Kinds ohne
  Fastpath: Null/Nil gleich, alle übrigen (Arr, Map, Obj, ...) nie.
- Gemischte Kinds und Fehlerwerte deopten VOR der Ausführung zurück
  in die VM: dort gilt value.Equal mit ToString/ValuesAreEqual
  ("1" = 1 -> true, Bool-Kandidaten über IsTruthy, Str/Fremd über
  ToString) - Parität per TestProgramme `vergleiche`, `gleichheit-str`,
  `gleichheit-bool-null`.
- Benchmarks (i7-8750H, -benchtime 1000x): gleichheit+vergleiche 0,42 ms
  JIT vs. 6,3 ms interpretiert (~15x); Matrix-Unveränderter Stand
  (~0,77 ms vs. ~7,1 ms).

## Runde 3: Rundungs-Fastpaths (math.Floor/Ceil/Trunc/Int)
- `math.Floor`, `math.Ceil`, `math.Trunc` und `math.Int` sind nativ:
  `roundsd` im SSE4.1-Enkodierer (66 0F 3A 0B /r ib, Modi 1=Floor,
  2=Ceil, 3=Trunc) ist bitidentisch zu Go `math.Floor`/`math.Ceil`/
  `math.Trunc`; NaN, ±Inf und ±0 laufen unverändert durch, `Int ==
  Trunc` (stdlib-Semantik). Ergebnis landet direkt im Operanden-Slot
  (arg = d-1).
- CPU-Fähigkeit wird über klauspost/cpuid/v2 geprüft (`cpuid.SSE4`,
  jetzt direct dependency v2.3.0): ohne SSE4.1 bleiben die Calls in
  der VM; überschattete Variablen deoptieren wie gehabt vor dem Call.
- Parität: TestProgramm `mathe-rundung` (Brüche an der 0, Vorzeichen
  von -0, Grenzbereiche jenseits von 2^31, 100000-Schleife); der
  Native-Proof `TestMatheRundungNative` zeigt je Funktion Regions=1
  und Skipped=2 (nur PRINT/HALT in der VM).
- Benchmark (i7-8750H, 1M-Iterationsloop mit 3 Rundungs-Calls pro
  Durchlauf; -benchtime 10x, count=3): VM-Interpreter ~0,87 s vs. JIT
  ~0,043 s (~20x). e2e über `vbxc build windows` + `vbxr` liefern
  identische Ausgaben.

## Cache-/Maschinencode-Invalidierung (Runde 4)
- neuer Codegen-Fingerprint (jit/fingerprint.go): eine Kanarien-Region
  (Arithmetik, Vergleiche, Bool-Logik, Verzweigung, For-Schleife,
  1D/2D-Array-Zugriff, math.Abs/Sign/Sqrt/Floor/Ceil/Trunc-Aufrufe)
  wird per sync.Once einmal übersetzt; der SHA-256 über Maschinencode
  und Einstiegstabelle fließt in `CacheKey` ein (Prefix jetzt
  "vbxc-jit-cache-v2"). Änderungen an Parser, Compiler oder
  Codegenerator ändern damit automatisch den Key - alte Cache-Dateien
  verfallen ohne manuelles Anheben einer Konstanten, und insb. Code
  aus einer fehlerhaften Generatorfassung (z. B. die OpEq-rbx-Crash-
  Version) kann keinen stale-Auftritt mehr haben.
- belt-and-suspenders: `cacheVersion` 3 -> 4 (Format-Abbruch für
  Codegen-Änderungen, die die Kanarien-Region nicht sichtbar machen)
  und `machineFileVersion` 1 -> 2 (alte VBXM-Dateien werden mit
  Hinweis "bitte neu erzeugen mit 'vbxc build windows|linux ...'"
  abgelehnt statt still geladen).
- neue Tests: Codegen-Determinismus (zweimal Prepare = byteidentisch),
  Fingerabdruck-Stabilität, PrepareCached-Rundreise (Hit + fremd-Key
  = Miss), VBXM-Version-Ablehnung und VBXM-Rundreise.

## Runde 5: Mod-Fastpath (math.Mod)
- `math.Mod` ist nativ (binärer Call analog math.Min/Max): cvttsd2si
  spiegelt `int()` bitidentisch auf amd64, danach `cqo` + `idiv`
  (Quotient verworfen, Rest nach cvtsi2sd zurück als float64). Neue
  Encoder-Hilfen cqo (48 99) und idivR (F7 /7, REX.W).
- Fehler-/Sonderfälle bleiben vollständig in der VM: int(y) == 0 (auch
  y = 0.5 oder -0.5) deoptiert, damit der Fehlerwert "Modulo durch
  0" wie gehabt von der VM erzeugt wird; INT64_min-Ergebnisse der
  Konversion (NaN, Inf, übergrosse Werte - die Go-Konvertierung ist
  dort implementierungsabhängig) und darüber der Hardware-Trap-Fall
  INT64_min % -1 deoptieren ebenfalls. Nicht-Num-Kinds (Str-Parse,
  Objekte) deoptieren wie bei Min/Max.
- Parität: TestProgramm `mathe-mod` (Vorzeichen folgt dem Dividenden,
  Bruchteile werden abgeschnitten, 1e15/1e18/1e20/9.3e18-Konversionen,
  Fehlerfälle, Variablenpfad, 100000-Schleife); Native-Proof
  `TestModNative` zeigt Regions=1, Skipped=2.
- Benchmark (i7-8750H, 1M-Iterationsloop mit math.Mod; -benchtime 10x,
  count=3): VM-Interpreter ~0,26 s vs. JIT ~0,019 s (~13,7x).
  e2e über `vbxc build` + `vbxr` liefern identische Ausgaben.

## Runde 6: ForEach-Fastpath (EachInit/EachNext)
- `OpEachInit`/`OpEachNext` laufen nativ, wenn das iterierbare Objekt ein
  1D-Array ist. EachInit prüft Kind (kArr) und die Existenz beider
  Loop-Variablen (Anlegen via GetRef wäre ein Go-Aufruf - fehlende
  deoptieren), pusht Zähler (Num 0) und Schlüsselliste (Null) für
  1D-Arrays und fällt zur EachNext-Tiefe d+2. EachNext liest den
  Zähler, prüft Grenzen (cvttsd2si wie `int()`, negative/NaN-Zähler
  deoptieren wie in der VM) und schreibt pro Element nur
  Num/Bool/Null in die Loop-Variable (Kind+Num bzw. die 8 Bool-Bytes,
  Null-Kind); Str-/Arr-/Map-Elemente deoptieren entsprechend ihrer
  VM-Behandlung. Die Zwei-Variablen-Form (`For Each k, v In a`)
  schreibt zusätzlich die Schlüsselvariable mit float64(i) nativ.
- Analyzer-Erweiterungen in `jit/analyze.go`: `collectSlots` (EachInit:
  A + B-1; EachNext: B und C-1; immer non-local), `supported()`
  (beide Ops nur mit übersetzungszeit-benennbaren Namen; EachInit nur
  für 1D-Arrays) und `minDepth` (EachInit 1, EachNext 3). Dataflow:
  EachInit erhöht die Tiefe um 2, EachNext trägt sein Sprungziel
  (Schleifenende) und den Fall-Through mit derselben Tiefe ein.
- Zwei Codegen-Bugs beim ersten Anlauf, beide über Maschinencode-Dumps
  und eine Budget-Sonde (Budget 1 erzwingt Rückkehr in die VM an jedem
  Rückwärtssprung) gefunden:
  1. Das `imul` für die Elementadresse zerstörte RAX (i) *vor* der
     cvttsd2si-Spiegelung des Zählers - der Zähler wurde zu
     float64(128*i + 1): der Loop endete spätestens nach zwei
     Iterationen (die dritte liest 129 >= Länge) und die
     Zwei-Variablen-Form lieferte 1284 statt 186. Fix: i liegt in RDI
     (Scratchregister), bevor der Datenzeiger mul'iert wird.
  2. Der Zähler wurde vor dem Element-Dispatch fortgeschrieben -
     Deopt-Pfade (Str-Elemente etc.) kehrten nicht in einen
     unberührten Zustand zurück: die VM setzt vor EachNext fort, liest
     den schon erhöhten Zähler und überspringt damit das aktuelle
     Element (Textelemente begannen bei "bcd" statt "abcd"). Fix:
     Zähler-Schrieb nur auf dem gemeinsamen `done`-Label, das nur
     tatsächlich geschriebene Elemente erreichen; `out` (erschöpft)
     und alle Deopt-Pfade lassen den Zähler unverändert.
- Parität: TestProgramm `each` in `vm/jit_test.go` (13 erwartete
  Zeilen): reine Num-Arrays mit/ohne zwei Variablen, Bool-Array,
  1000-Element-Loop, Exit For, leerer Loop, Str-Elemente (Deopt-Pfad),
  2D-Rows (Deopt-Pfad), Map-Keys (VM-Pfad), gemischte Kinds und
  verschachtelte ForEach-Loops - alle identisch zwischen
  Interpretation und JIT.
- Benchmark (i7-8750H, 1M-Element-Num-Array; -benchtime 10x, count=3):
  VM-Interpreter ~0,338 s vs. JIT ~0,081 s pro Durchlauf (~4,2x für
  den vollständig nativen Schleifenkörper).
  e2e über `vbxc build windows` + `vbxr` liefern identische Ausgaben.

## Runde 7: Winkel-/Prozent-/Konstanten-Fastpaths
- `math.DegToRad`/`math.RadToDeg` (unär), `math.Percent`/`math.PercentOf`
  (binär) und die Konstanten `math.Pi()`/`math.E()` (ohne Argumente)
  laufen nativ, wenn die Argumente Num sind und keine Variable denselben
  Namen überschattet. Die Fastpaths behalten exakt die
  Operationsreihenfolge der Standardbibliothek bei (DegToRad:
  `x * math.Pi / 180`, RadToDeg: `x * 180 / math.Pi`, Percent:
  `p / 100 * v`, PercentOf: `p / t * 100`) - dadurch auch bei
  Rundungen bitidentisch.
- DegToRad/RadToDeg haben keine Sonderfälle (IEEE: NaN/±Inf/±0 laufen
  bei beiden Umrechnungen identisch durch); Percent liefert bei p = NaN
  einfach das NaN-Ergebnis wie die VM, PercentOf deoptiert bei Nenner 0
  oder NaN und überlässt dem VM-Pfad den Fehlerwert
  "Division durch 0 in PercentOf".
- Konstanten-Aufrufe (`nativeMathConstantCall` in `jit/analyze.go`):
  pushen nur Kind+Num auf den Stack; eine gleichnamige
  Variablenzelle existiert/ist belegt -> Deopt (die VM gewinnt
  Überschattungen, wie bei allen Built-ins).
- Kanarien-Region (`jit/fingerprint.go`) erweitert um Mod, Percent,
  PercentOf und Pi()/E(): Codegen-Änderungen an diesen Pfaden
  invalidieren den Laufcache jetzt automatisch über den
  Codegen-Fingerprint.
- Parität: TestProgramm `mathe-winkel-prozent` in `vm/jit_test.go`:
  Konstanten, alle kritischen Winkel/Multiplikationen inklusive
  großer Literale (1e20-Klasse), Rücksprüfung
  `RadToDeg(DegToRad(x))`, Prozent-/Anteil-Fälle, Fehlerwerte
  (Nenner 0, ungültige Str-/Bool-Argumente; Nativ > Deopt über
  `IsError`) und eine 100000-Iterations-Schleife - identische
  Ausgaben mit/ohne JIT.
- Native-Proof `TestWinkelProzentNative` (jit/runding_test.go):
  je Fall Regions=1 und Skipped=2 (nur PRINT/HALT zurück in der VM).
- Benchmark (i7-8750H, 1M-Iterationsloop mit DegToRad + Percent +
  PercentOf; -benchtime 10x, count=3): VM-Interpreter ~0,78 s vs.
  JIT ~0,035 s pro Durchlauf (~22x).
  e2e über `vbxc build windows` + `vbxr` liefern identische Ausgaben.

## Runde 8: Round-/RoundBank-Fastpaths
- `math.Round` und `math.RoundBank` laufen nativ, wenn die Argumente
  Num sind und (bei zweiter Stelle) nachkommastellen-frei zur Null
  runden: die stdlib ist fn(x*pow)/pow mit pow = Pow(10, Trunc(d));
  fuer Trunc(d) == 0 (auch d in (-1,1), z. B. 0.4) ist pow == 1 und
  x*1/1 exakt, das Ergebnis also fn(x). Alles andere (z. B. d == 1)
  deoptiert zur VM, welche dasselbe Pow liefert.
- `Round` ist eine bitgenaue Spiegelung des Go-Algorithmus (Integer-
  Logik auf dem Float-Bit-Muster, keine Naiv-Rundung: die beruechtigte
  0.49999999999999994-Falle der x+0.5-Formel vermieden). Encoder-
  Erweiterungen in `jit/asm_amd64.go`: subRR, notRR, andRI und die
  Shifts shlRI/shrRI (imm8) plus shlRC/shrRC (CL-Shift fuer die
  variable Shift-Weite der Masken `half >> e` und `fracMask >> e`).
- `RoundBank` == math.RoundToEven == IEEE roundToIntegralTiesToEven ==
  roundsd Modus 0 (ein Befehl); Gate über die SSE4.1-Fähigkeit.
  Kanarien-Region (`jit/fingerprint.go`) um beide Calls erweitert -
  Cache-Invalidierung wieder automatisch über den Codegen-Fingerprint.
- Erster Anlauf einen Codegen-Fehler: im RoundBank-Zweig wurde das
  roundsd-Ergebnis nie in den Slot zurueckgeschrieben (Roundbank
  schien den Input "unveraendert" zu liefern); durch das Paritaets-
  Programm sofort offen geworden (Round via movMR-Pfad funktionierte).
- Parität: TestProgramm `mathe-run` in `vm/jit_test.go`: .5-Grenzen
  (halb weg von null), RoundBank-Ties-Even-Grenzen (2.5 -> 2, 3.5 -> 4,
  -2.5 -> -2), Traps 0.5±2^-54, 1234567890123456/.7.5 (Paritaetszweig
  > 2^51), 4503599627370495.5 (2^52-Grenze), Riesen-Ganzzahl (echt
  unverändert), Nachkommastellen-Formen (Deopt-Pfad, identisches Pow-
  Ergebnis) und eine 100000-Iterations-Schleife - identische Ausgaben.
- Native-Proof `TestRoundNative` (jit/runding_test.go): je Fall und
  ein-/zweistellige Form Regions=1, Skipped=2.
- Benchmark (i7-8750H, 1M-Iterationsloop mit Round + RoundBank;
  -benchtime 10x, count=3): VM-Interpreter ~0,58 s vs. JIT ~0,030 s
  pro Durchlauf (~19,5x).
  e2e über `vbxc build windows` + `vbxr` liefern identische Ausgaben.

## Runde 9: Clamp-Fastpath
- `math.Clamp` läuft nativ mit genau drei Num-Argumenten: die stdlib
  ist math.Max(lo, math.Min(x, hi)) - der Native-Pfad ist eine
  Komposition aus denselben Min/Max-Vergleichen wie die 2-Arg-Formen
  (Runde 3/4), also ohne neue Mathe-Logik: Min(x, hi) in xmm0
  (ccB/ccA/ccP wie die Vorlage, Equal-Zeros via OR = -0 gewinnt),
  danach Max(m, lo) gegen lo im Speicher (AND = +0 gewinnt).
- Deopt-Disziplin: alle Guard-Typchecks (Namenszelle, 3x Kind==Num)
  und beide NaN-Faelle (ccP) deopten, bevor irgendein Slot
  beschrieben wird - ein Deopt nach der Min-Teilrechnung darf die
  Args-Zellen (d-3..d-1) nicht veraendert haben (Paritaet: die VM
  re-executed den Call aus denselben Zellen).
- Str-Argumente deoptieren (mathArg-Konvertierung bleibt VM-Pfad);
  bei lo = NaN liefern beide Pfade NaN, weil der zweite ccP-Deopt
  vor jedem Write faellt.
- Kanarien-Region (`jit/fingerprint.go`) um `math.Clamp` erweitert -
  Cache-Invalidierung wieder automatisch ueber den Codegen-Fingerprint.
- Paritaet: TestProgramm `mathe-clamp` in `vm/jit_test.go`: beide
  Grenzfaelle (x<lo, x>hi), +-0-Formen inkl. vertauschter Grenzen
  (Min schraenkt auf das Intervall ein), eps-Trap an 1 (2^-54),
  Str-Konvertierung ueber Deopt ("7" -> 5), IsError-Paritaet fuer
  ungueltige Argumente und eine 100000-Iterations-Schleife -
  identische Ausgaben. NaN ist im Quelltext nicht konstruierbar
  (keine Exponent-Literale); die Deckung laeuft ueber die identischen
  ccP-Deopts der Min/Max-Teilpfade.
- Native-Proof `TestClampNative` (jit/runding_test.go): Regions=1,
  Skipped=2 (nur PRINT/HALT in der VM).
- Benchmark (i7-8750H, 1M-Iterationsloop mit Clamp
  (i*0.001, 100, 900); -benchtime 10x, count=3): VM-Interpreter
  ~0,39 s vs. JIT ~0,023 s pro Durchlauf (~17,5x).
  e2e ueber `vbxc build windows` + `vbxr` liefern identische Ausgaben.

## Maschinencode-Dateien (.vbb "VBXM", neu in vbxc)
- `vbxc build <datei.vb> -t windows|linux` erzeugt eine Maschinencode-Datei
  ("VBXM"-Format, amd64): Chunk (als Daten für nicht-nativ übersetzte
  Instruktionen, Disassembly, Module), adressunabhängige Code-Regionen und
  Einstiegstabelle in einer Datei mit Plattform-Header. Ohne -t entsteht
  weiterhin die portable Bytecode-Datei ("VBXB").
- Ladezeit über neues `jit.LoadRunFile`: erkennt das Format am Magic,
  prüft beim "VBXM"-Format die Plattform ("Datei enthält windows/amd64-
  Maschinencode und läuft nicht auf ...") und richtet das Programm per
  `jit.Install` ein (Einstiege gegen den Chunk geprüft, Stub-Ziele
  verdrahtet; der Code ruft niemals Go-Funktionen auf und enthält keine
  absoluten Adressen des erzeugenden Prozesses).
- `vbxc datei.vbb` und `vbxr` laufen bei VBXM-Dateien direkt aus der Datei:
  keine Übersetzung zur Laufzeit, keine `.vbb.cache`-Datei. `--nojit`
  interpretiert den eingebetteten Chunk (identische Semantik).
- `vbxc dis` zeigt bei Maschinencode-Dateien die Codezustand-Statistik und
  lädt den Chunk aus der Datei.
- Jat-Typen exportiert (`jit.SavedEntry`/`jit.SavedProgram` + `Translate`
  /`Install`/`Saved`), Cache-Version auf 3 (alte `.vbb.cache`-Dateien
  ungültig, werden neu geschrieben). Round-trip- und Plattform-Mismatch-
  Tests in `vm/machine_test.go`.

## WASM-Plugins (neu in vbxc)
- Neues Paket `plugin`: lädt `plugins/<namespace>.wasm` (Suchpfad
  `VBX_PLUGIN_PATH` oder `plugins/` neben der Exe) und registriert die
  beschriebenen Funktionen in der stdlib-Registry. Portierte Plugin-Dateien
  des Interpreters: `neu/loadwasm.go`, `neu/modules.go` (bleiben als Referenz
  hinter `//go:build vbx_legacy_main`).
- Bindung auf beiden Ausführungswegen: direkt interpretiert (`vbxc
  skript.vb` -> evaluator lädt die `#use`-Module via `plugin.LoadModules`)
  und aus Bytecode (`vbxc build` schreibt `Chunk.Modules` mit gob mit,
  `vm.Run` lädt sie am Anfang von `Run` -> gültig für `vbxc datei.vbb`
  und `vbxr` inkl. JIT). Eingebaute Namespaces überspringt
  `plugin.LoadModules` selbst. Diagnose: `VBX_PLUGIN_DEBUG`.
- `vbxr` erfasst `#use` beim Auto-Kompilieren einer `.vb`-Datei jetzt über
  `blankDirectives` (lokale Kopie des Musters aus cmd/vbxc) und setzt
  `Chunk.Modules`.
- `plugin.FindMissing` prüft ohne Ladeversuch, welche `#use`-Namen als
  Plugin nicht vorliegen; `vbxc build` gibt dazu eine nicht-blockierende
  Warnung (Build- und Ausführmaschine können unterschiedliche
  Plugin-Verzeichnisse haben; Fehlschlag erst zur Laufzeit).
- `stdlib.Register` erhöht eine Generation; der Evaluator aktualisiert
  seine Builtin-Snapshots bei geänderter Generation (nicht pro Aufruf).
- Beispiel-Plugin: `examples/plugin/testplugin` (GOOS=wasip1,
  `go build -buildmode=c-shared`); Testskript `plugins/plugintest.vb`,
  getestet über evaluator, `vbxc datei.vbb` und `vbxr` (.vbb und .vb).

## Struktur
- `package main` aufgeteilt in `token`, `lexer`, `ast`, `parser`.
- `TokenType` heißt jetzt `token.Type`. `Tokenize`/`TokenizeFrom` sind exportiert.
- `token.go`, `lexer.go` und `parser.go` nutzen einen Dot-Import von `token`
  (Konstanten wie `IDENT`, `PLUS` ohne Präfix), damit der Code 1:1 vergleichbar bleibt.

## Token / Lexer
- `Token` hat `Line int`. Block-Kommentare tragen ihre Startzeile.
- `TokenType.String()` ist ein indiziertes Array. `RETURN` und `NOTHING` fehlten
  im alten Switch und lieferten `"TOKEN"`.

## AST
- `Expr` und `Stmt` sind echte Interfaces mit Marker-Methode. Jeder Knoten bettet
  `ast.Loc{File, Line}` ein (`Pos()`).
- Entfernt: alle Interpreter-Caches (`cachedLookupEnv`, `cachedDefEnv`, `cachedPtr`,
  `cachedConst`, `cachedValue`, `constChecked`, `dotChecked`, `isDotted`, `objName`,
  `fieldName`).
- Entfernt, weil der Parser sie nie erzeugt: `ArrayAccessNode`, `VarNode.Index1/Index2`.
  (Im Interpreter prüfen, ob der Evaluator sie irgendwo selbst baut.)
- Entfernt: `ScanResult` (NAS-Überbleibsel), `RuntimeError`, `SubNode/FuncNode.RequiredParams`
  (jetzt `ast.RequiredParams(params)`).
- Neu: `BoolNode`, `NullNode` (vorher `Value{Kind: KindBool}` bzw. `NullVal()` direkt im AST).
- Neu: `IfBranch` statt anonymer Struct.
- `MapIndexNode`, `IsNode`, `RangeNode` sind Teil von `ast`.
- `TryNode` hat `HasCatch`/`HasFinally`. Vorher bedeutete `nil` = "Zweig fehlt", ein
  Catch mit leerem Body (nur Kommentar) war dadurch nicht von "kein Catch" zu
  unterscheiden. Das war die Ursache des Fehlers
  "Ein 'Try'-Block braucht mindestens einen 'Catch'- oder 'Finally'-Zweig".
  Im Interpreter: Prüfung auf `CatchBody == nil` / `FinallyBody == nil` ersetzen.

## Parser
- Kein `*Environment` mehr. `LoadModules`/`ExtractUse` laufen über `parser.Options`
  (`ExtractUse`, `OnModules`).
- Einstieg: `parser.ParseSource(src, opts)` (lext, parst, fängt Panics, gibt `*ParseError`).
- `currentLine()` liest die Zeile aus dem Token statt alle Tokens zu zählen.
- Include-Zustand (`active`, `seen`) liegt in `includeState` pro Lauf statt in globalen
  Maps mit Mutex. Der Include-Cache entfällt (durch Include-once nie ein Treffer im selben Lauf).
- Knoten aus Include-Dateien tragen den Dateinamen in `Loc.File`.
- `parseStmt`: die zweite `NEWLINE`-Schleife nach `skipStuff()` entfällt (redundant).
- Unverändert übernommen (auch die Eigenheiten): Sub-Parser bei Include bekommt `loopDepth`,
  aber nicht `loopStack`/`procStack`.

## Bekannte Eigenheiten, die der Compiler beachten muss
- `CallExprNode` ist Funktionsaufruf ODER Array-Lesezugriff (`arr(i)`): Auflösung erst zur Laufzeit.
- `VarNode.Name` und `CallNode.Name` können Punkte enthalten (`objekt.feld`, `file.Copy`).
- `Dim a, b` und `include` liefern `MultiStmtNode`, der Compiler muss ihn flachziehen.
- `AssignNode{IsDeclaration: false}` ist Zuweisung, `true` ist Dim/Const.

## Evaluator (direktes Ausführen)
- `run` ist kein Befehl mehr: `vbxc <datei.vb>` führt direkt aus, `vbxc build`
  ist der einzige Weg, der `.vbb` schreibt. `.vbb` läuft über die VM, `.vb`
  über das neue Paket `evaluator`.
- `evaluator/` ist 1:1 aus dem Interpreter übernommen (`evaluator.go`,
  `runfile.go`, `debug.go`), angebunden über `evaluator/bridge.go`
  (Aliase/Wrapper nach `ast`, `token`, `value`, `stdlib`).
- Der Ast bleibt zustandsfrei: die Interpreter-Caches (Assign, CompoundAssign,
  VarNode, BinOp, Punkt-Notation) liegen in `nodeCaches sync.Map`, der
  Select-Dispatch-Cache in `selectDispatches sync.Map` (Schlüssel: Ast-Knoten).
- `case Value:` im Evaluator durch `*BoolNode`/`*NullNode` ersetzt; tote Cases
  `*CallNode`/`*MultiStmtNode` in `evalExpr`, toter Block `VarNode.Index1/Index2`
  und `checkArrayIndex` entfernt.
- `fn.RequiredParams` -> `requiredParams(fn.Params)` (= `ast.RequiredParams`).
- Neu in `stdlib`: `ActiveLog() *os.File` (Print-Logging im Evaluator) und
  `RunWithShutdownHook(run func())` (Skriptlauf hängt am Signal-/OBS-Hook).
- `#requires` wird nur geprüft, wenn die Version numerisch ist
  (`isNumericVersion`) – Dev-Bauten (`Version = "dev"`) überspringen den Check.

## JIT (nur `vbxr` + VM, im Interpreter nicht vorhanden)
- Neues Paket `jit`: `vbxr` übersetzt den Bytecode beim Start einmalig in
  echten x86-64-Maschinencode und führt ihn neben der Interpretation aus.
  Es wird dabei **keine Datei** geschrieben (kein .exe/.so/Temp-File); der
  Speicher ist W^X (RW allokieren, kopieren, RX umschalten, Cache flushen).
- Anbindung: `vm.SetJIT(jit.Prepare(chunk, 0))`, `Environment.VarRef/LocalRef`
  als `jit.Env`, Einstiegstabelle pro `exec`-Call, skip-once nach jedem Fenster.
- Sicherheit: nur Kind/Num/Bool- und Null-Schreibungen im Code (keine lebenden
  Go-Zeiger, keine Write-Barrier); Werte mit Zeigern, NaN-Vergleiche, unbekannte
  Variablen und Konstanten-Verletzungen gehen über Deopt-Guards zurück in die
  Interpretation (Resume-ip = der Opcode selbst). Budget-Safepoints (100 000)
  an jedem Rückwärtssprung, weil der Code nicht asynchron gepreemptet werden darf.
- Bewusste Grenzen: keine Funktionsaufrufe, Arrays, Maps, Select Case und
  Try/Catch im Maschinencode – diese Pfade laufen interpretiert (identische
  Semantik). Nicht-amd64 baut über Fallback, dann rein interpretiert.
- Schalter: `vbxr --nojit`, `vbxr --jit-stats`, `VBX_JIT=0|off|no|false|stats`.
- Benchmark (`go test -bench . ./vm/`, 20 000 Schleifeniterationen, i7-8750H):
  5 983 967 ns/op interpretiert gegenüber 435 967 ns/op mit JIT (~13×).
- Tests: `vm/jit_test.go` (Parität JIT vs. interpretiert über 6 Programme,
  Statistik, Benchmarks), `jit/jit_test.go` (W^X-Block führt Maschinencode aus,
  leerer Block wird abgelehnt).
