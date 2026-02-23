package tui

import (
    "fmt"
    "strings"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
)

// SetupResult is returned by SetupModel when the user confirms.
type SetupResult struct {
    Devices   int
    Duration  string       // empty = unlimited, otherwise e.g. "5m"
    Providers map[string]int // distribution, sums to 100
    Start     bool         // false = user cancelled
}

type setupField int

const (
    fieldDevices   setupField = 0
    fieldDuration  setupField = 1
    fieldProviders setupField = 2
)

var providerOrder = []string{"cta", "mta", "mbta", "septa"}

var devicePresets = []int{1, 5, 10, 25, 50}

var durationPresets = []struct {
    label string
    value string
}{
    {"30s", "30s"},
    {"1m", "1m"},
    {"2m", "2m"},
    {"5m", "5m"},
    {"10m", "10m"},
    {"unlimited", ""},
}

// SetupModel is a bubbletea model for the pre-run configuration screen.
type SetupModel struct {
    width            int
    height           int
    serverURL        string
    field            setupField
    devIdx           int
    durIdx           int
    providerIdx      int
    enabledProviders map[string]bool
    Result           SetupResult
}

// NewSetupModel creates the setup model. CLI flag values are used as defaults.
func NewSetupModel(serverURL string, defaultDevices int, defaultDuration string) *SetupModel {
    devIdx := 1 // fallback: 5 devices
    for i, n := range devicePresets {
        if n == defaultDevices {
            devIdx = i
            break
        }
    }
    durIdx := 5 // fallback: unlimited
    for i, d := range durationPresets {
        if d.value == defaultDuration {
            durIdx = i
            break
        }
    }
    return &SetupModel{
        serverURL: serverURL,
        devIdx:    devIdx,
        durIdx:    durIdx,
        enabledProviders: map[string]bool{
            "cta":   true,
            "mta":   true,
            "mbta":  true,
            "septa": true,
        },
    }
}

// EnabledProviderDist returns an equal distribution across enabled providers,
// summing exactly to 100.
func (m *SetupModel) EnabledProviderDist() map[string]int {
    var enabled []string
    for _, p := range providerOrder {
        if m.enabledProviders[p] {
            enabled = append(enabled, p)
        }
    }
    if len(enabled) == 0 {
        // Fallback: enable all
        result := make(map[string]int)
        base := 100 / len(providerOrder)
        rem := 100 % len(providerOrder)
        for i, p := range providerOrder {
            if i < rem {
                result[p] = base + 1
            } else {
                result[p] = base
            }
        }
        return result
    }
    result := make(map[string]int)
    base := 100 / len(enabled)
    rem := 100 % len(enabled)
    for i, p := range enabled {
        if i < rem {
            result[p] = base + 1
        } else {
            result[p] = base
        }
    }
    return result
}

func (m *SetupModel) Init() tea.Cmd { return nil }

func (m *SetupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height

    case tea.KeyMsg:
        switch msg.String() {
        case "ctrl+c", "q":
            m.Result = SetupResult{Start: false}
            return m, tea.Quit
        case "tab", "down", "j":
            m.field = (m.field + 1) % 3
        case "shift+tab", "up", "k":
            m.field = (m.field + 2) % 3
        case "left", "h":
            switch m.field {
            case fieldDevices:
                if m.devIdx > 0 {
                    m.devIdx--
                }
            case fieldDuration:
                if m.durIdx > 0 {
                    m.durIdx--
                }
            case fieldProviders:
                if m.providerIdx > 0 {
                    m.providerIdx--
                }
            }
        case "right", "l":
            switch m.field {
            case fieldDevices:
                if m.devIdx < len(devicePresets)-1 {
                    m.devIdx++
                }
            case fieldDuration:
                if m.durIdx < len(durationPresets)-1 {
                    m.durIdx++
                }
            case fieldProviders:
                if m.providerIdx < len(providerOrder)-1 {
                    m.providerIdx++
                }
            }
        case " ":
            if m.field == fieldProviders {
                p := providerOrder[m.providerIdx]
                m.enabledProviders[p] = !m.enabledProviders[p]
            }
        case "enter":
            if m.field == fieldProviders {
                // Toggle current provider; don't submit
                p := providerOrder[m.providerIdx]
                m.enabledProviders[p] = !m.enabledProviders[p]
            } else {
                m.Result = SetupResult{
                    Devices:   devicePresets[m.devIdx],
                    Duration:  durationPresets[m.durIdx].value,
                    Providers: m.EnabledProviderDist(),
                    Start:     true,
                }
                return m, tea.Quit
            }
        }
    }
    return m, nil
}

func (m *SetupModel) View() string {
    if m.width == 0 {
        return "Loading..."
    }

    // ── styles ──
    titleSt := lipgloss.NewStyle().
        Bold(true).
        Foreground(lipgloss.Color("15")).
        Background(lipgloss.Color("57")).
        Padding(0, 2)

    boxSt := lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        BorderForeground(lipgloss.Color("57")).
        Padding(1, 3)

    labelActiveSt := lipgloss.NewStyle().
        Bold(true).
        Foreground(lipgloss.Color("99"))

    labelSt := lipgloss.NewStyle().
        Foreground(lipgloss.Color("244"))

    chosenSt := lipgloss.NewStyle().
        Bold(true).
        Foreground(lipgloss.Color("0")).
        Background(lipgloss.Color("99")).
        Padding(0, 1)

    optionSt := lipgloss.NewStyle().
        Foreground(lipgloss.Color("244")).
        Padding(0, 1)

    enabledSt := lipgloss.NewStyle().
        Foreground(lipgloss.Color("42")).
        Padding(0, 1)

    disabledSt := lipgloss.NewStyle().
        Foreground(lipgloss.Color("240")).
        Strikethrough(true).
        Padding(0, 1)

    cursorEnabledSt := lipgloss.NewStyle().
        Bold(true).
        Foreground(lipgloss.Color("0")).
        Background(lipgloss.Color("42")).
        Padding(0, 1)

    cursorDisabledSt := lipgloss.NewStyle().
        Bold(true).
        Foreground(lipgloss.Color("0")).
        Background(lipgloss.Color("240")).
        Padding(0, 1)

    hintSt := lipgloss.NewStyle().
        Foreground(lipgloss.Color("240"))

    serverSt := lipgloss.NewStyle().
        Foreground(lipgloss.Color("42"))

    // ── device row ──
    devLbl := labelSt.Render("  Devices  ")
    if m.field == fieldDevices {
        devLbl = labelActiveSt.Render("▶ Devices  ")
    }
    var devOpts []string
    for i, n := range devicePresets {
        s := fmt.Sprintf("%d", n)
        if i == m.devIdx {
            devOpts = append(devOpts, chosenSt.Render(s))
        } else {
            devOpts = append(devOpts, optionSt.Render(s))
        }
    }

    // ── duration row ──
    durLbl := labelSt.Render("  Duration ")
    if m.field == fieldDuration {
        durLbl = labelActiveSt.Render("▶ Duration ")
    }
    var durOpts []string
    for i, d := range durationPresets {
        if i == m.durIdx {
            durOpts = append(durOpts, chosenSt.Render(d.label))
        } else {
            durOpts = append(durOpts, optionSt.Render(d.label))
        }
    }

    // ── providers row ──
    provLbl := labelSt.Render("  Providers")
    if m.field == fieldProviders {
        provLbl = labelActiveSt.Render("▶ Providers")
    }
    var provOpts []string
    for i, p := range providerOrder {
        enabled := m.enabledProviders[p]
        isCursor := m.field == fieldProviders && i == m.providerIdx
        label := strings.ToUpper(p)
        if isCursor {
            if enabled {
                provOpts = append(provOpts, cursorEnabledSt.Render("["+label+"]"))
            } else {
                provOpts = append(provOpts, cursorDisabledSt.Render("["+label+"]"))
            }
        } else if enabled {
            provOpts = append(provOpts, enabledSt.Render(label))
        } else {
            provOpts = append(provOpts, disabledSt.Render(label))
        }
    }

    inner := strings.Join([]string{
        titleSt.Render(" CommuteLive Load Test "),
        "",
        "  Server:   " + serverSt.Render(m.serverURL),
        "",
        devLbl + strings.Join(devOpts, " "),
        "",
        durLbl + strings.Join(durOpts, " "),
        "",
        provLbl + " " + strings.Join(provOpts, " "),
        "",
        hintSt.Render("  ↑↓/Tab  switch row     ←→  select/move     Space/Enter  toggle     q  quit"),
    }, "\n")

    box := boxSt.Render(inner)

    return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
