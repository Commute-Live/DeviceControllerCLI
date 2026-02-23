package tui

import (
    "fmt"
    "strings"
    "time"

    "github.com/charmbracelet/bubbles/key"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
    "github.com/charmbracelet/x/ansi"
    "github.com/commute-live/loadtest/device"
    "github.com/commute-live/loadtest/runner"
)

// Keybindings
type keyMap struct {
    Up      key.Binding
    Down    key.Binding
    Quit    key.Binding
    Refresh key.Binding
    Filter  key.Binding
    Help    key.Binding
}

var keys = keyMap{
    Up:      key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
    Down:    key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
    Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit+cleanup")),
    Refresh: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh device")),
    Filter:  key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "filter errors")),
    Help:    key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
}

// Model is the root bubbletea model for the load test TUI.
type Model struct {
    devices     []*device.MockDevice
    stats       *runner.Stats
    selected    int
    filterError bool
    showHelp    bool
    width       int
    height      int
    startedAt   time.Time
}

// NewModel creates a new TUI model.
func NewModel(devices []*device.MockDevice, stats *runner.Stats) *Model {
    return &Model{
        devices:   devices,
        stats:     stats,
        startedAt: time.Now(),
    }
}

func (m *Model) Init() tea.Cmd {
    return nil
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {

    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height

    case tea.KeyMsg:
        if m.showHelp {
            m.showHelp = false
            return m, nil
        }
        switch {
        case key.Matches(msg, keys.Quit):
            return m, tea.Quit
        case key.Matches(msg, keys.Up):
            if m.selected > 0 {
                m.selected--
            }
        case key.Matches(msg, keys.Down):
            visible := m.visibleDevices()
            if m.selected < len(visible)-1 {
                m.selected++
            }
        case key.Matches(msg, keys.Refresh):
            visible := m.visibleDevices()
            if m.selected < len(visible) {
                d := visible[m.selected]
                go func() { _ = d.ForceRefresh() }()
            }
        case key.Matches(msg, keys.Filter):
            m.filterError = !m.filterError
            m.selected = 0
        case key.Matches(msg, keys.Help):
            m.showHelp = !m.showHelp
        }

    case runner.TickMsg:
        // stats refresh — just re-render
    case device.Event:
        // device state change — just re-render
    }

    return m, nil
}

func (m *Model) visibleDevices() []*device.MockDevice {
    if !m.filterError {
        return m.devices
    }
    var errored []*device.MockDevice
    for _, d := range m.devices {
        if d.GetState() == device.StateError {
            errored = append(errored, d)
        }
    }
    return errored
}

func (m *Model) View() string {
    // Don't render until bubbletea tells us the terminal size.
    if m.width == 0 || m.height == 0 {
        return "Loading..."
    }

    if m.showHelp {
        return m.helpView()
    }

    // Layout constants.
    // listWidth is the inner content width of the left panel (no padding).
    // dividerWidth = 3 (" │ ")
    // detailWidth fills the remainder.
    const listWidth = 24
    const dividerWidth = 3
    // 2 rows: 1 header + 1 footer
    bodyHeight := m.height - 2
    if bodyHeight < 1 {
        bodyHeight = 1
    }
    // detailWidth = total terminal width - listWidth - dividerWidth
    detailWidth := m.width - listWidth - dividerWidth
    if detailWidth < 10 {
        detailWidth = 10
    }

    visible := m.visibleDevices()
    if len(visible) > 0 && m.selected >= len(visible) {
        m.selected = len(visible) - 1
    }

    var selectedDevice *device.MockDevice
    if len(visible) > 0 {
        selectedDevice = visible[m.selected]
    }

    // Render panels — inner content only, no lipgloss padding.
    listContent := renderList(visible, m.selected, listWidth, bodyHeight)
    detailContent := renderDetail(selectedDevice, detailWidth, bodyHeight)

    // Build body by joining lines side-by-side manually.
    // This avoids lipgloss JoinHorizontal padding surprises.
    listLines := splitPad(listContent, listWidth, bodyHeight)
    detailLines := splitPad(detailContent, detailWidth, bodyHeight)
    divLines := make([]string, bodyHeight)
    for i := range divLines {
        divLines[i] = dividerStyle.Render(" │ ")
    }

    var bodyLines []string
    for i := 0; i < bodyHeight; i++ {
        bodyLines = append(bodyLines, listLines[i]+divLines[i]+detailLines[i])
    }
    body := strings.Join(bodyLines, "\n")

    header := headerStyle.Width(m.width).Render(m.statsBarView())
    footer := footerStyle.Width(m.width).Render(
        "↑↓ navigate  q quit+cleanup  r refresh  e filter errors  ? help",
    )

    return header + "\n" + body + "\n" + footer
}

func (m *Model) statsBarView() string {
    elapsed := time.Since(m.startedAt)
    h := int(elapsed.Hours())
    min := int(elapsed.Minutes()) % 60
    sec := int(elapsed.Seconds()) % 60

    active := m.stats.ActiveDevices.Load()
    mqttTotal := m.stats.MQTTTotal.Load()
    errors := m.stats.ErrorCount.Load()
    msgsPerSec := m.stats.MsgsPerSec()

    return fmt.Sprintf(
        "Devices: %d  Active: %d  MQTT msgs: %s  %.1f/s  Errors: %d  Elapsed: %02d:%02d:%02d",
        m.stats.TotalDevices,
        active,
        formatNumber(mqttTotal),
        msgsPerSec,
        errors,
        h, min, sec,
    )
}

func (m *Model) helpView() string {
    help := `  CommuteLive Load Test — Keybindings

  ↑ / k       Navigate device list up
  ↓ / j       Navigate device list down
  q           Quit + trigger cleanup
  r           Force refresh selected device
  e           Toggle filter: errored devices only
  ?           Toggle this help overlay

  Press any key to close.`
    return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
        helpStyle.Render(help))
}

// splitPad splits a string into exactly n lines, each truncated and padded to
// exactly w visible characters using ANSI-aware operations.
func splitPad(s string, w, n int) []string {
    if w < 1 {
        w = 1
    }
    raw := strings.Split(s, "\n")
    out := make([]string, n)
    for i := 0; i < n; i++ {
        var line string
        if i < len(raw) {
            line = raw[i]
        }
        // Truncate to w visible chars (ANSI-safe)
        line = ansi.Truncate(line, w, "")
        // Pad to exactly w visible chars
        vw := ansi.StringWidth(line)
        if vw < w {
            line += strings.Repeat(" ", w-vw)
        }
        out[i] = line
    }
    return out
}

func formatNumber(n int64) string {
    if n < 1000 {
        return fmt.Sprintf("%d", n)
    }
    if n < 1_000_000 {
        return fmt.Sprintf("%d,%03d", n/1000, n%1000)
    }
    return fmt.Sprintf("%d,%03d,%03d", n/1_000_000, (n/1000)%1000, n%1000)
}

// Styles — no padding on panels; width is set at render time.
var (
    headerStyle = lipgloss.NewStyle().
        Bold(true).
        Foreground(lipgloss.Color("15")).
        Background(lipgloss.Color("57"))

    footerStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color("240")).
        Background(lipgloss.Color("235"))

    dividerStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color("240"))

    helpStyle = lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        BorderForeground(lipgloss.Color("99")).
        Padding(1, 2)
)
