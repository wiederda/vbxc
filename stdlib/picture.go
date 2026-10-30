package stdlib

import (
	"fmt"
	"image"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"vbxc/value"

	"github.com/KarpelesLab/gowebp"
	ico "github.com/Kodeworks/golang-image-ico"
	"github.com/disintegration/imaging"
	"github.com/gen2brain/jpegxl" // registriert jxl automatisch für image.Decode
	"github.com/kbinani/screenshot"
)

// Liefert Argument i als String, oder "" wenn es fehlt.
func argStr(args []value.Value, i int) string {
	if i < len(args) {
		return value.ToString(args[i])
	}
	return ""
}

func init() {
	// Registriert WebP global bei Go, damit imaging.Open (und image.Decode
	// generell) .webp-Dateien automatisch erkennt und liest.
	image.RegisterFormat("webp", "RIFF????WEBP", gowebp.Decode, decodeWebpConfig)
}

func decodeWebpConfig(r io.Reader) (image.Config, error) {
	// gowebp bietet kein separates DecodeConfig, daher einmal voll decodieren.
	img, err := gowebp.Decode(r)
	if err != nil {
		return image.Config{}, err
	}
	b := img.Bounds()
	return image.Config{ColorModel: img.ColorModel(), Width: b.Dx(), Height: b.Dy()}, nil
}

// normalizeFormat bringt das Zielformat in Kleinschreibung und mappt jpeg -> jpg.
func normalizeFormat(f string) (string, bool) {
	f = strings.ToLower(strings.TrimSpace(f))
	switch f {
	case "jpeg":
		return "jpg", true
	case "jpg", "png", "webp", "jxl", "ico":
		return f, true
	}
	return f, false
}

func initPictureFunctions() {
	if builtins == nil {
		builtins = make(map[string]BuiltinInfo)
	}

	ns := "picture."

	// ---------------- picture.Convert ----------------
	Register(ns+"Convert", "picture", "in, out, fmt [, w, h, q]",
		"Konvertiert ein Bild. Formate: jpg, png, webp, jxl (auch als Input). Bei ico: w/h/q durch Größen-Liste (16,32) ersetzen.",
		func(args []value.Value) value.Value {
			if len(args) < 3 {
				return value.ErrorVal("usage: picture.Convert(infile, outfile, format [, width, height, quality | icoSizes])")
			}

			inFile, errIn := absPathVal(argStr(args, 0))
			if errIn != nil {
				return *errIn
			}
			outFile, errOut := absPathVal(argStr(args, 1))
			if errOut != nil {
				return *errOut
			}

			format, ok := normalizeFormat(argStr(args, 2))
			if !ok {
				return value.ErrorVal("Nicht unterstütztes Format: " + format)
			}

			if err := convertImage(inFile, outFile, format,
				argStr(args, 3), argStr(args, 4), argStr(args, 5)); err != nil {
				return value.ErrorVal(err.Error())
			}
			return value.NullVal()
		})

	// ---------------- picture.ConvertAll ----------------
	Register(ns+"ConvertAll", "picture", "inDir, outDir, fmt [, w, h, q]",
		"Konvertiert alle Dateien eines Ordners. Gibt 'Erfolg: n, Fehler: m' zurück.",
		func(args []value.Value) value.Value {
			if len(args) < 3 {
				return value.ErrorVal("usage: picture.ConvertAll(inDir, outDir, format [, width, height, quality | icoSizes])")
			}

			inDir, errIn := absPathVal(argStr(args, 0))
			if errIn != nil {
				return *errIn
			}
			outDir, errOut := absPathVal(argStr(args, 1))
			if errOut != nil {
				return *errOut
			}

			format, ok := normalizeFormat(argStr(args, 2))
			if !ok {
				return value.ErrorVal("Nicht unterstütztes Format: " + format)
			}

			entries, err := os.ReadDir(inDir)
			if err != nil {
				return value.ErrorVal("Quellverzeichnis nicht lesbar: " + err.Error())
			}
			if err := os.MkdirAll(outDir, 0o755); err != nil {
				return value.ErrorVal("Zielverzeichnis nicht erstellbar: " + err.Error())
			}

			wStr, hStr, qStr := argStr(args, 3), argStr(args, 4), argStr(args, 5)

			success, failed := 0, 0
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				name := e.Name()
				base := strings.TrimSuffix(name, filepath.Ext(name))
				// Bei ico entstehen daraus <base>_<size>.ico
				outFile := filepath.Join(outDir, base+"."+format)

				if err := convertImage(filepath.Join(inDir, name), outFile, format, wStr, hStr, qStr); err != nil {
					failed++
				} else {
					success++
				}
			}
			return value.StrVal(fmt.Sprintf("Erfolg: %d, Fehler: %d", success, failed))
		})

	// ---------------- picture.Snapshot ----------------
	Register(ns+"Snapshot", "picture", "outFile [, monitorIndex]",
		"Erstellt einen Screenshot eines Monitors.",
		func(args []value.Value) value.Value {
			if len(args) < 1 {
				return value.ErrorVal("usage: picture.Snapshot(outFile, [idx])")
			}
			outFile, errVal := absPathVal(argStr(args, 0))
			if errVal != nil {
				return *errVal
			}
			idx := 0
			if len(args) >= 2 {
				idx = int(args[1].Num)
			}
			if idx < 0 || idx >= screenshot.NumActiveDisplays() {
				return value.ErrorVal("Monitor nicht gefunden")
			}
			bounds := screenshot.GetDisplayBounds(idx)
			img, err := screenshot.CaptureRect(bounds)
			if err != nil {
				return value.ErrorVal("Screenshot fehlgeschlagen: " + err.Error())
			}
			// Format ergibt sich hier bewusst aus der Dateiendung (.png, .jpg)
			if err := imaging.Save(img, outFile); err != nil {
				return value.ErrorVal("Fehler beim Speichern des Screenshots: " + err.Error())
			}
			return value.NullVal()
		})
}

// convertImage ist der gemeinsame Kern von Convert und ConvertAll.
// Pfade müssen bereits absolut sein, format bereits normalisiert (normalizeFormat).
// Bei format == "ico" enthält wStr die Größenliste (z.B. "16,32,48").
func convertImage(inFile, outFile, format, wStr, hStr, qStr string) error {
	src, err := imaging.Open(inFile) // erkennt jpg/png/webp/jxl am Inhalt, nicht an der Endung
	if err != nil {
		return fmt.Errorf("Bild-Ladefehler: %w", err)
	}

	if format == "ico" {
		if strings.TrimSpace(wStr) == "" {
			return fmt.Errorf("Fehlende ICO-Größen (z.B. '16,32,48')")
		}
		return exportIco(src, outFile, wStr)
	}

	width, _ := strconv.Atoi(strings.TrimSpace(wStr))
	height, _ := strconv.Atoi(strings.TrimSpace(hStr))
	quality := 100
	if q, err := strconv.Atoi(strings.TrimSpace(qStr)); err == nil {
		quality = q
	}

	if width > 0 && height > 0 {
		src = imaging.Fill(src, width, height, imaging.Center, imaging.Lanczos)
	} else if width > 0 || height > 0 {
		src = imaging.Resize(src, width, height, imaging.Lanczos)
	}

	var saveErr error
	switch format {
	case "jpg":
		saveErr = saveImaging(src, outFile, imaging.JPEG, imaging.JPEGQuality(quality))
	case "png":
		saveErr = saveImaging(src, outFile, imaging.PNG)
	case "webp":
		saveErr = saveWebp(src, outFile, quality)
	case "jxl":
		saveErr = saveJxl(src, outFile, quality)
	default:
		return fmt.Errorf("Nicht unterstütztes Format: %s", format)
	}
	if saveErr != nil {
		return fmt.Errorf("Speicherfehler: %w", saveErr)
	}
	return nil
}

// saveImaging kodiert explizit im angegebenen Format, unabhängig von der Dateiendung.
func saveImaging(img image.Image, outFile string, f imaging.Format, opts ...imaging.EncodeOption) error {
	file, err := os.Create(outFile)
	if err != nil {
		return err
	}
	defer file.Close()
	return imaging.Encode(file, img, f, opts...)
}

func saveWebp(img image.Image, outFile string, quality int) error {
	f, err := os.Create(outFile)
	if err != nil {
		return err
	}
	defer f.Close()

	if quality <= 0 || quality >= 100 {
		return gowebp.Encode(f, img, nil) // VP8L, verlustfrei
	}
	return gowebp.Encode(f, img, &gowebp.Options{
		Lossy:   true,
		Quality: float32(quality),
		Method:  4,
	})
}

func saveJxl(img image.Image, outFile string, quality int) error {
	f, err := os.Create(outFile)
	if err != nil {
		return err
	}
	defer f.Close()

	return jpegxl.Encode(f, img, jpegxl.Options{
		Quality: quality,
		Effort:  jpegxl.DefaultEffort,
	})
}

// exportIco schreibt pro Größe eine Datei <base>_<size>.ico (gültig: 1–256).
func exportIco(src image.Image, outFile, sizesStr string) error {
	base := strings.TrimSuffix(outFile, filepath.Ext(outFile))
	written, failed := 0, 0

	for _, s := range strings.Split(sizesStr, ",") {
		size, _ := strconv.Atoi(strings.TrimSpace(s))
		if size < 1 || size > 256 {
			continue
		}
		resized := imaging.Resize(src, size, size, imaging.Lanczos)

		f, err := os.Create(fmt.Sprintf("%s_%d.ico", base, size))
		if err != nil {
			failed++
			continue
		}
		if err := ico.Encode(f, resized); err != nil {
			failed++
		} else {
			written++
		}
		f.Close()
	}

	if written == 0 && failed == 0 {
		return fmt.Errorf("Keine gültigen ICO-Größen (1-256) angegeben")
	}
	if failed > 0 {
		return fmt.Errorf("Einige ICOs fehlgeschlagen (%d von %d)", failed, written+failed)
	}
	return nil
}
func init() {
	initPictureFunctions()
}
