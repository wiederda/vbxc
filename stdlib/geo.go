// ------------------------
// stdlib_geo.go (OSM-only, Array, interne URL-Kodierung)
// ------------------------

package stdlib

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	valpkg "vbxc/value"
)

// Basis-URL für Nominatim
var geoBaseURL = "https://nominatim.openstreetmap.org/search?format=json&limit=1&q="

// InitGeoFunctions registriert Geo-Funktionen
func initGeoFunctions() {
	if builtins == nil {
		builtins = make(map[string]BuiltinInfo)
	}

	ns := "geo."

	Register(ns+"Lookup", "geo", "city", "Sucht Geodaten via OSM. Gibt Array zurück: [Stadt, Region, Land, Lat, Lon].", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.Value{Kind: valpkg.KindArr, Arr: []valpkg.Value{valpkg.StrVal("error: no city provided")}}
		}

		city := valpkg.ToString(args[0])
		urlStr := geoBaseURL + url.QueryEscape(city) // interne URL-Kodierung

		req, err := http.NewRequest("GET", urlStr, nil)
		if err != nil {
			return valpkg.Value{Kind: valpkg.KindArr, Arr: []valpkg.Value{valpkg.StrVal("error: request creation failed: " + err.Error())}}
		}
		req.Header.Set("User-Agent", "vbx-geo")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return valpkg.Value{Kind: valpkg.KindArr, Arr: []valpkg.Value{valpkg.StrVal("error: network error: " + err.Error())}}
		}
		defer resp.Body.Close()

		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return valpkg.Value{Kind: valpkg.KindArr, Arr: []valpkg.Value{valpkg.StrVal("error: read response failed: " + err.Error())}}
		}

		var result []map[string]interface{}
		if err := json.Unmarshal(data, &result); err != nil || len(result) == 0 {
			return valpkg.Value{Kind: valpkg.KindArr, Arr: []valpkg.Value{valpkg.StrVal("error: no results or invalid response")}}
		}

		place := result[0]

		// Werte extrahieren
		cityName := "unknown"
		state := "unknown"
		country := "unknown"
		lat := "unknown"
		lon := "unknown"

		if displayName, ok := place["display_name"].(string); ok {
			parts := strings.Split(displayName, ",")
			if len(parts) >= 1 {
				cityName = strings.TrimSpace(parts[0])
			}
			if len(parts) >= 2 {
				state = strings.TrimSpace(parts[1])
			}
			if len(parts) >= 3 {
				country = strings.TrimSpace(parts[len(parts)-1])
			}
		}

		if latStr, ok := place["lat"].(string); ok {
			lat = latStr
		}
		if lonStr, ok := place["lon"].(string); ok {
			lon = lonStr
		}

		return valpkg.Value{Kind: valpkg.KindArr, Arr: []valpkg.Value{
			valpkg.StrVal(cityName),
			valpkg.StrVal(state),
			valpkg.StrVal(country),
			valpkg.StrVal(lat),
			valpkg.StrVal(lon),
		}}
	})
}

func init() { initGeoFunctions() }
