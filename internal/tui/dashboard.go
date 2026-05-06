package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/worldseed/worldseed/internal/packets"
)

type snapshot struct {
	Name, EntropyProfile string
	UniverseAgeTicks     string
	ArchiveIntegrity     float64
	RecentEvents         []string
}
type tickMsg snapshot

type model struct {
	universe, rootDir string
	snap              snapshot
	started           time.Time
	err               error
}

func Run(rootDir, universeName string) error {
	p := tea.NewProgram(model{universe: universeName, rootDir: rootDir, started: time.Now()})
	_, err := p.Run()
	return err
}
func (m model) Init() tea.Cmd { return poll(m.rootDir, m.universe) }
func poll(root, u string) tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg {
		client := http.Client{Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			d := net.Dialer{}
			return d.DialContext(ctx, "unix", packets.SocketPath(root, u))
		}}}
		resp, err := client.Get("http://worldseed/snapshot")
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		var s snapshot
		_ = json.NewDecoder(resp.Body).Decode(&s)
		return tickMsg(s)
	})
}
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch x := msg.(type) {
	case error:
		m.err = x
		return m, poll(m.rootDir, m.universe)
	case tickMsg:
		m.snap = snapshot(x)
		return m, poll(m.rootDir, m.universe)
	case tea.KeyMsg:
		if x.String() == "q" || x.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if x.String() == "p" {
			go firePreserve(m.rootDir, m.universe)
		}
	}
	return m, nil
}
func (m model) View() string {
	title := lipgloss.NewStyle().Bold(true).Render("Worldseed Observatory")
	return fmt.Sprintf("%s\nUniverse: %s\nEntropy Profile: %s\nUptime: %s\nUniverse Age: %s ticks\nArchive Integrity: %.2f\nActive Civilisations: 1\nSignal: awaiting deep-band telemetry\n\nRecent timeline events:\n- %s\n\n[p] preserve archive   [q] disconnect\n", title, m.snap.Name, m.snap.EntropyProfile, time.Since(m.started).Round(time.Second), m.snap.UniverseAgeTicks, m.snap.ArchiveIntegrity, join(m.snap.RecentEvents))
}
func join(v []string) string {
	if len(v) == 0 {
		return "No significant events recorded."
	}
	return v[0]
}

func firePreserve(root, u string) {
	client := http.Client{Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		d := net.Dialer{}
		return d.DialContext(ctx, "unix", packets.SocketPath(root, u))
	}}}
	req, _ := http.NewRequest(http.MethodPost, "http://worldseed/interventions/preserve_archive", nil)
	_, _ = client.Do(req)
}
