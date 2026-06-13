package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type SelfImprover struct {
	RoseRoot string
}

func NewSelfImprover(roseRoot string) *SelfImprover {
	return &SelfImprover{RoseRoot: roseRoot}
}

type CodeFile struct {
	Path    string
	Content string
}

func (s *SelfImprover) ReadAllSource() ([]CodeFile, error) {
	if s.RoseRoot == "" {
		return nil, fmt.Errorf("rose root not set")
	}

	var files []CodeFile
	extensions := map[string]bool{".go": true}

	err := filepath.Walk(s.RoseRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			if info != nil && info.IsDir() && (info.Name() == ".git" || info.Name() == "vendor") {
				return filepath.SkipDir
			}
			return nil
		}
		if !extensions[filepath.Ext(info.Name())] {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		relPath, _ := filepath.Rel(s.RoseRoot, path)
		files = append(files, CodeFile{Path: relPath, Content: string(data)})
		return nil
	})

	return files, err
}

func (s *SelfImprover) ReadFile(packagePath string) (*CodeFile, error) {
	fullPath := filepath.Join(s.RoseRoot, packagePath)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("file not found: %s", packagePath)
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, err
	}
	return &CodeFile{Path: packagePath, Content: string(data)}, nil
}

func (s *SelfImprover) ApplyEdit(path string, oldContent, newContent string) error {
	fullPath := filepath.Join(s.RoseRoot, path)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return fmt.Errorf("read file for edit: %w", err)
	}

	content := string(data)
	if !strings.Contains(content, oldContent) {
		return fmt.Errorf("oldContent not found in %s", path)
	}

	updated := strings.Replace(content, oldContent, newContent, 1)
	if err := os.WriteFile(fullPath, []byte(updated), 0644); err != nil {
		return fmt.Errorf("write edited file: %w", err)
	}
	return nil
}

func (s *SelfImprover) CreateFile(path, content string) error {
	fullPath := filepath.Join(s.RoseRoot, path)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return fmt.Errorf("create dirs: %w", err)
	}
	if _, err := os.Stat(fullPath); err == nil {
		return fmt.Errorf("file already exists: %s", path)
	}
	return os.WriteFile(fullPath, []byte(content), 0644)
}

func (s *SelfImprover) Analyze() (string, error) {
	files, err := s.ReadAllSource()
	if err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Rose source code analysis (%d files):\n\n", len(files)))

	for _, f := range files {
		info := analyzeFile(f)
		b.WriteString(fmt.Sprintf("  %s (%d lines, %d funcs, %d structs)\n",
			f.Path, info.Lines, info.Functions, info.Structs))
	}

	b.WriteString(fmt.Sprintf("\nTotal: %d Go source files\n", len(files)))
	return b.String(), nil
}

type FileAnalysis struct {
	Lines     int
	Functions int
	Structs   int
	Imports   []string
}

func analyzeFile(f CodeFile) FileAnalysis {
	var a FileAnalysis
	lines := strings.Split(f.Content, "\n")
	a.Lines = len(lines)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "func ") || strings.HasPrefix(trimmed, "func (") {
			a.Functions++
		}
		if strings.HasPrefix(trimmed, "type ") && strings.Contains(trimmed, "struct") {
			a.Structs++
		}
		if strings.HasPrefix(trimmed, "import") {
			a.Imports = append(a.Imports, trimmed)
		}
	}
	return a
}

func (s *SelfImprover) BuildContext() string {
	if s.RoseRoot == "" {
		return ""
	}

	return fmt.Sprintf(`You are Rose, a self-improving coding assistant.
Your source code lives at: %s

You have the ability to:
1. Read your own source code
2. Propose improvements to yourself
3. Edit your own files
4. Create new files in your codebase
5. Learn from every interaction across all projects

When you learn something valuable, consider:
- Adding it to your memory/ directory
- Improving your prompting strategies
- Fixing bugs in your own execution
- Adding new capabilities to your sandbox

Source tree:
%s
`, s.RoseRoot, s.formatTree())
}

func (s *SelfImprover) formatTree() string {
	files, err := s.ReadAllSource()
	if err != nil {
		return "(unavailable)"
	}

	dirs := make(map[string][]string)
	for _, f := range files {
		dir := filepath.Dir(f.Path)
		dirs[dir] = append(dirs[dir], filepath.Base(f.Path))
	}

	var b strings.Builder
	var printDir func(dir string, indent int)
	printDir = func(dir string, indent int) {
		prefix := strings.Repeat("  ", indent)
		b.WriteString(fmt.Sprintf("%s%s/\n", prefix, dir))
		for _, f := range dirs[dir] {
			b.WriteString(fmt.Sprintf("%s  %s\n", prefix, f))
		}
	}

	for dir := range dirs {
		if dir == "." {
			for _, f := range dirs[dir] {
				b.WriteString(fmt.Sprintf("  %s\n", f))
			}
			delete(dirs, dir)
			break
		}
	}

	for dir := range dirs {
		printDir(dir, 1)
	}

	return b.String()
}
