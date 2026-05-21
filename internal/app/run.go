package app

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/svg153/engram-monitor-tui/internal/api"
)

func Run(client api.Service) error {
	program := tea.NewProgram(New(client), tea.WithAltScreen())
	_, err := program.Run()
	return err
}
