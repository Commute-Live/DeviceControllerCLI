package device

import (
    "fmt"
    "sync"
    "time"

    "github.com/commute-live/loadtest/providers"
    "github.com/google/uuid"
)

// State represents the lifecycle stage of a MockDevice.
type State int

const (
    StateInit State = iota
    StateRegistering
    StateAuthenticating
    StateLinking
    StateConfiguring
    StateActive
    StateError
    StateDone
)

func (s State) String() string {
    switch s {
    case StateInit:
        return "INIT"
    case StateRegistering:
        return "REGISTERING"
    case StateAuthenticating:
        return "AUTHENTICATING"
    case StateLinking:
        return "LINKING"
    case StateConfiguring:
        return "CONFIGURING"
    case StateActive:
        return "ACTIVE"
    case StateError:
        return "ERROR"
    case StateDone:
        return "DONE"
    default:
        return "UNKNOWN"
    }
}

// HTTPLogEntry records a single HTTP request/response.
type HTTPLogEntry struct {
    Timestamp time.Time
    Method    string
    Path      string
    Status    int
    OK        bool
}

func (e HTTPLogEntry) String() string {
    mark := "✓"
    if !e.OK {
        mark = "✗"
    }
    return fmt.Sprintf("%s  %-6s %-40s  %d %s",
        e.Timestamp.Format("15:04:05"), e.Method, e.Path, e.Status, mark)
}

// MQTTMessage records an incoming MQTT message.
type MQTTMessage struct {
    Timestamp time.Time
    Topic     string
    Payload   string
}

func (m MQTTMessage) String() string {
    return fmt.Sprintf("%s  %s", m.Timestamp.Format("15:04:05"), m.Payload)
}

// MockDevice represents one simulated ESP32 device going through the full lifecycle.
type MockDevice struct {
    mu sync.RWMutex

    // Identity
    ShortID  string // last 8 chars of UUID for display
    DeviceID string // "loadtest-{uuid}"
    Email    string // "loadtest-{uuid}@test.invalid"
    Password string

    // Assignment
    Stop providers.Stop

    // State
    State      State
    ErrorMsg   string
    HTTPLog    []HTTPLogEntry
    MQTTMsgs   []MQTTMessage
    MQTTCount  int
    StartedAt  time.Time
    ActiveAt   time.Time

    // Internal transport
    httpClient *httpClient
    mqttClient *mqttClient
    stopCh     chan struct{}
    doneCh     chan struct{}
}

// New creates a new MockDevice with the given provider assignment.
func New(serverURL, secretKey, mqttHost, mqttUsername, mqttPassword string, mqttPort int, stop providers.Stop) *MockDevice {
    id := uuid.New().String()
    deviceID := "loadtest-" + id
    email := "loadtest-" + id + "@test.invalid"
    password := uuid.New().String()

    d := &MockDevice{
        ShortID:  id[:8],
        DeviceID: deviceID,
        Email:    email,
        Password: password,
        Stop:     stop,
        State:    StateInit,
        StartedAt: time.Now(),
        stopCh:   make(chan struct{}),
        doneCh:   make(chan struct{}),
    }
    d.httpClient = newHTTPClient(serverURL, secretKey, d)
    d.mqttClient = newMQTTClient(mqttHost, mqttPort, mqttUsername, mqttPassword, d)
    return d
}

// Run executes the full device lifecycle and blocks until stopped or done.
func (d *MockDevice) Run(eventCh chan<- Event) {
    defer close(d.doneCh)

    steps := []struct {
        name string
        fn   func() error
    }{
        {"register device", d.httpClient.registerDevice},
        {"register user", d.httpClient.registerUser},
        {"login", d.httpClient.login},
        {"link device", d.httpClient.linkDevice},
        {"set config", d.httpClient.setConfig},
        {"get config", d.httpClient.getConfig},
        {"connect mqtt", d.mqttClient.connect},
        {"subscribe mqtt", d.mqttClient.subscribe},
    }

    for _, step := range steps {
        select {
        case <-d.stopCh:
            d.setState(StateDone)
            return
        default:
        }
        if err := step.fn(); err != nil {
            d.setError(fmt.Sprintf("%s: %v", step.name, err))
            eventCh <- Event{DeviceID: d.DeviceID, Type: EventError}
            return
        }
    }

    d.mu.Lock()
    d.State = StateActive
    d.ActiveAt = time.Now()
    d.mu.Unlock()
    eventCh <- Event{DeviceID: d.DeviceID, Type: EventActive}

    <-d.stopCh
    d.cleanup(eventCh)
}

// Shutdown signals the device to shut down gracefully.
func (d *MockDevice) Shutdown() {
    select {
    case <-d.stopCh:
    default:
        close(d.stopCh)
    }
}

// Wait blocks until the device has finished its lifecycle.
func (d *MockDevice) Wait() {
    <-d.doneCh
}

// ForceRefresh triggers a manual refresh of transit data for this device.
func (d *MockDevice) ForceRefresh() error {
    return d.httpClient.refresh()
}

// GetState returns the current state (thread-safe).
func (d *MockDevice) GetState() State {
    d.mu.RLock()
    defer d.mu.RUnlock()
    return d.State
}

// GetHTTPLog returns a copy of the HTTP log (thread-safe).
func (d *MockDevice) GetHTTPLog() []HTTPLogEntry {
    d.mu.RLock()
    defer d.mu.RUnlock()
    cp := make([]HTTPLogEntry, len(d.HTTPLog))
    copy(cp, d.HTTPLog)
    return cp
}

// GetMQTTMsgs returns a copy of recent MQTT messages (thread-safe).
func (d *MockDevice) GetMQTTMsgs() []MQTTMessage {
    d.mu.RLock()
    defer d.mu.RUnlock()
    cp := make([]MQTTMessage, len(d.MQTTMsgs))
    copy(cp, d.MQTTMsgs)
    return cp
}

// GetMQTTCount returns total MQTT messages received (thread-safe).
func (d *MockDevice) GetMQTTCount() int {
    d.mu.RLock()
    defer d.mu.RUnlock()
    return d.MQTTCount
}

// GetErrorMsg returns the last error message (thread-safe).
func (d *MockDevice) GetErrorMsg() string {
    d.mu.RLock()
    defer d.mu.RUnlock()
    return d.ErrorMsg
}

// addHTTPLog appends an HTTP log entry (must be called with mu held or with lock).
func (d *MockDevice) addHTTPLog(entry HTTPLogEntry) {
    d.mu.Lock()
    d.HTTPLog = append(d.HTTPLog, entry)
    d.mu.Unlock()
}

// addMQTTMsg appends an MQTT message (thread-safe).
func (d *MockDevice) addMQTTMsg(msg MQTTMessage) {
    d.mu.Lock()
    d.MQTTMsgs = append(d.MQTTMsgs, msg)
    d.MQTTCount++
    d.mu.Unlock()
}

func (d *MockDevice) setState(s State) {
    d.mu.Lock()
    d.State = s
    d.mu.Unlock()
}

func (d *MockDevice) setError(msg string) {
    d.mu.Lock()
    d.State = StateError
    d.ErrorMsg = msg
    d.mu.Unlock()
}

func (d *MockDevice) cleanup(eventCh chan<- Event) {
    _ = d.httpClient.logout()
    d.mqttClient.disconnect()
    d.setState(StateDone)
    eventCh <- Event{DeviceID: d.DeviceID, Type: EventDone}
}

// Event types emitted by devices to the runner.
type EventType int

const (
    EventActive EventType = iota
    EventError
    EventDone
    EventMQTT
)

// Event is emitted by a device to signal a lifecycle change.
type Event struct {
    DeviceID string
    Type     EventType
}
