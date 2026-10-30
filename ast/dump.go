package ast

import (
	"fmt"
	"io"
	"reflect"
	"strings"
)

// Dump schreibt einen eingerückten Baum (mit Zeilennummern) nach w.
// Arbeitet per Reflection und braucht daher keine Pflege bei neuen Knoten.
func Dump(w io.Writer, n any) {
	dumpValue(w, "", reflect.ValueOf(n), 0)
}

func dumpValue(w io.Writer, name string, v reflect.Value, depth int) {
	ind := strings.Repeat("  ", depth)
	label := ind
	if name != "" {
		label = ind + name + ": "
	}

	for v.Kind() == reflect.Interface || v.Kind() == reflect.Ptr {
		if v.IsNil() {
			fmt.Fprintf(w, "%snil\n", label)
			return
		}
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Struct:
		hdr := v.Type().Name()
		if loc := v.FieldByName("Loc"); loc.IsValid() {
			l := loc.Interface().(Loc)
			if l.File != "" {
				hdr += fmt.Sprintf(" @%s:%d", l.File, l.Line)
			} else {
				hdr += fmt.Sprintf(" @%d", l.Line)
			}
		}
		fmt.Fprintf(w, "%s%s\n", label, hdr)
		for i := 0; i < v.NumField(); i++ {
			f := v.Type().Field(i)
			if f.Name == "Loc" {
				continue
			}
			dumpValue(w, f.Name, v.Field(i), depth+1)
		}

	case reflect.Slice:
		if v.Len() == 0 {
			fmt.Fprintf(w, "%s[]\n", label)
			return
		}
		fmt.Fprintf(w, "%s[%d]\n", label, v.Len())
		for i := 0; i < v.Len(); i++ {
			dumpValue(w, "-", v.Index(i), depth+1)
		}

	case reflect.String:
		fmt.Fprintf(w, "%s%q\n", label, v.String())

	default:
		fmt.Fprintf(w, "%s%v\n", label, v.Interface())
	}
}
