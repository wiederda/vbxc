// ------------------------
// stdlib_folder.go
// ------------------------

package stdlib

import (
	"bufio"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	valpkg "vbxc/value"
)

func initFolderFunctions() {
	if builtins == nil {
		builtins = make(map[string]BuiltinInfo)
	}

	ns := "folder."

	// folder.Create
	Register(ns+"Create", "folder", "path", "Erstellt ein Verzeichnis rekursiv. Gibt True zurück, wenn der Ordner existiert oder erfolgreich erstellt wurde.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.ErrorVal("folder.Create: Pfad fehlt")
		}
		path, errVal := absPathVal(args[0].Str)
		if errVal != nil {
			return *errVal
		}
		if err := os.MkdirAll(path, 0755); err != nil {
			return valpkg.ErrorVal("folder.Create: " + err.Error())
		}
		return valpkg.BoolVal(true)
	})

	// folder.Size(path [, human])
	Register(ns+"Size", "folder", "path [, human]", "Gibt die Gesamtgröße eines Verzeichnisses zurück.", func(args []valpkg.Value) valpkg.Value {
		path, _ := absPathVal(getArg(args, 0).Str)

		_, _, size := FolderStats(path, nil)

		if len(args) >= 2 && valpkg.ToBool(args[1]) {
			return valpkg.StrVal(formatHuman(size))
		}

		return valpkg.NumVal(float64(size))
	})

	// folder.Delete
	Register(ns+"Delete", "folder", "path [, force]", "Löscht einen Ordner komplett. Mit 'force' wird versucht, Schreibschutz zu ignorieren.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 || args[0].Str == "" {
			return valpkg.ErrorVal("folder.Delete benötigt einen Pfad")
		}

		path, errVal := absPathVal(args[0].Str)
		if errVal != nil {
			return *errVal
		}

		path = fileLongPath(path)

		force := false
		if len(args) >= 2 && fileTruthy(args[1]) {
			force = true
		}

		if force {
			os.Chmod(path, 0666)
		}

		err := os.RemoveAll(path)
		if err == nil || os.IsNotExist(err) {
			return valpkg.Value{Kind: valpkg.KindArr, Arr: []valpkg.Value{}}
		}

		if force {
			var allPaths []string
			var remaining []valpkg.Value

			filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
				if err == nil {
					allPaths = append(allPaths, fileLongPath(p))
				}
				return nil
			})

			for i := len(allPaths) - 1; i >= 0; i-- {
				p := allPaths[i]
				os.Chmod(p, 0666)
				err := os.Remove(p)
				if err != nil && !os.IsNotExist(err) {
					info, statErr := os.Stat(p)
					if statErr == nil && !info.IsDir() {
						remaining = append(remaining, valpkg.Value{Kind: valpkg.KindStr, Str: p})
					}
				}
			}
			return valpkg.Value{Kind: valpkg.KindArr, Arr: remaining}
		}

		return valpkg.Value{Kind: valpkg.KindArr, Arr: []valpkg.Value{{Kind: valpkg.KindStr, Str: path}}}
	})

	// folder.ModTime
	Register(ns+"ModTime", "folder", "path", "Gibt das Datum der letzten Änderung im ISO-Format (RFC3339) zurück.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.ErrorVal("usage: folder.ModTime(path)")
		}
		path, errVal := absPathVal(args[0].Str)
		if errVal != nil {
			return *errVal
		}
		info, err := os.Stat(path)
		if err != nil {
			return valpkg.ErrorVal("Änderungsdatum konnte nicht gelesen werden: " + err.Error())
		}
		return valpkg.StrVal(info.ModTime().Format(time.RFC3339))
	})

	// folder.CreateTime
	Register(ns+"CreateTime", "folder", "path", "Gibt das Erstellungsdatum des Ordners zurück.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.ErrorVal("usage: folder.CreateTime(path)")
		}
		path, errVal := absPathVal(args[0].Str)
		if errVal != nil {
			return *errVal
		}
		ctime, _, _, err := fileNamespaceFolderTimes(path)
		if err != nil {
			return valpkg.ErrorVal("Erstellungszeit konnte nicht gelesen werden: " + err.Error())
		}
		return valpkg.StrVal(ctime.Format(time.RFC3339))
	})

	// folder.AccessTime
	Register(ns+"AccessTime", "folder", "path", "Gibt den Zeitpunkt des letzten Zugriffs zurück.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.ErrorVal("usage: folder.AccessTime(path)")
		}
		path, errVal := absPathVal(args[0].Str)
		if errVal != nil {
			return *errVal
		}
		_, atime, _, err := fileNamespaceFolderTimes(path)
		if err != nil {
			return valpkg.ErrorVal("Zeitstempel (Access) konnte nicht gelesen werden: " + err.Error())
		}
		return valpkg.StrVal(atime.Format(time.RFC3339))
	})

	// folder.Copy
	Register(ns+"Copy", "folder", "src, dst, [progress], [network], [failIfExists]", "Kopiert einen Ordner rekursiv. progress=True zeigt Fortschritt. network=True optimiert für Netzwerk-Transfers. failIfExists=True bricht ab, wenn dst bereits existiert (Standard: es wird in ein vorhandenes Ziel hineingemergt/überschrieben).", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 2 {
			return valpkg.ErrorVal("usage: folder.Copy(src, dst, [progress], [network], [failIfExists])")
		}

		src, e1 := absPathVal(args[0].Str)
		dst, e2 := absPathVal(args[1].Str)
		if e1 != nil {
			return *e1
		}
		if e2 != nil {
			return *e2
		}

		showProgress := false
		if len(args) >= 3 {
			showProgress = (args[2].Str == "true" || (args[2].Kind == valpkg.KindBool && args[2].Bool))
		}

		networkMode := false
		if len(args) >= 4 {
			networkMode = (args[3].Str == "true" || (args[3].Kind == valpkg.KindBool && args[3].Bool))
		}

		// Optional: Standard false (bisheriges Verhalten unverändert) --
		// anders als file.Copy ist der Default hier bewusst permissiv
		// (in ein vorhandenes Ziel hineinmergen), siehe Doku.
		failIfExists := false
		if len(args) >= 5 {
			failIfExists = fileTruthy(args[4])
		}

		if failIfExists {
			if _, err := os.Stat(dst); err == nil {
				return valpkg.ErrorVal("folder.Copy: Ziel existiert bereits: " + dst)
			}
		}

		type job struct {
			src, dst string
			mode     os.FileMode
		}

		numWorkers := runtime.NumCPU()
		if networkMode {
			numWorkers = numWorkers * 4
			if numWorkers > 16 {
				numWorkers = 16
			}
		} else {
			if numWorkers > 4 {
				numWorkers = 4
			}
		}

		var totalFiles int64
		var copiedFiles int64

		if showProgress {
			filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
				if err == nil && !d.IsDir() {
					atomic.AddInt64(&totalFiles, 1)
				}
				return nil
			})
			mode := "lokal"
			if networkMode {
				mode = "Netzwerk"
			}
			fmt.Printf("Kopiere %d Dateien nach %s [%d Worker, Modus: %s]\n",
				totalFiles, dst, numWorkers, mode)
		}

		jobChan := make(chan job, numWorkers*4)
		var wg sync.WaitGroup

		var progressDone chan struct{}
		if showProgress {
			progressDone = make(chan struct{})
			go func() {
				ticker := time.NewTicker(100 * time.Millisecond)
				defer ticker.Stop()
				for {
					select {
					case <-ticker.C:
						done := atomic.LoadInt64(&copiedFiles)
						pct := 0
						if totalFiles > 0 {
							pct = int(float64(done) / float64(totalFiles) * 100)
						}
						filled := pct / 2
						fmt.Printf("\r[%s%s] %d/%d (%d%%)",
							strings.Repeat("█", filled),
							strings.Repeat("░", 50-filled),
							done, totalFiles, pct)
					case <-progressDone:
						return
					}
				}
			}()
		}

		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := range jobChan {
					copyFileInternalBuffered(j.src, j.dst, j.mode)
					if showProgress {
						atomic.AddInt64(&copiedFiles, 1)
					}
				}
			}()
		}

		filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			rel, _ := filepath.Rel(src, p)
			target := filepath.Join(dst, rel)

			if d.IsDir() {
				info, _ := d.Info()
				os.MkdirAll(target, info.Mode())
			} else {
				info, _ := d.Info()
				jobChan <- job{src: p, dst: target, mode: info.Mode()}
			}
			return nil
		})

		close(jobChan)
		wg.Wait()

		if showProgress {
			close(progressDone)
			fmt.Printf("\r[%s] %d/%d (100%%)\nFertig.\n",
				strings.Repeat("█", 50),
				totalFiles, totalFiles)
		}

		return valpkg.NullVal()
	})

	// folder.Move
	Register(ns+"Move", "folder", "src, dst, [failIfExists]", "Verschiebt einen Ordner (Rename mit Fallback auf Copy/Delete). failIfExists=True bricht ab, wenn dst bereits existiert (Standard: es wird ggf. in ein vorhandenes Ziel hineingemergt).",
		func(args []valpkg.Value) valpkg.Value {
			if len(args) < 2 {
				return valpkg.ErrorVal("usage: folder.Move(src, dst, [failIfExists])")
			}
			src, _ := absPathVal(args[0].Str)
			dst, _ := absPathVal(args[1].Str)

			// Optional: Standard false, identisch zum bisherigen Verhalten.
			failIfExists := false
			if len(args) >= 3 {
				failIfExists = fileTruthy(args[2])
			}

			if failIfExists {
				if _, err := os.Stat(dst); err == nil {
					return valpkg.ErrorVal("folder.Move: Ziel existiert bereits: " + dst)
				}
			}

			if err := os.Rename(src, dst); err == nil {
				return valpkg.NullVal()
			}

			if copyInfo, ok := builtins[ns+"Copy"]; ok {
				copyArgs := []valpkg.Value{valpkg.StrVal(src), valpkg.StrVal(dst)}
				if failIfExists {
					// progress=false, network=false, failIfExists=true durchreichen,
					// damit der Fallback dieselbe Prüfung anwendet -- falls
					// os.Rename z.B. aus einem anderen Grund als "existiert
					// bereits" fehlschlug, dst zwischenzeitlich aber doch da ist.
					copyArgs = []valpkg.Value{valpkg.StrVal(src), valpkg.StrVal(dst), valpkg.BoolVal(false), valpkg.BoolVal(false), valpkg.BoolVal(true)}
				}

				copyRes := copyInfo.Fn(copyArgs)
				if copyRes.Kind == valpkg.KindError {
					return copyRes
				}
			} else {
				return valpkg.ErrorVal("interner Fehler: folder.Copy nicht gefunden")
			}

			os.RemoveAll(src)
			return valpkg.NullVal()
		},
	)

	// folder.Rename
	Register(ns+"Rename", "folder", "oldPath, newPath", "Benennt einen Ordner um (auf demselben Volume).", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 2 {
			return valpkg.ErrorVal("usage: folder.Rename(oldPath, newPath)")
		}
		oldPath, errVal1 := absPathVal(args[0].Str)
		if errVal1 != nil {
			return *errVal1
		}
		newPath, errVal2 := absPathVal(args[1].Str)
		if errVal2 != nil {
			return *errVal2
		}
		err := os.Rename(oldPath, newPath)
		if err != nil {
			return valpkg.ErrorVal("Umbenennen fehlgeschlagen: " + err.Error())
		}
		return valpkg.NullVal()
	})

	// folder.PathCombine
	Register(ns+"PathCombine", "folder", "parts...", "Verbindet mehrere Pfadsegmente sicher zu einem Gesamtpfad.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 2 {
			return valpkg.ErrorVal("folder.PathCombine benötigt mindestens zwei Pfadteile")
		}
		parts := make([]string, 0, len(args))
		for _, v := range args {
			parts = append(parts, v.Str)
		}
		return valpkg.Value{Kind: valpkg.KindStr, Str: filepath.Join(parts...)}
	})

	// folder.GetDirectories
	// folder.GetDirectories
	Register(ns+"GetDirectories", "folder", "path [, ignore]", "Gibt ein Array mit den Namen aller direkten Unterverzeichnisse zurück. 'ignore' schließt Ordner mit diesem Namen aus (Komma-getrennt oder Array).", func(args []valpkg.Value) valpkg.Value {
		rawDir := "."
		if len(args) >= 1 && args[0].Str != "" {
			rawDir = args[0].Str
		}

		dir, errVal := absPathVal(rawDir)
		if errVal != nil {
			return *errVal
		}

		ignore := map[string]bool{}

		if len(args) >= 2 {
			switch args[1].Kind {
			case valpkg.KindArr:
				for _, v := range args[1].Arr {
					name := strings.TrimSpace(v.Str)
					if name != "" {
						ignore[name] = true
					}
				}

			default:
				for _, s := range strings.Split(args[1].Str, ",") {
					name := strings.TrimSpace(s)
					if name != "" {
						ignore[name] = true
					}
				}
			}
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			return valpkg.ErrorVal("Verzeichnis konnte nicht gelesen werden: " + err.Error())
		}

		results := make([]valpkg.Value, 0)

		for _, entry := range entries {
			// Nur direkte Unterordner
			if !entry.IsDir() {
				continue
			}

			// Ignore
			if ignore[entry.Name()] {
				continue
			}

			results = append(results, valpkg.StrVal(entry.Name()))
		}

		sort.SliceStable(results, func(i, j int) bool {
			return naturalLess(results[i].Str, results[j].Str)
		})

		return valpkg.Value{
			Kind: valpkg.KindArr,
			Arr:  results,
		}
	})

	// folder.GetFiles
	Register(ns+"GetFiles", "folder", "path [, pattern, recursive, fullPath]", "Gibt ein Array mit Dateien im Verzeichnis zurück.", func(args []valpkg.Value) valpkg.Value {
		opts := parseFolderArgs(args)

		var out []valpkg.Value

		err := WalkFolder(opts, func(it FolderItem) error {
			if it.Info != nil && !it.Info.IsDir() {
				name := it.Name

				if opts.FullPath {
					name = it.Path
				}

				out = append(out, valpkg.StrVal(name))
			}

			return nil
		})

		if err != nil {
			return valpkg.ErrorVal(err.Error())
		}

		// Natürliche Sortierung
		sort.SliceStable(out, func(i, j int) bool {
			return naturalLess(out[i].Str, out[j].Str)
		})

		return valpkg.Value{
			Kind: valpkg.KindArr,
			Arr:  out,
		}
	})

	Register(ns+"GetFilesMax", "folder", "folders [, pattern, recursive, fullPath]", "Gibt ein Array mit Dateien aus mehreren Verzeichnissen zurück.", func(args []valpkg.Value) valpkg.Value {
		if len(args) == 0 || args[0].Kind != valpkg.KindArr {
			return valpkg.ErrorVal("GetFilesMax: erster Parameter muss ein Array von Ordnerpfaden sein")
		}

		folders := args[0]
		rest := args[1:]

		var out []valpkg.Value

		for _, f := range folders.Arr {
			// pro Ordner die restlichen Parameter (pattern, recursive, fullPath) übernehmen
			folderArgs := append([]valpkg.Value{f}, rest...)
			opts := parseFolderArgs(folderArgs)

			err := WalkFolder(opts, func(it FolderItem) error {
				if it.Info != nil && !it.Info.IsDir() {
					name := it.Name

					if opts.FullPath {
						name = it.Path
					}

					out = append(out, valpkg.StrVal(name))
				}

				return nil
			})

			if err != nil {
				return valpkg.ErrorVal(err.Error())
			}
		}

		// Natürliche Sortierung
		sort.SliceStable(out, func(i, j int) bool {
			return naturalLess(out[i].Str, out[j].Str)
		})

		return valpkg.Value{
			Kind: valpkg.KindArr,
			Arr:  out,
		}
	})

	// folder.GetSubFolders
	Register(ns+"GetSubFolders", "folder", "path [, pattern, recursive, fullPath]", "Gibt ein Array mit Unterverzeichnissen zurück.", func(args []valpkg.Value) valpkg.Value {
		opts := parseFolderArgs(args)

		if len(args) >= 5 {
			opts.Ignore = make(map[string]bool)
			switch args[4].Kind {
			case valpkg.KindArr:
				for _, v := range args[4].Arr {
					name := strings.TrimSpace(v.Str)
					if name != "" {
						opts.Ignore[name] = true
					}
				}
			default:
				for _, s := range strings.Split(args[4].Str, ",") {
					name := strings.TrimSpace(s)
					if name != "" {
						opts.Ignore[name] = true
					}
				}
			}
		}

		var out []valpkg.Value

		err := WalkFolder(opts, func(it FolderItem) error {
			if it.Info != nil && it.Info.IsDir() && it.Path != opts.Path {
				name := it.Name

				if opts.FullPath {
					name = it.Path
				}

				out = append(out, valpkg.StrVal(name))
			}

			return nil
		})

		if err != nil {
			return valpkg.ErrorVal(err.Error())
		}

		// Natürliche Sortierung wie bei GetFiles
		sort.SliceStable(out, func(i, j int) bool {
			return naturalLess(out[i].Str, out[j].Str)
		})

		return valpkg.Value{
			Kind: valpkg.KindArr,
			Arr:  out,
		}
	})

	// folder.CreateSymlink
	Register(ns+"CreateSymlink", "folder", "target, linkPath", "Erstellt einen Symlink auf einen Ordner.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 2 {
			return valpkg.ErrorVal("folder.CreateSymlink(target, linkPath) benötigt 2 Pfade")
		}
		return createSymlinkInternal(args[0].Str, args[1].Str, true)
	})

	// folder.EmptyFolder
	Register(ns+"EmptyFolder", "folder", "path [, force]", "Löscht den Inhalt eines Ordners, behält den Ordner selbst.", FolderEmpty)

	// folder.isFolderEmpty
	Register(ns+"isFolderEmpty", "folder", "path", "Prüft, ob ein Ordner leer ist.", FolderIsEmpty)

	// folder.Exists
	Register(ns+"Exists", "folder", "bool", "Prüft, ob der Pfad ein existierender Ordner ist.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 || args[0].Str == "" {
			return valpkg.BoolVal(false)
		}
		path, errVal := absPathVal(args[0].Str)
		if errVal != nil {
			return valpkg.BoolVal(false)
		}
		info, err := os.Stat(path)
		return valpkg.BoolVal(err == nil && info.IsDir())
	})

	// folder.FindDuplicates
	Register(ns+"FindDuplicates", "folder", "path [, pattern]", "Findet Dateien mit identischer Größe und gruppiert sie.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.ErrorVal("usage: folder.FindDuplicates(path [, pattern])")
		}

		var roots []string
		switch args[0].Kind {
		case valpkg.KindArr:
			for _, v := range args[0].Arr {
				r, errVal := absPathVal(v.Str)
				if errVal != nil {
					return *errVal
				}
				roots = append(roots, r)
			}
		default:
			r, errVal := absPathVal(args[0].Str)
			if errVal != nil {
				return *errVal
			}
			roots = append(roots, r)
		}

		pattern := "*"
		if len(args) >= 2 {
			pattern = args[1].Str
		}

		sizeGroups := make(map[int64][]valpkg.Value)

		for _, root := range roots {
			_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return nil
				}
				ok, _ := filepath.Match(pattern, d.Name())
				if !ok {
					return nil
				}
				info, err := d.Info()
				if err != nil {
					return nil
				}
				size := info.Size()
				if size == 0 {
					return nil
				}
				sizeGroups[size] = append(sizeGroups[size], valpkg.Value{
					Kind: valpkg.KindMap,
					Map: map[string]valpkg.Value{
						"path": valpkg.StrVal(p),
						"size": valpkg.NumVal(float64(size)),
					},
				})
				return nil
			})
		}

		var result []valpkg.Value
		for _, group := range sizeGroups {
			if len(group) > 1 {
				result = append(result, valpkg.Value{Kind: valpkg.KindArr, Arr: group})
			}
		}

		return valpkg.Value{Kind: valpkg.KindArr, Arr: result}
	})

	// Noch mal prüfen
	// folder.FindInFiles
	Register(ns+"FindInFiles", "folder", "path, ext, search [, pattern, flags]", "Durchsucht Dateien zeilenweise nach Text. Flags: i=case-insensitive.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 3 {
			return valpkg.ErrorVal("usage: folder.FindInFiles(path, ext, search [, pattern, flags])")
		}

		root, errVal := absPathVal(args[0].Str)
		if errVal != nil {
			return *errVal
		}

		ext := args[1].Str
		search := args[2].Str

		pattern := "*"
		if len(args) >= 4 {
			pattern = args[3].Str
		}

		flags := ""
		if len(args) >= 5 {
			flags = args[4].Str
		}

		ignoreCase := strings.Contains(flags, "i")

		results := make(chan valpkg.Value, 1024)
		var wg sync.WaitGroup
		sem := make(chan struct{}, 8)

		_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if ext != "" && filepath.Ext(d.Name()) != ext {
				return nil
			}
			ok, _ := filepath.Match(pattern, d.Name())
			if !ok {
				return nil
			}

			wg.Add(1)
			sem <- struct{}{}

			go func(filePath string) {
				defer wg.Done()
				defer func() { <-sem }()

				file, err := os.Open(filePath)
				if err != nil {
					return
				}
				defer file.Close()

				scanner := bufio.NewScanner(file)
				scanner.Buffer(make([]byte, 1024), 1024*1024*10)

				lineNumber := 0

				for scanner.Scan() {
					lineNumber++

					line := scanner.Text()
					match := false

					if ignoreCase {
						match = strings.Contains(strings.ToLower(line), strings.ToLower(search))
					} else {
						match = strings.Contains(line, search)
					}

					if match {
						results <- valpkg.Value{
							Kind: valpkg.KindArr,
							Arr: []valpkg.Value{
								valpkg.NumVal(float64(lineNumber)),
								valpkg.StrVal(line),
								valpkg.StrVal(filePath),
							},
						}
					}
				}

				if err := scanner.Err(); err != nil {
					return
				}
			}(p)

			return nil
		})

		go func() {
			wg.Wait()
			close(results)
		}()

		var out []valpkg.Value
		for r := range results {
			out = append(out, r)
		}

		return valpkg.Value{Kind: valpkg.KindArr, Arr: out}
	})

	// folder.CheckHash
	Register(ns+"CheckHash", "folder", "groupsArray [, progress]", "Validiert Dubletten mittels Quick-Hash + SHA256 (parallelisiert).", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 || args[0].Kind != valpkg.KindArr {
			return valpkg.ErrorVal("usage: folder.CheckHash(groupsArray [, progress])")
		}

		showProgress := len(args) >= 2 && args[1].Kind == valpkg.KindBool && args[1].Bool

		const workers = 8

		var allDuplicates [][]valpkg.Value
		totalGroups := len(args[0].Arr)

		for gi, groupVal := range args[0].Arr {
			if groupVal.Kind != valpkg.KindArr {
				continue
			}

			if showProgress {
				fmt.Printf("[CheckHash] Gruppe %d/%d (%d Dateien) – Quick-Hash ...\n", gi+1, totalGroups, len(groupVal.Arr))
			}

			// --- Stufe 1: Quick-Hash parallel ---
			type qhResult struct {
				file valpkg.Value
				qh   string
				err  error
			}

			files := groupVal.Arr
			qhJobs := make(chan valpkg.Value, len(files))
			qhResults := make(chan qhResult, len(files))

			var wg1 sync.WaitGroup
			for w := 0; w < workers; w++ {
				wg1.Add(1)
				go func() {
					defer wg1.Done()
					for fileVal := range qhJobs {
						path := fileVal.Map["path"].Str
						qh, err := quickHash(path)
						qhResults <- qhResult{file: fileVal, qh: qh, err: err}
					}
				}()
			}
			for _, fileVal := range files {
				qhJobs <- fileVal
			}
			close(qhJobs)

			go func() {
				wg1.Wait()
				close(qhResults)
			}()

			qhMap := make(map[string][]valpkg.Value)
			for r := range qhResults {
				if r.err != nil {
					continue
				}
				qhMap[r.qh] = append(qhMap[r.qh], r.file)
			}

			// --- Stufe 2: SHA256 parallel, nur für Quick-Hash-Kandidaten ---
			for _, candidates := range qhMap {
				if len(candidates) < 2 {
					continue
				}

				if showProgress {
					fmt.Printf("[CheckHash]   SHA256 für %d Kandidaten ...\n", len(candidates))
				}

				type fhResult struct {
					file valpkg.Value
					h    string
					err  error
				}

				fhJobs := make(chan valpkg.Value, len(candidates))
				fhResults := make(chan fhResult, len(candidates))

				var wg2 sync.WaitGroup
				for w := 0; w < workers; w++ {
					wg2.Add(1)
					go func() {
						defer wg2.Done()
						for fileVal := range fhJobs {
							path := fileVal.Map["path"].Str
							if showProgress {
								fmt.Printf("[CheckHash]     %s\n", path)
							}
							h, err := fileHash(path)
							fhResults <- fhResult{file: fileVal, h: h, err: err}
						}
					}()
				}
				for _, fileVal := range candidates {
					fhJobs <- fileVal
				}
				close(fhJobs)

				go func() {
					wg2.Wait()
					close(fhResults)
				}()

				hashes := make(map[string][]valpkg.Value)
				for r := range fhResults {
					if r.err != nil {
						continue
					}
					hashes[r.h] = append(hashes[r.h], r.file)
				}

				for _, group := range hashes {
					if len(group) > 1 {
						allDuplicates = append(allDuplicates, group)
					}
				}
			}
		}

		return valpkg.Value{Kind: valpkg.KindArr2D, Arr2D: allDuplicates}
	})
	// folder.Count
	Register(ns+"Count", "folder", "path [, ignore]", "Gibt Anzahl Ordner, Dateien und Gesamtgröße zurück.", func(args []valpkg.Value) valpkg.Value {
		path, _ := absPathVal(getArg(args, 0).Str)

		ignore := map[string]bool{}
		if len(args) >= 2 {
			arg := getArg(args, 1)
			switch arg.Kind {
			case valpkg.KindArr:
				for _, v := range arg.Arr {
					name := strings.TrimSpace(v.Str)
					if name != "" {
						ignore[name] = true
					}
				}
			case valpkg.KindArr2D:
				return valpkg.ErrorVal("folder.Count: 'ignore' erwartet eine flache Liste (1D), kein 2D-Array")
			default:
				for _, s := range strings.Split(arg.Str, ",") {
					name := strings.TrimSpace(s)
					if name != "" {
						ignore[name] = true
					}
				}
			}
		}

		files, dirs, size := FolderStats(path, ignore)

		return valpkg.StrVal(fmt.Sprintf(
			"Ordner: %d, Dateien: %d | Größe: %s",
			dirs, files, formatHuman(size),
		))
	})

	// folder.SecureDelete
	Register(ns+"SecureDelete", "folder", "path", "Löscht ein Verzeichnis rekursiv durch Crypto-Shredding.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.ErrorVal("folder.SecureDelete(path)")
		}
		root, errVal := absPathVal(args[0].Str)
		if errVal != nil {
			return *errVal
		}

		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return fmt.Errorf("Zugriffsfehler bei '%s': %w", path, err)
			}
			if !d.IsDir() {
				if err := secureDeleteFile(path); err != nil {
					return fmt.Errorf("Shredding fehlgeschlagen für '%s': %w", path, err)
				}
			}
			return nil
		})

		if err != nil {
			return valpkg.ErrorVal(err.Error())
		}

		if err := os.RemoveAll(root); err != nil {
			return valpkg.ErrorVal("Ordnerstruktur konnte nicht gelöscht werden: " + err.Error())
		}

		return valpkg.BoolVal(true)
	})
}

// =============================================================================
// HELPER
// =============================================================================

func secureDeleteFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("Datei nicht gefunden: %w", err)
	}

	key := make([]byte, 32)
	iv := make([]byte, 16)
	if _, err := rand.Read(key); err != nil {
		return fmt.Errorf("Schlüsselgenerierung fehlgeschlagen: %w", err)
	}
	if _, err := rand.Read(iv); err != nil {
		return fmt.Errorf("IV-Generierung fehlgeschlagen: %w", err)
	}

	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("Zugriff verweigert: %w", err)
	}
	defer f.Close()

	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("Cipher-Initialisierung fehlgeschlagen: %w", err)
	}
	stream := cipher.NewCTR(block, iv)

	buf := make([]byte, 64*1024)
	var offset int64
	for offset < info.Size() {
		n, err := f.ReadAt(buf, offset)
		if n > 0 {
			stream.XORKeyStream(buf[:n], buf[:n])
			if _, err := f.WriteAt(buf[:n], offset); err != nil {
				return fmt.Errorf("Schreibfehler beim Überschreiben: %w", err)
			}
		}
		if err != nil {
			break
		}
		offset += int64(n)
	}

	f.Sync()
	f.Truncate(0)
	f.Close()

	randomName := fmt.Sprintf("shred_%x%x", key[:4], iv[:4])
	newPath := filepath.Join(filepath.Dir(path), randomName)
	if err := os.Rename(path, newPath); err != nil {
		return fmt.Errorf("Umbenennen fehlgeschlagen: %w", err)
	}
	if err := os.Remove(newPath); err != nil {
		return fmt.Errorf("Löschen fehlgeschlagen: %w", err)
	}

	return nil
}

// =============================================================================
// WALK / STATS
// =============================================================================

type FolderOptions struct {
	Path      string
	Pattern   string
	Recursive bool
	FullPath  bool
	Ignore    map[string]bool
	Flags     string
}

type FolderItem struct {
	Path string
	Name string
	Info fs.FileInfo
	Err  error
}

func WalkFolder(opts FolderOptions, fn func(FolderItem) error) error {

	root := opts.Path

	return filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {

		if err != nil {
			return nil
		}

		// -------------------------
		// IGNORE FILTER
		// -------------------------
		if opts.Ignore != nil {
			if opts.Ignore[d.Name()] {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		// -------------------------
		// NON-RECURSIVE MODE
		// -------------------------
		if !opts.Recursive && p != root {
			if d.IsDir() {
				return filepath.SkipDir // Unterverzeichnisse skippen
			}
			// Datei im direkten Root → durchlassen
		}

		// -------------------------
		// PATTERN FILTER
		// Root selbst nie filtern (d.Name() wäre der Ordnername, nicht eine Datei)
		// -------------------------
		if p != root && opts.Pattern != "" && opts.Pattern != "*" {
			ok, _ := filepath.Match(opts.Pattern, d.Name())
			if !ok {
				if d.IsDir() {
					return nil // Ordner überspringen aber nicht skippen (Geschwister bleiben erreichbar)
				}
				return nil
			}
		}

		// -------------------------
		// FILE INFO
		// -------------------------
		var info fs.FileInfo
		if d.Type().IsRegular() || d.IsDir() {
			info, _ = d.Info()
		}

		return fn(FolderItem{
			Path: p,
			Name: d.Name(),
			Info: info,
		})
	})
}

func parseFolderArgs(args []valpkg.Value) FolderOptions {
	opts := FolderOptions{
		Path:      ".",
		Pattern:   "*",
		Recursive: false,
		FullPath:  true,
		Ignore:    nil,
	}

	if len(args) >= 1 && args[0].Str != "" {
		opts.Path = args[0].Str
	}
	if len(args) >= 2 && args[1].Str != "" {
		opts.Pattern = args[1].Str
	}
	if len(args) >= 3 {
		opts.Recursive = fileTruthy(args[2])
	}
	if len(args) >= 4 {
		opts.FullPath = fileTruthy(args[3])
	}
	if len(args) >= 5 {
		opts.Ignore = make(map[string]bool)
		switch args[4].Kind {
		case valpkg.KindArr:
			for _, v := range args[4].Arr {
				name := strings.TrimSpace(v.Str)
				if name != "" {
					opts.Ignore[name] = true
				}
			}
		default:
			for _, s := range strings.Split(args[4].Str, ",") {
				name := strings.TrimSpace(s)
				if name != "" {
					opts.Ignore[name] = true
				}
			}
		}
	}
	return opts
}

func FolderStats(path string, ignore map[string]bool) (files, dirs int64, size int64) {

	opts := FolderOptions{
		Path:      path,
		Recursive: true, // FolderStats muss immer rekursiv zählen
		Ignore:    ignore,
	}

	_ = WalkFolder(opts, func(it FolderItem) error {
		if it.Info == nil {
			return nil
		}
		if it.Info.IsDir() {
			dirs++
		} else {
			files++
			size += it.Info.Size()
		}
		return nil
	})

	return
}

func init() { initFolderFunctions() }
