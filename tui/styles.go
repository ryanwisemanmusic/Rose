package tui

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	subtle    = lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#383838"}
	highlight = lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7B56DB"}
	special   = lipgloss.AdaptiveColor{Light: "#43BF6D", Dark: "#73F59F"}
	errorCol  = lipgloss.AdaptiveColor{Light: "#FF0000", Dark: "#FF5555"}
	warnCol   = lipgloss.AdaptiveColor{Light: "#FFA500", Dark: "#FFA500"}
	refPink   = lipgloss.AdaptiveColor{Light: "#D46BA8", Dark: "#F2A7D3"}
	codeBg    = lipgloss.AdaptiveColor{Light: "#F0F0F0", Dark: "#1E1E1E"}
)

var (
	AppStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(highlight).
			Padding(1, 2)

	HeaderStyle = lipgloss.NewStyle().
			Foreground(highlight).
			Bold(true).
			MarginBottom(1)

	HeaderSubStyle = lipgloss.NewStyle().
			Foreground(subtle).
			MarginBottom(1)

	MessageStyle = lipgloss.NewStyle().
			Padding(0, 1)

	UserTagStyle = lipgloss.NewStyle().
			Foreground(special).
			Bold(true)

	AssistantTagStyle = lipgloss.NewStyle().
				Foreground(highlight).
				Bold(true)

	SystemTagStyle = lipgloss.NewStyle().
			Foreground(warnCol).
			Bold(true)

	CodeBlockStyle = lipgloss.NewStyle().
			Background(codeBg).
			Padding(0, 2).
			Width(80)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(errorCol)

	SuccessStyle = lipgloss.NewStyle().
			Foreground(special)

	InputStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(subtle).
			Padding(0, 1)

	StatusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#343433", Dark: "#C1C6B2"}).
			Background(lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#353533"}).
			Padding(0, 1).
			Width(80)

	StatusTextStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#FFFDF5", Dark: "#FFFDF5"}).
			Background(lipgloss.AdaptiveColor{Light: "#FF5F87", Dark: "#FF5F87"}).
			Padding(0, 1).
			MarginRight(1)

	HelpStyle = lipgloss.NewStyle().
			Foreground(subtle)

	WarnStyle = lipgloss.NewStyle().
			Foreground(warnCol).
			Bold(true)

	HighlightStyle = lipgloss.NewStyle().
			Foreground(highlight).
			Bold(true)

	ReferenceStyle = lipgloss.NewStyle().
			Foreground(refPink).
			Bold(true)

	ReferencePickerStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(refPink).
				Padding(0, 1)

	ReferencePickerItemStyle = lipgloss.NewStyle().
					Foreground(subtle)

	ReferencePickerSelectedStyle = lipgloss.NewStyle().
					Foreground(refPink).
					Bold(true)
)
