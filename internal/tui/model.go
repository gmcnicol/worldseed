package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gmcnicol/worldseed/internal/sim"
	"github.com/gmcnicol/worldseed/internal/storage"
)

type Model struct {
	store   *storage.Store
	started time.Time
	state   storage.DashboardState
	err     error
	width   int
	height  int
}

type stateMsg storage.DashboardState
type errMsg error
type interventionMsg struct{}

func New(store *storage.Store, started time.Time) Model {
	return Model{store: store, started: started}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.loadState(), tick())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "p":
			return m, m.preserveArchive()
		case "r":
			return m, m.loadState()
		}
	case stateMsg:
		m.err = nil
		m.state = storage.DashboardState(msg)
		return m, nil
	case errMsg:
		m.err = msg
		return m, nil
	case interventionMsg:
		return m, m.loadState()
	case time.Time:
		return m, tea.Batch(m.loadState(), tick())
	}
	return m, nil
}

func (m Model) View() string {
	if m.state.Universe.ID == "" && m.err == nil {
		return "\n  opening archive stream...\n"
	}
	width := m.width
	if width < 72 {
		width = 72
	}
	panelWidth := width - 4
	if panelWidth > 96 {
		panelWidth = 96
	}

	title := titleStyle.Render("WORLDSEED ARCHIVE NODE")
	u := m.state.Universe
	header := fmt.Sprintf("%s\n%s", title, mutedStyle.Render("local ssh observatory / autonomous shard"))
	if m.err != nil {
		header += "\n" + dangerStyle.Render("diagnostic: "+m.err.Error())
	}

	body := strings.Builder{}
	body.WriteString("\n")
	body.WriteString(label("universe") + value(u.Name) + "\n")
	body.WriteString(label("uptime") + value(m.state.Uptime.Round(time.Second).String()) + "\n")
	body.WriteString(label("age") + value(fmt.Sprintf("%d archive cycles", u.Age)) + "\n")
	body.WriteString(label("entropy") + bar(u.Entropy, panelWidth-18) + fmt.Sprintf(" %.3f\n", u.Entropy))
	body.WriteString(label("archive") + bar(u.ArchiveIntegrity, panelWidth-18) + fmt.Sprintf(" %.3f\n", u.ArchiveIntegrity))
	body.WriteString(label("signals") + value(fmt.Sprintf("%d pending / quiet lattice", m.state.SignalCount)) + "\n")
	body.WriteString("\n")
	body.WriteString(sectionStyle.Render("ACTIVE CIVILISATIONS") + "\n")
	if len(m.state.ActiveCivilisations) == 0 {
		body.WriteString(mutedStyle.Render("  no active civilisations remain in projection") + "\n")
	}
	for _, entity := range m.state.ActiveCivilisations {
		state, err := storage.DecodeCivilisationState(entity.State)
		if err != nil {
			continue
		}
		body.WriteString(fmt.Sprintf("  %s %s\n", entity.Name, mutedStyle.Render(fmt.Sprintf("stability %.2f / doctrine %.2f", state.Stability, state.Doctrine))))
	}
	body.WriteString("\n")
	body.WriteString(sectionStyle.Render("RECENT TIMELINE") + "\n")
	if len(m.state.RecentEvents) == 0 {
		body.WriteString(mutedStyle.Render("  no timeline entries recorded") + "\n")
	}
	for _, event := range m.state.RecentEvents {
		body.WriteString(fmt.Sprintf("  %s %s\n", mutedStyle.Render(fmt.Sprintf("t+%04d", event.ValidTime)), event.Summary))
	}
	body.WriteString("\n")
	body.WriteString(mutedStyle.Render("[p] preserve archive  [r] refresh  [q] disconnect"))

	return panelStyle.Width(panelWidth).Render(header + body.String())
}

func (m Model) loadState() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		u, err := m.store.LoadUniverse(ctx)
		if err != nil {
			return errMsg(err)
		}
		active, err := m.store.ActiveCivilisations(ctx, u.ID)
		if err != nil {
			return errMsg(err)
		}
		events, err := m.store.RecentEvents(ctx, u.ID, 8)
		if err != nil {
			return errMsg(err)
		}
		signals, err := m.store.SignalCount(ctx, u.ID)
		if err != nil {
			return errMsg(err)
		}
		return stateMsg(storage.DashboardState{
			Universe:            u,
			ActiveCivilisations: active,
			RecentEvents:        events,
			SignalCount:         signals,
			Uptime:              time.Since(m.started),
		})
	}
}

func (m Model) preserveArchive() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := sim.RequestPreserveArchive(ctx, m.store); err != nil {
			return errMsg(err)
		}
		return interventionMsg{}
	}
}

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return t })
}

func label(s string) string {
	return mutedStyle.Width(12).Render(strings.ToUpper(s))
}

func value(s string) string {
	return archiveStyle.Render(s)
}

func bar(v float64, width int) string {
	if width < 8 {
		width = 8
	}
	filled := int(v * float64(width))
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}
	return archiveStyle.Render(strings.Repeat("#", filled)) + mutedStyle.Render(strings.Repeat("-", width-filled))
}

var (
	panelStyle = lipgloss.NewStyle().
			Padding(1, 2).
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("238"))
	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")).
			Bold(true)
	sectionStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("250")).
			Bold(true)
	mutedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244"))
	archiveStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("151"))
	dangerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("203"))
)
