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

	if m.execPhase != phaseIdle && len(m.messages) > 0 && m.messages[len(m.messages)-1].role == "assistant" && m.messages[len(m.messages)-1].content == "" {
		b.WriteString(AssistantTagStyle.Render("● Rose"))
		b.WriteString("\n")
		b.WriteString(m.spinner.View())
		b.WriteString("\n")
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

	b.WriteString(HeaderStyle.Render(fmt.Sprintf("Rose Help  ·  %s", m.workspace.ProjectName)))
	b.WriteString("\n\n")

	if m.workspace.RoseRoot != "" && !m.workspace.IsInRoseProject() {
		b.WriteString(SystemTagStyle.Render(fmt.Sprintf("✦ Running from %s  ·  Source at %s",
			m.workspace.ProjectName, m.workspace.RoseRoot)))
		b.WriteString("\n\n")
	}

	b.WriteString(HeaderSubStyle.Render("Commands"))
	b.WriteString("\n")

	type helpEntry struct {
		key  string
		desc string
	}
	helpItems := []helpEntry{
		{"enter", "Send message"},
		{"@path", "Reference files/folders in prompt"},
		{"ctrl+e", "Toggle code/chat mode"},
		{"ctrl+t", "Select model"},
		{"ctrl+s", "Self-improve (reflect on own code)"},
		{"ctrl+u", "Update Rose from git + rebuild"},
		{"ctrl+l", "Clear conversation"},
		{"ctrl+h", "Show this help"},
		{"tab", "Execute raw code in input"},
		{"esc", "Cancel/interrupt"},
		{"ctrl+c", "Quit"},
	}

	b.WriteString("\n")
	b.WriteString(HeaderSubStyle.Render("Auto-Fix System"))
	b.WriteString("\n")
	b.WriteString("  When code execution fails, Rose automatically:\n")
	b.WriteString("  1. Reads the error output\n")
	b.WriteString("  2. Fixes the code\n")
	b.WriteString("  3. Re-executes (up to 5 attempts)\n")
	b.WriteString("  4. Stores the fix pattern for future learning\n")

	for _, h := range helpItems {
		b.WriteString(fmt.Sprintf("  %-12s %s\n",
			lipgloss.NewStyle().Foreground(highlight).Render(h.key),
			h.desc,
		))
	}

	b.WriteString("\n")
	b.WriteString(HeaderSubStyle.Render("Workspace"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  Dir:     %s\n", m.workspace.CurrentDir))
	b.WriteString(fmt.Sprintf("  Project: %s\n", m.workspace.ProjectName))
	if m.workspace.IsGitRepo {
		b.WriteString(fmt.Sprintf("  Git:     %s\n", m.workspace.GitRoot))
	}
	if len(m.workspace.Languages) > 0 {
		b.WriteString(fmt.Sprintf("  Langs:   %s\n", strings.Join(m.workspace.Languages, ", ")))
	}
	if m.workspace.RoseRoot != "" {
		b.WriteString(fmt.Sprintf("  Rose:    %s\n", m.workspace.RoseRoot))
	}

	if m.store != nil {
		total, success, projects, langs, _ := m.store.GetStats()
		b.WriteString("\n")
		b.WriteString(HeaderSubStyle.Render("Learning Stats"))
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("  Experiences: %d total, %d successes\n", total, success))
		b.WriteString(fmt.Sprintf("  Projects:    %d\n", projects))
		if len(langs) > 0 {
			var langList []string
			for l, c := range langs {
				langList = append(langList, fmt.Sprintf("%s(%d)", l, c))
			}
			b.WriteString(fmt.Sprintf("  Languages:   %s\n", strings.Join(langList, ", ")))
		}
	}

	b.WriteString("\n")
	b.WriteString(HelpStyle.Render("Press q or esc to return"))

	return b.String()
}

func (m model) renderPermission() string {
	var b strings.Builder

	b.WriteString(WarnStyle.Render("🔒 Permission Required"))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("Rose wants to access:\n  %s\n\n", m.permRef.Resolved))
	b.WriteString("This path is outside the current project boundary.\n\n")
	b.WriteString(HighlightStyle.Render("(y)") + " Allow once\n")
	b.WriteString(HighlightStyle.Render("(a)") + " Always allow this session\n")
	b.WriteString(HighlightStyle.Render("(n)") + " Deny\n")

	return b.String()
}

func (m model) renderMain() string {
	header := lipgloss.JoinHorizontal(
		lipgloss.Top,
		HeaderStyle.Render("Rose"),
		HeaderSubStyle.Render("  ·  "),
	)

	modeTag := "chat"
	if m.mode == modeCode {
		modeTag = "code"
	} else if m.mode == modeSelfReflect {
		modeTag = "self"
	}

	selfAware := ""
	if m.workspace.RoseRoot != "" && !m.workspace.IsInRoseProject() {
		selfAware = " ✦"
	}

	statusLine := fmt.Sprintf(" %s%s | %s",
		modeTag,
		selfAware,
		m.status,
	)

	viewportContent := m.viewport.View()
	if viewportContent == "" {
		viewportContent = "Welcome to Rose.\n\n" +
			"  ctrl+t  select model\n" +
			"  ctrl+e  code mode\n" +
			"  ctrl+s  self-improve (analyze own source)\n" +
			"  ctrl+u  update Rose from git\n" +
			"  ctrl+h  help\n" +
			"  ctrl+c  quit\n"
		if m.workspace.RoseRoot != "" && !m.workspace.IsInRoseProject() {
			viewportContent += fmt.Sprintf("\n✦ Self-aware: running from %s\n   Source at %s\n",
				m.workspace.ProjectName, m.workspace.RoseRoot)
		}
	}

	phaseIndicator := ""
	switch m.execPhase {
	case phaseWaitingLLM:
		phaseIndicator = m.spinner.View() + " thinking..."
	case phaseExecuting:
		phaseIndicator = m.spinner.View() + " executing..."
	case phaseFixing:
		phaseIndicator = m.spinner.View() + fmt.Sprintf(" fixing (attempt %d/5)...", m.fixAttempt)
	}

	content := lipgloss.JoinVertical(
		lipgloss.Top,
		header,
		StatusBarStyle.Render(statusLine),
		viewportContent,
	)

	if phaseIndicator != "" {
		content = lipgloss.JoinVertical(lipgloss.Top, content,
			lipgloss.NewStyle().Foreground(warnCol).Render(phaseIndicator))
	}

	content = lipgloss.JoinVertical(lipgloss.Top, content, m.input.View())

	vpHeight := m.height - 14 - lipgloss.Height(header)
	if vpHeight < 5 {
		vpHeight = 5
	}
	m.viewport.Height = vpHeight

	return content
}
