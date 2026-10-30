// ------------------------
// stdlib_git.go
// ------------------------

package stdlib

import (
	"fmt"
	"os"
	"strings"
	"time"
	valpkg "vbxc/value"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

// =============================================================================
// Git-Funktionen
// =============================================================================

// registerGitFuncs registriert alle git.* Funktionen im VBX-Namespace.
// Basiert auf go-git (reines Go, kein externes Git-Binary nötig).
func initGitFunctions() {

	if builtins == nil {
		builtins = make(map[string]BuiltinInfo)
	}

	ns := "git."

	// ---------------------------------------------------------------
	// Clone / CloneWithKey
	// ---------------------------------------------------------------

	Register(ns+"Clone", "git", "url [, path]",
		"Klont ein Git-Repository ohne Authentifizierung. Rückgabe: Bool - true bei Erfolg",
		func(args []valpkg.Value) valpkg.Value {

			if len(args) < 1 {
				return valpkg.ErrorVal("git.Clone: Erwartet mindestens url.")
			}

			url := args[0].Str

			rawPath := ""
			if len(args) >= 2 {
				rawPath = args[1].Str
			}

			path, errVal := cloneTargetPath(rawPath, url, "Clone")
			if errVal != nil {
				return *errVal
			}

			_, err := git.PlainClone(path, false, &git.CloneOptions{
				URL: url,
			})
			if err != nil {
				return valpkg.ErrorVal(fmt.Sprintf(
					"git.Clone: Klonen von %q nach %q fehlgeschlagen: %v",
					url, path, err))
			}

			return valpkg.Value{
				Kind: valpkg.KindBool,
				Bool: true,
			}
		})

	Register(ns+"CloneWithToken", "git", "url, token [, path] [, username]",
		"Klont ein Git-Repository per HTTPS mit Token-Authentifizierung. Rückgabe: Bool - true bei Erfolg",
		func(args []valpkg.Value) valpkg.Value {

			if len(args) < 2 {
				return valpkg.ErrorVal("git.CloneWithToken: Erwartet mindestens url und token.")
			}

			url := args[0].Str
			token := args[1].Str

			rawPath := ""
			if len(args) >= 3 {
				rawPath = args[2].Str
			}

			path, errVal := cloneTargetPath(rawPath, url, "CloneWithToken")
			if errVal != nil {
				return *errVal
			}

			username := "x-access-token"

			if len(args) >= 4 {
				if args[3].Kind != valpkg.KindStr {
					return valpkg.ErrorVal(
						"git.CloneWithToken: username muss ein String sein.")
				}

				if args[3].Str != "" {
					username = args[3].Str
				}
			}

			_, err := git.PlainClone(path, false, &git.CloneOptions{
				URL: url,
				Auth: &http.BasicAuth{
					Username: username,
					Password: token,
				},
			})

			if err != nil {
				return valpkg.ErrorVal(fmt.Sprintf(
					"git.CloneWithToken: Klonen von %q nach %q fehlgeschlagen: %v",
					url, path, err))
			}

			return valpkg.Value{
				Kind: valpkg.KindBool,
				Bool: true,
			}
		})

	Register(ns+"CloneWithKey", "git", "url, keyPath [, path] [, knownHostsPath]",
		"Klont ein Git-Repository per SSH mit Schlüssel-Authentifizierung. Rückgabe: Bool - true bei Erfolg",
		func(args []valpkg.Value) valpkg.Value {

			if len(args) < 2 {
				return valpkg.ErrorVal(
					"git.CloneWithKey: Erwartet mindestens url und keyPath.")
			}

			url := args[0].Str

			keyPath, errVal := absPathVal(args[1].Str)
			if errVal != nil {
				return *errVal
			}

			rawPath := ""
			if len(args) >= 3 {
				rawPath = args[2].Str
			}

			path, errVal := cloneTargetPath(rawPath, url, "CloneWithKey")
			if errVal != nil {
				return *errVal
			}

			hostKeyCallback, errVal := buildHostKeyCallback(args, 3)
			if errVal != nil {
				return *errVal
			}

			auth, err := gitssh.NewPublicKeysFromFile(
				"git",
				keyPath,
				"",
			)
			if err != nil {
				return valpkg.ErrorVal(fmt.Sprintf(
					"git.CloneWithKey: Key %q konnte nicht geladen werden: %v",
					keyPath, err))
			}
			auth.HostKeyCallback = hostKeyCallback

			_, err = git.PlainClone(path, false, &git.CloneOptions{
				URL:  url,
				Auth: auth,
			})
			if err != nil {
				return valpkg.ErrorVal(fmt.Sprintf(
					"git.CloneWithKey: Klonen von %q nach %q fehlgeschlagen: %v",
					url, path, err))
			}

			return valpkg.Value{
				Kind: valpkg.KindBool,
				Bool: true,
			}
		})

	// ---------------------------------------------------------------
	// Pull / Push / Fetch
	// ---------------------------------------------------------------

	Register(ns+"Pull", "git", "[path] [, keyPath] [, knownHostsPath] [, token] [, username]",
		"Holt Änderungen vom Remote und führt sie in den aktuellen Branch zusammen. Rückgabe: Bool - true bei Erfolg (auch wenn bereits aktuell)",
		func(args []valpkg.Value) valpkg.Value {
			rawPath := ""
			if len(args) >= 1 {
				rawPath = args[0].Str
			}

			path, errVal := repoPathOrDefault(rawPath)
			if errVal != nil {
				return *errVal
			}

			auth, errVal := gitAuthFromArgs(args, 1, 2, 3, 4, "git.Pull")
			if errVal != nil {
				return *errVal
			}

			repo, err := git.PlainOpen(path)
			if err != nil {
				return valpkg.ErrorVal(fmt.Sprintf(
					"git.Pull: %q ist kein gültiges Repository: %v",
					path, err))
			}

			wt, err := repo.Worktree()
			if err != nil {
				return valpkg.ErrorVal(fmt.Sprintf(
					"git.Pull: Worktree nicht lesbar: %v",
					err))
			}

			err = wt.Pull(&git.PullOptions{
				Auth: auth,
			})
			if err != nil && err != git.NoErrAlreadyUpToDate {
				return valpkg.ErrorVal(fmt.Sprintf(
					"git.Pull: Pull fehlgeschlagen: %v",
					err))
			}

			return valpkg.Value{
				Kind: valpkg.KindBool,
				Bool: true,
			}
		})

	Register(ns+"Push", "git", "[path] [, keyPath] [, knownHostsPath] [, token] [, username]",
		"Sendet lokale Commits an den Remote. Rückgabe: Bool - true bei Erfolg (auch wenn nichts zu pushen war)",
		func(args []valpkg.Value) valpkg.Value {
			rawPath := ""
			if len(args) >= 1 {
				rawPath = args[0].Str
			}

			path, errVal := repoPathOrDefault(rawPath)
			if errVal != nil {
				return *errVal
			}

			auth, errVal := gitAuthFromArgs(args, 1, 2, 3, 4, "git.Push")
			if errVal != nil {
				return *errVal
			}

			repo, err := git.PlainOpen(path)
			if err != nil {
				return valpkg.ErrorVal(fmt.Sprintf(
					"git.Push: %q ist kein gültiges Repository: %v",
					path, err))
			}

			err = repo.Push(&git.PushOptions{
				Auth: auth,
			})
			if err != nil && err != git.NoErrAlreadyUpToDate {
				return valpkg.ErrorVal(fmt.Sprintf(
					"git.Push: Push fehlgeschlagen: %v",
					err))
			}

			return valpkg.Value{
				Kind: valpkg.KindBool,
				Bool: true,
			}
		})

	Register(ns+"Fetch", "git", "[path] [, keyPath] [, knownHostsPath] [, token] [, username]",
		"Holt Änderungen vom Remote, ohne sie zu übernehmen. Rückgabe: Bool - true bei Erfolg (auch wenn bereits aktuell)",
		func(args []valpkg.Value) valpkg.Value {
			rawPath := ""
			if len(args) >= 1 {
				rawPath = args[0].Str
			}

			path, errVal := repoPathOrDefault(rawPath)
			if errVal != nil {
				return *errVal
			}

			auth, errVal := gitAuthFromArgs(args, 1, 2, 3, 4, "git.Fetch")
			if errVal != nil {
				return *errVal
			}

			repo, err := git.PlainOpen(path)
			if err != nil {
				return valpkg.ErrorVal(fmt.Sprintf(
					"git.Fetch: %q ist kein gültiges Repository: %v",
					path, err))
			}

			err = repo.Fetch(&git.FetchOptions{
				Auth: auth,
			})
			if err != nil && err != git.NoErrAlreadyUpToDate {
				return valpkg.ErrorVal(fmt.Sprintf(
					"git.Fetch: Fetch fehlgeschlagen: %v",
					err))
			}

			return valpkg.Value{
				Kind: valpkg.KindBool,
				Bool: true,
			}
		})

	// ---------------------------------------------------------------
	// Add / Commit / Remove
	// ---------------------------------------------------------------

	Register(ns+"Add", "git", "pattern [, path]",
		"Fügt Dateien zum Git-Index hinzu (staging). Rückgabe: Bool - true bei Erfolg",
		func(args []valpkg.Value) valpkg.Value {
			if len(args) < 1 {
				return valpkg.ErrorVal("git.Add: Erwartet mindestens pattern.")
			}
			pattern := args[0].Str

			rawPath := ""
			if len(args) >= 2 {
				rawPath = args[1].Str
			}

			path, errVal := repoPathOrDefault(rawPath)
			if errVal != nil {
				return *errVal
			}

			repo, err := git.PlainOpen(path)
			if err != nil {
				return valpkg.ErrorVal(fmt.Sprintf(
					"git.Add: %q ist kein gültiges Repository: %v",
					path, err))
			}

			wt, err := repo.Worktree()
			if err != nil {
				return valpkg.ErrorVal(fmt.Sprintf(
					"git.Add: Worktree nicht lesbar: %v",
					err))
			}

			_, err = wt.Add(pattern)
			if err != nil {
				return valpkg.ErrorVal(fmt.Sprintf(
					"git.Add: %q konnte nicht hinzugefügt werden: %v",
					pattern, err))
			}

			return valpkg.Value{
				Kind: valpkg.KindBool,
				Bool: true,
			}
		})

	Register(ns+"Commit", "git", "message [, path]",
		"Erstellt einen Commit mit allen gestagten Änderungen. Rückgabe: String - Hash des erzeugten Commits",
		func(args []valpkg.Value) valpkg.Value {
			if len(args) < 1 {
				return valpkg.ErrorVal("git.Commit: Erwartet mindestens message.")
			}
			message := args[0].Str

			rawPath := ""
			if len(args) >= 2 {
				rawPath = args[1].Str
			}

			path, errVal := repoPathOrDefault(rawPath)
			if errVal != nil {
				return *errVal
			}

			repo, err := git.PlainOpen(path)
			if err != nil {
				return valpkg.ErrorVal(fmt.Sprintf(
					"git.Commit: %q ist kein gültiges Repository: %v",
					path, err))
			}

			wt, err := repo.Worktree()
			if err != nil {
				return valpkg.ErrorVal(fmt.Sprintf(
					"git.Commit: Worktree nicht lesbar: %v",
					err))
			}

			hash, err := wt.Commit(message, &git.CommitOptions{
				Author: &object.Signature{
					Name:  "VBX",
					Email: "vbx@localhost",
					When:  time.Now(),
				},
			})
			if err != nil {
				return valpkg.ErrorVal(fmt.Sprintf(
					"git.Commit: Commit fehlgeschlagen: %v",
					err))
			}

			return valpkg.Value{
				Kind: valpkg.KindStr,
				Str:  hash.String(),
			}
		})

	Register(ns+"Remove", "git", "pattern [, path]",
		"Entfernt eine Datei aus Git-Index und Arbeitsverzeichnis. Rückgabe: Bool - true bei Erfolg",
		func(args []valpkg.Value) valpkg.Value {
			if len(args) < 1 {
				return valpkg.ErrorVal("git.Remove: Erwartet mindestens pattern.")
			}
			pattern := args[0].Str

			rawPath := ""
			if len(args) >= 2 {
				rawPath = args[1].Str
			}

			path, errVal := repoPathOrDefault(rawPath)
			if errVal != nil {
				return *errVal
			}

			repo, err := git.PlainOpen(path)
			if err != nil {
				return valpkg.ErrorVal(fmt.Sprintf(
					"git.Remove: %q ist kein gültiges Repository: %v",
					path, err))
			}

			wt, err := repo.Worktree()
			if err != nil {
				return valpkg.ErrorVal(fmt.Sprintf(
					"git.Remove: Worktree nicht lesbar: %v",
					err))
			}

			_, err = wt.Remove(pattern)
			if err != nil {
				return valpkg.ErrorVal(fmt.Sprintf(
					"git.Remove: %q konnte nicht entfernt werden: %v",
					pattern, err))
			}

			return valpkg.Value{
				Kind: valpkg.KindBool,
				Bool: true,
			}
		})

	Register(ns+"QuickPush", "git", "message [, pattern] [, path] [, keyPath] [, knownHostsPath] [, token] [, username]",
		"Stagt alle Änderungen, erstellt einen Commit und pusht ihn – in einem Aufruf. Rückgabe: String - Hash des erzeugten Commits",
		func(args []valpkg.Value) valpkg.Value {
			if len(args) < 1 {
				return valpkg.ErrorVal("git.QuickPush: Erwartet mindestens message.")
			}
			message := args[0].Str

			pattern := "."
			if len(args) >= 2 && args[1].Str != "" {
				pattern = args[1].Str
			}

			rawPath := ""
			if len(args) >= 3 {
				rawPath = args[2].Str
			}

			path, errVal := repoPathOrDefault(rawPath)
			if errVal != nil {
				return *errVal
			}

			auth, errVal := gitAuthFromArgs(args, 3, 4, 5, 6, "git.QuickPush")
			if errVal != nil {
				return *errVal
			}

			repo, err := git.PlainOpen(path)
			if err != nil {
				return valpkg.ErrorVal(fmt.Sprintf(
					"git.QuickPush: %q ist kein gültiges Repository: %v",
					path, err))
			}

			wt, err := repo.Worktree()
			if err != nil {
				return valpkg.ErrorVal(fmt.Sprintf(
					"git.QuickPush: Worktree nicht lesbar: %v",
					err))
			}

			_, err = wt.Add(pattern)
			if err != nil {
				return valpkg.ErrorVal(fmt.Sprintf(
					"git.QuickPush: %q konnte nicht gestaged werden: %v",
					pattern, err))
			}

			hash, err := wt.Commit(message, &git.CommitOptions{
				Author: &object.Signature{
					Name:  "VBX",
					Email: "vbx@localhost",
					When:  time.Now(),
				},
			})
			if err != nil {
				return valpkg.ErrorVal(fmt.Sprintf(
					"git.QuickPush: Commit fehlgeschlagen: %v",
					err))
			}

			err = repo.Push(&git.PushOptions{
				Auth: auth,
			})
			if err != nil && err != git.NoErrAlreadyUpToDate {
				return valpkg.ErrorVal(fmt.Sprintf(
					"git.QuickPush: Commit %q erstellt, aber Push fehlgeschlagen: %v",
					hash.String(), err))
			}

			return valpkg.Value{
				Kind: valpkg.KindStr,
				Str:  hash.String(),
			}
		})

	// ---------------------------------------------------------------
	// Status / Log / Diff
	// ---------------------------------------------------------------

	Register(ns+"Status", "git", "[path]",
		"Liefert den Status des Arbeitsverzeichnisses. Rückgabe: Map mit Arrays 'modified', 'added', 'deleted', 'untracked' (jeweils Dateipfade)",
		func(args []valpkg.Value) valpkg.Value {
			rawPath := ""
			if len(args) >= 1 {
				rawPath = args[0].Str
			}

			path, errVal := repoPathOrDefault(rawPath)
			if errVal != nil {
				return *errVal
			}

			repo, err := git.PlainOpen(path)
			if err != nil {
				return valpkg.ErrorVal(fmt.Sprintf(
					"git.Status: %q ist kein gültiges Repository: %v",
					path, err))
			}

			wt, err := repo.Worktree()
			if err != nil {
				return valpkg.ErrorVal(fmt.Sprintf(
					"git.Status: Worktree nicht lesbar: %v",
					err))
			}

			status, err := wt.Status()
			if err != nil {
				return valpkg.ErrorVal(fmt.Sprintf(
					"git.Status: Status-Abfrage fehlgeschlagen: %v",
					err))
			}

			var modified, added, deleted, untracked []valpkg.Value

			for file, s := range status {
				v := valpkg.Value{
					Kind: valpkg.KindStr,
					Str:  file,
				}

				switch s.Worktree {
				case git.Modified:
					modified = append(modified, v)

				case git.Added:
					added = append(added, v)

				case git.Deleted:
					deleted = append(deleted, v)

				case git.Untracked:
					untracked = append(untracked, v)
				}
			}

			result := map[string]valpkg.Value{
				"modified": {
					Kind: valpkg.KindArr,
					Arr:  modified,
				},
				"added": {
					Kind: valpkg.KindArr,
					Arr:  added,
				},
				"deleted": {
					Kind: valpkg.KindArr,
					Arr:  deleted,
				},
				"untracked": {
					Kind: valpkg.KindArr,
					Arr:  untracked,
				},
			}

			return valpkg.Value{
				Kind: valpkg.KindMap,
				Map:  result,
			}
		})

	Register(ns+"Log", "git", "[limit] [, path]",
		"Liefert die Commit-Historie. Rückgabe: Array von Maps mit 'hash', 'author', 'message', 'date'",
		func(args []valpkg.Value) valpkg.Value {
			limit := 0
			if len(args) >= 1 {
				limit = int(args[0].Num)
			}

			rawPath := ""
			if len(args) >= 2 {
				rawPath = args[1].Str
			}

			path, errVal := repoPathOrDefault(rawPath)
			if errVal != nil {
				return *errVal
			}

			repo, err := git.PlainOpen(path)
			if err != nil {
				return valpkg.ErrorVal(fmt.Sprintf(
					"git.Log: %q ist kein gültiges Repository: %v",
					path, err))
			}

			ref, err := repo.Head()
			if err != nil {
				return valpkg.ErrorVal(fmt.Sprintf(
					"git.Log: HEAD nicht auflösbar: %v",
					err))
			}

			iter, err := repo.Log(&git.LogOptions{
				From: ref.Hash(),
			})
			if err != nil {
				return valpkg.ErrorVal(fmt.Sprintf(
					"git.Log: Historie nicht lesbar: %v",
					err))
			}
			defer iter.Close()

			var entries []valpkg.Value
			count := 0

			for {
				if limit > 0 && count >= limit {
					break
				}

				c, err := iter.Next()
				if err != nil {
					break
				}

				entries = append(entries, valpkg.Value{
					Kind: valpkg.KindMap,
					Map: map[string]valpkg.Value{
						"hash": {
							Kind: valpkg.KindStr,
							Str:  c.Hash.String(),
						},
						"author": {
							Kind: valpkg.KindStr,
							Str:  c.Author.Name,
						},
						"message": {
							Kind: valpkg.KindStr,
							Str:  c.Message,
						},
						"date": {
							Kind: valpkg.KindStr,
							Str:  c.Author.When.Format("2006-01-02 15:04:05"),
						},
					},
				})

				count++
			}

			return valpkg.Value{
				Kind: valpkg.KindArr,
				Arr:  entries,
			}
		})

	Register(ns+"Diff", "git", "commitA [, commitB] [, path]",
		"Zeigt den Diff zwischen zwei Commits. Rückgabe: String - unified Diff",
		func(args []valpkg.Value) valpkg.Value {
			if len(args) < 1 {
				return valpkg.ErrorVal("git.Diff: Erwartet mindestens commitA.")
			}
			hashA := args[0].Str

			hashB := ""
			if len(args) >= 2 {
				hashB = args[1].Str
			}

			rawPath := ""
			if len(args) >= 3 {
				rawPath = args[2].Str
			}

			path, errVal := repoPathOrDefault(rawPath)
			if errVal != nil {
				return *errVal
			}

			repo, err := git.PlainOpen(path)
			if err != nil {
				return valpkg.ErrorVal(fmt.Sprintf(
					"git.Diff: %q ist kein gültiges Repository: %v",
					path, err))
			}

			revA, err := repo.ResolveRevision(plumbing.Revision(hashA))
			if err != nil {
				return valpkg.ErrorVal(fmt.Sprintf(
					"git.Diff: Commit %q nicht auflösbar: %v",
					hashA, err))
			}

			commitA, err := repo.CommitObject(*revA)
			if err != nil {
				return valpkg.ErrorVal(fmt.Sprintf(
					"git.Diff: Commit %q nicht lesbar: %v",
					hashA, err))
			}

			var commitB *object.Commit

			if hashB == "" {
				head, err := repo.Head()
				if err != nil {
					return valpkg.ErrorVal(fmt.Sprintf(
						"git.Diff: HEAD nicht auflösbar: %v",
						err))
				}

				commitB, err = repo.CommitObject(head.Hash())
				if err != nil {
					return valpkg.ErrorVal(fmt.Sprintf(
						"git.Diff: HEAD-Commit nicht lesbar: %v",
						err))
				}
			} else {
				revB, err := repo.ResolveRevision(plumbing.Revision(hashB))
				if err != nil {
					return valpkg.ErrorVal(fmt.Sprintf(
						"git.Diff: Commit %q nicht auflösbar: %v",
						hashB, err))
				}

				commitB, err = repo.CommitObject(*revB)
				if err != nil {
					return valpkg.ErrorVal(fmt.Sprintf(
						"git.Diff: Commit %q nicht lesbar: %v",
						hashB, err))
				}
			}

			patch, err := commitA.Patch(commitB)
			if err != nil {
				return valpkg.ErrorVal(fmt.Sprintf(
					"git.Diff: Patch-Erstellung fehlgeschlagen: %v",
					err))
			}

			return valpkg.Value{
				Kind: valpkg.KindStr,
				Str:  patch.String(),
			}
		})

	// ---------------------------------------------------------------
	// Reset / ResetHard
	// ---------------------------------------------------------------

	Register(ns+"ResetHard", "git", "commitHash [, path]",
		"Setzt das Repository hart auf einen bestimmten Commit zurück. Rückgabe: Bool - true bei Erfolg",
		func(args []valpkg.Value) valpkg.Value {
			if len(args) < 1 {
				return valpkg.ErrorVal("git.ResetHard: Erwartet mindestens commitHash.")
			}
			hashStr := args[0].Str

			rawPath := ""
			if len(args) >= 2 {
				rawPath = args[1].Str
			}

			path, errVal := repoPathOrDefault(rawPath)
			if errVal != nil {
				return *errVal
			}

			repo, err := git.PlainOpen(path)
			if err != nil {
				return valpkg.ErrorVal(fmt.Sprintf(
					"git.ResetHard: %q ist kein gültiges Repository: %v",
					path, err))
			}

			wt, err := repo.Worktree()
			if err != nil {
				return valpkg.ErrorVal(fmt.Sprintf(
					"git.ResetHard: Worktree nicht lesbar: %v",
					err))
			}

			rev, err := repo.ResolveRevision(plumbing.Revision(hashStr))
			if err != nil {
				return valpkg.ErrorVal(fmt.Sprintf(
					"git.ResetHard: Commit %q nicht auflösbar: %v",
					hashStr, err))
			}

			err = wt.Reset(&git.ResetOptions{
				Commit: *rev,
				Mode:   git.HardReset,
			})
			if err != nil {
				return valpkg.ErrorVal(fmt.Sprintf(
					"git.ResetHard: Reset auf %q fehlgeschlagen: %v",
					hashStr, err))
			}

			return valpkg.Value{
				Kind: valpkg.KindBool,
				Bool: true,
			}
		})

	Register(ns+"Reset", "git", "[mode] [, path]",
		"Hebt das Staging von Änderungen auf, ohne Dateien im Arbeitsverzeichnis zu verändern. Rückgabe: Bool - true bei Erfolg",
		func(args []valpkg.Value) valpkg.Value {
			mode := "mixed"
			if len(args) >= 1 && args[0].Str != "" {
				mode = args[0].Str
			}

			rawPath := ""
			if len(args) >= 2 {
				rawPath = args[1].Str
			}

			path, errVal := repoPathOrDefault(rawPath)
			if errVal != nil {
				return *errVal
			}

			repo, err := git.PlainOpen(path)
			if err != nil {
				return valpkg.ErrorVal(fmt.Sprintf(
					"git.Reset: %q ist kein gültiges Repository: %v",
					path, err))
			}

			wt, err := repo.Worktree()
			if err != nil {
				return valpkg.ErrorVal(fmt.Sprintf(
					"git.Reset: Worktree nicht lesbar: %v",
					err))
			}

			resetMode := git.MixedReset

			if mode == "soft" {
				resetMode = git.SoftReset
			}

			head, err := repo.Head()
			if err != nil {
				return valpkg.ErrorVal(fmt.Sprintf(
					"git.Reset: HEAD nicht auflösbar: %v",
					err))
			}

			err = wt.Reset(&git.ResetOptions{
				Commit: head.Hash(),
				Mode:   resetMode,
			})
			if err != nil {
				return valpkg.ErrorVal(fmt.Sprintf(
					"git.Reset: Reset fehlgeschlagen: %v",
					err))
			}

			return valpkg.Value{
				Kind: valpkg.KindBool,
				Bool: true,
			}
		})

	// ---------------------------------------------------------------
	// Branch / Checkout / Sonstiges
	// ---------------------------------------------------------------

	Register(ns+"CurrentBranch", "git", "[path]",
		"Liefert den Namen des aktuell ausgecheckten Branches. Rückgabe: String - Branch-Name (z.B. 'main')",
		func(args []valpkg.Value) valpkg.Value {
			rawPath := ""
			if len(args) >= 1 {
				rawPath = args[0].Str
			}

			path, errVal := repoPathOrDefault(rawPath)
			if errVal != nil {
				return *errVal
			}

			repo, err := git.PlainOpen(path)
			if err != nil {
				return valpkg.ErrorVal(fmt.Sprintf(
					"git.CurrentBranch: %q ist kein gültiges Repository: %v",
					path, err))
			}

			head, err := repo.Head()
			if err != nil {
				return valpkg.ErrorVal(fmt.Sprintf(
					"git.CurrentBranch: HEAD nicht auflösbar: %v",
					err))
			}

			return valpkg.Value{
				Kind: valpkg.KindStr,
				Str:  head.Name().Short(),
			}
		})

	Register(ns+"Checkout", "git", "branch [, create] [, path]",
		"Wechselt den Branch oder erstellt einen neuen. Rückgabe: Bool - true bei Erfolg",
		func(args []valpkg.Value) valpkg.Value {
			if len(args) < 1 {
				return valpkg.ErrorVal("git.Checkout: Erwartet mindestens branch.")
			}
			branch := args[0].Str

			create := false
			if len(args) >= 2 {
				create = args[1].Bool
			}

			rawPath := ""
			if len(args) >= 3 {
				rawPath = args[2].Str
			}

			path, errVal := repoPathOrDefault(rawPath)
			if errVal != nil {
				return *errVal
			}

			repo, err := git.PlainOpen(path)
			if err != nil {
				return valpkg.ErrorVal(fmt.Sprintf(
					"git.Checkout: %q ist kein gültiges Repository: %v",
					path, err))
			}

			wt, err := repo.Worktree()
			if err != nil {
				return valpkg.ErrorVal(fmt.Sprintf(
					"git.Checkout: Worktree nicht lesbar: %v",
					err))
			}

			err = wt.Checkout(&git.CheckoutOptions{
				Branch: plumbing.NewBranchReferenceName(branch),
				Create: create,
			})
			if err != nil {
				return valpkg.ErrorVal(fmt.Sprintf(
					"git.Checkout: Wechsel zu Branch %q fehlgeschlagen: %v",
					branch, err))
			}

			return valpkg.Value{
				Kind: valpkg.KindBool,
				Bool: true,
			}
		})

	Register(ns+"IsRepo", "git", "[path]",
		"Prüft, ob ein Verzeichnis ein gültiges Git-Repository ist. Rückgabe: Bool - true, wenn gültiges Repository",
		func(args []valpkg.Value) valpkg.Value {
			rawPath := ""
			if len(args) >= 1 {
				rawPath = args[0].Str
			}

			path, errVal := repoPathOrDefault(rawPath)
			if errVal != nil {
				return *errVal
			}

			_, err := git.PlainOpen(path)

			return valpkg.Value{
				Kind: valpkg.KindBool,
				Bool: err == nil,
			}
		})
}

// cloneTargetPath löst den Zielpfad für Clone-Varianten absolut auf und prüft,
// dass dort noch kein Verzeichnis existiert (go-git bricht sonst mit einem
// weniger sprechenden Fehler ab).
//
// Ist raw leer, wird der Zielverzeichnisname wie bei 'git clone' aus der URL
// abgeleitet (letztes Pfadsegment ohne '.git'-Endung), relativ zum aktuellen
// Arbeitsverzeichnis.
func cloneTargetPath(raw string, url string, funcName string) (string, *valpkg.Value) {

	target := raw
	if target == "" {
		target = deriveCloneDirName(url)
	}

	path, errVal := absPathVal(target)
	if errVal != nil {
		return "", errVal
	}

	if _, err := os.Stat(path); err == nil {
		v := valpkg.ErrorVal(fmt.Sprintf(
			"git.%s: Zielverzeichnis %q existiert bereits.",
			funcName, path))
		return "", &v

	} else if !os.IsNotExist(err) {
		v := valpkg.ErrorVal(fmt.Sprintf(
			"git.%s: Zielverzeichnis %q kann nicht geprüft werden: %v",
			funcName, path, err))
		return "", &v
	}

	return path, nil
}

// deriveCloneDirName leitet aus einer Git-URL den Standard-Verzeichnisnamen ab,
// so wie es 'git clone <url>' ohne explizites Zielverzeichnis tut:
// letztes Pfadsegment, '.git'-Endung entfernt.
//
// Beispiele:
//
//	https://github.com/user/repo.git -> repo
//	git@github.com:user/repo.git     -> repo
//	https://github.com/user/repo     -> repo
func deriveCloneDirName(url string) string {

	u := strings.TrimSuffix(strings.TrimSpace(url), "/")

	// Letztes Segment nach '/' oder ':' (SSH-Kurzform user@host:pfad)
	idx := strings.LastIndexAny(u, "/:")

	name := u
	if idx >= 0 {
		name = u[idx+1:]
	}

	name = strings.TrimSuffix(name, ".git")

	if name == "" {
		name = "repo"
	}

	return name
}

// repoPathOrDefault löst einen Repository-Pfad absolut auf. Ist raw leer,
// wird das aktuelle Arbeitsverzeichnis (".") verwendet - analog dazu, wie
// 'git status'/'git pull'/etc. ohne Pfadangabe im aktuellen Verzeichnis
// arbeiten.
func repoPathOrDefault(raw string) (string, *valpkg.Value) {

	target := raw
	if target == "" {
		target = "."
	}

	return absPathVal(target)
}

// gitAuthFromArgs baut optionale Authentifizierung für Remote-Operationen
// (Pull/Push/Fetch/QuickPush) aus positionellen Argumenten. keyIdx hat
// Vorrang vor tokenIdx, falls beide gesetzt sind. Sind weder keyPath noch
// token angegeben, wird (nil, nil) zurückgegeben - die Operation läuft dann
// ohne explizite Auth (funktioniert nur bei unauthentifizierten Remotes).
func gitAuthFromArgs(args []valpkg.Value, keyIdx int, knownHostsIdx int, tokenIdx int, usernameIdx int, funcName string) (transport.AuthMethod, *valpkg.Value) {

	keyPathRaw := ""
	if len(args) > keyIdx {
		keyPathRaw = args[keyIdx].Str
	}

	token := ""
	if len(args) > tokenIdx {
		token = args[tokenIdx].Str
	}

	if keyPathRaw != "" {
		keyPath, errVal := absPathVal(keyPathRaw)
		if errVal != nil {
			return nil, errVal
		}

		hostKeyCallback, errVal := buildHostKeyCallback(args, knownHostsIdx)
		if errVal != nil {
			return nil, errVal
		}

		auth, err := gitssh.NewPublicKeysFromFile("git", keyPath, "")
		if err != nil {
			v := valpkg.ErrorVal(fmt.Sprintf(
				"%s: Key %q konnte nicht geladen werden: %v",
				funcName, keyPath, err))
			return nil, &v
		}
		auth.HostKeyCallback = hostKeyCallback

		return auth, nil
	}

	if token != "" {
		username := "x-access-token"
		if len(args) > usernameIdx && args[usernameIdx].Str != "" {
			username = args[usernameIdx].Str
		}

		return &http.BasicAuth{
			Username: username,
			Password: token,
		}, nil
	}

	return nil, nil
}

func init() { initGitFunctions() }
