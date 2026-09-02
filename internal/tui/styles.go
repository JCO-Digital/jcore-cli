package tui

import "github.com/charmbracelet/lipgloss"

var (
	styleTitle    = lipgloss.NewStyle().Bold(true).Padding(0, 1)
	styleHeader   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("245")).PaddingLeft(1)
	styleSelected = lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57"))
	styleStatus   = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Padding(0, 1)
	styleError    = lipgloss.NewStyle().Foreground(lipgloss.Color("204")).Padding(0, 1)
	styleHelp     = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Padding(0, 1)
	styleDesc     = lipgloss.NewStyle().Foreground(lipgloss.Color("248")).Padding(0, 1)

	styleBadgeDefault = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	styleBadgeGlobal  = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	styleBadgeProject = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	styleBadgeLocal   = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
)
