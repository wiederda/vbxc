package stdlib

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	valpkg "vbxc/value"
)

// ------------------------------------------------------------
// MakeMKV
// ------------------------------------------------------------

type makeMKVTitle struct {
	Index             int // interner MakeMKV-Titelindex
	Chapters          int
	Duration          int64
	SizeBytes         int64
	Name              string
	Playlist          string
	FileName          string
	AudioLanguages    []string
	SubtitleLanguages []string
}

type makeMKVDisc struct {
	Name  string
	Drive int
	Path  string
}

type makeMKVStream struct {
	TitleIndex int
	Stream     int
	Type       string
	Language   string
}

type makeMKVStreamKey struct {
	TitleIndex int
	Stream     int
}

const makeMKVInfoTimeout = 5 * time.Minute

// ------------------------------------------------------------
// Registrierung
// ------------------------------------------------------------

func initMakeMKVFunctions() {
	makeMKVInitJob()
	if builtins == nil {
		builtins = make(map[string]BuiltinInfo)
	}

	ns := "makemkv."

	// --------------------------------------------------------
	// makemkv.GetDiscInfo(drive, [executable])
	// --------------------------------------------------------
	Register(
		ns+"GetDiscInfo",
		"makemkv",
		"drive, [executable]",
		"Liest die Informationen einer MakeMKV-Disc. Rückgabe: [DiscName, Drive, DrivePath].",
		func(args []valpkg.Value) valpkg.Value {
			if len(args) < 1 {
				return valpkg.ErrorVal("makemkv.GetDiscInfo: drive benötigt")
			}

			drive := int(args[0].Num)

			executable := ""
			if len(args) >= 2 {
				executable = strings.TrimSpace(valpkg.ToString(args[1]))
			}

			info, err := makeMKVDiscInfo(drive, executable)
			if err != nil {
				return valpkg.ErrorVal("makemkv.GetDiscInfo: " + err.Error())
			}

			return valpkg.Value{
				Kind: valpkg.KindArr,
				Arr: []valpkg.Value{
					valpkg.StrVal(info.Name),
					valpkg.NumVal(float64(info.Drive)),
					valpkg.StrVal(info.Path),
				},
			}
		},
	)

	// --------------------------------------------------------
	// makemkv.GetTitles(drive, minDuration, language, subtitle, [executable])
	// --------------------------------------------------------
	Register(
		ns+"GetTitles",
		"makemkv",
		"drive, minDuration, language, subtitle, [executable]",
		"Liest die Titel einer MakeMKV-Disc und filtert nach Mindestdauer (in Minuten), Audiosprache und Untertitelsprache (je Array, leeres Array = kein Filter; z. B. [\"ger\"]). Optional kann der Pfad zu makemkvcon angegeben werden. Rückgabe je Titel: [Index, DauerSekunden, GrößeBytes, Name, Playlist, Dateiname, AudioSprachen, UntertitelSprachen, Kapitel].",
		func(args []valpkg.Value) valpkg.Value {
			if len(args) < 4 {
				return valpkg.ErrorVal("makemkv.GetTitles: drive, minDuration, language und subtitle benötigt")
			}

			drive := int(args[0].Num)
			minDuration := int64(args[1].Num)

			language, err := makeMKVLanguageList(args[2])
			if err != nil {
				return valpkg.ErrorVal("makemkv.GetTitles: " + err.Error())
			}

			subtitle, err := makeMKVLanguageList(args[3])
			if err != nil {
				return valpkg.ErrorVal("makemkv.GetTitles: " + err.Error())
			}

			executable := ""
			if len(args) >= 5 {
				executable = strings.TrimSpace(valpkg.ToString(args[4]))
			}

			titles, err := makeMKVInfo(drive, executable)
			if err != nil {
				return valpkg.ErrorVal("makemkv.GetTitles: " + err.Error())
			}

			result := make([]valpkg.Value, 0)

			for _, title := range titles {
				if title.Duration < minDuration*60 {
					continue
				}

				// Erste Audiosprache ist Pflicht.
				if len(language) > 0 &&
					!makeMKVHasLanguage(title.AudioLanguages, language[:1]) {
					continue
				}

				if len(language) > 0 &&
					!makeMKVHasLanguage(title.AudioLanguages, language) {
					continue
				}

				if len(subtitle) > 0 &&
					!makeMKVHasLanguage(title.SubtitleLanguages, subtitle) {
					continue
				}

				result = append(result, valpkg.Value{
					Kind: valpkg.KindArr,
					Arr: []valpkg.Value{
						valpkg.NumVal(float64(title.Index)),
						valpkg.NumVal(float64(title.Duration)),
						valpkg.NumVal(float64(title.SizeBytes)),
						valpkg.StrVal(title.Name),
						valpkg.StrVal(title.Playlist),
						valpkg.StrVal(title.FileName),
						makeMKVStringArray(title.AudioLanguages),
						makeMKVStringArray(title.SubtitleLanguages),
						valpkg.NumVal(float64(title.Chapters)),
					},
				})
			}

			return valpkg.Value{
				Kind: valpkg.KindArr,
				Arr:  result,
			}
		},
	)

	// --------------------------------------------------------
	// makemkv.Rip(drive, title, outputPath, [executable])
	// --------------------------------------------------------
	Register(
		ns+"Rip",
		"makemkv",
		"drive, title, outputPath, [executable]",
		"Rippt einen mit GetTitles ermittelten Titel synchron in outputPath und wartet bis MakeMKV fertig ist. Rückgabe: Pfad der erzeugten MKV-Datei.",
		func(args []valpkg.Value) valpkg.Value {
			if len(args) < 3 {
				return valpkg.ErrorVal("makemkv.Rip: drive, title und outputPath benötigt")
			}

			drive := int(args[0].Num)

			title := args[1]
			if title.Kind != valpkg.KindArr || len(title.Arr) < 1 {
				return valpkg.ErrorVal("makemkv.Rip: title muss ein Titel-Array aus GetTitles sein")
			}

			// GetTitles liefert an Position 0 den MakeMKV-Titelindex.
			index := int(title.Arr[0].Num)
			if index < 0 {
				return valpkg.ErrorVal("makemkv.Rip: ungültiger Titelindex")
			}

			outputPath := strings.TrimSpace(valpkg.ToString(args[2]))
			if outputPath == "" {
				return valpkg.ErrorVal("makemkv.Rip: outputPath darf nicht leer sein")
			}

			executable := ""
			if len(args) >= 4 {
				executable = strings.TrimSpace(valpkg.ToString(args[3]))
			}

			if err := os.MkdirAll(outputPath, 0o755); err != nil {
				return valpkg.ErrorVal("makemkv.Rip: " + err.Error())
			}

			before := makeMKVListMKV(outputPath)

			source := fmt.Sprintf("disc:%d", drive)

			// Rippen darf beliebig lange dauern -> kein Timeout.
			_, err := makeMKVRun(
				0,
				executable,
				"-r",
				"mkv",
				source,
				strconv.Itoa(index),
				outputPath,
			)
			if err != nil {
				return valpkg.ErrorVal("makemkv.Rip: " + err.Error())
			}

			after := makeMKVListMKV(outputPath)

			names := make([]string, 0, len(after))
			for name := range after {
				if !before[name] {
					names = append(names, name)
				}
			}

			if len(names) == 0 {
				return valpkg.ErrorVal("makemkv.Rip: keine neue Datei in " + outputPath)
			}

			sort.Strings(names)

			return valpkg.StrVal(filepath.Join(outputPath, names[0]))
		},
	)
}

// ------------------------------------------------------------
// MakeMKV aufrufen
// ------------------------------------------------------------

// makeMKVRun startet makemkvcon synchron. timeout == 0 bedeutet kein Timeout.
func makeMKVRun(
	timeout time.Duration,
	executable string,
	args ...string,
) (string, error) {
	if executable == "" {
		executable = os.Getenv("MAKEMKVCON")
	}

	if executable == "" {
		executable = "makemkvcon"
	}

	ctx := context.Background()

	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, executable, args...)

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	if err := cmd.Start(); err != nil {
		return "", err
	}

	makeMKVAttachJob(cmd)

	err := cmd.Wait()
	text := strings.TrimSpace(buf.String())

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return text, fmt.Errorf("Zeitüberschreitung nach %v", timeout)
		}

		if text != "" {
			return text, fmt.Errorf("%v: %s", err, makeMKVTail(text, 5))
		}

		return text, err
	}

	return text, nil
}

func makeMKVTail(text string, lines int) string {
	parts := strings.Split(text, "\n")

	if len(parts) <= lines {
		return text
	}

	return strings.Join(parts[len(parts)-lines:], "\n")
}

func makeMKVInfo(drive int, executable string) ([]makeMKVTitle, error) {
	source := fmt.Sprintf("disc:%d", drive)

	output, err := makeMKVRun(
		makeMKVInfoTimeout,
		executable,
		"-r",
		"info",
		source,
	)
	if err != nil {
		return nil, err
	}

	return parseMakeMKVInfo(output)
}

func makeMKVListMKV(dir string) map[string]bool {
	result := make(map[string]bool)

	entries, _ := os.ReadDir(dir)

	for _, e := range entries {
		if !e.IsDir() &&
			strings.EqualFold(filepath.Ext(e.Name()), ".mkv") {
			result[e.Name()] = true
		}
	}

	return result
}

// ------------------------------------------------------------
// MakeMKV-Ausgabe parsen
// ------------------------------------------------------------

func parseMakeMKVInfo(output string) ([]makeMKVTitle, error) {
	titles := make(map[int]*makeMKVTitle)

	streams := make(map[makeMKVStreamKey]*makeMKVStream)

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)

		if line == "" {
			continue
		}

		colon := strings.IndexByte(line, ':')

		if colon <= 0 {
			continue
		}

		recordType := line[:colon]
		recordData := line[colon+1:]

		fields, err := parseMakeMKVCSV(recordData)
		if err != nil {
			continue
		}

		switch recordType {
		case "TINFO":
			parseMakeMKVTInfo(fields, titles)

		case "SINFO":
			parseMakeMKVSInfo(fields, streams)
		}
	}

	// --------------------------------------------------------
	// Streams deterministisch (Titel, Stream) den Titeln zuordnen
	// --------------------------------------------------------

	keys := make([]makeMKVStreamKey, 0, len(streams))

	for key := range streams {
		keys = append(keys, key)
	}

	sort.Slice(keys, func(i, j int) bool {
		if keys[i].TitleIndex != keys[j].TitleIndex {
			return keys[i].TitleIndex < keys[j].TitleIndex
		}

		return keys[i].Stream < keys[j].Stream
	})

	for _, key := range keys {
		stream := streams[key]

		title, ok := titles[stream.TitleIndex]
		if !ok {
			continue
		}

		lang := strings.ToLower(strings.TrimSpace(stream.Language))
		if lang == "" {
			continue
		}

		switch strings.ToLower(strings.TrimSpace(stream.Type)) {
		case "audio":
			title.AudioLanguages = appendUniqueString(
				title.AudioLanguages,
				lang,
			)

		case "subtitle", "subtitles":
			title.SubtitleLanguages = appendUniqueString(
				title.SubtitleLanguages,
				lang,
			)
		}
	}

	// --------------------------------------------------------
	// Titel sortiert zurückgeben
	// --------------------------------------------------------

	numbers := make([]int, 0, len(titles))

	for number := range titles {
		numbers = append(numbers, number)
	}

	sort.Ints(numbers)

	result := make([]makeMKVTitle, 0, len(numbers))

	for _, number := range numbers {
		result = append(result, *titles[number])
	}

	return result, nil
}

// ------------------------------------------------------------
// TINFO
// ------------------------------------------------------------

func parseMakeMKVTInfo(
	fields []string,
	titles map[int]*makeMKVTitle,
) {
	if len(fields) < 4 {
		return
	}

	index, err := strconv.Atoi(fields[0])
	if err != nil {
		return
	}

	property, err := strconv.Atoi(fields[1])
	if err != nil {
		return
	}

	title, ok := titles[index]

	if !ok {
		title = &makeMKVTitle{
			Index: index,
		}

		titles[index] = title
	}

	value := fields[3]

	switch property {
	case 2:
		title.Name = value

	case 8:
		// Kapitelanzahl
		if chapters, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
			title.Chapters = chapters
		}

	case 9:
		if duration, ok := parseMakeMKVDuration(value); ok {
			title.Duration = duration
		}

	case 11:
		if size, err := strconv.ParseInt(
			strings.TrimSpace(value),
			10,
			64,
		); err == nil {
			title.SizeBytes = size
		}

	case 16:
		title.Playlist = value

	case 27:
		title.FileName = value
	}
}

// ------------------------------------------------------------
// SINFO
// ------------------------------------------------------------

func parseMakeMKVSInfo(
	fields []string,
	streams map[makeMKVStreamKey]*makeMKVStream,
) {
	// Beispiel:
	//
	// SINFO:1,1,1,6202,"Audio"
	// SINFO:1,1,3,0,"eng"
	// SINFO:1,1,4,0,"English"
	//
	// CSV:
	// [0] Titelindex
	// [1] Streamnummer
	// [2] Property
	// [3] Code
	// [4] Wert

	if len(fields) < 5 {
		return
	}

	titleIndex, err := strconv.Atoi(fields[0])
	if err != nil {
		return
	}

	streamNumber, err := strconv.Atoi(fields[1])
	if err != nil {
		return
	}

	property, err := strconv.Atoi(fields[2])
	if err != nil {
		return
	}

	code, err := strconv.Atoi(fields[3])
	if err != nil {
		code = 0
	}

	value := fields[4]

	key := makeMKVStreamKey{
		TitleIndex: titleIndex,
		Stream:     streamNumber,
	}

	stream, ok := streams[key]

	if !ok {
		stream = &makeMKVStream{
			TitleIndex: titleIndex,
			Stream:     streamNumber,
		}

		streams[key] = stream
	}

	switch property {
	case 1:
		switch code {
		case 6201:
			stream.Type = "video"

		case 6202:
			stream.Type = "audio"

		case 6203:
			stream.Type = "subtitle"

		default:
			stream.Type = strings.ToLower(strings.TrimSpace(value))
		}

	case 3:
		// Tatsächliche Sprache des Streams.
		//
		// Property 28/29 darf hier NICHT verwendet werden.
		stream.Language = strings.ToLower(strings.TrimSpace(value))
	}
}

// ------------------------------------------------------------
// CSV
// ------------------------------------------------------------

func parseMakeMKVCSV(data string) ([]string, error) {
	reader := csv.NewReader(strings.NewReader(data))

	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true

	return reader.Read()
}

// ------------------------------------------------------------
// Dauer
// ------------------------------------------------------------

func parseMakeMKVDuration(value string) (int64, bool) {
	value = strings.TrimSpace(value)

	parts := strings.Split(value, ":")

	if len(parts) != 3 {
		return 0, false
	}

	hours, err1 := strconv.ParseInt(parts[0], 10, 64)
	minutes, err2 := strconv.ParseInt(parts[1], 10, 64)
	seconds, err3 := strconv.ParseInt(parts[2], 10, 64)

	if err1 != nil ||
		err2 != nil ||
		err3 != nil {
		return 0, false
	}

	if hours < 0 ||
		minutes < 0 ||
		minutes > 59 ||
		seconds < 0 ||
		seconds > 59 {
		return 0, false
	}

	return hours*3600 +
		minutes*60 +
		seconds, true
}

// ------------------------------------------------------------
// Sprachen
// ------------------------------------------------------------

// makeMKVNormalizeLang vereinheitlicht gängige Aliase auf die
// 3-Buchstaben-Codes, die MakeMKV liefert.
func makeMKVNormalizeLang(lang string) string {
	lang = strings.ToLower(strings.TrimSpace(lang))

	switch lang {
	case "de", "deu", "ger":
		return "ger"
	case "en", "eng":
		return "eng"
	}

	return lang
}

func makeMKVLanguageList(value valpkg.Value) ([]string, error) {
	if value.Kind != valpkg.KindArr {
		return nil, fmt.Errorf("Sprache muss als Array angegeben werden")
	}

	result := make([]string, 0, len(value.Arr))

	for _, item := range value.Arr {
		lang := strings.ToLower(strings.TrimSpace(valpkg.ToString(item)))

		if lang == "" {
			continue
		}

		result = appendUniqueString(result, lang)
	}

	return result, nil
}

func makeMKVHasLanguage(
	available []string,
	required []string,
) bool {
	for _, wanted := range required {
		wanted = makeMKVNormalizeLang(wanted)

		for _, have := range available {
			if makeMKVNormalizeLang(have) == wanted {
				return true
			}
		}
	}

	return false
}

func makeMKVStringArray(values []string) valpkg.Value {
	result := make([]valpkg.Value, 0, len(values))

	for _, value := range values {
		result = append(result, valpkg.StrVal(value))
	}

	return valpkg.Value{
		Kind: valpkg.KindArr,
		Arr:  result,
	}
}

func appendUniqueString(
	values []string,
	value string,
) []string {
	value = strings.TrimSpace(value)

	if value == "" {
		return values
	}

	for _, existing := range values {
		if strings.EqualFold(existing, value) {
			return values
		}
	}

	return append(values, value)
}

// ------------------------------------------------------------
// Disc-Informationen
// ------------------------------------------------------------

func makeMKVDiscInfo(
	drive int,
	executable string,
) (makeMKVDisc, error) {
	source := fmt.Sprintf("disc:%d", drive)

	output, err := makeMKVRun(
		makeMKVInfoTimeout,
		executable,
		"-r",
		"info",
		source,
	)
	if err != nil {
		return makeMKVDisc{}, err
	}

	var result makeMKVDisc
	var foundDrive bool
	var drvName string

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		colon := strings.IndexByte(line, ':')
		if colon <= 0 {
			continue
		}

		recordType := line[:colon]
		recordData := line[colon+1:]

		fields, err := parseMakeMKVCSV(recordData)
		if err != nil {
			continue
		}

		switch recordType {
		case "DRV":
			if len(fields) < 7 {
				continue
			}

			recordDrive, err := strconv.Atoi(fields[0])
			if err != nil || recordDrive != drive {
				continue
			}

			result.Drive = recordDrive
			result.Path = fields[6]
			foundDrive = true

			// Feld 5 ist der Disc-/Volume-Name.
			if drvName == "" && strings.TrimSpace(fields[5]) != "" {
				drvName = strings.TrimSpace(fields[5])
			}

		case "TINFO":
			if len(fields) < 4 {
				continue
			}

			titleIndex, err := strconv.Atoi(fields[0])
			if err != nil || titleIndex < 0 {
				continue
			}

			property, err := strconv.Atoi(fields[1])
			if err != nil {
				continue
			}

			// Property 2 = Titelname.
			// Dieser Name hat Vorrang vor dem DRV-Disc-Namen.
			if property == 2 && result.Name == "" {
				name := strings.TrimSpace(fields[3])
				if name != "" {
					result.Name = name
				}
			}
		}
	}

	if !foundDrive {
		return makeMKVDisc{}, fmt.Errorf(
			"kein Laufwerk %d gefunden",
			drive,
		)
	}

	// Fallback auf den Disc-Namen aus DRV.
	if result.Name == "" {
		result.Name = drvName
	}

	// Letzter Fallback: immer einen gültigen eindeutigen Namen erzeugen.
	if result.Name == "" {
		result.Name = "Unbekannt_" + time.Now().Format("20060102_150405")
	}

	return result, nil
}

func init() {
	initMakeMKVFunctions()
}
