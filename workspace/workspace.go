package workspace

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Context struct {
	CurrentDir   string
	ProjectName  string
	ProjectRoot  string
	IsGitRepo    bool
	GitRoot      string
	Languages    []string
	RoseRoot     string
}

func Detect() *Context {
	c := &Context{}
	c.CurrentDir, _ = os.Getwd()

	c.ProjectName = filepath.Base(c.CurrentDir)
	c.ProjectRoot = c.CurrentDir

	if gitRoot, err := exec.Command("git", "rev-parse", "--show-toplevel").Output(); err == nil {
		c.IsGitRepo = true
		c.GitRoot = strings.TrimSpace(string(gitRoot))
		c.ProjectName = filepath.Base(c.GitRoot)
		c.ProjectRoot = c.GitRoot
	}

	c.Languages = detectLanguages(c.ProjectRoot)
	c.RoseRoot = findRoseRoot()
	return c
}

func (c *Context) IsInRoseProject() bool {
	return c.RoseRoot != "" && c.ProjectRoot == c.RoseRoot
}

func (c *Context) Summary() string {
	var parts []string
	parts = append(parts, "project: "+c.ProjectName)
	if c.IsGitRepo {
		parts = append(parts, "git: yes")
	}
	if len(c.Languages) > 0 {
		parts = append(parts, "langs: "+strings.Join(c.Languages, ","))
	}
	if c.RoseRoot != "" && c.RoseRoot != c.ProjectRoot {
		parts = append(parts, "rose: "+c.RoseRoot)
	}
	return strings.Join(parts, " | ")
}

func detectLanguages(root string) []string {
	var langs []string
	seen := make(map[string]bool)
	maxDepth := 4
	skipped := map[string]bool{
		"node_modules": true, "vendor": true, "target": true,
		".git": true, ".svn": true, "dist": true, "build": true,
		"__pycache__": true, ".next": true, "public": true,
	}

	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if skipped[info.Name()] {
				return filepath.SkipDir
			}
			depth := strings.Count(strings.TrimPrefix(path, root), string(os.PathSeparator))
			if depth > maxDepth {
				return filepath.SkipDir
			}
			return nil
		}
		ext := filepath.Ext(info.Name())
		lang := extToLang(ext)
		if lang != "" && !seen[lang] {
			seen[lang] = true
			langs = append(langs, lang)
		}
		if len(langs) >= 5 {
			return filepath.SkipAll
		}
		return nil
	})

	return langs
}

func extToLang(ext string) string {
	switch ext {
	case ".go":
		return "Go"
	case ".ts", ".tsx":
		return "TypeScript"
	case ".js", ".jsx":
		return "JavaScript"
	case ".py":
		return "Python"
	case ".rs":
		return "Rust"
	case ".zig":
		return "Zig"
	case ".rb":
		return "Ruby"
	case ".java":
		return "Java"
	case ".c", ".h":
		return "C"
	case ".cpp", ".hpp", ".cc":
		return "C++"
	case ".swift":
		return "Swift"
	case ".kt", ".kts":
		return "Kotlin"
	default:
		return ""
	}
}

func findRoseRoot() string {
	execPath, err := os.Executable()
	if err != nil {
		return ""
	}

	realPath, err := filepath.EvalSymlinks(execPath)
	if err != nil {
		realPath = execPath
	}

	candidates := []string{
		filepath.Dir(filepath.Dir(realPath)),
		filepath.Dir(realPath),
		filepath.Join(os.Getenv("HOME"), "Rose"),
		filepath.Join(os.Getenv("HOME"), "rose"),
	}

	for _, dir := range candidates {
		mainGo := filepath.Join(dir, "main.go")
		if info, err := os.Stat(mainGo); err == nil && !info.IsDir() {
			if goMod := filepath.Join(dir, "go.mod"); true {
				if _, err := os.Stat(goMod); err == nil {
					return dir
				}
			}
		}
	}

	cwd, _ := os.Getwd()
	for dir := cwd; dir != "/"; dir = filepath.Dir(dir) {
		mainGo := filepath.Join(dir, "main.go")
		if info, err := os.Stat(mainGo); err == nil && !info.IsDir() {
			if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
				return dir
			}
		}
	}

	return ""
}
