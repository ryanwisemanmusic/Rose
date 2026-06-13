package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type referenceItem struct {
	Path  string
	IsDir bool
	Empty bool
}

type referencePicker struct {
	RootPath string
	Active   bool
	Query    string
	Start    int
	Items    []referenceItem
	Cursor   int
}

func newReferencePicker(projectRoot string) referencePicker {
	return referencePicker{RootPath: projectRoot}
}

func (p *referencePicker) Update(input string) {
	start, query, ok := activeReferenceQuery(input)
	if !ok {
		p.Close()
		return
	}

	p.Active = true
	p.Query = query
	p.Start = start
	p.Items = findReferenceItems(p.RootPath, query, 10)
	if p.Cursor >= len(p.Items) {
		p.Cursor = max(0, len(p.Items)-1)
	}
}

func (p *referencePicker) Close() {
	p.Active = false
	p.Query = ""
	p.Start = 0
	p.Items = nil
	p.Cursor = 0
}

func (p *referencePicker) Move(delta int) {
	if len(p.Items) == 0 {
		return
	}
	p.Cursor = (p.Cursor + delta + len(p.Items)) % len(p.Items)
}

func (p referencePicker) Selected() (referenceItem, bool) {
	if !p.Active || len(p.Items) == 0 || p.Cursor < 0 || p.Cursor >= len(p.Items) {
		return referenceItem{}, false
	}
	return p.Items[p.Cursor], true
}

func activeReferenceQuery(input string) (start int, query string, ok bool) {
	if input == "" {
		return 0, "", false
	}

	lastAt := strings.LastIndex(input, "@")
	if lastAt < 0 {
		return 0, "", false
	}
	if lastAt > 0 {
		prev := input[lastAt-1]
		if !isReferenceBoundary(prev) {
			return 0, "", false
		}
	}

	query = input[lastAt+1:]
	if strings.ContainsAny(query, " \t\r\n") {
		return 0, "", false
	}
	return lastAt, query, true
}

func isReferenceBoundary(ch byte) bool {
	switch ch {
	case ' ', '\t', '\n', '\r', '(', ')', '[', ']', '{', '}', '"', '\'', ',', '.', ';', ':':
		return true
	default:
		return false
	}
}

func findReferenceItems(root, query string, limit int) []referenceItem {
	query = strings.ToLower(strings.TrimPrefix(filepath.ToSlash(query), "./"))
	var matches []referenceItem

	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		name := entry.Name()
		if path != root && entry.IsDir() && shouldSkipReferenceDir(name) {
			return filepath.SkipDir
		}
		if path == root || shouldSkipReferenceName(name) {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if !referenceMatches(rel, query) {
			return nil
		}

		item := referenceItem{Path: rel, IsDir: entry.IsDir()}
		if entry.IsDir() {
			item.Empty = isEmptyReferenceDir(path)
		}
		matches = append(matches, item)
		return nil
	})

	sort.Slice(matches, func(i, j int) bool {
		iScore := referenceScore(matches[i].Path, query)
		jScore := referenceScore(matches[j].Path, query)
		if iScore == jScore {
			if matches[i].IsDir != matches[j].IsDir {
				return matches[i].IsDir
			}
			return matches[i].Path < matches[j].Path
		}
		return iScore < jScore
	})

	if len(matches) > limit {
		return matches[:limit]
	}
	return matches
}

func referenceMatches(path, query string) bool {
	if query == "" {
		return true
	}
	path = strings.ToLower(path)
	base := strings.ToLower(filepath.Base(path))
	return strings.Contains(path, query) || strings.Contains(base, query)
}

func referenceScore(path, query string) int {
	if query == "" {
		return 10
	}
	path = strings.ToLower(path)
	base := strings.ToLower(filepath.Base(path))
	switch {
	case strings.HasPrefix(path, query):
		return 0
	case strings.HasPrefix(base, query):
		return 1
	case strings.Contains(base, query):
		return 2
	case strings.Contains(path, query):
		return 3
	default:
		return 9
	}
}

func shouldSkipReferenceDir(name string) bool {
	if shouldSkipReferenceName(name) {
		return true
	}
	switch name {
	case ".git", ".svn", "node_modules", "vendor", "target", "dist", "build", "__pycache__", ".next":
		return true
	default:
		return false
	}
}

func shouldSkipReferenceName(name string) bool {
	return strings.HasPrefix(name, "._")
}

func isEmptyReferenceDir(path string) bool {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if shouldSkipReferenceName(entry.Name()) {
			continue
		}
		return false
	}
	return true
}

func (m *model) updateReferencePicker() {
	m.refPicker.Update(m.input.Value())
}

func (m *model) acceptReferenceSelection() {
	item, ok := m.refPicker.Selected()
	if !ok {
		return
	}

	value := m.input.Value()
	start := m.refPicker.Start
	if start < 0 || start >= len(value) {
		start = len(value)
	}

	next := value[:start] + "@" + item.Path + " "
	m.input.SetValue(next)
	m.input.CursorEnd()
	if !containsString(m.selectedRefs, item.Path) {
		m.selectedRefs = append(m.selectedRefs, item.Path)
	}
	m.refPicker.Close()
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func (m model) renderReferencePicker() string {
	var lines []string

	if len(m.selectedRefs) > 0 {
		var refs []string
		for _, ref := range m.selectedRefs {
			refs = append(refs, ReferenceStyle.Render("@"+ref))
		}
		lines = append(lines, "refs "+strings.Join(refs, " "))
	}

	if m.refPicker.Active {
		if len(m.refPicker.Items) == 0 {
			lines = append(lines, ReferencePickerItemStyle.Render(fmt.Sprintf("@%s  no matches", m.refPicker.Query)))
		} else {
			for i, item := range m.refPicker.Items {
				prefix := "  "
				style := ReferencePickerItemStyle
				if i == m.refPicker.Cursor {
					prefix = "› "
					style = ReferencePickerSelectedStyle
				}
				kind := "file"
				if item.IsDir {
					kind = "dir"
					if item.Empty {
						kind = "empty dir"
					}
				}
				lines = append(lines, style.Render(fmt.Sprintf("%s@%s  %s", prefix, item.Path, kind)))
			}
		}
	}

	if len(lines) == 0 {
		return ""
	}

	return styleForTotalWidth(ReferencePickerStyle, m.innerWidth()).Render(strings.Join(lines, "\n"))
}

func colorReferenceTokens(text string) string {
	var out strings.Builder
	for i := 0; i < len(text); {
		if text[i] != '@' || (i > 0 && !isReferenceBoundary(text[i-1])) {
			out.WriteByte(text[i])
			i++
			continue
		}

		j := i + 1
		for j < len(text) && !isReferenceBoundary(text[j]) {
			j++
		}
		if j == i+1 {
			out.WriteByte(text[i])
			i++
			continue
		}
		out.WriteString(ReferenceStyle.Render(text[i:j]))
		i = j
	}
	return out.String()
}
