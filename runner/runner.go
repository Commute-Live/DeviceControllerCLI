package runner

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/commute-live/loadtest/device"
	"github.com/commute-live/loadtest/providers"
)

// Config holds all runtime parameters for the load test.
type Config struct {
	ServerURL    string
	SecretKey    string
	MQTTHost     string
	MQTTPort     int
	MQTTUsername string
	MQTTPassword string
	Devices      int
	Providers    map[string]int
	Duration     time.Duration
}

// Stats holds aggregate counters shared with the TUI.
type Stats struct {
	TotalDevices  int
	ActiveDevices atomic.Int64
	ErrorCount    atomic.Int64
	MQTTTotal     atomic.Int64
	StartedAt     time.Time
	mqttWindow    [5]int64
	windowIdx     int
	windowMu      sync.Mutex
}

// MsgsPerSec returns the rolling 5-second average of MQTT msgs/sec.
func (s *Stats) MsgsPerSec() float64 {
	s.windowMu.Lock()
	defer s.windowMu.Unlock()
	var sum int64
	for _, v := range s.mqttWindow {
		sum += v
	}
	return float64(sum) / 5.0
}

// TickMsg is sent to the TUI every second for stats refresh.
type TickMsg time.Time

// Runner orchestrates N mock devices.
type Runner struct {
	Cfg     Config
	Devices []*device.MockDevice
	Stats   *Stats
	EventCh chan device.Event
	StopCh  chan struct{}
}

// New creates a Runner and initialises all mock devices.
func New(cfg Config) (*Runner, error) {
	providerAssignments := providers.AssignProviders(cfg.Devices, cfg.Providers)

	r := &Runner{
		Cfg:     cfg,
		EventCh: make(chan device.Event, cfg.Devices*4),
		StopCh:  make(chan struct{}),
		Stats: &Stats{
			TotalDevices: cfg.Devices,
			StartedAt:    time.Now(),
		},
	}

	for i := 0; i < cfg.Devices; i++ {
		provKey := providerAssignments[i]
		stop, ok := providers.PickStop(provKey)
		if !ok {
			return nil, fmt.Errorf("no stops configured for provider %q", provKey)
		}
		d := device.New(
			cfg.ServerURL,
			cfg.SecretKey,
			cfg.MQTTHost,
			cfg.MQTTUsername,
			cfg.MQTTPassword,
			cfg.MQTTPort,
			stop,
		)
		r.Devices = append(r.Devices, d)
	}
	return r, nil
}

// Start launches all device goroutines and the event processor. It does NOT block.
// The caller must call Wait or Shutdown.
func (r *Runner) Start(wg *sync.WaitGroup, program *tea.Program) {
	go r.processEvents(program)

	for _, d := range r.Devices {
		wg.Add(1)
		go func(d *device.MockDevice) {
			defer wg.Done()
			d.Run(r.EventCh)
		}(d)
	}
}

// Shutdown signals all devices to stop. Safe to call multiple times.
func (r *Runner) Shutdown() {
	select {
	case <-r.StopCh:
	default:
		close(r.StopCh)
	}
	for _, d := range r.Devices {
		d.Shutdown()
	}
}

// WatchSignals watches OS signals and the optional duration timer.
// When triggered it calls Shutdown and then quits the TUI program.
func (r *Runner) WatchSignals(program *tea.Program, duration time.Duration) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	var durationCh <-chan time.Time
	if duration > 0 {
		durationCh = time.After(duration)
	}

	go func() {
		select {
		case <-sigCh:
		case <-durationCh:
		case <-r.StopCh:
			return
		}
		r.Shutdown()
		if program != nil {
			program.Quit()
		}
	}()
}

// PrintCleanupSQL prints a SQL snippet to clean up loadtest records.
func (r *Runner) PrintCleanupSQL() {
	fmt.Println("\n--- Cleanup SQL ---")
	fmt.Println("-- Run this on staging DB to remove all loadtest records:")
	fmt.Printf("DELETE FROM users WHERE email LIKE 'loadtest-%%';\nDELETE FROM devices WHERE id LIKE 'loadtest-%%';")
	emails := make([]string, 0, len(r.Devices))
	for _, d := range r.Devices {
		emails = append(emails, d.Email)
	}
	fmt.Printf("-- Emails created:\n--   %s\n", strings.Join(emails, "\n--   "))
	fmt.Printf("-- Device IDs created:\n")
	for _, d := range r.Devices {
		fmt.Printf("--   %s\n", d.DeviceID)
	}
}

func (r *Runner) processEvents(program *tea.Program) {
	mqttTicker := time.NewTicker(1 * time.Second)
	defer mqttTicker.Stop()
	var lastTotal int64

	for {
		select {
		case ev, ok := <-r.EventCh:
			if !ok {
				return
			}
			switch ev.Type {
			case device.EventActive:
				r.Stats.ActiveDevices.Add(1)
			case device.EventError:
				r.Stats.ErrorCount.Add(1)
			case device.EventDone:
				active := r.Stats.ActiveDevices.Load()
				if active > 0 {
					r.Stats.ActiveDevices.Add(-1)
				}
			}
			if program != nil {
				program.Send(ev)
			}
		case <-mqttTicker.C:
			var total int64
			for _, d := range r.Devices {
				total += int64(d.GetMQTTCount())
			}
			r.Stats.MQTTTotal.Store(total)
			delta := total - lastTotal
			lastTotal = total
			r.Stats.windowMu.Lock()
			r.Stats.mqttWindow[r.Stats.windowIdx%5] = delta
			r.Stats.windowIdx++
			r.Stats.windowMu.Unlock()
			if program != nil {
				program.Send(TickMsg(time.Now()))
			}
		case <-r.StopCh:
			return
		}
	}
}
