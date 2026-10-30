// ------------------------
// stdlib_kuma.go
// ------------------------

package stdlib

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	valpkg "vbxc/value"

	"github.com/gorilla/websocket"
)

// ---------- TOTP (RFC 6238) ----------

func kumaGenerateTOTP(secret string) (string, error) {
	secret = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(secret), " ", ""))
	if n := len(secret) % 8; n != 0 {
		secret += strings.Repeat("=", 8-n)
	}
	key, err := base32.StdEncoding.DecodeString(secret)
	if err != nil {
		return "", fmt.Errorf("ungültiges TOTP-Secret: %w", err)
	}

	counter := uint64(time.Now().Unix() / 30)
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, counter)

	mac := hmac.New(sha1.New, key)
	mac.Write(buf)
	sum := mac.Sum(nil)

	offset := sum[len(sum)-1] & 0x0f
	code := (uint32(sum[offset])&0x7f)<<24 |
		(uint32(sum[offset+1])&0xff)<<16 |
		(uint32(sum[offset+2])&0xff)<<8 |
		(uint32(sum[offset+3]) & 0xff)
	code %= uint32(math.Pow10(6))

	return fmt.Sprintf("%06d", code), nil
}

// ---------- Minimaler Socket.IO-v4-Client ----------

type kumaClient struct {
	conn    *websocket.Conn
	mu      sync.Mutex
	ackID   int
	pending map[int]chan json.RawMessage
	closeCh chan struct{}
}

// kumaPollingHandshake holt sich per HTTP-Long-Polling die Session-ID (sid).
// Uptime Kuma / Engine.IO v4 erwartet diesen Schritt VOR dem WebSocket-Upgrade,
// ein direkter WebSocket-Connect ohne vorherige sid wird sonst kommentarlos ignoriert.
func kumaPollingHandshake(httpScheme, host string) (string, error) {
	pollURL := fmt.Sprintf("%s://%s/socket.io/?EIO=4&transport=polling", httpScheme, host)

	resp, err := http.Get(pollURL)
	if err != nil {
		return "", fmt.Errorf("polling-handshake fehlgeschlagen: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("polling-antwort nicht lesbar: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("polling-handshake: HTTP %d: %s", resp.StatusCode, string(body))
	}

	s := string(body)
	if len(s) == 0 || s[0] != '0' {
		return "", fmt.Errorf("unerwartetes open-Paket: %s", s)
	}

	var open struct {
		Sid string `json:"sid"`
	}
	if err := json.Unmarshal([]byte(s[1:]), &open); err != nil {
		return "", fmt.Errorf("sid nicht lesbar: %w", err)
	}
	if open.Sid == "" {
		return "", fmt.Errorf("keine sid im open-Paket erhalten")
	}
	return open.Sid, nil
}

func kumaDial(server string) (*kumaClient, error) {
	u, err := url.Parse(server)
	if err != nil {
		return nil, fmt.Errorf("ungültige Server-URL: %w", err)
	}
	httpScheme := "http"
	wsScheme := "ws"
	if u.Scheme == "https" {
		httpScheme = "https"
		wsScheme = "wss"
	}

	sid, err := kumaPollingHandshake(httpScheme, u.Host)
	if err != nil {
		return nil, err
	}

	wsURL := fmt.Sprintf("%s://%s/socket.io/?EIO=4&transport=websocket&sid=%s", wsScheme, u.Host, sid)
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("websocket-upgrade fehlgeschlagen: %w", err)
	}

	c := &kumaClient{
		conn:    conn,
		pending: make(map[int]chan json.RawMessage),
		closeCh: make(chan struct{}),
	}

	// Engine.IO-Upgrade-Probe: "2probe" senden, "3probe" erwarten, dann "5" (upgrade) bestätigen.
	if err := conn.WriteMessage(websocket.TextMessage, []byte("2probe")); err != nil {
		conn.Close()
		return nil, fmt.Errorf("probe senden fehlgeschlagen: %w", err)
	}
	if _, msg, err := conn.ReadMessage(); err != nil || string(msg) != "3probe" {
		conn.Close()
		return nil, fmt.Errorf("keine gültige probe-Antwort erhalten")
	}
	if err := conn.WriteMessage(websocket.TextMessage, []byte("5")); err != nil {
		conn.Close()
		return nil, fmt.Errorf("upgrade-bestätigung fehlgeschlagen: %w", err)
	}

	// Socket.IO Namespace-Connect (Typ 40)
	if err := conn.WriteMessage(websocket.TextMessage, []byte("40")); err != nil {
		conn.Close()
		return nil, err
	}
	if _, msg, err := conn.ReadMessage(); err != nil || !strings.HasPrefix(string(msg), "40") {
		conn.Close()
		return nil, fmt.Errorf("keine Connect-Bestätigung erhalten")
	}

	go c.readLoop()
	return c, nil
}

func (c *kumaClient) readLoop() {
	defer close(c.closeCh)
	for {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		s := string(msg)
		switch {
		case s == "2":
			c.conn.WriteMessage(websocket.TextMessage, []byte("3"))
		case strings.HasPrefix(s, "43"):
			rest := s[2:]
			i := 0
			for i < len(rest) && rest[i] >= '0' && rest[i] <= '9' {
				i++
			}
			id, _ := strconv.Atoi(rest[:i])
			var arr []json.RawMessage
			if err := json.Unmarshal([]byte(rest[i:]), &arr); err == nil && len(arr) > 0 {
				c.mu.Lock()
				if ch, ok := c.pending[id]; ok {
					ch <- arr[0]
					delete(c.pending, id)
				}
				c.mu.Unlock()
			}
		}
	}
}

func (c *kumaClient) call(event string, args []interface{}, timeout time.Duration) (json.RawMessage, error) {
	frame := append([]interface{}{event}, args...)
	payload, err := json.Marshal(frame)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.ackID++
	id := c.ackID
	ch := make(chan json.RawMessage, 1)
	c.pending[id] = ch
	c.mu.Unlock()

	msg := fmt.Sprintf("42%d%s", id, payload)
	if err := c.conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
		return nil, err
	}

	select {
	case res := <-ch:
		return res, nil
	case <-time.After(timeout):
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, fmt.Errorf("timeout bei %s", event)
	case <-c.closeCh:
		return nil, fmt.Errorf("verbindung geschlossen während %s", event)
	}
}

func (c *kumaClient) close() {
	c.conn.Close()
}

// kumaLogin verbindet, loggt ein (mit optionalem TOTP) und gibt den Client zurück.
func kumaLogin(server, user, password, secret string) (*kumaClient, error) {
	client, err := kumaDial(server)
	if err != nil {
		return nil, err
	}

	loginData := map[string]interface{}{
		"username": user,
		"password": password,
	}
	if secret != "" {
		token, err := kumaGenerateTOTP(secret)
		if err != nil {
			client.close()
			return nil, err
		}
		loginData["token"] = token
	}

	res, err := client.call("login", []interface{}{loginData}, 10*time.Second)
	if err != nil {
		client.close()
		return nil, fmt.Errorf("login fehlgeschlagen: %w", err)
	}

	var loginRes struct {
		OK            bool   `json:"ok"`
		TokenRequired bool   `json:"tokenRequired"`
		Msg           string `json:"msg"`
	}
	if err := json.Unmarshal(res, &loginRes); err != nil {
		client.close()
		return nil, fmt.Errorf("login-antwort nicht lesbar: %w", err)
	}
	if loginRes.TokenRequired {
		client.close()
		return nil, fmt.Errorf("2FA erforderlich, aber kein/falsches Secret angegeben")
	}
	if !loginRes.OK {
		client.close()
		return nil, fmt.Errorf("login fehlgeschlagen: %s", loginRes.Msg)
	}

	return client, nil
}

// kumaValueToIntSlice wandelt ein valpkg.Value (Arr von Str/Num) in []int um.
func kumaValueToIntSlice(v valpkg.Value) []int {
	var ids []int
	if v.Kind != valpkg.KindArr {
		return ids
	}
	for _, item := range v.Arr {
		switch item.Kind {
		case valpkg.KindStr:
			if n, err := strconv.Atoi(strings.TrimSpace(item.Str)); err == nil {
				ids = append(ids, n)
			}
		case valpkg.KindNum:
			ids = append(ids, int(item.Num))
		}
	}
	return ids
}

func kumaOk(err error) valpkg.Value {
	if err != nil {
		return valpkg.Value{Kind: valpkg.KindStr, Str: "error: " + err.Error()}
	}
	return valpkg.Value{Kind: valpkg.KindStr, Str: "OK"}
}

// ---------- Registrierung ----------

func initKumaFunctions() {
	if builtins == nil {
		builtins = make(map[string]BuiltinInfo)
	}

	ns := "kuma."

	Register(ns+"SetMaintenance", "kuma",
		"server, user, password, monitorIDs, minutes, [secret]",
		"Legt eine einmalige Wartungspause an und ordnet sie den angegebenen Monitoren zu. secret optional für 2FA (TOTP). Gibt die maintenanceID zurück.",
		func(args []valpkg.Value) valpkg.Value {
			if len(args) < 5 {
				return valpkg.Value{Kind: valpkg.KindStr, Str: "error: server, user, password, monitorIDs und minutes benötigt"}
			}

			server := args[0].Str
			user := args[1].Str
			password := args[2].Str
			monitorIDs := kumaValueToIntSlice(args[3])
			minutes, err := strconv.Atoi(strings.TrimSpace(valpkg.ToString(args[4])))
			if err != nil {
				return valpkg.Value{Kind: valpkg.KindStr, Str: "error: minutes muss eine Zahl sein"}
			}
			secret := ""
			if len(args) >= 6 {
				secret = args[5].Str
			}

			if len(monitorIDs) == 0 {
				return valpkg.Value{Kind: valpkg.KindStr, Str: "error: monitorIDs muss mindestens eine ID enthalten"}
			}

			client, err := kumaLogin(server, user, password, secret)
			if err != nil {
				return kumaOk(err)
			}
			defer client.close()

			start := time.Now()
			end := start.Add(time.Duration(minutes) * time.Minute)

			maintenanceData := map[string]interface{}{
				"title":          fmt.Sprintf("Server-Update %s", start.Format("2006-01-02 15:04")),
				"description":    "Automatisch gesetzt via VBX",
				"strategy":       "single",
				"active":         true,
				"intervalDay":    1,
				"dateRange":      []string{start.Format("2006-01-02 15:04:05"), end.Format("2006-01-02 15:04:05")},
				"weekdays":       []int{},
				"daysOfMonth":    []interface{}{},
				"timezoneOption": "Europe/Berlin",
			}

			res, err := client.call("addMaintenance", []interface{}{maintenanceData}, 10*time.Second)
			if err != nil {
				return kumaOk(err)
			}

			var addRes struct {
				OK            bool   `json:"ok"`
				MaintenanceID int    `json:"maintenanceID"`
				Msg           string `json:"msg"`
			}
			if err := json.Unmarshal(res, &addRes); err != nil {
				return kumaOk(fmt.Errorf("addMaintenance-antwort nicht lesbar: %w", err))
			}
			if !addRes.OK {
				return kumaOk(fmt.Errorf("wartung anlegen fehlgeschlagen: %s", addRes.Msg))
			}

			monitors := make([]map[string]int, len(monitorIDs))
			for i, id := range monitorIDs {
				monitors[i] = map[string]int{"id": id}
			}

			if _, err := client.call("addMonitorMaintenance", []interface{}{addRes.MaintenanceID, monitors}, 10*time.Second); err != nil {
				return kumaOk(err)
			}

			return valpkg.Value{Kind: valpkg.KindStr, Str: strconv.Itoa(addRes.MaintenanceID)}
		})

	Register(ns+"StopMaintenance", "kuma",
		"server, user, password, maintenanceID, [secret]",
		"Beendet eine laufende Wartungspause vorzeitig.",
		func(args []valpkg.Value) valpkg.Value {
			if len(args) < 4 {
				return valpkg.Value{Kind: valpkg.KindStr, Str: "error: server, user, password und maintenanceID benötigt"}
			}

			server := args[0].Str
			user := args[1].Str
			password := args[2].Str
			maintenanceID, err := strconv.Atoi(strings.TrimSpace(valpkg.ToString(args[3])))
			if err != nil {
				return valpkg.Value{Kind: valpkg.KindStr, Str: "error: maintenanceID muss eine Zahl sein"}
			}
			secret := ""
			if len(args) >= 5 {
				secret = args[4].Str
			}

			client, err := kumaLogin(server, user, password, secret)
			if err != nil {
				return kumaOk(err)
			}
			defer client.close()

			_, err = client.call("editMaintenance", []interface{}{
				map[string]interface{}{"id": maintenanceID, "active": false},
			}, 10*time.Second)
			return kumaOk(err)
		})

	Register(ns+"DeleteMaintenance", "kuma",
		"server, user, password, maintenanceID, [secret]",
		"Löscht ein Wartungsfenster vollständig (z.B. nach Abschluss, um die Übersicht sauber zu halten).",
		func(args []valpkg.Value) valpkg.Value {
			if len(args) < 4 {
				return valpkg.Value{Kind: valpkg.KindStr, Str: "error: server, user, password und maintenanceID benötigt"}
			}

			server := args[0].Str
			user := args[1].Str
			password := args[2].Str
			maintenanceID, err := strconv.Atoi(strings.TrimSpace(valpkg.ToString(args[3])))
			if err != nil {
				return valpkg.Value{Kind: valpkg.KindStr, Str: "error: maintenanceID muss eine Zahl sein"}
			}
			secret := ""
			if len(args) >= 5 {
				secret = args[4].Str
			}

			client, err := kumaLogin(server, user, password, secret)
			if err != nil {
				return kumaOk(err)
			}
			defer client.close()

			_, err = client.call("deleteMaintenance", []interface{}{maintenanceID}, 10*time.Second)
			return kumaOk(err)
		})

	Register(ns+"PauseMonitor", "kuma",
		"server, user, password, monitorID, [secret]",
		"Pausiert einen einzelnen Monitor direkt (ohne Maintenance-Fenster).",
		func(args []valpkg.Value) valpkg.Value {
			if len(args) < 4 {
				return valpkg.Value{Kind: valpkg.KindStr, Str: "error: server, user, password und monitorID benötigt"}
			}
			server, user, password := args[0].Str, args[1].Str, args[2].Str
			monitorID, err := strconv.Atoi(strings.TrimSpace(valpkg.ToString(args[3])))
			if err != nil {
				return valpkg.Value{Kind: valpkg.KindStr, Str: "error: monitorID muss eine Zahl sein"}
			}
			secret := ""
			if len(args) >= 5 {
				secret = args[4].Str
			}

			client, err := kumaLogin(server, user, password, secret)
			if err != nil {
				return kumaOk(err)
			}
			defer client.close()

			_, err = client.call("pauseMonitor", []interface{}{monitorID}, 10*time.Second)
			return kumaOk(err)
		})

	Register(ns+"ResumeMonitor", "kuma",
		"server, user, password, monitorID, [secret]",
		"Setzt einen pausierten Monitor wieder fort.",
		func(args []valpkg.Value) valpkg.Value {
			if len(args) < 4 {
				return valpkg.Value{Kind: valpkg.KindStr, Str: "error: server, user, password und monitorID benötigt"}
			}
			server, user, password := args[0].Str, args[1].Str, args[2].Str
			monitorID, err := strconv.Atoi(strings.TrimSpace(valpkg.ToString(args[3])))
			if err != nil {
				return valpkg.Value{Kind: valpkg.KindStr, Str: "error: monitorID muss eine Zahl sein"}
			}
			secret := ""
			if len(args) >= 5 {
				secret = args[4].Str
			}

			client, err := kumaLogin(server, user, password, secret)
			if err != nil {
				return kumaOk(err)
			}
			defer client.close()

			_, err = client.call("resumeMonitor", []interface{}{monitorID}, 10*time.Second)
			return kumaOk(err)
		})
}

func init() { initKumaFunctions() }
