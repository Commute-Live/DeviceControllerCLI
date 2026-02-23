package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/commute-live/loadtest/runner"
	"github.com/commute-live/loadtest/tui"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "loadtest",
	Short: "CommuteLive load test CLI — simulates N mock ESP32 devices",
	RunE:  runLoadTest,
}

var (
	flagDevices   int
	flagProviders string
	flagDuration  string
	flagForce     bool
	flagNoMenu    bool
)

func init() {
	rootCmd.Flags().IntVar(&flagDevices, "devices", 5, "Default number of devices shown in setup menu")
	rootCmd.Flags().StringVar(&flagProviders, "providers", "cta=25,mta=25,mbta=25,septa=25", `Provider distribution, e.g. "cta=30,mta=40,mbta=20,septa=10" (must sum to 100)`)
	rootCmd.Flags().StringVar(&flagDuration, "duration", "", `Default duration shown in setup menu, e.g. "5m" (default: unlimited)`)
	rootCmd.Flags().BoolVar(&flagForce, "force", false, "Skip staging URL safety check")
	rootCmd.Flags().BoolVar(&flagNoMenu, "no-menu", false, "Skip interactive setup menu and use flags directly")
}

// Execute is the entry point called from main.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runLoadTest(cmd *cobra.Command, args []string) error {
	serverURL := os.Getenv("LOADTEST_SERVER_URL")
	if serverURL == "" {
		return fmt.Errorf("LOADTEST_SERVER_URL is required but not set")
	}

	secretKey := os.Getenv("LOADTEST_SECRET_KEY")
	if secretKey == "" {
		return fmt.Errorf("LOADTEST_SECRET_KEY is required but not set")
	}

	if !strings.Contains(serverURL, "staging") && !flagForce {
		fmt.Fprintf(os.Stderr, "WARNING: LOADTEST_SERVER_URL (%q) does not contain \"staging\".\n", serverURL)
		fmt.Fprintln(os.Stderr, "This will run against a non-staging server. Use --force to proceed anyway.")
		os.Exit(1)
	}

	mqttHost := os.Getenv("LOADTEST_MQTT_HOST")
	if mqttHost == "" {
		return fmt.Errorf("LOADTEST_MQTT_HOST is required but not set")
	}

	mqttPort := 1883
	if v := os.Getenv("LOADTEST_MQTT_PORT"); v != "" {
		p, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("invalid LOADTEST_MQTT_PORT: %w", err)
		}
		mqttPort = p
	}

	mqttUsername := "commutelive"
	if v := os.Getenv("LOADTEST_MQTT_USERNAME"); v != "" {
		mqttUsername = v
	}

	mqttPassword := "commutelive"
	if v := os.Getenv("LOADTEST_MQTT_PASSWORD"); v != "" {
		mqttPassword = v
	}

	providerDist, err := parseProviderDist(flagProviders)
	if err != nil {
		return err
	}

	// ── Step 1: Interactive setup menu ──────────────────────────────────────
	devices := flagDevices
	durationStr := flagDuration

	if !flagNoMenu {
		setupModel := tui.NewSetupModel(serverURL, flagDevices, flagDuration)
		setupP := tea.NewProgram(setupModel, tea.WithAltScreen())
		finalModel, err := setupP.Run()
		if err != nil {
			return fmt.Errorf("setup menu error: %w", err)
		}
		result := finalModel.(*tui.SetupModel).Result
		if !result.Start {
			fmt.Println("Cancelled.")
			return nil
		}
		devices = result.Devices
		durationStr = result.Duration
		providerDist = result.Providers
	}

	var duration time.Duration
	if durationStr != "" {
		duration, err = time.ParseDuration(durationStr)
		if err != nil {
			return fmt.Errorf("invalid duration %q: %w", durationStr, err)
		}
	}

	// ── Step 2: Build runner and start load test ─────────────────────────────
	cfg := runner.Config{
		ServerURL:    serverURL,
		SecretKey:    secretKey,
		MQTTHost:     mqttHost,
		MQTTPort:     mqttPort,
		MQTTUsername: mqttUsername,
		MQTTPassword: mqttPassword,
		Devices:      devices,
		Providers:    providerDist,
		Duration:     duration,
	}

	r, err := runner.New(cfg)
	if err != nil {
		return err
	}

	model := tui.NewModel(r.Devices, r.Stats)
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())

	// Device goroutines run independently; we wait for them only after shutdown.
	var deviceWg sync.WaitGroup
	r.Start(&deviceWg, p)
	r.WatchSignals(p, duration)

	// Block here until the user quits (q / Ctrl+C) or duration fires.
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "TUI error:", err)
	}

	// TUI has exited — stop all devices and wait for them to finish.
	r.Shutdown()
	deviceWg.Wait()
	r.PrintCleanupSQL()
	return nil
}

func parseProviderDist(s string) (map[string]int, error) {
	result := make(map[string]int)
	total := 0
	for _, part := range strings.Split(s, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("invalid provider distribution entry %q (expected key=value)", part)
		}
		pct, err := strconv.Atoi(strings.TrimSpace(kv[1]))
		if err != nil {
			return nil, fmt.Errorf("invalid percentage for provider %q: %w", kv[0], err)
		}
		result[strings.TrimSpace(kv[0])] = pct
		total += pct
	}
	if total != 100 {
		return nil, fmt.Errorf("provider percentages must sum to 100, got %d", total)
	}
	return result, nil
}
