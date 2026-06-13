package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m model) renderMessages() string {
	var b strings.Builder
	messageStyle := m.messageStyle()

	for _, msg := range m.messages {
		switch msg.role {
		case "user":
			b.WriteString(UserTagStyle.Render("▶ You"))
			b.WriteString("\n")
			b.WriteString(messageStyle.Render(colorReferenceTokens(msg.content)))
		case "assistant":
			if strings.TrimSpace(msg.content) == "" {
				continue
			}
			b.WriteString(AssistantTagStyle.Render("● Rose"))
			b.WriteString("\n")
			b.WriteString(messageStyle.Render(renderContent(msg.content)))
		case "system":
			b.WriteString(SystemTagStyle.Render("◆ System"))
			b.WriteString("\n")
			b.WriteString(messageStyle.Render(msg.content))
		}
		b.WriteString("\n\n")
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
		{"cmd+c/v", "Copy selected terminal text / paste"},
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
	m.resizeComponents()
	innerWidth := m.innerWidth()
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

	displayStatus := m.status
	if phase := m.phaseStatus(); phase != "" {
		displayStatus = fmt.Sprintf("model: %s | %s | %s", m.config.ActiveModel, phase, m.workspace.Summary())
	}

	statusLine := fmt.Sprintf(" %s%s | %s",
		modeTag,
		selfAware,
		displayStatus,
	)
	statusWidth := max(1, innerWidth-StatusBarStyle.GetHorizontalFrameSize())
	statusLine = fitLine(statusLine, statusWidth)

	vpHeight := m.availableViewportHeight(lipgloss.Height(header))
	if vpHeight < 5 {
		vpHeight = 5
	}
	m.viewport.Height = vpHeight

	viewportContent := m.viewport.View()
	if viewportContent == "" {
		viewportContent = "Welcome to Rose.\n\n" +
			"  ctrl+t  select model\n" +
			"  ctrl+e  code mode\n" +
			"  ctrl+s  self-improve (analyze own source)\n" +
			"  ctrl+u  update Rose from git\n" +
			"  ctrl+h  help\n" +
			"  cmd+c  copy selected terminal text\n" +
			"  cmd+v  paste\n" +
			"  ctrl+c  quit\n"
		if m.workspace.RoseRoot != "" && !m.workspace.IsInRoseProject() {
			viewportContent += fmt.Sprintf("\n✦ Self-aware: running from %s\n   Source at %s\n",
				m.workspace.ProjectName, m.workspace.RoseRoot)
		}
	}

	content := lipgloss.JoinVertical(
		lipgloss.Top,
		header,
		styleForTotalWidth(StatusBarStyle, innerWidth).Render(statusLine),
		viewportContent,
	)

	content = lipgloss.JoinVertical(lipgloss.Top, content, m.renderComposer())

	return content
}

func (m *model) resizeComponents() {
	innerWidth := m.innerWidth()
	m.viewport.Width = innerWidth

	inputWidth := innerWidth - InputStyle.GetHorizontalFrameSize()
	if inputWidth < 1 {
		inputWidth = 1
	}
	m.input.SetWidth(inputWidth)

	inputHeight := 3
	if m.height > 36 {
		inputHeight = 4
	}
	if m.height > 50 {
		inputHeight = 5
	}
	m.input.SetHeight(inputHeight)

	vpHeight := m.availableViewportHeight(2)
	if vpHeight < 5 {
		vpHeight = 5
	}
	m.viewport.Height = vpHeight
}

func (m model) innerWidth() int {
	width := m.width
	if width <= 0 {
		width = 86
	}
	inner := width - AppStyle.GetHorizontalFrameSize()
	if inner < 24 {
		inner = 24
	}
	return inner
}

func (m model) renderComposer() string {
	picker := m.renderReferencePicker()
	input := m.renderInput()
	if picker == "" {
		return input
	}
	return lipgloss.JoinVertical(lipgloss.Top, picker, input)
}

func (m model) renderInput() string {
	return styleForTotalWidth(InputStyle, m.innerWidth()).Render(m.input.View())
}

func (m model) availableViewportHeight(headerHeight int) int {
	height := m.height
	if height <= 0 {
		height = 28
	}
	innerHeight := height - AppStyle.GetVerticalFrameSize()
	statusHeight := 1
	gap := 1
	return innerHeight - headerHeight - statusHeight - m.composerHeight() - gap
}

func (m model) composerHeight() int {
	height := m.input.Height() + InputStyle.GetVerticalFrameSize()
	if picker := m.renderReferencePicker(); picker != "" {
		height += lipgloss.Height(picker)
	}
	return height
}

func (m model) messageStyle() lipgloss.Style {
	width := m.viewport.Width
	if width <= 0 {
		width = m.innerWidth()
	}
	width -= m.viewport.Style.GetHorizontalFrameSize()
	if width < 1 {
		width = 1
	}
	return styleForTotalWidth(MessageStyle, width)
}

func styleForTotalWidth(style lipgloss.Style, totalWidth int) lipgloss.Style {
	contentWidth := totalWidth - style.GetHorizontalFrameSize()
	if contentWidth < 1 {
		contentWidth = 1
	}
	return style.Width(contentWidth).MaxWidth(contentWidth)
}

func fitLine(s string, width int) string {
	if width <= 0 || lipgloss.Width(s) <= width {
		return s
	}
	if width <= 3 {
		runes := []rune(s)
		if len(runes) <= width {
			return s
		}
		return string(runes[:width])
	}
	suffix := "..."
	var b strings.Builder
	for _, r := range s {
		next := b.String() + string(r)
		if lipgloss.Width(next)+len(suffix) > width {
			break
		}
		b.WriteRune(r)
	}
	return b.String() + suffix
}

func (m model) phaseStatus() string {
	switch m.execPhase {
	case phaseWaitingLLM:
		return m.spinner.View() + " thinking..."
	case phaseExecuting:
		return m.spinner.View() + " working..."
	case phaseFixing:
		return m.spinner.View() + fmt.Sprintf(" fixing (attempt %d/5)...", m.fixAttempt)
	default:
		return ""
	}
}
