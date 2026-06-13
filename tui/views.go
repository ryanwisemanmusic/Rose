package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m model) renderMessages() string {
	var b strings.Builder

	for _, msg := range m.messages {
		switch msg.role {
		case "user":
			b.WriteString(UserTagStyle.Render("▶ You"))
			b.WriteString("\n")
			b.WriteString(MessageStyle.Render(msg.content))
		case "assistant":
			b.WriteString(AssistantTagStyle.Render("● Rose"))
			b.WriteString("\n")
			b.WriteString(MessageStyle.Render(renderContent(msg.content)))
		case "system":
			b.WriteString(SystemTagStyle.Render("◆ System"))
			b.WriteString("\n")
			b.WriteString(MessageStyle.Render(msg.content))
		}
		b.WriteString("\n\n")
	}

	if m.streaming && m.partial != "" {
		b.WriteString(AssistantTagStyle.Render("● Rose"))
		b.WriteString("\n")
		b.WriteString(MessageStyle.Render(renderContent(m.partial)))
		b.WriteString("\n\n")
		b.WriteString(m.spinner.View())
	}

	return b.String()
}

func renderContent(content string) string {
	var result strings.Builder
	lines := strings.Split(content, "\n")
	inCode := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			if inCode {
				inCode = false
				result.WriteString("\n")
				continue
			}
			inCode = true
			lang := strings.TrimPrefix(trimmed, "```")
			lang = strings.TrimSpace(lang)
			if lang != "" && !strings.Contains(lang, " ") {
				result.WriteString(fmt.Sprintf("[code: %s]\n", lang))
			} else {
				result.WriteString("[code]\n")
			}
			continue
		}

		if inCode {
			result.WriteString("  " + line + "\n")
		} else {
			result.WriteString(line + "\n")
		}
	}

	return result.String()
}

func (m model) renderModelList() string {
	var b strings.Builder

	b.WriteString(HeaderStyle.Render("Select Model"))
	b.WriteString("\n\n")

	for i, model := range m.availModels {
		cursor := "  "
		if i == m.cursor {
			cursor = "▸ "
		}

		line := fmt.Sprintf("%s%s", cursor, model.Name)
		if i == m.cursor {
			line = lipgloss.NewStyle().Foreground(highlight).Render(line)
		} else {
			line = lipgloss.NewStyle().Foreground(subtle).Render(line)
		}
		b.WriteString(line)

		if i == m.cursor {
			b.WriteString(fmt.Sprintf("  %s", model.Description))
			b.WriteString(fmt.Sprintf("  [%s]", model.Size))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(HelpStyle.Render("↑/↓ navigate · enter select · esc cancel"))

	return b.String()
}

func (m model) renderHelp() string {
	var b strings.Builder

	b.WriteString(HeaderStyle.Render("Rose Help"))
	b.WriteString("\n\n")

	helpItems := []struct {
		key  string
		desc string
	}{
		{"enter", "Send message"},
		{"ctrl+e", "Toggle code/chat mode"},
		{"ctrl+t", "Select model"},
		{"ctrl+l", "Clear conversation"},
		{"ctrl+h", "Show this help"},
		{"tab", "Execute code in input"},
		{"esc", "Cancel/interrupt"},
		{"ctrl+c", "Quit"},
	}

	for _, h := range helpItems {
		b.WriteString(fmt.Sprintf("  %-12s %s\n",
			lipgloss.NewStyle().Foreground(highlight).Render(h.key),
			h.desc,
		))
	}

	b.WriteString("\n")
	b.WriteString(HelpStyle.Render("Press q or esc to return"))

	return b.String()
}

func (m model) renderMain() string {
	header := lipgloss.JoinHorizontal(
		lipgloss.Top,
		HeaderStyle.Render("Rose"),
		HeaderSubStyle.Render("  ·  coding assistant"),
	)

	modeTag := "chat"
	if m.mode == modeCode {
		modeTag = "code"
	}

	statusLine := fmt.Sprintf(" %s | %s | %s",
		modeTag,
		m.status,
		HelpStyle.Render("ctrl+h help"),
	)

	viewportContent := m.viewport.View()
	if viewportContent == "" {
		viewportContent = "Welcome to Rose. Start by typing a message below.\n\nYou can ask questions, write code, or run commands.\nPress ctrl+h for help."
	}

	content := lipgloss.JoinVertical(
		lipgloss.Top,
		header,
		StatusBarStyle.Render(statusLine),
		viewportContent,
		m.input.View(),
	)

	vpHeight := m.height - 12 - lipgloss.Height(header) - lipgloss.Height(statusLine)
	if vpHeight < 5 {
		vpHeight = 5
	}
	m.viewport.Height = vpHeight

	return content
}
