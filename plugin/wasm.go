package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"

	"vbxc/stdlib"
	"vbxc/value"
)

const expectedABIVersion = 1

// ------------------------------------------------------------
// Plugin-Beschreibung
// ------------------------------------------------------------

type pluginFuncDesc struct {
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
	Params      string `json:"params"`
	Description string `json:"description"`
}

type pluginHandle struct {
	runtime wazero.Runtime
	module  api.Module
}

var pluginBaseDir string
var loadedPlugins = map[string]*pluginHandle{}

// ------------------------------------------------------------
// Host Bridge Datentypen
// ------------------------------------------------------------

type hostFFmpegRequest struct {
	Args []string `json:"args"`
}

type hostFFmpegResponse struct {
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
	Code   int    `json:"code"`
}

type hostDockerRequest struct {
	Args  []string `json:"args"`
	Stdin string   `json:"stdin,omitempty"`
}

type hostDockerResponse struct {
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
	Code   int    `json:"code"`
}

type hostWriteFileRequest struct {
	Path string `json:"path"`
	Data string `json:"data"`
}

type hostWriteFileResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

type hostReplaceFileRequest struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

type hostReplaceFileResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// ------------------------------------------------------------
// FFmpeg auf dem Host ausführen
// ------------------------------------------------------------

func resolveFFmpegExecutable() string {
	if p := os.Getenv("VBX_FFMPEG"); p != "" {
		return p
	}
	if runtime.GOOS == "windows" {
		return "ffmpeg.exe"
	}
	return "ffmpeg"
}

func ffmpegExecHost(req hostFFmpegRequest) hostFFmpegResponse {

	if len(req.Args) == 0 {
		return hostFFmpegResponse{
			Stderr: "ffmpeg_exec: keine Argumente angegeben",
			Code:   -1,
		}
	}

	args := translateFFmpegArgs(req.Args)

	cmd := exec.Command(resolveFFmpegExecutable(), args...)

	var stdout strings.Builder
	var stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	code := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		} else {
			code = -1
		}
	}

	return hostFFmpegResponse{
		Stdout: strings.TrimSpace(stdout.String()),
		Stderr: strings.TrimSpace(stderr.String()),
		Code:   code,
	}
}

// ------------------------------------------------------------
// WASI-Gastpfad -> Host-Pfad
//
// Windows: /c/Media/test.mkv -> C:\Media\test.mkv
// Linux/macOS: Pfad bleibt unverändert.
// ------------------------------------------------------------

func guestToHostPath(path string) string {
	if runtime.GOOS != "windows" {
		return path
	}
	if len(path) >= 3 && path[0] == '/' &&
		((path[1] >= 'a' && path[1] <= 'z') || (path[1] >= 'A' && path[1] <= 'Z')) &&
		path[2] == '/' {
		drive := strings.ToUpper(string(path[1]))
		rest := strings.ReplaceAll(path[3:], "/", string(os.PathSeparator))
		return drive + ":\\" + rest
	}
	return path
}

func translateFFmpegArgs(args []string) []string {
	result := make([]string, len(args))
	for i, arg := range args {
		// Nur absolute Windows-WASI-Pfade übersetzen.
		if runtime.GOOS == "windows" && len(arg) >= 3 && arg[0] == '/' &&
			((arg[1] >= 'a' && arg[1] <= 'z') || (arg[1] >= 'A' && arg[1] <= 'Z')) &&
			arg[2] == '/' {
			result[i] = guestToHostPath(arg)
			continue
		}
		result[i] = arg
	}
	return result
}

// ------------------------------------------------------------
// Docker auf dem Host ausführen
// ------------------------------------------------------------

var dockerDeniedFlags = []string{
	"--privileged",
	"--cap-add",
	"--security-opt",
	"--pid=host",
	"--pid",
	"--network=host",
	"--ipc=host",
	"--uts=host",
	"--userns=host",
	"--device",
}

var dockerDeniedMountSources = []string{
	"/",
	"/etc",
	"/root",
	"/var/run/docker.sock",
	"/proc",
	"/sys",
}

func validateDockerArgs(args []string) error {
	for i, arg := range args {
		lower := strings.ToLower(arg)

		for _, denied := range dockerDeniedFlags {
			if lower == denied || strings.HasPrefix(lower, denied+"=") {
				return fmt.Errorf("Flag %q ist nicht erlaubt", arg)
			}
		}

		if lower == "--pid" && i+1 < len(args) && strings.EqualFold(args[i+1], "host") {
			return fmt.Errorf("--pid host ist nicht erlaubt")
		}

		if lower == "-v" || lower == "--volume" || lower == "--mount" {
			if i+1 >= len(args) {
				continue
			}
			if err := validateMountSpec(args[i+1]); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateMountSpec(spec string) error {
	var source string

	// --mount source=...,target=...
	if strings.Contains(spec, "=") && !strings.Contains(spec, ":") {
		for _, part := range strings.Split(spec, ",") {
			kv := strings.SplitN(part, "=", 2)
			if len(kv) == 2 && strings.EqualFold(kv[0], "source") {
				source = kv[1]
			}
		}
	} else {
		// -v source:target
		parts := strings.Split(spec, ":")
		if len(parts) >= 1 {
			source = parts[0]
		}
	}

	source = filepath.Clean(source)

	for _, denied := range dockerDeniedMountSources {
		if source == denied {
			return fmt.Errorf("Mount-Quelle %q ist nicht erlaubt", source)
		}
	}

	return nil
}

func dockerExecHost(req hostDockerRequest) hostDockerResponse {

	if err := validateDockerArgs(req.Args); err != nil {
		return hostDockerResponse{
			Stdout: "",
			Stderr: "docker_exec: " + err.Error(),
			Code:   -1,
		}
	}

	cmd := exec.Command("docker", req.Args...)

	if req.Stdin != "" {
		cmd.Stdin = strings.NewReader(req.Stdin)
	}

	var stdout strings.Builder
	var stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	code := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		} else {
			code = -1
		}
	}

	return hostDockerResponse{
		Stdout: strings.TrimSpace(stdout.String()),
		Stderr: strings.TrimSpace(stderr.String()),
		Code:   code,
	}
}

// ------------------------------------------------------------
// Datei auf dem Host ersetzen (replace_file)
//
// Source und Target kommen aus dem WASM-Gast und werden deshalb
// zunächst in Host-Pfade übersetzt. Windows sichert das Original
// erst auf eine Backup-Datei (mit Rollback).
// ------------------------------------------------------------

func replaceFileHost(req hostReplaceFileRequest) hostReplaceFileResponse {

	if strings.TrimSpace(req.Source) == "" {
		return hostReplaceFileResponse{Error: "keine Quelldatei angegeben"}
	}

	if strings.TrimSpace(req.Target) == "" {
		return hostReplaceFileResponse{Error: "keine Zieldatei angegeben"}
	}

	source := guestToHostPath(req.Source)
	target := guestToHostPath(req.Target)

	sourceAbs, err := filepath.Abs(source)
	if err != nil {
		return hostReplaceFileResponse{Error: fmt.Sprintf("Quelldatei konnte nicht aufgelöst werden: %v", err)}
	}

	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return hostReplaceFileResponse{Error: fmt.Sprintf("Zieldatei konnte nicht aufgelöst werden: %v", err)}
	}

	if _, err := os.Stat(sourceAbs); err != nil {
		return hostReplaceFileResponse{
			Error: fmt.Sprintf("Quelldatei existiert nicht: %s: %v", sourceAbs, err),
		}
	}

	if sourceAbs == targetAbs {
		return hostReplaceFileResponse{Error: "Quelle und Ziel sind identisch"}
	}

	// Linux/macOS: os.Rename ersetzt die vorhandene Datei.
	if runtime.GOOS != "windows" {
		if err := os.Rename(sourceAbs, targetAbs); err != nil {
			return hostReplaceFileResponse{Error: fmt.Sprintf("Datei konnte nicht ersetzt werden: %v", err)}
		}
		return hostReplaceFileResponse{OK: true}
	}

	// Windows: Backup + zwei Renames mit Rollback.
	backup := targetAbs + ".vbx-tag-backup.tmp"
	_ = os.Remove(backup)

	if err := os.Rename(targetAbs, backup); err != nil {
		return hostReplaceFileResponse{
			Error: fmt.Sprintf("Originaldatei konnte nicht gesichert werden: %v", err),
		}
	}

	if err := os.Rename(sourceAbs, targetAbs); err != nil {
		rollbackErr := os.Rename(backup, targetAbs)
		if rollbackErr != nil {
			return hostReplaceFileResponse{
				Error: fmt.Sprintf(
					"Neue Datei konnte nicht übernommen werden: %v; Originaldatei konnte auch nicht wiederhergestellt werden: %v",
					err, rollbackErr,
				),
			}
		}
		return hostReplaceFileResponse{Error: fmt.Sprintf("Neue Datei konnte nicht übernommen werden: %v", err)}
	}

	if err := os.Remove(backup); err != nil {
		// Die Ersetzung war erfolgreich; das Backup bleibt im Fehlerfall
		// liegen, damit keine Daten verloren gehen.
		return hostReplaceFileResponse{
			Error: fmt.Sprintf("Datei wurde ersetzt, aber das Backup konnte nicht gelöscht werden: %v", err),
		}
	}

	return hostReplaceFileResponse{OK: true}
}

// ------------------------------------------------------------
// Plugin-Basisverzeichnis + sicherer Join (write_file)
// ------------------------------------------------------------

func resolvePluginBaseDir() (string, error) {
	if pluginBaseDir != "" {
		return pluginBaseDir, nil
	}

	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("Arbeitsverzeichnis konnte nicht ermittelt werden: %w", err)
	}

	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}

	pluginBaseDir = abs
	return pluginBaseDir, nil
}

// safeJoin verhindert Pfad-Escape (write_file).
func safeJoin(base, requested string) (string, error) {
	joined := filepath.Join(base, requested)

	cleanBase := filepath.Clean(base)
	cleanJoined := filepath.Clean(joined)

	if cleanJoined != cleanBase &&
		!strings.HasPrefix(cleanJoined+string(os.PathSeparator), cleanBase+string(os.PathSeparator)) {
		return "", fmt.Errorf("Pfad %q liegt außerhalb des erlaubten Verzeichnisses", requested)
	}

	return cleanJoined, nil
}

// ------------------------------------------------------------
// Guest-Dateisystem (WASI)
//
// Damit Plugins direkte Dateizugriffe (os.Stat, os.Open, ...) nutzen
// können, braucht das instanziierte WASI-Modul mindestens eine
// Preopened-Directory.
//
// Linux/macOS: 1:1-Mount von "/" auf "/".
// Windows: jedes vorhandene Laufwerk unter /c, /d, ... (WSL-Konvention).
// ------------------------------------------------------------

func buildGuestFSConfig() (wazero.FSConfig, error) {
	fsConfig := wazero.NewFSConfig()

	if runtime.GOOS != "windows" {
		return fsConfig.WithDirMount("/", "/"), nil
	}

	mounted := false
	for letter := 'A'; letter <= 'Z'; letter++ {
		drive := string(letter) + `:\`
		if _, err := os.Stat(drive); err != nil {
			continue
		}
		guestRoot := "/" + strings.ToLower(string(letter))
		fsConfig = fsConfig.WithDirMount(drive, guestRoot)
		mounted = true
	}

	if !mounted {
		return nil, fmt.Errorf("kein Laufwerk zum Mounten gefunden")
	}

	return fsConfig, nil
}

// ------------------------------------------------------------
// Host-Pfad <-> Gast-Pfad Übersetzung (Argument-Strings)
// ------------------------------------------------------------

func looksLikeHostPath(s string) bool {
	if s == "." {
		return true
	}
	if strings.HasPrefix(s, "/") {
		return true
	}
	// Windows: "C:\" oder "C:/"
	if len(s) >= 3 &&
		((s[0] >= 'A' && s[0] <= 'Z') || (s[0] >= 'a' && s[0] <= 'z')) &&
		s[1] == ':' &&
		(s[2] == '\\' || s[2] == '/') {
		return true
	}
	return false
}

func hostToGuestPath(hostPath string) (string, error) {
	abs, err := filepath.Abs(hostPath)
	if err != nil {
		return "", err
	}

	if runtime.GOOS != "windows" {
		return filepath.ToSlash(abs), nil
	}

	if len(abs) < 2 || abs[1] != ':' {
		return "", fmt.Errorf("kein gültiger Windows-Pfad: %q", abs)
	}

	drive := strings.ToLower(string(abs[0]))
	rest := strings.ReplaceAll(abs[2:], `\`, "/")

	return "/" + drive + rest, nil
}

// ------------------------------------------------------------
// Plugin-Verzeichnis auflösen
// ------------------------------------------------------------

func pluginDir() (string, error) {
	if p := os.Getenv("VBX_PLUGIN_PATH"); p != "" {
		abs, err := filepath.Abs(p)
		if err != nil {
			return "", fmt.Errorf("VBX_PLUGIN_PATH konnte nicht aufgelöst werden: %w", err)
		}
		return abs, nil
	}

	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("Pfad der vbxc-Executable konnte nicht ermittelt werden: %w", err)
	}

	return filepath.Join(filepath.Dir(exePath), "plugins"), nil
}

// ------------------------------------------------------------
// Host Bridge in WASM bereitstellen
// ------------------------------------------------------------

// allocateAndWrite ist der gemeinsame Rückgabe-Weg aller
// Bridge-Funktionen: Antwort-JSON ins Plugin-Speicherbild schreiben und
// als (ptr<<32)|len packen.
func allocateAndWrite(
	ctx context.Context,
	mod api.Module,
	caller string,
	resultBytes []byte,
) uint64 {

	allocFn := mod.ExportedFunction("alloc")
	if allocFn == nil {
		pluginDebugf("%s: Plugin exportiert keine Funktion 'alloc'", caller)
		return 0
	}

	res, err := allocFn.Call(ctx, uint64(len(resultBytes)))
	if err != nil || len(res) == 0 {
		pluginDebugf("%s: Plugin-alloc fehlgeschlagen: %v", caller, err)
		return 0
	}

	ptr := uint32(res[0])

	if !mod.Memory().Write(ptr, resultBytes) {
		pluginDebugf("%s: Antwort konnte nicht in WASM-Speicher geschrieben werden", caller)
		return 0
	}

	return (uint64(ptr) << 32) | uint64(len(resultBytes))
}

func instantiateHostBridge(ctx context.Context, rt wazero.Runtime) error {

	builder := rt.NewHostModuleBuilder("vbx_host")

	// ============================================================
	// docker_exec
	// ============================================================

	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, mod api.Module, reqPtr, reqLen uint32) uint64 {

			reqBytes, ok := mod.Memory().Read(reqPtr, reqLen)
			if !ok {
				return hostBridgeError(ctx, mod, "docker_exec: Request konnte nicht aus WASM-Speicher gelesen werden")
			}

			var req hostDockerRequest
			if err := json.Unmarshal(reqBytes, &req); err != nil {
				return hostBridgeError(ctx, mod, "docker_exec: ungültiger Request: "+err.Error())
			}

			result := dockerExecHost(req)

			resultBytes, err := json.Marshal(result)
			if err != nil {
				return hostBridgeError(ctx, mod, "docker_exec: Host-Antwort konnte nicht kodiert werden: "+err.Error())
			}

			return allocateAndWrite(ctx, mod, "docker_exec", resultBytes)

		}).
		Export("docker_exec")

	// ============================================================
	// write_file
	// ============================================================

	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, mod api.Module, reqPtr, reqLen uint32) uint64 {

			reqBytes, ok := mod.Memory().Read(reqPtr, reqLen)
			if !ok {
				return hostBridgeError(ctx, mod, "write_file: Request konnte nicht aus WASM-Speicher gelesen werden")
			}

			var req hostWriteFileRequest
			if err := json.Unmarshal(reqBytes, &req); err != nil {
				return hostBridgeError(ctx, mod, "write_file: ungültiger Request: "+err.Error())
			}

			if strings.TrimSpace(req.Path) == "" {
				return hostBridgeError(ctx, mod, "write_file: kein Dateipfad angegeben")
			}

			base, err := resolvePluginBaseDir()
			if err != nil {
				return hostBridgeError(ctx, mod, "write_file: "+err.Error())
			}

			safePath, err := safeJoin(base, req.Path)
			if err != nil {
				return hostBridgeError(ctx, mod, "write_file: "+err.Error())
			}

			if err := os.WriteFile(safePath, []byte(req.Data), 0644); err != nil {
				return hostBridgeError(ctx, mod, "write_file: Datei konnte nicht geschrieben werden: "+err.Error())
			}

			result := hostWriteFileResponse{OK: true}

			resultBytes, err := json.Marshal(result)
			if err != nil {
				return hostBridgeError(ctx, mod, "write_file: Antwort konnte nicht kodiert werden: "+err.Error())
			}

			return allocateAndWrite(ctx, mod, "write_file", resultBytes)

		}).
		Export("write_file")

	// ============================================================
	// ffmpeg_exec
	// ============================================================

	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, mod api.Module, reqPtr, reqLen uint32) uint64 {

			reqBytes, ok := mod.Memory().Read(reqPtr, reqLen)
			if !ok {
				return hostBridgeError(ctx, mod, "ffmpeg_exec: Request konnte nicht aus WASM-Speicher gelesen werden")
			}

			var req hostFFmpegRequest
			if err := json.Unmarshal(reqBytes, &req); err != nil {
				return hostBridgeError(ctx, mod, "ffmpeg_exec: ungültiger Request: "+err.Error())
			}

			result := ffmpegExecHost(req)

			resultBytes, err := json.Marshal(result)
			if err != nil {
				return hostBridgeError(ctx, mod, "ffmpeg_exec: Host-Antwort konnte nicht kodiert werden: "+err.Error())
			}

			return allocateAndWrite(ctx, mod, "ffmpeg_exec", resultBytes)

		}).
		Export("ffmpeg_exec")

	// ============================================================
	// replace_file
	// ============================================================

	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, mod api.Module, reqPtr, reqLen uint32) uint64 {

			reqBytes, ok := mod.Memory().Read(reqPtr, reqLen)
			if !ok {
				return hostBridgeError(ctx, mod, "replace_file: Request konnte nicht aus WASM-Speicher gelesen werden")
			}

			var req hostReplaceFileRequest
			if err := json.Unmarshal(reqBytes, &req); err != nil {
				return hostBridgeError(ctx, mod, "replace_file: ungültiger Request: "+err.Error())
			}

			result := replaceFileHost(req)

			resultBytes, err := json.Marshal(result)
			if err != nil {
				return hostBridgeError(ctx, mod, "replace_file: Host-Antwort konnte nicht kodiert werden: "+err.Error())
			}

			return allocateAndWrite(ctx, mod, "replace_file", resultBytes)

		}).
		Export("replace_file")

	_, err := builder.Instantiate(ctx)
	return err
}

// hostBridgeError liefert die Standard-Fehlerantwort der Bridge
// (hostDockerResponse-Form: {stdout, stderr, code}).
func hostBridgeError(
	ctx context.Context,
	mod api.Module,
	message string,
) uint64 {

	result := hostDockerResponse{
		Stdout: "",
		Stderr: message,
		Code:   -1,
	}

	data, err := json.Marshal(result)
	if err != nil {
		return 0
	}

	return allocateAndWrite(ctx, mod, "host_bridge_error", data)
}

// ------------------------------------------------------------
// Compilation Cache
// ------------------------------------------------------------

var wasmCache wazero.CompilationCache

func getCompilationCache() (wazero.CompilationCache, error) {
	if wasmCache != nil {
		return wasmCache, nil
	}

	var cacheDir string

	if p := strings.TrimSpace(os.Getenv("VBX_PLUGIN_CACHE")); p != "" {
		abs, err := filepath.Abs(p)
		if err != nil {
			return nil, fmt.Errorf("VBX_PLUGIN_CACHE konnte nicht aufgelöst werden: %w", err)
		}
		cacheDir = abs
	} else {
		base, err := os.UserCacheDir()
		if err != nil {
			return nil, fmt.Errorf("Cache-Verzeichnis konnte nicht ermittelt werden: %w", err)
		}
		cacheDir = filepath.Join(base, "vbx", "plugin-cache")
	}

	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("Cache-Verzeichnis %q konnte nicht angelegt werden: %w", cacheDir, err)
	}

	cache, err := wazero.NewCompilationCacheWithDir(cacheDir)
	if err != nil {
		return nil, fmt.Errorf("Plugin-Compilation-Cache konnte nicht angelegt werden: %w", err)
	}

	wasmCache = cache
	return cache, nil
}

// ------------------------------------------------------------
// Plugin laden
// ------------------------------------------------------------

// Load instanziiert plugins/<ns>.wasm und registriert die vom Plugin
// beschriebenen Funktionen in der stdlib-Registry. Ein bereits geladenes
// Plugin wird übersprungen. Nach erfolgreichem Laden müssen
// stdlib-Snapshots (Evaluator) neu aufgebaut werden - Register erhöht die
// Generation automatisch.
func Load(ns string) error {

	ctx := context.Background()

	pluginDebugf("Load(%q)", ns)

	if _, exists := loadedPlugins[ns]; exists {
		pluginDebugf("%s bereits geladen", ns)
		return nil
	}

	if err := validateNamespace(ns); err != nil {
		return err
	}

	dir, err := pluginDir()
	if err != nil {
		pluginDebugf("pluginDir FEHLER: %v", err)
		return err
	}

	pluginDebugf("Plugin-Verzeichnis: %s", dir)

	wasmPath := filepath.Join(dir, ns+".wasm")

	pluginDebugf("WASM-Pfad: %s", wasmPath)

	wasmBytes, err := os.ReadFile(wasmPath)
	if err != nil {
		pluginDebugf("ReadFile FEHLER: %v", err)
		return fmt.Errorf("Plugin %q konnte nicht gelesen werden: %w (Suchpfad: %s)", ns, err, dir)
	}

	pluginDebugf("WASM geladen: %d Bytes", len(wasmBytes))

	cache, err := getCompilationCache()
	if err != nil {
		pluginDebugf("CompilationCache FEHLER: %v", err)
		return err
	}

	pluginDebugf("Runtime wird erstellt")

	rt := wazero.NewRuntimeWithConfig(
		ctx,
		wazero.NewRuntimeConfig().
			WithCompilationCache(cache),
	)

	success := false

	defer func() {
		if !success {
			pluginDebugf("Runtime wird wegen Fehler geschlossen")
			_ = rt.Close(ctx)
		}
	}()

	// WASI laden
	pluginDebugf("WASI wird geladen")

	if _, err := wasi_snapshot_preview1.Instantiate(ctx, rt); err != nil {
		pluginDebugf("WASI FEHLER: %v", err)
		return fmt.Errorf("Plugin %q: WASI konnte nicht geladen werden: %w", ns, err)
	}

	// Host Bridge laden
	pluginDebugf("Host-Bridge wird geladen")

	if err := instantiateHostBridge(ctx, rt); err != nil {
		pluginDebugf("Host-Bridge FEHLER: %v", err)
		return err
	}

	// WASM kompilieren
	pluginDebugf("WASM wird kompiliert")

	compiled, err := rt.CompileModule(ctx, wasmBytes)
	if err != nil {
		pluginDebugf("Compile FEHLER: %v", err)
		return fmt.Errorf("Plugin %q konnte nicht kompiliert werden: %w", ns, err)
	}

	// Guest-Dateisystem konfigurieren
	pluginDebugf("Guest-Dateisystem wird konfiguriert")

	fsConfig, err := buildGuestFSConfig()
	if err != nil {
		pluginDebugf("FS-Config FEHLER: %v", err)
		return fmt.Errorf("Plugin %q: Dateisystem-Konfiguration fehlgeschlagen: %w", ns, err)
	}

	// WASM instanziieren (WASI Reactor; Startfunktionen verzögert)
	pluginDebugf("WASM wird instanziiert")

	mod, err := rt.InstantiateModule(
		ctx,
		compiled,
		wazero.NewModuleConfig().
			WithStartFunctions().
			WithFSConfig(fsConfig),
	)

	if err != nil {
		pluginDebugf("Instantiate FEHLER: %v", err)
		return fmt.Errorf("Plugin %q konnte nicht instanziiert werden: %w", ns, err)
	}

	pluginDebugf("WASM instanziiert")

	// Go WASI Reactor initialisieren
	initializeFn := mod.ExportedFunction("_initialize")
	if initializeFn == nil {
		return fmt.Errorf("Plugin %q: _initialize Export fehlt", ns)
	}

	pluginDebugf("_initialize wird aufgerufen")

	if _, err := initializeFn.Call(ctx); err != nil {
		pluginDebugf("_initialize FEHLER: %v", err)
		return fmt.Errorf("Plugin %q: _initialize fehlgeschlagen: %w", ns, err)
	}

	pluginDebugf("_initialize erfolgreich")

	// ABI-Exports prüfen
	abiVerFn := mod.ExportedFunction("vbx_abi_version")
	allocFn := mod.ExportedFunction("alloc")
	deallocFn := mod.ExportedFunction("dealloc")
	callFn := mod.ExportedFunction("vbx_call")
	describeFn := mod.ExportedFunction("vbx_describe")

	pluginDebugf("Exports: abi=%v alloc=%v dealloc=%v call=%v describe=%v",
		abiVerFn != nil, allocFn != nil, deallocFn != nil, callFn != nil, describeFn != nil)

	if abiVerFn == nil || allocFn == nil || deallocFn == nil || callFn == nil || describeFn == nil {
		return fmt.Errorf("Plugin %q: erforderliche Exports fehlen", ns)
	}

	// ABI-Version prüfen
	pluginDebugf("ABI wird geprüft")

	res, err := abiVerFn.Call(ctx)
	if err != nil {
		pluginDebugf("ABI Call FEHLER: %v", err)
		return fmt.Errorf("Plugin %q: vbx_abi_version fehlgeschlagen: %w", ns, err)
	}

	if len(res) == 0 {
		return fmt.Errorf("Plugin %q: vbx_abi_version liefert keinen Rückgabewert", ns)
	}

	abiVersion := uint32(res[0])

	pluginDebugf("ABI-Version: %d", abiVersion)

	if abiVersion != expectedABIVersion {
		return fmt.Errorf("Plugin %q: inkompatible ABI-Version %d, erwartet %d", ns, abiVersion, expectedABIVersion)
	}

	// Plugin-Beschreibung lesen
	pluginDebugf("vbx_describe wird aufgerufen")

	res, err = describeFn.Call(ctx)
	if err != nil {
		pluginDebugf("vbx_describe FEHLER: %v", err)
		return fmt.Errorf("Plugin %q: vbx_describe fehlgeschlagen: %w", ns, err)
	}

	if len(res) == 0 {
		return fmt.Errorf("Plugin %q: vbx_describe liefert keinen Rückgabewert", ns)
	}

	descPtr, descLen := unpackPtrLen(res[0])

	pluginDebugf("%s: ptr=%d len=%d raw=0x%016x", ns, descPtr, descLen, res[0])

	if descLen == 0 {
		return fmt.Errorf("Plugin %q: vbx_describe liefert eine leere Beschreibung", ns)
	}

	descBytes, ok := mod.Memory().Read(descPtr, descLen)
	if !ok {
		return fmt.Errorf("Plugin %q: Beschreibung nicht lesbar (ptr=%d, len=%d)", ns, descPtr, descLen)
	}

	pluginDebugf("description=%q", string(descBytes))

	var entries []pluginFuncDesc

	if err := json.Unmarshal(descBytes, &entries); err != nil {
		return fmt.Errorf("Plugin %q: ungültige Beschreibung: %w", ns, err)
	}

	pluginDebugf("%d Funktionen beschrieben", len(entries))

	if len(entries) == 0 {
		return fmt.Errorf("Plugin %q: keine Funktionen beschrieben", ns)
	}

	// Funktionen registrieren
	for _, entry := range entries {
		entry := entry

		pluginDebugf("Registrierung: %s.%s", entry.Namespace, entry.Name)

		wrapperFn := func(args []value.Value) value.Value {
			return callPluginFunction(
				ctx,
				ns,
				mod,
				allocFn,
				deallocFn,
				callFn,
				entry.Name,
				args,
			)
		}

		fullName := entry.Name
		if entry.Namespace != "" {
			fullName = entry.Namespace + "." + entry.Name
		}

		stdlib.Register(fullName, entry.Namespace, entry.Params, entry.Description, wrapperFn)
	}

	// Plugin als geladen markieren
	loadedPlugins[ns] = &pluginHandle{
		runtime: rt,
		module:  mod,
	}

	success = true

	pluginDebugf("Plugin %q erfolgreich geladen", ns)

	return nil
}

// ------------------------------------------------------------
// Plugin-Funktion aufrufen
// ------------------------------------------------------------

func callPluginFunction(
	ctx context.Context,
	ns string,
	mod api.Module,
	allocFn api.Function,
	deallocFn api.Function,
	callFn api.Function,
	fnName string,
	args []value.Value,
) value.Value {

	argsJSON, err := json.Marshal(toJSONValues(args))
	if err != nil {
		return value.ErrorVal(fmt.Sprintf("Plugin %q, Funktion %q: Argumente konnten nicht kodiert werden: %v", ns, fnName, err))
	}

	// Argumente in Plugin-Speicher schreiben
	argsPtr, err := writeToPluginMemory(ctx, mod, allocFn, argsJSON)
	if err != nil {
		return value.ErrorVal(fmt.Sprintf("Plugin %q, Funktion %q: %v", ns, fnName, err))
	}

	defer freePluginMemory(ctx, deallocFn, argsPtr, uint32(len(argsJSON)))

	// Funktionsnamen in Plugin-Speicher schreiben
	namePtr, err := writeToPluginMemory(ctx, mod, allocFn, []byte(fnName))
	if err != nil {
		return value.ErrorVal(fmt.Sprintf("Plugin %q, Funktion %q: %v", ns, fnName, err))
	}

	defer freePluginMemory(ctx, deallocFn, namePtr, uint32(len(fnName)))

	// Plugin-Funktion aufrufen
	res, err := callFn.Call(
		ctx,
		uint64(namePtr),
		uint64(len(fnName)),
		uint64(argsPtr),
		uint64(len(argsJSON)),
	)

	if err != nil {
		return value.ErrorVal(fmt.Sprintf("Plugin %q, Funktion %q: %v", ns, fnName, err))
	}

	if len(res) == 0 {
		return value.ErrorVal(fmt.Sprintf("Plugin %q, Funktion %q: kein Rückgabewert", ns, fnName))
	}

	// Rückgabe-Pointer und Länge auspacken
	resPtr, resLen := unpackPtrLen(res[0])

	resultJSON, ok := mod.Memory().Read(resPtr, resLen)
	if !ok {
		return value.ErrorVal(fmt.Sprintf("Plugin %q, Funktion %q: Rückgabewert nicht lesbar (ptr=%d, len=%d)", ns, fnName, resPtr, resLen))
	}

	// Rückgabespeicher des Plugins freigeben
	defer freePluginMemory(ctx, deallocFn, resPtr, resLen)

	return fromJSONValue(resultJSON)
}

// ------------------------------------------------------------
// Speicherhilfsfunktionen
// ------------------------------------------------------------

func unpackPtrLen(packed uint64) (ptr, length uint32) {
	return uint32(packed >> 32), uint32(packed)
}

func writeToPluginMemory(
	ctx context.Context,
	mod api.Module,
	allocFn api.Function,
	data []byte,
) (uint32, error) {

	res, err := allocFn.Call(ctx, uint64(len(data)))
	if err != nil {
		return 0, fmt.Errorf("alloc fehlgeschlagen: %w", err)
	}

	if len(res) == 0 {
		return 0, fmt.Errorf("alloc liefert keinen Rückgabewert")
	}

	ptr := uint32(res[0])

	if !mod.Memory().Write(ptr, data) {
		return 0, fmt.Errorf("Schreiben in Plugin-Speicher fehlgeschlagen (ptr=%d, len=%d)", ptr, len(data))
	}

	return ptr, nil
}

func freePluginMemory(
	ctx context.Context,
	deallocFn api.Function,
	ptr, size uint32,
) {
	_, _ = deallocFn.Call(ctx, uint64(ptr), uint64(size))
}

// ------------------------------------------------------------
// Value <-> JSON
// ------------------------------------------------------------

type jsonValue struct {
	Type    string               `json:"type"`
	Num     float64              `json:"num,omitempty"`
	Str     string               `json:"str,omitempty"`
	Bool    bool                 `json:"bool,omitempty"`
	Arr     []jsonValue          `json:"arr,omitempty"`
	Arr2D   [][]jsonValue        `json:"arr2d,omitempty"`
	Map     map[string]jsonValue `json:"map,omitempty"`
	Bytes   []byte               `json:"bytes,omitempty"`
	Message string               `json:"message,omitempty"`
}

func toJSONValue(v value.Value) jsonValue {

	switch v.Kind {

	case value.KindNum:
		return jsonValue{Type: "num", Num: v.Num}

	case value.KindStr:
		s := v.Str
		if looksLikeHostPath(s) {
			if translated, err := hostToGuestPath(s); err == nil {
				s = translated
			}
		}
		return jsonValue{Type: "str", Str: s}

	case value.KindBool:
		return jsonValue{Type: "bool", Bool: v.Bool}

	case value.KindArr:
		arr := make([]jsonValue, len(v.Arr))
		for i, el := range v.Arr {
			arr[i] = toJSONValue(el)
		}
		return jsonValue{Type: "arr", Arr: arr}

	case value.KindArr2D:
		arr2d := make([][]jsonValue, len(v.Arr2D))
		for i, row := range v.Arr2D {
			r := make([]jsonValue, len(row))
			for j, el := range row {
				r[j] = toJSONValue(el)
			}
			arr2d[i] = r
		}
		return jsonValue{Type: "arr2d", Arr2D: arr2d}

	case value.KindMap:
		m := make(map[string]jsonValue, len(v.Map))
		for k, el := range v.Map {
			m[k] = toJSONValue(el)
		}
		return jsonValue{Type: "map", Map: m}

	case value.KindBytes:
		return jsonValue{Type: "bytes", Bytes: v.Bytes}

	case value.KindError:
		return jsonValue{Type: "error", Message: v.Str}

	case value.KindObj:
		return jsonValue{
			Type:    "error",
			Message: "Objects werden von Plugin-Funktionen noch nicht unterstützt",
		}

	case value.KindNull:
		return jsonValue{Type: "null"}

	default:
		return jsonValue{Type: "empty"}
	}
}

func toJSONValues(vals []value.Value) []jsonValue {
	out := make([]jsonValue, len(vals))
	for i, v := range vals {
		out[i] = toJSONValue(v)
	}
	return out
}

func fromJSONValue(data []byte) value.Value {
	var jv jsonValue

	if err := json.Unmarshal(data, &jv); err != nil {
		return value.ErrorVal("Ungültige Plugin-Antwort: " + err.Error())
	}

	return jsonValueToValue(jv)
}

func jsonValueToValue(jv jsonValue) value.Value {

	switch jv.Type {

	case "num":
		return value.NumVal(jv.Num)

	case "str":
		return value.StrVal(jv.Str)

	case "bool":
		return value.BoolVal(jv.Bool)

	case "arr":
		arr := make([]value.Value, len(jv.Arr))
		for i, el := range jv.Arr {
			arr[i] = jsonValueToValue(el)
		}
		return value.Value{Kind: value.KindArr, Arr: arr}

	case "arr2d":
		arr2d := make([][]value.Value, len(jv.Arr2D))
		for i, row := range jv.Arr2D {
			r := make([]value.Value, len(row))
			for j, el := range row {
				r[j] = jsonValueToValue(el)
			}
			arr2d[i] = r
		}
		return value.Value{Kind: value.KindArr2D, Arr2D: arr2d}

	case "map":
		m := make(map[string]value.Value, len(jv.Map))
		for k, el := range jv.Map {
			m[k] = jsonValueToValue(el)
		}
		return value.Value{Kind: value.KindMap, Map: m}

	case "bytes":
		return value.Value{Kind: value.KindBytes, Bytes: jv.Bytes}

	case "empty":
		return value.NullVal()

	case "error":
		return value.ErrorVal(jv.Message)

	case "null":
		return value.NullVal()

	default:
		return value.NilValue
	}
}
