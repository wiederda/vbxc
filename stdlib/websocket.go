package stdlib

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	valpkg "vbxc/value"

	"github.com/gorilla/websocket"
)

// ------------------------------------------------------------
// WebSocket-Verbindungen
// ------------------------------------------------------------

var errWebSocketTimeout = errors.New("Timeout beim Warten auf eine Nachricht")
var obsRequestNext atomic.Int64

type obsRequestResponse struct {
	Op int `json:"op"`
	D  struct {
		RequestID     string `json:"requestId"`
		RequestStatus struct {
			Result  bool   `json:"result"`
			Code    int    `json:"code"`
			Comment string `json:"comment"`
		} `json:"requestStatus"`
		ResponseData struct {
			OutputActive bool `json:"outputActive"`
		} `json:"responseData"`
	} `json:"d"`
}

type vbxWebSocketMessage struct {
	messageType int
	data        []byte
}

type vbxWebSocketConnection struct {
	conn         *websocket.Conn
	obsRecording atomic.Bool
	readMu       sync.Mutex
	writeMu      sync.Mutex
	stateMu      sync.Mutex

	messages chan vbxWebSocketMessage
	done     chan struct{}
	stop     chan struct{}
	stopOnce sync.Once
	readErr  error
}

var (
	webSocketConnectionsMu sync.RWMutex
	webSocketConnections   = make(map[int64]*vbxWebSocketConnection)
	webSocketNextHandle    atomic.Int64
)

func obsRequest(
	connection *vbxWebSocketConnection,
	requestType string,
	timeout time.Duration,
) (obsRequestResponse, error) {
	connection.readMu.Lock()
	defer connection.readMu.Unlock()

	requestID := fmt.Sprintf("vbx-%d", obsRequestNext.Add(1))

	request := map[string]interface{}{
		"op": 6,
		"d": map[string]interface{}{
			"requestType": requestType,
			"requestId":   requestID,
		},
	}

	payload, err := json.Marshal(request)
	if err != nil {
		return obsRequestResponse{}, err
	}

	if err := sendWebSocketText(connection, string(payload)); err != nil {
		return obsRequestResponse{}, fmt.Errorf(
			"OBS-Request %s konnte nicht gesendet werden: %v",
			requestType, err,
		)
	}

	deadline := time.Now().Add(timeout)

	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return obsRequestResponse{}, fmt.Errorf(
				"Timeout bei OBS-Request %s", requestType,
			)
		}

		msg, err := connection.nextMessage(remaining)
		if err != nil {
			return obsRequestResponse{}, fmt.Errorf(
				"OBS-Request %s: %v", requestType, err,
			)
		}

		if msg.messageType != websocket.TextMessage {
			continue
		}

		var response obsRequestResponse
		if err := json.Unmarshal(msg.data, &response); err != nil {
			continue
		}

		// Opcode 5 sind Ereignisse, Opcode 7 sind Request-Antworten.
		if response.Op != 7 {
			continue
		}

		// Antworten auf andere Requests ignorieren.
		if response.D.RequestID != requestID {
			continue
		}

		if !response.D.RequestStatus.Result {
			return response, fmt.Errorf(
				"OBS-Request %s fehlgeschlagen (Code %d): %s",
				requestType,
				response.D.RequestStatus.Code,
				response.D.RequestStatus.Comment,
			)
		}

		return response, nil
	}
}

// Führt run() aus. Ctrl+C/SIGTERM stoppt währenddessen von VBX gestartete
// OBS-Aufnahmen und beendet den Prozess; nach run() (egal wie es endete)
// werden übrig gebliebene Aufnahmen ebenfalls gestoppt.
func runWithShutdownHook(run func()) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	done := make(chan struct{})
	go func() {
		select {
		case <-sigCh:
			StopAllOBSRecordings()
			os.Exit(130)
		case <-done:
		}
	}()

	defer func() {
		signal.Stop(sigCh)
		close(done)
		StopAllOBSRecordings()
	}()

	run()
}

// RunWithShutdownHook ist die öffentliche Variante für den Evaluator
// (direktes Ausführen einer .vb-Datei): der Skriptlauf hängt damit wie im
// Interpreter am Signal- und OBS-Aufnahme-Hook.
func RunWithShutdownHook(run func()) { runWithShutdownHook(run) }

// Stoppt alle von VBX gestarteten OBS-Aufnahmen. Für Ctrl+C/SIGTERM/Skriptende.
func StopAllOBSRecordings() {
	webSocketConnectionsMu.RLock()
	conns := make([]*vbxWebSocketConnection, 0, len(webSocketConnections))
	for _, c := range webSocketConnections {
		conns = append(conns, c)
	}
	webSocketConnectionsMu.RUnlock()

	for _, c := range conns {
		if !c.obsRecording.Swap(false) {
			continue
		}

		// Normalfall: sauber mit Antwort abwarten.
		if c.readMu.TryLock() {
			c.readMu.Unlock() // obsRequest sperrt selbst
			_, _ = obsRequest(c, "StopRecord", 10*time.Second)
			continue
		}

		// Ein Receive blockiert gerade den Leser: Request ohne Antwort senden.
		payload := fmt.Sprintf(
			`{"op":6,"d":{"requestType":"StopRecord","requestId":"vbx-exit"}}`,
		)
		_ = sendWebSocketText(c, payload)
		time.Sleep(500 * time.Millisecond)
	}
}

// ------------------------------------------------------------
// Hintergrund-Leser
// ------------------------------------------------------------

func (c *vbxWebSocketConnection) readLoop() {
	defer close(c.done)

	for {
		messageType, message, err := c.conn.ReadMessage()
		if err != nil {
			c.stateMu.Lock()
			c.readErr = err
			c.stateMu.Unlock()
			return
		}

		msg := vbxWebSocketMessage{
			messageType: messageType,
			data:        message,
		}

		select {
		case c.messages <- msg:
		case <-c.stop:
			return
		}
	}
}

func (c *vbxWebSocketConnection) getReadError() error {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	return c.readErr
}

func (c *vbxWebSocketConnection) connectionError() error {
	if err := c.getReadError(); err != nil {
		return err
	}
	return fmt.Errorf("WebSocket-Verbindung wurde geschlossen")
}

func (c *vbxWebSocketConnection) nextMessage(
	timeout time.Duration,
) (vbxWebSocketMessage, error) {
	select {
	case msg := <-c.messages:
		return msg, nil
	default:
	}

	if timeout <= 0 {
		select {
		case msg := <-c.messages:
			return msg, nil
		case <-c.done:
			select {
			case msg := <-c.messages:
				return msg, nil
			default:
			}
			return vbxWebSocketMessage{}, c.connectionError()
		}
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case msg := <-c.messages:
		return msg, nil

	case <-c.done:
		select {
		case msg := <-c.messages:
			return msg, nil
		default:
		}
		return vbxWebSocketMessage{}, c.connectionError()

	case <-timer.C:
		return vbxWebSocketMessage{}, errWebSocketTimeout
	}
}

// ------------------------------------------------------------
// Verbindung und Hilfsfunktionen
// ------------------------------------------------------------

func validateWebSocketURL(rawURL string) error {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("Ungültige URL: %v", err)
	}

	if parsedURL.Scheme != "ws" && parsedURL.Scheme != "wss" {
		return fmt.Errorf("Die URL muss mit ws:// oder wss:// beginnen")
	}

	if parsedURL.Host == "" {
		return fmt.Errorf("In der URL fehlt der Host")
	}

	return nil
}

func createWebSocketConnection(
	rawURL string,
	headers http.Header,
	timeout time.Duration,
) (int64, *vbxWebSocketConnection, error) {
	if err := validateWebSocketURL(rawURL); err != nil {
		return 0, nil, err
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: timeout,
	}

	conn, response, err := dialer.Dial(rawURL, headers)
	if err != nil {
		message := err.Error()
		if response != nil {
			message += fmt.Sprintf(" (HTTP-Status %d)", response.StatusCode)
		}
		return 0, nil, fmt.Errorf("%s", message)
	}

	connection := &vbxWebSocketConnection{
		conn:     conn,
		messages: make(chan vbxWebSocketMessage, 64),
		done:     make(chan struct{}),
		stop:     make(chan struct{}),
	}

	handle := webSocketNextHandle.Add(1)

	webSocketConnectionsMu.Lock()
	webSocketConnections[handle] = connection
	webSocketConnectionsMu.Unlock()

	go connection.readLoop()

	return handle, connection, nil
}

func closeWebSocketConnection(connection *vbxWebSocketConnection) error {
	connection.stopOnce.Do(func() {
		close(connection.stop)
	})

	connection.writeMu.Lock()
	defer connection.writeMu.Unlock()

	_ = connection.conn.WriteControl(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(
			websocket.CloseNormalClosure,
			"VBX connection closed",
		),
		time.Now().Add(time.Second),
	)

	return connection.conn.Close()
}

func discardWebSocketConnection(
	handle int64,
	connection *vbxWebSocketConnection,
) {
	webSocketConnectionsMu.Lock()
	if webSocketConnections[handle] == connection {
		delete(webSocketConnections, handle)
	}
	webSocketConnectionsMu.Unlock()

	_ = closeWebSocketConnection(connection)
}

func sendWebSocketText(
	connection *vbxWebSocketConnection,
	message string,
) error {
	select {
	case <-connection.done:
		return fmt.Errorf("Verbindung ist geschlossen")
	default:
	}

	connection.writeMu.Lock()
	defer connection.writeMu.Unlock()

	if err := connection.conn.SetWriteDeadline(
		time.Now().Add(10 * time.Second),
	); err != nil {
		return err
	}

	return connection.conn.WriteMessage(
		websocket.TextMessage,
		[]byte(message),
	)
}

func webSocketHandle(value valpkg.Value) (int64, error) {
	handleNumber := value.Num

	if handleNumber <= 0 ||
		handleNumber != float64(int64(handleNumber)) {
		return 0, fmt.Errorf("Ungültiges WebSocket-Handle")
	}

	return int64(handleNumber), nil
}

func getWebSocketConnection(
	value valpkg.Value,
) (*vbxWebSocketConnection, error) {
	handle, err := webSocketHandle(value)
	if err != nil {
		return nil, err
	}

	webSocketConnectionsMu.RLock()
	connection, exists := webSocketConnections[handle]
	webSocketConnectionsMu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("Ungültiges oder geschlossenes WebSocket-Handle")
	}

	return connection, nil
}

// ------------------------------------------------------------
// OBS WebSocket v5
// ------------------------------------------------------------

type obsWebSocketEnvelope struct {
	Op int `json:"op"`
	D  struct {
		RPCVersion     int `json:"rpcVersion"`
		Authentication *struct {
			Challenge string `json:"challenge"`
			Salt      string `json:"salt"`
		} `json:"authentication"`
	} `json:"d"`
}

func obsAuthentication(password, salt, challenge string) string {
	secretHash := sha256.Sum256([]byte(password + salt))
	secret := base64.StdEncoding.EncodeToString(secretHash[:])

	authHash := sha256.Sum256([]byte(secret + challenge))
	return base64.StdEncoding.EncodeToString(authHash[:])
}

func receiveOBSMessage(
	connection *vbxWebSocketConnection,
	timeout time.Duration,
) (obsWebSocketEnvelope, error) {
	msg, err := connection.nextMessage(timeout)
	if err != nil {
		return obsWebSocketEnvelope{}, err
	}

	if msg.messageType != websocket.TextMessage {
		return obsWebSocketEnvelope{}, fmt.Errorf(
			"OBS hat eine nicht unterstützte Binärnachricht gesendet",
		)
	}

	var envelope obsWebSocketEnvelope
	if err := json.Unmarshal(msg.data, &envelope); err != nil {
		return obsWebSocketEnvelope{}, fmt.Errorf(
			"Ungültige OBS-WebSocket-Nachricht: %v", err,
		)
	}

	return envelope, nil
}

func connectOBS(
	rawURL string,
	password string,
	timeout time.Duration,
) (int64, error) {
	handle, connection, err := createWebSocketConnection(
		rawURL, nil, timeout,
	)
	if err != nil {
		return 0, fmt.Errorf("Verbindung fehlgeschlagen: %v", err)
	}

	fail := func(err error) (int64, error) {
		discardWebSocketConnection(handle, connection)
		return 0, err
	}

	hello, err := receiveOBSMessage(connection, timeout)
	if err != nil {
		return fail(fmt.Errorf("Hello-Nachricht konnte nicht gelesen werden: %v", err))
	}

	if hello.Op != 0 {
		return fail(fmt.Errorf(
			"Erwartete OBS-Hello-Nachricht (Opcode 0), erhalten: %d",
			hello.Op,
		))
	}

	rpcVersion := hello.D.RPCVersion
	if rpcVersion <= 0 {
		rpcVersion = 1
	}

	identify := map[string]interface{}{
		"op": 1,
		"d": map[string]interface{}{
			"rpcVersion": rpcVersion,
		},
	}

	if hello.D.Authentication != nil {
		auth := hello.D.Authentication

		if auth.Salt == "" || auth.Challenge == "" {
			return fail(fmt.Errorf(
				"OBS-Hello enthält unvollständige Authentifizierungsdaten",
			))
		}

		if password == "" {
			return fail(fmt.Errorf(
				"OBS verlangt ein Passwort, aber es wurde keines angegeben",
			))
		}

		identify["d"].(map[string]interface{})["authentication"] =
			obsAuthentication(password, auth.Salt, auth.Challenge)
	}

	payload, err := json.Marshal(identify)
	if err != nil {
		return fail(fmt.Errorf("Identify-Nachricht konnte nicht erstellt werden: %v", err))
	}

	if err := sendWebSocketText(connection, string(payload)); err != nil {
		return fail(fmt.Errorf("Identify-Nachricht konnte nicht gesendet werden: %v", err))
	}

	response, err := receiveOBSMessage(connection, timeout)
	if err != nil {
		return fail(fmt.Errorf("OBS-Anmeldung fehlgeschlagen: %v", err))
	}

	if response.Op != 2 {
		return fail(fmt.Errorf(
			"OBS hat die Anmeldung nicht bestätigt (Opcode %d)",
			response.Op,
		))
	}

	return handle, nil
}

// ------------------------------------------------------------
// Initialisierung
// ------------------------------------------------------------

func initWebSocketFunctions() {
	if builtins == nil {
		builtins = make(map[string]BuiltinInfo)
	}

	ns := "websocket."

	// --------------------------------------------------------
	// Connect
	// --------------------------------------------------------

	Register(ns+"Connect", "websocket", "url, [headers], [timeoutMs]", "Stellt eine WebSocket-Verbindung her. Unterstützt ws:// und wss://.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.ErrorVal("websocket.Connect(url, [headers], [timeoutMs]) benötigt eine URL")
		}

		rawURL := strings.TrimSpace(valpkg.ToString(args[0]))
		if rawURL == "" {
			return valpkg.ErrorVal("websocket.Connect: Die URL darf nicht leer sein")
		}

		timeout := 5 * time.Second
		var headers http.Header

		if len(args) >= 2 && args[1].Map != nil {
			headers = make(http.Header)
			for key, value := range args[1].Map {
				key = strings.TrimSpace(key)
				if key != "" {
					headers.Set(key, valpkg.ToString(value))
				}
			}
		}

		if len(args) >= 3 {
			timeoutMs := args[2].Num
			if timeoutMs <= 0 {
				return valpkg.ErrorVal("websocket.Connect: timeoutMs muss größer als 0 sein")
			}
			timeout = time.Duration(timeoutMs) * time.Millisecond
		}

		handle, _, err := createWebSocketConnection(rawURL, headers, timeout)
		if err != nil {
			return valpkg.ErrorVal(fmt.Sprintf("websocket.Connect: %v", err))
		}

		return valpkg.NumVal(float64(handle))
	},
	)

	// --------------------------------------------------------
	// OBSConnect
	// --------------------------------------------------------

	Register(ns+"OBSConnect", "websocket", "url, password, [timeoutMs]", "Verbindet sich mit OBS WebSocket v5, führt Hello/Identify aus und gibt erst nach erfolgreicher Anmeldung ein Handle zurück.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 2 {
			return valpkg.ErrorVal("websocket.OBSConnect(url, password, [timeoutMs]) benötigt URL und Passwort")
		}

		rawURL := strings.TrimSpace(valpkg.ToString(args[0]))
		password := valpkg.ToString(args[1])

		timeout := 5 * time.Second
		if len(args) >= 3 {
			timeoutMs := args[2].Num
			if timeoutMs <= 0 {
				return valpkg.ErrorVal("websocket.OBSConnect: timeoutMs muss größer als 0 sein")
			}
			timeout = time.Duration(timeoutMs) * time.Millisecond
		}

		handle, err := connectOBS(rawURL, password, timeout)
		if err != nil {
			return valpkg.ErrorVal(fmt.Sprintf("websocket.OBSConnect: %v", err))
		}

		return valpkg.NumVal(float64(handle))
	},
	)

	// --------------------------------------------------------
	// Send
	// --------------------------------------------------------

	Register(ns+"Send", "websocket", "handle, message", "Sendet eine Textnachricht über eine bestehende WebSocket-Verbindung.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 2 {
			return valpkg.ErrorVal("websocket.Send(handle, message) benötigt zwei Argumente")
		}

		connection, err := getWebSocketConnection(args[0])
		if err != nil {
			return valpkg.ErrorVal(err.Error())
		}

		if err := sendWebSocketText(connection, valpkg.ToString(args[1])); err != nil {
			return valpkg.ErrorVal(fmt.Sprintf("websocket.Send: %v", err))
		}

		return valpkg.BoolVal(true)
	},
	)

	// --------------------------------------------------------
	// Receive
	// --------------------------------------------------------

	Register(ns+"Receive", "websocket", "handle, [timeoutMs]", "Empfängt eine Textnachricht. Bei Timeout wird ein leerer String zurückgegeben; die Verbindung bleibt bestehen.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.ErrorVal("websocket.Receive(handle, [timeoutMs]) benötigt ein Handle")
		}

		connection, err := getWebSocketConnection(args[0])
		if err != nil {
			return valpkg.ErrorVal(err.Error())
		}

		var timeout time.Duration
		if len(args) >= 2 {
			timeoutMs := args[1].Num
			if timeoutMs <= 0 {
				return valpkg.ErrorVal("websocket.Receive: timeoutMs muss größer als 0 sein")
			}
			timeout = time.Duration(timeoutMs) * time.Millisecond
		}

		connection.readMu.Lock()
		defer connection.readMu.Unlock()

		msg, err := connection.nextMessage(timeout)
		if err != nil {
			if errors.Is(err, errWebSocketTimeout) {
				return valpkg.StrVal("")
			}
			return valpkg.ErrorVal(fmt.Sprintf("websocket.Receive: %v", err))
		}

		switch msg.messageType {
		case websocket.TextMessage:
			return valpkg.StrVal(string(msg.data))
		case websocket.BinaryMessage:
			return valpkg.ErrorVal("websocket.Receive: Binärnachrichten werden derzeit nicht unterstützt")
		default:
			return valpkg.ErrorVal(fmt.Sprintf(
				"websocket.Receive: Nicht unterstützter Nachrichtentyp %d",
				msg.messageType,
			))
		}
	},
	)

	// --------------------------------------------------------
	// IsConnected
	// --------------------------------------------------------

	Register(ns+"IsConnected", "websocket", "handle", "Prüft, ob die Verbindung noch nicht als geschlossen erkannt wurde.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.ErrorVal("websocket.IsConnected(handle) benötigt ein Handle")
		}

		connection, err := getWebSocketConnection(args[0])
		if err != nil {
			return valpkg.BoolVal(false)
		}

		select {
		case <-connection.done:
			return valpkg.BoolVal(false)
		default:
			return valpkg.BoolVal(true)
		}
	},
	)

	// --------------------------------------------------------
	// Close
	// --------------------------------------------------------

	Register(ns+"Close", "websocket", "handle", "Schließt eine WebSocket-Verbindung und entfernt das Handle.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.ErrorVal("websocket.Close(handle) benötigt ein Handle")
		}

		handle, err := webSocketHandle(args[0])
		if err != nil {
			return valpkg.ErrorVal(err.Error())
		}

		webSocketConnectionsMu.Lock()
		connection, exists := webSocketConnections[handle]
		if exists {
			delete(webSocketConnections, handle)
		}
		webSocketConnectionsMu.Unlock()

		if !exists {
			return valpkg.ErrorVal("websocket.Close: Ungültiges oder bereits geschlossenes Handle")
		}

		if err := closeWebSocketConnection(connection); err != nil {
			return valpkg.ErrorVal(fmt.Sprintf("websocket.Close: %v", err))
		}

		return valpkg.BoolVal(true)
	},
	)

	// --------------------------------------------------------
	// OBSGetRecordStatus
	// --------------------------------------------------------

	Register(ns+"OBSGetRecordStatus", "websocket", "handle", "Prüft, ob OBS aktuell eine Aufnahme durchführt.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.ErrorVal(
				"websocket.OBSGetRecordStatus(handle) benötigt ein Handle",
			)
		}

		connection, err := getWebSocketConnection(args[0])
		if err != nil {
			return valpkg.ErrorVal(err.Error())
		}

		response, err := obsRequest(
			connection,
			"GetRecordStatus",
			5*time.Second,
		)
		if err != nil {
			return valpkg.ErrorVal(fmt.Sprintf(
				"websocket.OBSGetRecordStatus: %v", err,
			))
		}

		return valpkg.BoolVal(response.D.ResponseData.OutputActive)
	},
	)

	// --------------------------------------------------------
	// OBSStartRecord
	// --------------------------------------------------------

	Register(ns+"OBSStartRecord", "websocket", "handle", "Startet die OBS-Aufnahme mit den aktuell eingestellten OBS-Aufnahmeoptionen.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.ErrorVal(
				"websocket.OBSStartRecord(handle) benötigt ein Handle",
			)
		}

		connection, err := getWebSocketConnection(args[0])
		if err != nil {
			return valpkg.ErrorVal(err.Error())
		}

		// Keine zweite Aufnahme starten, wenn OBS bereits aufnimmt.
		status, err := obsRequest(
			connection,
			"GetRecordStatus",
			5*time.Second,
		)
		if err != nil {
			return valpkg.ErrorVal(fmt.Sprintf(
				"websocket.OBSStartRecord: Statusprüfung fehlgeschlagen: %v",
				err,
			))
		}

		if status.D.ResponseData.OutputActive {
			return valpkg.ErrorVal(
				"websocket.OBSStartRecord: OBS nimmt bereits auf",
			)
		}

		_, err = obsRequest(
			connection,
			"StartRecord",
			10*time.Second,
		)
		if err != nil {
			return valpkg.ErrorVal(fmt.Sprintf(
				"websocket.OBSStartRecord: %v", err,
			))
		}

		connection.obsRecording.Store(true) // <-- NEU

		return valpkg.BoolVal(true)
	},
	)

	// --------------------------------------------------------
	// OBSStopRecord
	// --------------------------------------------------------

	Register(ns+"OBSStopRecord", "websocket", "handle", "Stoppt die OBS-Aufnahme und wartet auf die erfolgreiche OBS-Request-Antwort.", func(args []valpkg.Value) valpkg.Value {
		if len(args) < 1 {
			return valpkg.ErrorVal(
				"websocket.OBSStopRecord(handle) benötigt ein Handle",
			)
		}

		connection, err := getWebSocketConnection(args[0])
		if err != nil {
			return valpkg.ErrorVal(err.Error())
		}

		status, err := obsRequest(
			connection,
			"GetRecordStatus",
			5*time.Second,
		)
		if err != nil {
			return valpkg.ErrorVal(fmt.Sprintf(
				"websocket.OBSStopRecord: Statusprüfung fehlgeschlagen: %v",
				err,
			))
		}

		if !status.D.ResponseData.OutputActive {
			return valpkg.ErrorVal(
				"websocket.OBSStopRecord: OBS nimmt derzeit nicht auf",
			)
		}

		_, err = obsRequest(
			connection,
			"StopRecord",
			30*time.Second,
		)
		if err != nil {
			return valpkg.ErrorVal(fmt.Sprintf(
				"websocket.OBSStopRecord: %v", err,
			))
		}

		connection.obsRecording.Store(false) // <-- NEU

		return valpkg.BoolVal(true)
	},
	)
}

func init() { initWebSocketFunctions() }
