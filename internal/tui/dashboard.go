package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type model struct {
	universe string
	status   string
}

func Run(universeName string) error {
	m := model{universe: universeName, status: "connected"}
	p := tea.NewProgram(m)
	_, err := p.Run()
	return err
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m model) View() string {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("69")).Render("Worldseed Archive Console")
	return fmt.Sprintf("%s\n\nUniverse: %s\nStatus: %s\n\nPress q to disconnect.\n", title, m.universe, m.status)
}
