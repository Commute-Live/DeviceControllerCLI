package device

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "net/http/cookiejar"
    "net/url"
    "strings"
    "time"
)

type httpClient struct {
    base      string
    secretKey string
    device    *MockDevice
    client    *http.Client
}

func newHTTPClient(serverURL, secretKey string, d *MockDevice) *httpClient {
    jar, _ := cookiejar.New(nil)
    return &httpClient{
        base:      strings.TrimRight(serverURL, "/"),
        secretKey: secretKey,
        device:    d,
        client: &http.Client{
            // No global timeout — each call passes its own via doWithTimeout.
            Jar: jar,
        },
    }
}

// do performs an HTTP request with the default 15-second timeout.
func (h *httpClient) do(method, path string, body interface{}) (int, []byte, error) {
    return h.doWithTimeout(method, path, body, 15*time.Second)
}

// doWithTimeout performs an HTTP request with a caller-specified timeout.
func (h *httpClient) doWithTimeout(method, path string, body interface{}, timeout time.Duration) (int, []byte, error) {
    var bodyReader io.Reader
    if body != nil {
        b, err := json.Marshal(body)
        if err != nil {
            return 0, nil, err
        }
        bodyReader = bytes.NewReader(b)
    }

    req, err := http.NewRequest(method, h.base+path, bodyReader)
    if err != nil {
        return 0, nil, err
    }
    if body != nil {
        req.Header.Set("Content-Type", "application/json")
    }
    if h.secretKey != "" {
        req.Header.Set("X-Loadtest-Key", h.secretKey)
    }

    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()
    req = req.WithContext(ctx)

    resp, err := h.client.Do(req)
    if err != nil {
        h.device.addHTTPLog(HTTPLogEntry{
            Timestamp: time.Now(),
            Method:    method,
            Path:      path,
            Status:    0,
            OK:        false,
        })
        return 0, nil, err
    }
    defer resp.Body.Close()

    respBody, _ := io.ReadAll(resp.Body)
    ok := resp.StatusCode >= 200 && resp.StatusCode < 300
    h.device.addHTTPLog(HTTPLogEntry{
        Timestamp: time.Now(),
        Method:    method,
        Path:      path,
        Status:    resp.StatusCode,
        OK:        ok,
    })

    return resp.StatusCode, respBody, nil
}

func (h *httpClient) registerDevice() error {
    h.device.setState(StateRegistering)
    payload := map[string]string{
        "id":       h.device.DeviceID,
        "timezone": "America/New_York",
    }
    status, _, err := h.do("POST", "/device/register", payload)
    if err != nil {
        return err
    }
    if status != 200 && status != 201 {
        return fmt.Errorf("register device returned %d", status)
    }
    return nil
}

func (h *httpClient) registerUser() error {
    payload := map[string]string{
        "email":    h.device.Email,
        "password": h.device.Password,
    }
    status, _, err := h.do("POST", "/user/register", payload)
    if err != nil {
        return err
    }
    if status != 200 && status != 201 {
        return fmt.Errorf("register user returned %d", status)
    }
    return nil
}

func (h *httpClient) login() error {
    h.device.setState(StateAuthenticating)
    payload := map[string]string{
        "email":    h.device.Email,
        "password": h.device.Password,
    }
    status, _, err := h.do("POST", "/auth/login", payload)
    if err != nil {
        return err
    }
    if status != 200 && status != 201 {
        return fmt.Errorf("login returned %d", status)
    }
    return nil
}

func (h *httpClient) linkDevice() error {
    h.device.setState(StateLinking)
    payload := map[string]string{
        "deviceId": h.device.DeviceID,
    }
    status, _, err := h.do("POST", "/user/device/link", payload)
    if err != nil {
        return err
    }
    if status != 200 && status != 201 {
        return fmt.Errorf("link device returned %d", status)
    }
    return nil
}

func (h *httpClient) setConfig() error {
    h.device.setState(StateConfiguring)
    stop := h.device.Stop
    // Server expects { lines: [{ provider, line, stop, direction }] }
    payload := map[string]interface{}{
        "lines": []map[string]string{
            {
                "provider":  stop.ProviderID,
                "line":      stop.Line,
                "stop":      stop.StopID,
                "direction": stop.Direction,
            },
        },
    }
    path := "/device/" + url.PathEscape(h.device.DeviceID) + "/config"
    // Config POST triggers a live provider fetch on the server — give it 90s.
    status, _, err := h.doWithTimeout("POST", path, payload, 90*time.Second)
    if err != nil {
        return err
    }
    if status != 200 && status != 201 {
        return fmt.Errorf("set config returned %d", status)
    }
    return nil
}

func (h *httpClient) getConfig() error {
    path := "/device/" + url.PathEscape(h.device.DeviceID) + "/config"
    status, _, err := h.do("GET", path, nil)
    if err != nil {
        return err
    }
    if status != 200 {
        return fmt.Errorf("get config returned %d", status)
    }
    return nil
}

func (h *httpClient) refresh() error {
    path := "/refresh/" + url.PathEscape(h.device.DeviceID)
    status, _, err := h.do("POST", path, nil)
    if err != nil {
        return err
    }
    if status != 200 && status != 201 {
        return fmt.Errorf("refresh returned %d", status)
    }
    return nil
}

func (h *httpClient) logout() error {
    status, _, err := h.do("POST", "/auth/logout", nil)
    if err != nil {
        return err
    }
    if status != 200 && status != 204 {
        return fmt.Errorf("logout returned %d", status)
    }
    return nil
}
