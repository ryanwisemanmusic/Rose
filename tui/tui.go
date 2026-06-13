package tui

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/ryanwi/rose/config"
	"github.com/ryanwi/rose/context"
	"github.com/ryanwi/rose/core"
	"github.com/ryanwi/rose/llm"
	"github.com/ryanwi/rose/memory"
	"github.com/ryanwi/rose/permission"
	"github.com/ryanwi/rose/sandbox"
	"github.com/ryanwi/rose/workspace"
)

type mode int

const (
	modeChat mode = iota
	modeCode
	modeSelectModel
	modeSelfReflect
	modePermission
	modeHelp
)

type message struct {
	role    string
	content string
}

type execPhase int

const (
	phaseIdle execPhase = iota
	phaseWaitingLLM
	phaseExecuting
	phaseFixing
)

type model struct {
	config     *config.Config
	llmClient  *llm.Client
	store      *memory.Store
	executor   *sandbox.Executor
	learner    *memory.Learner
	improver   *core.SelfImprover
	workspace  *workspace.Context
	referencer *context.Referencer
	docReader  *context.DocReader

	width      int
	height     int
	mode       mode
	execPhase  execPhase

	messages   []message
	viewport   viewport.Model
	input      textinput.Model
	spinner    spinner.Model

	status     string
	err        error

	permMgr      *permission.Manager
	permRef      context.Reference
	permRefs     []context.Reference
	permResolved string

	ollamaModels []llm.Model
	availModels  []llm.Model
	cursor       int

	conversation     []llm.Message
	inFlightMsgIdx   int
	fixAttempt       int
	currentCode      string
	currentLang      string
	recentContext    []context.Reference
}

func InitialModel(cfg *config.Config, store *memory.Store, executor *sandbox.Executor, ws *workspace.Context) model {
	ti := textinput.New()
	ti.Placeholder = "Ask Rose anything... (@path for context)"
	ti.Focus()
	ti.CharLimit = 4000
	ti.Width = 70

	vp := viewport.New(80, 20)
	vp.Style = lipgloss.NewStyle().Padding(0, 1)
	vp.KeyMap = viewport.DefaultKeyMap()

	s := spinner.New()
	s.Style = lipgloss.NewStyle().Foreground(highlight)
	s.Spinner = spinner.Dot

	improver := core.NewSelfImprover(cfg.RoseRoot)
	permMgr := permission.NewManager(ws.ProjectRoot)

	status := fmt.Sprintf("model: %s | %s", cfg.ActiveModel, ws.Summary())
	if ws.RoseRoot != "" && ws.RoseRoot != ws.ProjectRoot {
		status += " | ✦ self-aware"
	}

	return model{
		config:     cfg,
		llmClient:  llm.NewClient(cfg.OllamaHost),
		store:      store,
		executor:   executor,
		learner:    memory.NewLearner(store),
		improver:   improver,
		workspace:  ws,
		permMgr:    permMgr,
		referencer: context.NewReferencer(ws.ProjectRoot, permMgr),
		docReader:  context.NewDocReader(),
		viewport:   vp,
		input:      ti,
		spinner:    s,
		mode:       modeChat,
		execPhase:  phaseIdle,
		status:     status,
		conversation: []llm.Message{
			{Role: "system", Content: buildSystemPrompt(cfg.ActiveModel, ws, improver)},
		},
	}
}

func buildSystemPrompt(modelName string, ws *workspace.Context, improver *core.SelfImprover) string {
	var b strings.Builder

	b.WriteString("You are Rose, an intelligent self-improving programming assistant.\n\n")
	b.WriteString("## Current Context\n")
	b.WriteString(fmt.Sprintf("- Working directory: %s\n", ws.CurrentDir))
	b.WriteString(fmt.Sprintf("- Project: %s\n", ws.ProjectName))
	if ws.IsGitRepo {
		b.WriteString(fmt.Sprintf("- Git root: %s\n", ws.GitRoot))
	}
	if len(ws.Languages) > 0 {
		b.WriteString(fmt.Sprintf("- Languages detected: %s\n", strings.Join(ws.Languages, ", ")))
	}

	b.WriteString("\n## Self-Awareness & Learning\n")
	b.WriteString("- You learn from every interaction across ALL projects\n")
	b.WriteString("- Knowledge is stored globally in ~/.rose/history.db\n")
	b.WriteString("- Past successes AND failures are retrieved to inform future responses\n")
	b.WriteString("- When code fails, read the error and fix it automatically\n")
	if ws.RoseRoot != "" {
		b.WriteString(fmt.Sprintf("- Your source code is at: %s\n", ws.RoseRoot))
		b.WriteString("- You can propose improvements to your own code\n")
	}

	b.WriteString("\n## Reference System\n")
	b.WriteString("- User can include @path to reference files and folders\n")
	b.WriteString("- @path resolves within the project root\n")
	b.WriteString("- @/absolute/path resolves absolute paths for docs\n")
	b.WriteString("- Use these to read documentation, existing code, etc.\n")

	b.WriteString("\n## Code Execution Protocol\n")
	b.WriteString("1. Write code with ```language blocks\n")
	b.WriteString("2. The system will automatically execute your code\n")
	b.WriteString("3. If execution fails, the error is returned to you\n")
	b.WriteString("4. Fix the code and the system will re-execute\n")
	b.WriteString("5. This repeats until success or max attempts reached\n")

	b.WriteString("\n## Learning Strategy\n")
	b.WriteString("- Abstract patterns: learn language-agnostic solutions\n")
	b.WriteString("- Store what works and what doesn't\n")
	b.WriteString("- Apply cross-project knowledge\n")

	b.WriteString("\n## Behavior\n")
	b.WriteString("- Be concise and direct\n")
	b.WriteString("- Use ```language for code blocks\n")
	b.WriteString("- If code exists in context, read it before suggesting changes\n")

	if strings.Contains(modelName, "1b") || strings.Contains(modelName, "4b") {
		b.WriteString("\n## Constraint\nYou are running on a lightweight model. Keep responses very short.\n")
	}

	return b.String()
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.loadModels())
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
		m.status = fmt.Sprintf("model: %s | %d models | %s",
			m.config.ActiveModel, len(m.availModels), m.workspace.Summary())

	case tea.KeyMsg:
		switch m.mode {
		case modeSelectModel:
			return m.updateModelSelection(msg)
		case modeSelfReflect:
			return m.updateSelfReflect(msg)
		case modePermission:
			return m.updatePermission(msg)
		case modeHelp:
			if msg.String() == "q" || msg.String() == "esc" {
				m.mode = modeChat
			}
		case modeChat, modeCode:
			return m.updateChat(msg)
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case llmStreamMsg:
		if msg.done {
			fullContent := msg.full
			m.messages[m.inFlightMsgIdx].content = fullContent
			m.viewport.SetContent(m.renderMessages())
			m.viewport.GotoBottom()

			code, lang := extractCodeBlock(fullContent)
			if code != "" {
				if lang == "" {
					lang = m.executor.DetectLanguage(code)
				}
				if lang != "" {
					m.currentCode = code
					m.currentLang = lang
					m.fixAttempt = 0
					m.conversation = append(m.conversation, llm.Message{Role: "assistant", Content: fullContent})
					return m, m.executeCurrent()
				}
			}

			m.conversation = append(m.conversation, llm.Message{Role: "assistant", Content: fullContent})
			m.execPhase = phaseIdle
			m.viewport.SetContent(m.renderMessages())
			m.viewport.GotoBottom()
			m.status = fmt.Sprintf("model: %s | ready | %s", m.config.ActiveModel, m.workspace.Summary())
		} else {
			m.messages[m.inFlightMsgIdx].content += msg.chunk
			m.viewport.SetContent(m.renderMessages())
			m.viewport.GotoBottom()
		}

	case execResultMsg:
		if msg.result.ExitCode == 0 {
			expResult := fmt.Sprintf("\n\n✓ **Execution succeeded** (%s):\n```\n%s\n```", msg.result.Duration, msg.result.Stdout)
			m.messages[m.inFlightMsgIdx].content += expResult
			m.conversation = append(m.conversation, llm.Message{Role: "assistant", Content: m.messages[m.inFlightMsgIdx].content})

			if m.store != nil {
				m.store.SaveExperience(
					m.conversation[len(m.conversation)-2].Content,
					m.messages[m.inFlightMsgIdx].content,
					m.currentCode, m.currentLang,
					msg.result.Stdout, msg.result.Stderr,
					0, m.workspace.ProjectName, int64(m.fixAttempt),
				)
			}

			m.execPhase = phaseIdle
			m.viewport.SetContent(m.renderMessages())
			m.viewport.GotoBottom()
			m.status = fmt.Sprintf("model: %s | ✓ exec %s | %s",
				m.config.ActiveModel, msg.result.Duration, m.workspace.Summary())
		} else {
			m.execPhase = phaseFixing
			m.fixAttempt++

			errorOutput := msg.result.Stderr
			if errorOutput == "" {
				errorOutput = msg.result.Stdout
			}

			expResult := fmt.Sprintf("\n\n✗ **Execution failed** (exit %d, %s):\n```\n%s\n```",
				msg.result.ExitCode, msg.result.Duration, errorOutput)
			m.messages[m.inFlightMsgIdx].content += expResult
			m.viewport.SetContent(m.renderMessages())
			m.viewport.GotoBottom()

			if m.fixAttempt >= 5 {
				m.messages = append(m.messages, message{role: "system",
					content: fmt.Sprintf("Gave up after %d fix attempts.", m.fixAttempt)})
				m.conversation = append(m.conversation,
					llm.Message{Role: "assistant", Content: m.messages[m.inFlightMsgIdx].content})
				m.execPhase = phaseIdle
				m.viewport.SetContent(m.renderMessages())
				m.viewport.GotoBottom()
				m.status = fmt.Sprintf("model: %s | ✗ failed after %d attempts", m.config.ActiveModel, m.fixAttempt)
				return m, nil
			}

			m.status = fmt.Sprintf("fixing (attempt %d/5)...", m.fixAttempt)
			m.conversation = append(m.conversation, llm.Message{Role: "assistant", Content: m.messages[m.inFlightMsgIdx].content})

			if m.store != nil {
				m.store.SaveExperience(
					m.conversation[len(m.conversation)-2].Content,
					m.messages[m.inFlightMsgIdx].content,
					m.currentCode, m.currentLang,
					msg.result.Stdout, msg.result.Stderr,
					msg.result.ExitCode, m.workspace.ProjectName, int64(m.fixAttempt-1),
				)
			}

			fixPrompt := fmt.Sprintf(`The code above failed with exit code %d.

Error output:
%s

Please fix the code. Output ONLY the corrected code block. Do not explain.
`, msg.result.ExitCode, errorOutput)

			m.messages = append(m.messages, message{role: "assistant", content: ""})
			m.inFlightMsgIdx = len(m.messages) - 1
			m.conversation = append(m.conversation, llm.Message{Role: "user", Content: fixPrompt})
			return m, m.streamLLM()
		}

	case errorMsg:
		m.execPhase = phaseIdle
		m.err = msg.err
		m.messages = append(m.messages, message{role: "system", content: fmt.Sprintf("Error: %s", msg.err)})
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()
		m.status = fmt.Sprintf("model: %s | error", m.config.ActiveModel)
	}

	return m, tea.Batch(cmds...)
}

func (m model) updateChat(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.execPhase != phaseIdle {
		switch msg.String() {
		case "esc", "ctrl+c":
			m.execPhase = phaseIdle
			m.status = fmt.Sprintf("model: %s | interrupted", m.config.ActiveModel)
			return m, nil
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit

	case "esc":
		return m, nil

	case "ctrl+l":
		m.messages = nil
		m.conversation = []llm.Message{
			{Role: "system", Content: buildSystemPrompt(m.config.ActiveModel, m.workspace, m.improver)},
		}
		m.viewport.SetContent("")
		m.status = fmt.Sprintf("model: %s | cleared | %s", m.config.ActiveModel, m.workspace.Summary())
		return m, nil

	case "ctrl+t":
		m.mode = modeSelectModel
		m.cursor = 0
		m.status = "select model (↑/↓ enter, esc cancel)"
		return m, nil

	case "ctrl+e":
		if m.mode == modeChat {
			m.mode = modeCode
			m.input.Placeholder = "Write code to execute..."
			m.status = "code mode | write code directly"
		} else {
			m.mode = modeChat
			m.input.Placeholder = "Ask Rose anything... (@path for context)"
			m.status = fmt.Sprintf("model: %s | chat | %s", m.config.ActiveModel, m.workspace.Summary())
		}
		return m, nil

	case "ctrl+s":
		if m.config.RoseRoot == "" {
			m.addSystemMsg("Rose root not set. Run 'make install' first.")
			return m, nil
		}
		m.mode = modeSelfReflect
		m.cursor = 0
		m.status = "self-improve: what should Rose change about itself?"
		m.input.Placeholder = "Describe the improvement..."
		m.input.SetValue("")
		return m, nil

	case "ctrl+u":
		return m, m.updateRoseRepo()

	case "ctrl+h":
		m.mode = modeHelp
		return m, nil

	case "enter":
		text := m.input.Value()
		if text == "" {
			return m, nil
		}
		m.input.SetValue("")

		resolved, refs, err := m.referencer.ResolveAll(text)
		if err == nil && len(refs) > 0 {
			m.recentContext = refs
			refSummary := m.referencer.SummarizeRefs(refs)
			m.addSystemMsg(fmt.Sprintf("Loaded %s", refSummary))

			for _, ref := range refs {
				if ref.Blocked {
					m.permRef = ref
					m.permRefs = refs
					m.permResolved = resolved
					m.mode = modePermission
					m.status = fmt.Sprintf("Allow access to %s? (y)es (n)o (a)lways", ref.Resolved)
					return m, nil
				}
			}
		}

		basePrompt := resolved
		if len(refs) > 0 {
			basePrompt += "\n\nUse the context above. Be specific about paths."
		}

		enhanced := m.learner.BuildPrompt(basePrompt, strings.Join(m.workspace.Languages, ","))

		m.messages = append(m.messages, message{role: "user", content: text})
		m.messages = append(m.messages, message{role: "assistant", content: ""})
		m.inFlightMsgIdx = len(m.messages) - 1
		m.conversation = append(m.conversation, llm.Message{Role: "user", Content: enhanced})

		m.execPhase = phaseWaitingLLM
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()
		m.status = fmt.Sprintf("model: %s | thinking...", m.config.ActiveModel)

		return m, m.streamLLM()

	case "tab":
		text := m.input.Value()
		if text != "" {
			code, lang := extractCodeBlock(text)
			if lang == "" {
				lang = m.executor.DetectLanguage(text)
			}
			if lang != "" && code != "" {
				m.messages = append(m.messages, message{role: "user", content: fmt.Sprintf("Running %s code...", lang)})
				m.messages = append(m.messages, message{role: "assistant", content: ""})
				m.inFlightMsgIdx = len(m.messages) - 1
				m.currentCode = code
				m.currentLang = lang
				m.fixAttempt = 0
				m.execPhase = phaseExecuting
				m.viewport.SetContent(m.renderMessages())
				m.viewport.GotoBottom()
				return m, m.executeCurrent()
			}
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m model) updateSelfReflect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeChat
		m.input.Placeholder = "Ask Rose anything... (@path for context)"
		m.status = fmt.Sprintf("model: %s | ready | %s", m.config.ActiveModel, m.workspace.Summary())
		return m, nil

	case "enter":
		text := m.input.Value()
		if text == "" {
			return m, nil
		}
		m.input.SetValue("")
		m.input.Placeholder = "Ask Rose anything... (@path for context)"
		m.messages = append(m.messages, message{role: "user", content: "[self-improve] " + text})
		m.messages = append(m.messages, message{role: "assistant", content: ""})
		m.inFlightMsgIdx = len(m.messages) - 1
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()
		m.status = "analyzing own source..."
		return m, m.selfReflect(text)
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m model) updatePermission(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m.permMgr.Grant(m.permRef.BlockPath, false)
		m.addSystemMsg(fmt.Sprintf("Allowed access to %s (once)", m.permRef.Resolved))

		_, refs, _ := m.referencer.ResolveAll(m.permResolved)
		for _, r := range refs {
			if r.Blocked {
				m.mode = modeChat
				m.status = fmt.Sprintf("model: %s | blocked", m.config.ActiveModel)
				return m, nil
			}
		}

		m.mode = modeChat
		if len(refs) > 0 {
			m.addSystemMsg(fmt.Sprintf("Loaded %s", m.referencer.SummarizeRefs(refs)))
		}
		m.status = fmt.Sprintf("model: %s | ready", m.config.ActiveModel)
		return m, nil

	case "a", "A":
		m.permMgr.Grant(m.permRef.BlockPath, true)
		m.addSystemMsg(fmt.Sprintf("Allowed access to %s (session)", m.permRef.Resolved))

		_, refs, _ := m.referencer.ResolveAll(m.permResolved)
		for _, r := range refs {
			if r.Blocked {
				m.mode = modeChat
				m.status = fmt.Sprintf("model: %s | blocked", m.config.ActiveModel)
				return m, nil
			}
		}

		m.mode = modeChat
		if len(refs) > 0 {
			m.addSystemMsg(fmt.Sprintf("Loaded %s", m.referencer.SummarizeRefs(refs)))
		}
		m.status = fmt.Sprintf("model: %s | ready", m.config.ActiveModel)
		return m, nil

	case "n", "N", "esc":
		m.permMgr.Deny(m.permRef.BlockPath)
		m.addSystemMsg(fmt.Sprintf("Denied access to %s", m.permRef.Resolved))
		m.mode = modeChat
		m.status = fmt.Sprintf("model: %s | ready", m.config.ActiveModel)
		return m, nil
	}
	return m, nil
}

func (m model) updateModelSelection(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeChat
		m.status = fmt.Sprintf("model: %s | %s", m.config.ActiveModel, m.workspace.Summary())
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
		m.conversation[0] = llm.Message{Role: "system", Content: buildSystemPrompt(selected, m.workspace, m.improver)}
		m.mode = modeChat
		m.status = fmt.Sprintf("model: %s | ready | %s", selected, m.workspace.Summary())
	}
	return m, nil
}

func (m model) streamLLM() tea.Cmd {
	return func() tea.Msg {
		full, err := m.llmClient.Chat(
			m.config.ActiveModel,
			m.conversation,
			llm.Options{
				Temperature: m.config.Temperature,
				MaxTokens:   m.config.MaxTokens,
			},
			func(chunk string) error { return nil },
		)
		if err != nil {
			return errorMsg{err: err}
		}
		return llmStreamMsg{done: true, full: full}
	}
}

func (m model) executeCurrent() tea.Cmd {
	m.execPhase = phaseExecuting
	return func() tea.Msg {
		exec, err := sandbox.NewExecutor(m.config.SandboxTimeout)
		if err != nil {
			return errorMsg{err: fmt.Errorf("create executor: %w", err)}
		}
		defer exec.Cleanup()

		result, err := exec.RunShell(m.currentCode, m.currentLang)
		if err != nil {
			return execResultMsg{
				result: &sandbox.Result{
					Stderr:   err.Error(),
					ExitCode: -1,
				},
			}
		}
		return execResultMsg{result: result}
	}
}

func (m model) selfReflect(query string) tea.Cmd {
	return func() tea.Msg {
		_, err := m.improver.ReadAllSource()
		if err != nil {
			return llmStreamMsg{done: true, full: fmt.Sprintf("Error reading source: %s", err)}
		}

		var ctx strings.Builder
		ctx.WriteString(m.improver.BuildContext())
		ctx.WriteString(fmt.Sprintf("\n\nUser request: %s", query))
		ctx.WriteString("\n\nPropose specific code changes with file paths and content.")

		prompt := []llm.Message{
			{Role: "system", Content: "You are Rose's self-improvement module. Analyze the codebase and propose concrete changes. Be specific."},
			{Role: "user", Content: ctx.String()},
		}

		result, err := m.llmClient.Chat(
			m.config.ActiveModel, prompt,
			llm.Options{Temperature: 0.4, MaxTokens: 4096}, nil,
		)
		if err != nil {
			return llmStreamMsg{done: true, full: fmt.Sprintf("Error: %s", err)}
		}
		return llmStreamMsg{done: true, full: result}
	}
}

func (m model) updateRoseRepo() tea.Cmd {
	return func() tea.Msg {
		if m.config.RoseRoot == "" {
			return errorMsg{fmt.Errorf("rose root not set")}
		}

		out, err := exec.Command("git", "-C", m.config.RoseRoot, "pull", "--rebase").CombinedOutput()
		if err != nil {
			return errorMsg{fmt.Errorf("git pull failed: %s", out)}
		}

		buildOut, err := exec.Command("go", "build", "-C", m.config.RoseRoot, "-o", "rose", ".").CombinedOutput()
		if err != nil {
			return errorMsg{fmt.Errorf("build failed: %s", buildOut)}
		}

		return selfApplyMsg{file: "rebuild from git"}
	}
}

func (m model) addSystemMsg(text string) {
	m.messages = append(m.messages, message{role: "system", content: text})
	m.viewport.SetContent(m.renderMessages())
	m.viewport.GotoBottom()
}

type llmStreamMsg struct {
	chunk string
	done  bool
	full  string
}

type errorMsg struct {
	err error
}

type execResultMsg struct {
	result *sandbox.Result
}

type selfApplyMsg struct {
	file string
	err  error
}

func (m model) View() string {
	var main string
	switch m.mode {
	case modeSelectModel:
		main = m.renderModelList()
	case modePermission:
		main = m.renderPermission()
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
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "```") {
			if !inBlock {
				inBlock = true
				lang := strings.TrimPrefix(t, "```")
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

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
