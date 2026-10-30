package stdlib

import "vbxc/value"

func init() {
	register := func(category, name, desc string, val value.Value) {
		RegisterConstant(category, name, desc, func([]value.Value) value.Value {
			return val
		})
	}

	register("Logic", "vbTrue", "Wahr", value.BoolVal(true))
	register("Logic", "vbFalse", "Falsch", value.BoolVal(false))
	register("Logic", "vbNullString", "Leerer String", value.StrVal(""))

	register("System", "vbCrLf", "Windows-Zeilenumbruch (CRLF)", value.StrVal("\r\n"))
	register("System", "vbNewLine", "System-Zeilenumbruch (LF)", value.StrVal("\n"))
	register("System", "vbTab", "Tabulator-Zeichen", value.StrVal("\t"))

	register("Color", "vbBlack", "Farbe: Schwarz", value.StrVal("\033[30m"))
	register("Color", "vbRed", "Farbe: Rot", value.StrVal("\033[91m"))
	register("Color", "vbGreen", "Farbe: Grün", value.StrVal("\033[92m"))
	register("Color", "vbYellow", "Farbe: Gelb", value.StrVal("\033[93m"))
	register("Color", "vbBlue", "Farbe: Blau", value.StrVal("\033[94m"))
	register("Color", "vbWhite", "Farbe: Weiß", value.StrVal("\033[37m"))
	register("Color", "vbCyan", "Farbe: Cyan (Hellblau)", value.StrVal("\033[36m"))
	register("Color", "vbMagenta", "Farbe: Magenta (Lila)", value.StrVal("\033[35m"))
	register("Color", "vbLightGray", "Farbe: Hellgrau", value.StrVal("\033[37m"))
	register("Color", "vbGray", "Farbe: Grau", value.StrVal("\033[90m"))

	register("Background", "vbBgBlack", "Hintergrund: Schwarz", value.StrVal("\033[40m"))
	register("Background", "vbBgRed", "Hintergrund: Rot", value.StrVal("\033[41m"))
	register("Background", "vbBgGreen", "Hintergrund: Grün", value.StrVal("\033[42m"))
	register("Background", "vbBgYellow", "Hintergrund: Gelb", value.StrVal("\033[43m"))
	register("Background", "vbBgBlue", "Hintergrund: Blau", value.StrVal("\033[44m"))
	register("Background", "vbBgWhite", "Hintergrund: Weiß", value.StrVal("\033[47m"))
	register("Background", "vbBgCyan", "Hintergrund: Cyan", value.StrVal("\033[46m"))
	register("Background", "vbBgMagenta", "Hintergrund: Magenta", value.StrVal("\033[45m"))

	register("Style", "vbBold", "Stil: Fett", value.StrVal("\033[1m"))
	register("Style", "vbUnderline", "Stil: Unterstrichen", value.StrVal("\033[4m"))
	register("Style", "vbNormal", "Reset: Stile & Farben zurücksetzen", value.StrVal("\033[0m"))
}
