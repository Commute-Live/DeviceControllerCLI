package tui

import (
    "fmt"
    "strings"

    "github.com/charmbracelet/lipgloss"
    "github.com/commute-live/loadtest/device"
)

var (
    selectedItemStyle = lipgloss.NewStyle().
        Bold(true).
        Foreground(lipgloss.Color("15")).
        Background(lipgloss.Color("57"))

    activeStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color("42"))

    errorStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color("196"))

    initStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color("243"))

    doneStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color("240"))
)

func stateIndicator(d *device.MockDevice) string {
    switch d.GetState() {
    case device.StateActive:
        return activeStyle.Render("*")
    case device.StateError:
        return errorStyle.Render("!")
    case device.StateDone:
        return doneStyle.Render("-")
    default:
        return initStyle.Render("~")
    }
}

func providerTag(d *device.MockDevice) string {
    p := strings.ToUpper(d.Stop.Provider)
    if len(p) > 3 {
        p = p[:3]
    }
    return p
}

// renderList renders the left panel content. Width is the usable column width.
func renderList(devices []*device.MockDevice, selected, width, height int) string {
    if width < 4 {
        width = 4
    }

    titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
    var rows []string
    rows = append(rows, titleStyle.Render("Devices"))
    rows = append(rows, strings.Repeat("─", width))

    maxItems := height - len(rows)
    if maxItems < 1 {
        maxItems = 1
    }

    // Scroll window
    start := 0
    if selected >= maxItems {
        start = selected - maxItems + 1
    }

    for i := start; i < len(devices) && i-start < maxItems; i++ {
        d := devices[i]
        indicator := stateIndicator(d)
        tag := providerTag(d)
        id := d.ShortID
        if len(id) > 8 {
            id = id[:8]
        }
        // "▶ * 1a2b3c4d [MTA]" — all ASCII-safe, no slicing ANSI strings
        prefix := "  "
        if i == selected {
            prefix = "> "
        }
        plain := fmt.Sprintf("%s%s %-8s[%3s]", prefix, indicator, id, tag)
        if i == selected {
            plain = selectedItemStyle.Render(plain)
        }
        rows = append(rows, plain)
    }

    if len(devices) == 0 {
        rows = append(rows, initStyle.Render("(no devices)"))
    }

    return strings.Join(rows, "\n")
}
