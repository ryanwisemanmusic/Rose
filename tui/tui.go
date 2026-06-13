package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/ryanwi/rose/config"
	"github.com/ryanwi/rose/llm"
	"github.com/ryanwi/rose/memory"
	"github.com/ryanwi/rose/sandbox"
)

type mode int

const (
	modeChat mode = iota
	modeCode
	modeSelectModel
	modeHelp
)

type message struct {
	role    string
	content string
}

type model struct {
	config     *config.Config
	llmClient  *llm.Client
	store      *memory.Store
	executor   *sandbox.Executor

	width      int
	height     int

	mode       mode

	messages   []message
	viewport   viewport.Model
	input      textinput.Model
	spinner    spinner.Model

	streaming  bool
	partial    string

	status     string
	err        error

	ollamaModels []llm.Model
	availModels  []llm.Model
	cursor       int

	conversation []llm.Message
}

func InitialModel(cfg *config.Config, store *memory.Store, executor *sandbox.Executor) model {
	ti := textinput.New()
	ti.Placeholder = "Ask Rose anything..."
	ti.Focus()
	ti.CharLimit = 2000
	ti.Width = 70

	vp := viewport.New(80, 20)
	vp.Style = lipgloss.NewStyle().Padding(0, 1)
	vp.KeyMap = viewport.DefaultKeyMap()

	s := spinner.New()
	s.Style = lipgloss.NewStyle().Foreground(highlight)
	s.Spinner = spinner.Dot

	return model{
		config:    cfg,
		llmClient: llm.NewClient(cfg.OllamaHost),
		store:     store,
		executor:  executor,
		viewport:  vp,
		input:     ti,
		spinner:   s,
		mode:      modeChat,
		status:    fmt.Sprintf("model: %s", cfg.ActiveModel),
		conversation: []llm.Message{
			{Role: "system", Content: defaultSystemPrompt(cfg.ActiveModel)},
		},
	}
}

func defaultSystemPrompt(modelName string) string {
	prompt := "You are Rose, an intelligent programming assistant inside a terminal.\n\n"
	prompt += "You help users write, debug, and understand code across any programming language.\n\n"
	prompt += "## Abilities\n"
	prompt += "- You can write code in any language\n"
	prompt += "- When asked to run code, you provide the code and the assistant will execute it\n"
	prompt += "- You learn from execution results and errors\n"
	prompt += "- You can explain complex concepts simply\n\n"
	prompt += "## Behavior\n"
	prompt += "- Be concise and direct\n"
	prompt += "- Provide code examples when helpful\n"
	prompt += "- Use markdown formatting\n"
	prompt += "- When writing code blocks, specify the language\n"
	prompt += "- If you're unsure, ask clarifying questions\n\n"
	prompt += "## Output Format\n"
	prompt += "- Use ```language for code blocks\n"
	prompt += "- Use **bold** for emphasis\n"
	prompt += "- Use bullet points for lists\n"

	if strings.Contains(modelName, "1b") || strings.Contains(modelName, "4b") {
		prompt += "\n## Constraint\nYou are running on a lightweight model. Keep responses very short and focused.\n"
	}

	return prompt
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.loadModels(),
	)
}

func (m model) loadModels() tea.Cmd {
	return func() tea.Msg {
		models, err := m.llmClient.ListModels()
		if err != nil {
			return modelsLoadedMsg{models: nil, err: err}
		}
		return modelsLoadedMsg{models: models}
	}
}

type modelsLoadedMsg struct {
	models []llm.Model
	err    error
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width - 4
		m.viewport.Height = msg.Height - 10
		m.input.Width = msg.Width - 10

	case modelsLoadedMsg:
		if msg.err == nil {
			m.ollamaModels = msg.models
			m.availModels = msg.models
		} else {
			m.availModels = llm.KnownModels
		}
		if len(m.availModels) == 0 {
			m.availModels = llm.KnownModels
		}
		m.status = fmt.Sprintf("model: %s | %d models available", m.config.ActiveModel, len(m.availModels))

	case tea.KeyMsg:
		switch m.mode {
		case modeSelectModel:
			return m.updateModelSelection(msg)
		case modeChat, modeCode:
			return m.updateChat(msg)
		case modeHelp:
			if msg.String() == "q" || msg.String() == "esc" {
				m.mode = modeChat
			}
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case streamMsg:
		if msg.done {
			m.streaming = false
			m.partial = ""

			fullContent := msg.full
			m.messages[len(m.messages)-1].content = fullContent

			code, lang := extractCodeBlock(fullContent)
			if code != "" {
				if lang == "" {
					lang = m.executor.DetectLanguage(code)
				}
				if lang != "" {
					m.status = fmt.Sprintf("executing %s code...", lang)
					return m, m.executeCode(code, lang, len(m.messages)-1)
				}
			}

			m.conversation = append(m.conversation, llm.Message{Role: "assistant", Content: fullContent})
			m.viewport.SetContent(m.renderMessages())
			m.viewport.GotoBottom()
			m.status = fmt.Sprintf("model: %s | ready", m.config.ActiveModel)
		} else {
			m.partial = msg.chunk
			m.messages[len(m.messages)-1].content += msg.chunk
			m.viewport.SetContent(m.renderMessages())
			m.viewport.GotoBottom()
		}

	case execResultMsg:
		code, lang := extractCodeBlock(m.messages[msg.msgIdx].content)
		expResult := fmt.Sprintf("\n\n**Execution Result** (exit: %d, %s):\n```\n", msg.result.ExitCode, msg.result.Duration)
		if msg.result.Stdout != "" {
			expResult += "stdout:\n" + msg.result.Stdout
		}
		if msg.result.Stderr != "" {
			expResult += "stderr:\n" + msg.result.Stderr
		}
		if msg.result.ExitCode != 0 {
			expResult += fmt.Sprintf("\nexit code: %d", msg.result.ExitCode)
		}
		expResult += "\n```"
		m.messages[msg.msgIdx].content += expResult

		if msg.store != nil {
			msg.store.SaveExperience(
				m.conversation[len(m.conversation)-2].Content,
				m.messages[msg.msgIdx].content,
				code, lang,
				msg.result.Stdout, msg.result.Stderr,
				msg.result.ExitCode, 0,
			)
		}

		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()

		exitStr := "✓"
		if msg.result.ExitCode != 0 {
			exitStr = fmt.Sprintf("✗ exit %d", msg.result.ExitCode)
		}
		m.status = fmt.Sprintf("model: %s | exec %s (%s)", m.config.ActiveModel, exitStr, msg.result.Duration)

	case errorMsg:
		m.streaming = false
		m.err = msg.err
		m.messages = append(m.messages, message{role: "system", content: fmt.Sprintf("Error: %s", msg.err)})
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()
		m.status = fmt.Sprintf("model: %s | error", m.config.ActiveModel)
	}

	return m, tea.Batch(cmds...)
}

func (m model) updateChat(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc":
		if m.streaming {
			m.streaming = false
			m.status = fmt.Sprintf("model: %s | interrupted", m.config.ActiveModel)
			return m, nil
		}
		return m, tea.Quit

	case "ctrl+l":
		m.messages = nil
		m.conversation = []llm.Message{
			{Role: "system", Content: defaultSystemPrompt(m.config.ActiveModel)},
		}
		m.viewport.SetContent("")
		return m, nil

	case "ctrl+t":
		m.mode = modeSelectModel
		m.cursor = 0
		m.status = "select model (↑/↓ to navigate, enter to select, esc to cancel)"
		return m, nil

	case "ctrl+e":
		if m.mode == modeChat {
			m.mode = modeCode
			m.input.Placeholder = "Write code to execute..."
			m.status = "code mode | write code to execute"
		} else {
			m.mode = modeChat
			m.input.Placeholder = "Ask Rose anything..."
			m.status = fmt.Sprintf("model: %s | chat mode", m.config.ActiveModel)
		}
		return m, nil

	case "ctrl+h":
		m.mode = modeHelp
		return m, nil

	case "enter":
		text := m.input.Value()
		if text == "" {
			return m, nil
		}
		m.input.SetValue("")
		m.messages = append(m.messages, message{role: "user", content: text})
		m.conversation = append(m.conversation, llm.Message{Role: "user", Content: text})
		m.messages = append(m.messages, message{role: "assistant", content: ""})
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()
		m.streaming = true

		currentMode := "chat"
		if m.mode == modeCode {
			currentMode = "code"
		}
		m.status = fmt.Sprintf("model: %s | %s | streaming...", m.config.ActiveModel, currentMode)

		return m, m.streamResponse(m.conversation)

	case "tab":
		text := m.input.Value()
		if text != "" && !m.streaming {
			code, lang := extractCodeBlock(text)
			if lang == "" {
				lang = m.executor.DetectLanguage(text)
			}
			if lang != "" && code != "" {
				m.messages = append(m.messages, message{role: "user", content: fmt.Sprintf("Running %s code...", lang)})
				m.viewport.SetContent(m.renderMessages())
				m.viewport.GotoBottom()
				m.status = fmt.Sprintf("executing %s...", lang)
				return m, m.executeCode(code, lang, len(m.messages)-1)
			}
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m model) updateModelSelection(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeChat
		m.status = fmt.Sprintf("model: %s | ready", m.config.ActiveModel)

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}

	case "down", "j":
		if m.cursor < len(m.availModels)-1 {
			m.cursor++
		}

	case "enter":
		selected := m.availModels[m.cursor].Name
		m.config.ActiveModel = selected
		m.config.Save()
		m.conversation[0] = llm.Message{Role: "system", Content: defaultSystemPrompt(selected)}
		m.mode = modeChat
		m.status = fmt.Sprintf("model: %s | ready", selected)
	}

	return m, nil
}

func (m model) streamResponse(conversation []llm.Message) tea.Cmd {
	return func() tea.Msg {
		full, err := m.llmClient.Chat(
			m.config.ActiveModel,
			conversation,
			llm.Options{
				Temperature: m.config.Temperature,
				MaxTokens:   m.config.MaxTokens,
			},
			func(chunk string) error {
				return nil
			},
		)
		if err != nil {
			return errorMsg{err: err}
		}
		return streamMsg{done: true, full: full}
	}
}

func (m model) executeCode(code, lang string, msgIdx int) tea.Cmd {
	return func() tea.Msg {
		exec, err := sandbox.NewExecutor(m.config.SandboxTimeout)
		if err != nil {
			return errorMsg{err: fmt.Errorf("create executor: %w", err)}
		}
		defer exec.Cleanup()

		result, err := exec.RunShell(code, lang)
		if err != nil {
			return execResultMsg{
				result: &sandbox.Result{
					Stderr: err.Error(),
					ExitCode: -1,
				},
				msgIdx: msgIdx,
				store:  m.store,
			}
		}
		return execResultMsg{
			result: result,
			msgIdx: msgIdx,
			store:  m.store,
		}
	}
}

type streamMsg struct {
	chunk string
	done  bool
	full  string
}

type errorMsg struct {
	err error
}

type execResultMsg struct {
	result *sandbox.Result
	msgIdx int
	store  *memory.Store
}

func (m model) View() string {
	var main string

	switch m.mode {
	case modeSelectModel:
		main = m.renderModelList()
	case modeHelp:
		main = m.renderHelp()
	default:
		main = m.renderMain()
	}

	return AppStyle.Render(main)
}

func extractCodeBlock(content string) (code string, language string) {
	lines := strings.Split(content, "\n")
	inBlock := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			if !inBlock {
				inBlock = true
				lang := strings.TrimPrefix(trimmed, "```")
				lang = strings.TrimSpace(lang)
				if lang != "" && !strings.Contains(lang, " ") {
					language = lang
				}
			} else {
				break
			}
		} else if inBlock {
			if code == "" {
				code = line
			} else {
				code += "\n" + line
			}
		}
	}
	return
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
