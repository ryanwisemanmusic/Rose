package context

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ryanwi/rose/permission"
)

type Reference struct {
	Original  string
	Resolved  string
	Content   string
	IsDir     bool
	Blocked   bool
	BlockPath string
}

type Referencer struct {
	ProjectRoot string
	MaxReadSize int64
	PermMgr     *permission.Manager
}

func NewReferencer(projectRoot string, permMgr *permission.Manager) *Referencer {
	return &Referencer{
		ProjectRoot: projectRoot,
		MaxReadSize: 100 * 1024,
		PermMgr:     permMgr,
	}
}

var refPattern = regexp.MustCompile(`@(\S+)`)

func (r *Referencer) ResolveAll(text string) (string, []Reference, error) {
	matches := refPattern.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return text, nil, nil
	}

	var refs []Reference
	seen := make(map[string]bool)

	for _, match := range matches {
		raw := match[1]
		if seen[raw] {
			continue
		}
		seen[raw] = true

		ref := r.resolveSafe(raw)
		refs = append(refs, ref)
	}

	result := text
	for _, ref := range refs {
		insert := fmt.Sprintf("\n[Context: %s]\n%s\n[/Context]\n", ref.Resolved, ref.Content)
		result = strings.Replace(result, ref.Original, insert, 1)
	}

	return result, refs, nil
}

func (r *Referencer) resolveSafe(raw string) Reference {
	if strings.HasPrefix(raw, "/") {
		return r.resolveChecked(raw, raw)
	}

	candidate := filepath.Join(r.ProjectRoot, raw)
	return r.resolveChecked(raw, candidate)
}

func (r *Referencer) resolveChecked(original, resolvedPath string) Reference {
	if !r.contained(resolvedPath) {
		needsPrompt, _ := r.PermMgr.RequestAccess(resolvedPath)
		if needsPrompt {
			return Reference{
				Original:  "@" + original,
				Resolved:  resolvedPath,
				Content:   fmt.Sprintf("(blocked: %s — outside project)", resolvedPath),
				Blocked:   true,
				BlockPath: resolvedPath,
			}
		}
		return Reference{
			Original: "@" + original,
			Content:  fmt.Sprintf("(blocked: %s)", resolvedPath),
		}
	}

	if allowed, _ := r.PermMgr.CheckAccess(resolvedPath); allowed {
		return r.resolveUnchecked(original, resolvedPath)
	}

	return Reference{
		Original:  "@" + original,
		Resolved:  resolvedPath,
		Content:   fmt.Sprintf("(blocked: %s)", resolvedPath),
		Blocked:   true,
		BlockPath: resolvedPath,
	}
}

func (r *Referencer) contained(path string) bool {
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	proj, err := filepath.Abs(r.ProjectRoot)
	if err != nil {
		return false
	}
	return strings.HasPrefix(abs, proj)
}

func (r *Referencer) resolveUnchecked(original, path string) Reference {
	info, err := os.Stat(path)
	if err != nil {
		return Reference{
			Original: "@" + original,
			Content:  fmt.Sprintf("(path not found: %s)", path),
		}
	}
	var out Reference
	if info.IsDir() {
		out, err = r.readDir(path)
	} else {
		out, err = r.readFile(path)
	}
	if err != nil {
		return Reference{
			Original: "@" + original,
			Content:  fmt.Sprintf("(error: %s)", err),
		}
	}
	out.Original = "@" + original
	out.Resolved = path
	return out
}

func (r *Referencer) readDir(path string) (Reference, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return Reference{}, fmt.Errorf("read dir: %w", err)
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Directory: %s\n", path))
	b.WriteString(fmt.Sprintf("(%d entries)\n", len(entries)))
	b.WriteString("\n")

	for _, e := range entries {
		info, _ := e.Info()
		size := ""
		if info != nil {
			size = fmt.Sprintf(" (%d bytes)", info.Size())
		}
		marker := "  "
		if e.IsDir() {
			marker = "[dir] "
		} else {
			marker = "[file] "
		}
		b.WriteString(fmt.Sprintf("%s%s%s\n", marker, e.Name(), size))
	}

	return Reference{
		Content: b.String(),
		IsDir:   true,
	}, nil
}

func (r *Referencer) readFile(path string) (Reference, error) {
	info, err := os.Stat(path)
	if err != nil {
		return Reference{}, fmt.Errorf("stat: %w", err)
	}

	if info.Size() > r.MaxReadSize {
		return Reference{}, fmt.Errorf("file too large (%d bytes, max %d)", info.Size(), r.MaxReadSize)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return Reference{}, fmt.Errorf("read: %w", err)
	}

	ext := filepath.Ext(path)
	return Reference{
		Content: fmt.Sprintf("```%s\n%s\n```", strings.TrimPrefix(ext, "."), string(data)),
	}, nil
}

func (r *Referencer) SummarizeRefs(refs []Reference) string {
	if len(refs) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("(%d references)", len(refs)))
	for _, ref := range refs {
		if ref.Blocked {
			b.WriteString(fmt.Sprintf(" [blocked:%s]", filepath.Base(ref.Resolved)))
		} else if ref.IsDir {
			b.WriteString(fmt.Sprintf(" dir:%s", filepath.Base(ref.Resolved)))
		} else {
			b.WriteString(fmt.Sprintf(" %s", filepath.Base(ref.Resolved)))
		}
	}
	return b.String()
}
