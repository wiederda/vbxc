// ------------------------
// stdlib_template.go
// ------------------------

package stdlib

import (
	"bytes"
	"os"
	"text/template"
	valpkg "vbxc/value"
)

func initTemplateFunctions() {
	if builtins == nil {
		builtins = make(map[string]BuiltinInfo)
	}

	ns := "template."

	// valueToTemplateData wandelt einen VBMini-valpkg.Value in Go-kompatible
	// Datenstrukturen um die text/template verarbeiten kann.
	valueToTemplateData := func(v valpkg.Value) interface{} {
		switch v.Kind {
		case valpkg.KindMap:
			m := make(map[string]interface{})
			for k, val := range v.Map {
				m[k] = valToInterface(val)
			}
			return m
		case valpkg.KindArr:
			arr := make([]interface{}, len(v.Arr))
			for i, val := range v.Arr {
				arr[i] = valToInterface(val)
			}
			return arr
		default:
			return valToInterface(v)
		}
	}

	// template.Render(templateStr, data)
	Register(ns+"Render", "template", "templateStr, data", "Rendert einen Template-String mit einer Map oder einem Array als Datenquelle.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 2 {
			return valpkg.ErrorVal("template.Render: templateStr und data benötigt")
		}

		tmplStr := valpkg.ToString(args[0])
		data := valueToTemplateData(args[1])

		tmpl, err := template.New("vbx").Parse(tmplStr)
		if err != nil {
			return valpkg.ErrorVal("template.Render: Ungültiges Template: " + err.Error())
		}

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			return valpkg.ErrorVal("template.Render: Ausführungsfehler: " + err.Error())
		}

		return valpkg.StrVal(buf.String())
	})

	// template.RenderFile(path, data)
	Register(ns+"RenderFile", "template", "path, data", "Lädt ein Template aus einer Datei und rendert es mit den übergebenen Daten.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 2 {
			return valpkg.ErrorVal("template.RenderFile: path und data benötigt")
		}

		path, errVal := absPathVal(args[0].Str)
		if errVal != nil {
			return *errVal
		}

		tmplBytes, err := os.ReadFile(path)
		if err != nil {
			return valpkg.ErrorVal("template.RenderFile: Datei nicht lesbar: " + err.Error())
		}

		data := valueToTemplateData(args[1])

		tmpl, err := template.New("vbx").Parse(string(tmplBytes))
		if err != nil {
			return valpkg.ErrorVal("template.RenderFile: Ungültiges Template: " + err.Error())
		}

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			return valpkg.ErrorVal("template.RenderFile: Ausführungsfehler: " + err.Error())
		}

		return valpkg.StrVal(buf.String())
	})

	// template.RenderToFile(templatePath, outPath, data)
	Register(ns+"RenderToFile", "template", "templatePath, outPath, data", "Rendert ein Template aus einer Datei direkt in eine Ausgabedatei.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 3 {
			return valpkg.ErrorVal("template.RenderToFile: templatePath, outPath und data benötigt")
		}

		tmplPath, errVal := absPathVal(args[0].Str)
		if errVal != nil {
			return *errVal
		}

		outPath, errVal := absPathVal(args[1].Str)
		if errVal != nil {
			return *errVal
		}

		tmplBytes, err := os.ReadFile(tmplPath)
		if err != nil {
			return valpkg.ErrorVal("template.RenderToFile: Template nicht lesbar: " + err.Error())
		}

		data := valueToTemplateData(args[2])

		tmpl, err := template.New("vbx").Parse(string(tmplBytes))
		if err != nil {
			return valpkg.ErrorVal("template.RenderToFile: Ungültiges Template: " + err.Error())
		}

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			return valpkg.ErrorVal("template.RenderToFile: Ausführungsfehler: " + err.Error())
		}

		// Atomar schreiben
		tmp := outPath + ".tmp"
		if err := os.WriteFile(tmp, buf.Bytes(), 0644); err != nil {
			return valpkg.ErrorVal("template.RenderToFile: Schreibfehler: " + err.Error())
		}
		if err := os.Rename(tmp, outPath); err != nil {
			os.Remove(tmp)
			return valpkg.ErrorVal("template.RenderToFile: Rename fehlgeschlagen: " + err.Error())
		}

		return valpkg.StrVal("OK")
	})
}

func init() { initTemplateFunctions() }
