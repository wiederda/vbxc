# 💿 makemkv.* – Disc-Ripping mit MakeMKV

Dient zum Auslesen von Discs und zum Rippen einzelner Titel über `makemkvcon`.
Der Pfad zu `makemkvcon` wird in dieser Reihenfolge ermittelt: Parameter `executable`, Umgebungsvariable `MAKEMKVCON`, danach `makemkvcon` aus dem `PATH`.

Wichtig: Welche Spuren in der MKV landen und wie Disc- und Titelnamen lauten, bestimmt MakeMKV selbst und nicht VBX. Details dazu stehen im Abschnitt **Einfluss der MakeMKV-Einstellungen** am Ende.

---

## makemkv.GetDiscInfo(drive [, executable])
- **Konkret:**
  Liest die Informationen einer MakeMKV-Disc.
  Der Disc-Name wird unverändert aus der Ausgabe von MakeMKV übernommen (erster Titelname). Seine Sprache hängt von den MakeMKV-Einstellungen ab.
  Zeitüberschreitung nach 5 Minuten.
- **Parameter:**
  - `drive`: MakeMKV-Laufwerksnummer (`disc:N`).
  - `executable`: Optional. Pfad zu `makemkvcon`.
- **Rückgabe:**
  `ArrVal`
  Format: `[DiscName, Drive, DrivePath]`
  `ErrorVal`, wenn das Laufwerk oder der Disc-Name nicht gefunden wird, `makemkvcon` nicht startet oder die Zeit überschritten wird.

---

## makemkv.GetTitles(drive, minDuration, language, subtitle [, executable])
- **Konkret:**
  Liest die Titel einer Disc und filtert sie nach Mindestdauer, Audiosprache und Untertitelsprache.
  Der Filter wählt nur **Titel** aus. Er bestimmt nicht, welche Spuren später in der MKV liegen (siehe Abschnitt am Ende).
  **Sprachfilter:** Der erste Eintrag des Arrays ist Pflicht, alle weiteren sind optional. Mit `{"deu","eng"}` wird ein Titel nur übernommen, wenn er eine deutsche Spur hat. Ein Titel nur mit englischer Spur wird ignoriert. Weitere Einträge beeinflussen die Titelauswahl nicht.
  Ein leeres Array schaltet den jeweiligen Sprachfilter ab. Audio- und Untertitelfilter gelten gemeinsam (UND).
  Sprachcodes sind 3-Buchstaben-Codes wie von MakeMKV geliefert. Aliase: `de`/`deu`/`ger` gelten als `ger`, `en`/`eng` als `eng`.
  Zeitüberschreitung nach 5 Minuten.
- **Parameter:**
  - `drive`: MakeMKV-Laufwerksnummer.
  - `minDuration`: Mindestdauer in **Minuten**.
  - `language`: `ArrVal` mit Audiosprachen, z. B. `{"deu","eng"}`. Der erste Eintrag ist Pflicht.
  - `subtitle`: `ArrVal` mit Untertitelsprachen. Gleiche Regel: Der erste Eintrag ist Pflicht.
  - `executable`: Optional. Pfad zu `makemkvcon`.
- **Rückgabe:**
  `ArrVal`
  Ein Eintrag pro Titel, aufsteigend nach Index sortiert. Leer, wenn kein Titel passt.
  Format je Titel: `[Index, DauerSekunden, GrößeBytes, Name, Playlist, Dateiname, AudioSprachen, UntertitelSprachen, Kapitel]`
  `AudioSprachen` und `UntertitelSprachen` sind `ArrVal` in Stream-Reihenfolge.
  `Index` ist der interne MakeMKV-Titelindex, den `Rip` verwendet.
  `Name` und `Dateiname` stammen von MakeMKV, ihre Sprache hängt von den MakeMKV-Einstellungen ab.

---

## makemkv.Rip(drive, title, outputPath [, executable])
- **Konkret:**
  Rippt einen mit `GetTitles` ermittelten Titel und wartet, bis MakeMKV fertig ist.
  Welche Audio- und Untertitelspuren in die MKV geschrieben werden, entscheidet die Standardauswahlregel von MakeMKV, nicht die Sprach-Arrays aus `GetTitles`.
  Kein Timeout, ein Rip darf beliebig lange dauern.
  Der Zielordner wird bei Bedarf angelegt.
  Die erzeugte Datei wird erkannt, indem der Zielordner vor und nach dem Rip verglichen wird. Wird gleichzeitig in denselben Ordner geschrieben, kann die falsche Datei erkannt werden.
  Windows: `makemkvcon` läuft in einem Job Object. Wird VBX beendet (Strg+C, Kill durch den VBX Editor, Absturz), beendet Windows auch `makemkvcon`. Eine dabei angefangene `.mkv`-Datei bleibt liegen.
- **Parameter:**
  - `drive`: MakeMKV-Laufwerksnummer.
  - `title`: Ein Titel-Array aus `GetTitles`. Verwendet wird nur der Index an Position 0.
  - `outputPath`: Zielordner.
  - `executable`: Optional. Pfad zu `makemkvcon`.
- **Rückgabe:**
  `StrVal`
  Pfad der erzeugten `.mkv`-Datei.
  `ErrorVal` bei ungültigem Titel, leerem `outputPath`, Fehler von `makemkvcon` (mit Exit-Code und den letzten Ausgabezeilen) oder wenn keine neue Datei entstanden ist.

---

## Einfluss der MakeMKV-Einstellungen

`makemkvcon` verwendet die Programmeinstellungen der MakeMKV-GUI. Eine Auswahlregel in einer Profildatei wird seit MakeMKV 1.14.0 nicht mehr gelesen. Daraus folgt:

- **Spuren in der MKV:** Sie richten sich nach der Standardauswahlregel (`app_DefaultSelectionString`), die in der GUI unter den erweiterten Einstellungen steht. Das Dropdown für die bevorzugte Sprache erlaubt nur eine Sprache. Für mehrere Sprachen muss die Regel dort von Hand eingetragen werden, zum Beispiel:
  ```
  -sel:all,+sel:(deu|eng|nolang),-sel:mvcvideo,=100:all,-10:favlang
  ```
  `favlang` steht für die bevorzugte Sprache aus dem Dropdown.
- **Die Regel passt zum Skript:** Fragt ein Skript andere Sprachen an als die Regel erlaubt, fehlen die Spuren in der MKV. `GetTitles` kann das nicht verhindern.
- **Namen:** Disc-Name, Titelname und Dateiname kommen von MakeMKV. Nach einer Änderung der Auswahlregel lautete der Disc-Name plötzlich englisch statt deutsch (`The X-Files` statt `Akte X`). Skripte sollten sich nicht darauf verlassen, dass Namen in einer bestimmten Sprache kommen, und Zielordner im Zweifel selbst benennen.
- **Globale Wirkung:** Eine Änderung der Regel gilt für die GUI und alle Skripte.