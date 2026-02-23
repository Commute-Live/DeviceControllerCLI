package tui

import (
    "fmt"
    "strings"

    "github.com/charmbracelet/lipgloss"
    "github.com/commute-live/loadtest/device"
)

var (
    sectionStyle = lipgloss.NewStyle().
        Bold(true).
        Foreground(lipgloss.Color("99"))

    dimStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color("243"))

    httpOKStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color("42"))

    httpErrStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color("196"))

    mqttMsgStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color("220"))
)

func stateLabel(d *device.MockDevice) string {
    state := d.GetState()
    switch state {
    case device.StateActive:
        return activeStyle.Render("● ACTIVE")
    case device.StateError:
        return errorStyle.Render("✗ ERROR")
    case device.StateDone:
        return doneStyle.Render("○ DONE")
    default:
        return initStyle.Render("◌ " + state.String())
    }
}

// renderDetail renders the right panel showing details for the selected device.
func renderDetail(d *device.MockDevice, width, height int) string {
    if height < 1 {
        height = 1
    }
    if width < 12 {
        width = 12
    }
    if d == nil {
        return dimStyle.Render("  No device selected")
    }

    var lines []string

    // Header info
    lines = append(lines,
        sectionStyle.Render("Device: ")+d.DeviceID,
        fmt.Sprintf("Provider: %-14s Stop: %-16s Dir: %s",
            d.Stop.ProviderID, d.Stop.StopID, d.Stop.Direction),
        "Status: "+stateLabel(d),
    )

    // Error message if any
    errMsg := d.GetErrorMsg()
    if errMsg != "" {
        lines = append(lines, httpErrStyle.Render("  Error: "+errMsg))
    }

    // HTTP log section
    lines = append(lines, "")
    lines = append(lines, sectionStyle.Render("─── HTTP Log ───"))

    httpLog := d.GetHTTPLog()
    maxHTTP := 10
    httpStart := 0
    if len(httpLog) > maxHTTP {
        httpStart = len(httpLog) - maxHTTP
    }
    if len(httpLog) == 0 {
        lines = append(lines, dimStyle.Render("  (no requests yet)"))
    }
    for _, entry := range httpLog[httpStart:] {
        mark := httpOKStyle.Render("✓")
        if !entry.OK {
            mark = httpErrStyle.Render("✗")
        }
        line := fmt.Sprintf("%s  %-6s %-38s %3d %s",
            dimStyle.Render(entry.Timestamp.Format("15:04:05")),
            entry.Method,
            truncate(entry.Path, 38),
            entry.Status,
            mark,
        )
        lines = append(lines, line)
        if len(lines) >= height {
            return strings.Join(lines[:height], "\n")
        }
    }

    // MQTT messages section
    lines = append(lines, "")
    lines = append(lines, sectionStyle.Render("─── MQTT Messages ───"))

    mqttMsgs := d.GetMQTTMsgs()
    remaining := height - len(lines)
    if remaining < 1 {
        remaining = 1
    }
    mqttStart := 0
    if len(mqttMsgs) > remaining {
        mqttStart = len(mqttMsgs) - remaining
    }
    if len(mqttMsgs) == 0 {
        lines = append(lines, dimStyle.Render("  (waiting for MQTT messages...)"))
    }
    for _, msg := range mqttMsgs[mqttStart:] {
        payload := truncate(msg.Payload, width-12)
        line := fmt.Sprintf("%s  %s",
            dimStyle.Render(msg.Timestamp.Format("15:04:05")),
            mqttMsgStyle.Render(payload),
        )
        lines = append(lines, line)
        if len(lines) >= height {
            break
        }
    }

    // Trim to fit height (safe: height >= 1 guaranteed above)
    if len(lines) > height {
        lines = lines[:height]
    }

    return strings.Join(lines, "\n")
}

func truncate(s string, max int) string {
    if len(s) <= max {
        return s
    }
    if max < 3 {
        return s[:max]
    }
    return s[:max-3] + "..."
}
