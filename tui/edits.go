package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/ryanwi/rose/sandbox"
)

type codeBlock struct {
	Info     string
	Language string
	Content  string
}

type fileEdit struct {
	Path       string
	Content    string
	Language   string
	BlockIndex int
}

type appliedEdit struct {
	Path    string
	Kind    string
	Lines   int
	Preview []string
}

var (
	fileAssignmentPattern = regexp.MustCompile(`(?i)(file|path|filename)\s*[:=]\s*["']?([^"'\s]+)`)
	filePathPattern       = regexp.MustCompile(`(?i)(^|[\s"'(])((([A-Za-z0-9_.-]+/)*[A-Za-z0-9_.-]+\.(cpp|cc|cxx|c|hpp|hh|hxx|h|go|py|js|jsx|ts|tsx|rs|zig|java|rb|swift|kt|kts|md|txt|json|yaml|yml|toml|sh|bash|mk))|(([A-Za-z0-9_.-]+/)?Makefile))($|[\s"',).:])`)
	folderPattern         = regexp.MustCompile(`(?i)(in|inside|under|within|into)?\s*(the\s+)?([A-Za-z0-9_.-]+)\s+(folder|directory|dir)\b`)
)

func extractCodeBlocks(content string) []codeBlock {
	var blocks []codeBlock
	var current strings.Builder
	var info string
	inBlock := false

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			if inBlock {
				blocks = append(blocks, codeBlock{
					Info:     info,
					Language: languageFromFence(info),
					Content:  strings.TrimRight(current.String(), "\n"),
				})
				current.Reset()
				info = ""
				inBlock = false
				continue
			}

			info = strings.TrimSpace(strings.TrimPrefix(trimmed, "```"))
			inBlock = true
			continue
		}

		if inBlock {
			current.WriteString(line)
			current.WriteString("\n")
		}
	}

	if inBlock {
		blocks = append(blocks, codeBlock{
			Info:     info,
			Language: languageFromFence(info),
			Content:  strings.TrimRight(current.String(), "\n"),
		})
	}

	return blocks
}

func inferFileEdits(blocks []codeBlock, prompt, response string) []fileEdit {
	text := prompt + "\n" + response
	candidates := collectFileCandidates(text)
	targetDir := inferTargetDir(text, candidates)
	usedPaths := make(map[string]bool)

	var edits []fileEdit
	for i, block := range blocks {
		if strings.TrimSpace(block.Content) == "" {
			continue
		}

		path := explicitPathFromInfo(block.Info)
		if path == "" {
			path = inferPathForBlock(block, candidates, targetDir, usedPaths)
		}
		if path == "" {
			continue
		}

		path = applyTargetDir(path, targetDir)
		path = normalizeCandidate(path)
		if path == "" || usedPaths[path] {
			continue
		}

		usedPaths[path] = true
		edits = append(edits, fileEdit{
			Path:       path,
			Content:    strings.TrimRight(block.Content, "\n") + "\n",
			Language:   block.Language,
			BlockIndex: i,
		})
	}

	return edits
}

func explicitPathFromInfo(info string) string {
	info = strings.TrimSpace(info)
	if info == "" {
		return ""
	}

	if match := fileAssignmentPattern.FindStringSubmatch(info); len(match) >= 3 {
		return normalizeCandidate(match[2])
	}

	fields := strings.Fields(info)
	if len(fields) == 1 && isKnownLanguage(fields[0]) {
		return ""
	}

	for _, field := range fields {
		field = normalizeCandidate(field)
		if field == "" || isKnownLanguage(field) {
			continue
		}
		if looksLikeFilePath(field) {
			return field
		}
	}

	return ""
}

func inferPathForBlock(block codeBlock, candidates []string, targetDir string, used map[string]bool) string {
	lang := normalizeLanguage(block.Language)
	if lang == "" {
		lang = languageFromFence(block.Info)
	}
	if lang == "" {
		return ""
	}

	for _, candidate := range candidates {
		path := applyTargetDir(candidate, targetDir)
		path = normalizeCandidate(path)
		if path == "" || used[path] {
			continue
		}
		if languageMatchesPath(lang, path) {
			return path
		}
	}

	if lang == "makefile" && targetDir != "" {
		path := normalizeCandidate(filepath.ToSlash(filepath.Join(targetDir, "Makefile")))
		if !used[path] {
			return path
		}
	}

	return ""
}

func collectFileCandidates(text string) []string {
	seen := make(map[string]bool)
	var candidates []string
	for _, match := range filePathPattern.FindAllStringSubmatch(text, -1) {
		if len(match) < 3 {
			continue
		}
		path := normalizeCandidate(match[2])
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		candidates = append(candidates, path)
	}
	return candidates
}

func inferTargetDir(text string, candidates []string) string {
	for _, candidate := range candidates {
		if strings.Contains(candidate, "/") {
			return normalizeCandidate(filepath.ToSlash(filepath.Dir(candidate)))
		}
	}

	for _, match := range folderPattern.FindAllStringSubmatch(text, -1) {
		if len(match) < 4 {
			continue
		}
		dir := normalizeCandidate(match[3])
		if dir != "" && dir != "." && !looksLikeFilePath(dir) {
			return dir
		}
	}

	return ""
}

func applyTargetDir(path, targetDir string) string {
	if targetDir == "" || path == "" || filepath.IsAbs(path) || strings.Contains(path, "/") {
		return path
	}
	return filepath.ToSlash(filepath.Join(targetDir, path))
}

func firstRunnableBlock(blocks []codeBlock, skip map[int]bool, executor interface{ DetectLanguage(string) string }) (string, string) {
	for i, block := range blocks {
		if skip != nil && skip[i] {
			continue
		}
		lang := normalizeLanguage(block.Language)
		if lang == "" {
			lang = executor.DetectLanguage(block.Content)
		}
		if isExecutableLanguage(lang) {
			return block.Content, lang
		}
	}
	return "", ""
}

func editedBlockIndexes(edits []fileEdit) map[int]bool {
	indexes := make(map[int]bool)
	for _, edit := range edits {
		indexes[edit.BlockIndex] = true
	}
	return indexes
}

func applyProjectFileEdits(projectRoot string, edits []fileEdit) ([]appliedEdit, []sandbox.FileChange, error) {
	var applied []appliedEdit
	var changes []sandbox.FileChange

	for _, edit := range edits {
		fullPath, relPath, err := resolveProjectPath(projectRoot, edit.Path)
		if err != nil {
			return applied, changes, err
		}

		kind := "modified"
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			kind = "created"
		} else if err != nil {
			return applied, changes, fmt.Errorf("stat %s: %w", relPath, err)
		}

		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return applied, changes, fmt.Errorf("create dirs for %s: %w", relPath, err)
		}
		if err := os.WriteFile(fullPath, []byte(edit.Content), 0644); err != nil {
			return applied, changes, fmt.Errorf("write %s: %w", relPath, err)
		}
		cleanupAppleDoubleForFile(fullPath)

		lines := splitPreviewLines(edit.Content, 18)
		applied = append(applied, appliedEdit{
			Path:    relPath,
			Kind:    kind,
			Lines:   countLines(edit.Content),
			Preview: lines,
		})
		changes = append(changes, sandbox.FileChange{
			Path: relPath,
			Kind: kind,
			Size: int64(len(edit.Content)),
		})
	}

	sort.Slice(applied, func(i, j int) bool { return applied[i].Path < applied[j].Path })
	sort.Slice(changes, func(i, j int) bool { return changes[i].Path < changes[j].Path })

	return applied, changes, nil
}

func cleanupAppleDoubleForFile(path string) {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	_ = os.Remove(filepath.Join(dir, "._"+base))
}

func resolveProjectPath(projectRoot, path string) (string, string, error) {
	root, err := filepath.Abs(projectRoot)
	if err != nil {
		return "", "", fmt.Errorf("resolve project root: %w", err)
	}

	clean := normalizeCandidate(path)
	if clean == "" {
		return "", "", fmt.Errorf("empty edit path")
	}

	fullPath := clean
	if !filepath.IsAbs(fullPath) {
		fullPath = filepath.Join(root, filepath.FromSlash(clean))
	}
	fullPath, err = filepath.Abs(fullPath)
	if err != nil {
		return "", "", fmt.Errorf("resolve %s: %w", clean, err)
	}

	rel, err := filepath.Rel(root, fullPath)
	if err != nil {
		return "", "", fmt.Errorf("check %s: %w", clean, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
		return "", "", fmt.Errorf("refusing to edit outside project: %s", clean)
	}

	return fullPath, filepath.ToSlash(rel), nil
}

func renderAppliedEdits(edits []appliedEdit) string {
	if len(edits) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n\n")
	for i, edit := range edits {
		if i > 0 {
			b.WriteString("\n")
		}
		title := "← Edit"
		if edit.Kind == "created" {
			title = "← Create"
		}
		b.WriteString(fmt.Sprintf("%s %s\n", title, edit.Path))
		for lineNo, line := range edit.Preview {
			b.WriteString(fmt.Sprintf("+ %4d  %s\n", lineNo+1, line))
		}
		if edit.Lines > len(edit.Preview) {
			b.WriteString(fmt.Sprintf("+ ... %d more line(s)\n", edit.Lines-len(edit.Preview)))
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func renderEditResponse(original string, edits []appliedEdit) string {
	prose := strings.TrimSpace(stripCodeBlocks(original))
	summary := strings.TrimSpace(renderAppliedEdits(edits))
	if prose == "" {
		return summary
	}
	if summary == "" {
		return prose
	}
	return prose + "\n\n" + summary
}

func stripCodeBlocks(content string) string {
	var b strings.Builder
	inBlock := false

	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inBlock = !inBlock
			continue
		}
		if inBlock {
			continue
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	return strings.TrimRight(b.String(), "\n")
}

func summarizeFileEdits(edits []fileEdit) string {
	var b strings.Builder
	for _, edit := range edits {
		b.WriteString(fmt.Sprintf("%s\n%s\n", edit.Path, edit.Content))
	}
	return b.String()
}

func languageFromFence(info string) string {
	fields := strings.Fields(strings.TrimSpace(info))
	if len(fields) == 0 {
		return ""
	}
	first := normalizeCandidate(fields[0])
	if looksLikeFilePath(first) {
		return languageForPath(first)
	}
	return normalizeLanguage(first)
}

func normalizeLanguage(lang string) string {
	lang = strings.ToLower(strings.TrimSpace(lang))
	lang = strings.Trim(lang, "`:=")
	switch lang {
	case "c++", "cc", "cxx":
		return "cpp"
	case "make", "mk":
		return "makefile"
	case "shell":
		return "bash"
	case "node":
		return "javascript"
	default:
		return lang
	}
}

func isKnownLanguage(lang string) bool {
	_, ok := knownLanguageSet[normalizeLanguage(lang)]
	return ok
}

func isExecutableLanguage(lang string) bool {
	switch normalizeLanguage(lang) {
	case "go", "zig", "typescript", "ts", "javascript", "js", "python", "py", "rust", "rs", "bash", "sh":
		return true
	default:
		return false
	}
}

func languageMatchesPath(lang, path string) bool {
	lang = normalizeLanguage(lang)
	if lang == "makefile" {
		return strings.EqualFold(filepath.Base(path), "Makefile")
	}
	for _, ext := range extensionsByLanguage[lang] {
		if strings.EqualFold(filepath.Ext(path), ext) {
			return true
		}
	}
	return false
}

func languageForPath(path string) string {
	if strings.EqualFold(filepath.Base(path), "Makefile") {
		return "makefile"
	}
	ext := strings.ToLower(filepath.Ext(path))
	for lang, exts := range extensionsByLanguage {
		for _, candidate := range exts {
			if ext == candidate {
				return lang
			}
		}
	}
	return ""
}

func looksLikeFilePath(path string) bool {
	path = normalizeCandidate(path)
	if path == "" {
		return false
	}
	if strings.EqualFold(filepath.Base(path), "Makefile") {
		return true
	}
	return languageForPath(path) != ""
}

func normalizeCandidate(path string) string {
	path = strings.TrimSpace(path)
	path = strings.Trim(path, "`'\".,;:()[]{}<>")
	path = strings.TrimPrefix(path, "./")
	if path == "" {
		return ""
	}
	return filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
}

func splitPreviewLines(content string, limit int) []string {
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	if len(lines) > limit {
		return lines[:limit]
	}
	return lines
}

func countLines(content string) int {
	content = strings.TrimRight(content, "\n")
	if content == "" {
		return 0
	}
	return len(strings.Split(content, "\n"))
}

var extensionsByLanguage = map[string][]string{
	"bash":       {".sh", ".bash"},
	"c":          {".c", ".h"},
	"cpp":        {".cpp", ".cc", ".cxx", ".hpp", ".hh", ".hxx", ".h"},
	"go":         {".go"},
	"java":       {".java"},
	"javascript": {".js", ".jsx"},
	"js":         {".js", ".jsx"},
	"json":       {".json"},
	"kotlin":     {".kt", ".kts"},
	"makefile":   {},
	"markdown":   {".md"},
	"python":     {".py"},
	"py":         {".py"},
	"ruby":       {".rb"},
	"rust":       {".rs"},
	"rs":         {".rs"},
	"swift":      {".swift"},
	"text":       {".txt"},
	"toml":       {".toml"},
	"typescript": {".ts", ".tsx"},
	"ts":         {".ts", ".tsx"},
	"yaml":       {".yaml", ".yml"},
	"zig":        {".zig"},
}

var knownLanguageSet = func() map[string]bool {
	set := make(map[string]bool)
	for lang := range extensionsByLanguage {
		set[lang] = true
	}
	for _, lang := range []string{"sh", "shell", "node", "make", "mk", "c++"} {
		set[normalizeLanguage(lang)] = true
	}
	return set
}()
