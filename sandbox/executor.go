package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type FileChange struct {
	Path string
	Kind string
	Size int64
}

type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Duration time.Duration
	WorkDir  string
	Changes  []FileChange
}

type Executor struct {
	WorkDir          string
	Timeout          time.Duration
	KeepDir          bool
	BlockGitWrite    bool
	ExperimentalMode bool
}

var gitWriteCommands = map[string]bool{
	"commit": true, "push": true, "merge": true, "rebase": true,
	"cherry-pick": true, "revert": true, "tag": true, "reset": true,
	"bisect": true, "notes": true, "replace": true, "update-ref": true,
	"worktree": true, "submodule": true, "gc": true, "prune": true,
	"clean": true, "rm": true, "mv": true, "checkout-index": true,
	"commit-tree": true, "write-tree": true, "mktag": true,
	"filter-branch": true, "am": true, "apply": true,
}

var gitReadCommands = map[string]bool{
	"log": true, "diff": true, "status": true, "show": true,
	"branch": true, "rev-parse": true, "config": true, "describe": true,
	"blame": true, "grep": true, "ls-files": true, "ls-tree": true,
	"cat-file": true, "diff-tree": true, "diff-files": true,
	"diff-index": true, "for-each-ref": true, "shortlog": true,
	"stash": true, "remote": true, "fetch": true, "pull": true,
	"checkout": true, "switch": true, "restore": true,
}

type blockedCommand struct {
	Pattern string
	Reason  string
}

var blockedPatterns = []blockedCommand{
	{Pattern: "rm -rf /", Reason: "catastrophic root deletion"},
	{Pattern: "rm -rf ~", Reason: "home directory deletion"},
	{Pattern: "rm -rf $HOME", Reason: "home directory deletion"},
	{Pattern: "rm -rf /*", Reason: "catastrophic root deletion"},
	{Pattern: "> /dev/sd", Reason: "raw disk write"},
	{Pattern: "> /dev/disk", Reason: "raw disk write"},
	{Pattern: "mkfs.", Reason: "filesystem format"},
	{Pattern: "dd if=", Reason: "raw disk operation"},
	{Pattern: ":(){ :|:& };:", Reason: "fork bomb"},
	{Pattern: "DROP TABLE", Reason: "database table destruction"},
	{Pattern: "DROP DATABASE", Reason: "database destruction"},
	{Pattern: "TRUNCATE TABLE", Reason: "database table truncation"},
	{Pattern: "chmod -R 777 /", Reason: "insecure root permissions"},
	{Pattern: "curl | sh", Reason: "piped remote execution"},
	{Pattern: "curl | bash", Reason: "piped remote execution"},
	{Pattern: "wget | sh", Reason: "piped remote execution"},
	{Pattern: "wget | bash", Reason: "piped remote execution"},
}

func checkDangerousCommand(command string) (blocked bool, reason string) {
	lower := strings.ToLower(command)
	for _, bc := range blockedPatterns {
		if strings.Contains(lower, strings.ToLower(bc.Pattern)) {
			return true, fmt.Sprintf("BLOCKED: command contains '%s' — %s", bc.Pattern, bc.Reason)
		}
	}
	return false, ""
}

func NewExecutor(timeoutSec int) (*Executor, error) {
	workDir, err := os.MkdirTemp("", "rose-sandbox-*")
	if err != nil {
		return nil, fmt.Errorf("create sandbox dir: %w", err)
	}
	return &Executor{
		WorkDir: workDir,
		Timeout: time.Duration(timeoutSec) * time.Second,
	}, nil
}

func (e *Executor) WriteFile(name, content string) error {
	path := filepath.Join(e.WorkDir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	return os.WriteFile(path, []byte(content), 0644)
}

func (e *Executor) ReadFile(name string) (string, error) {
	data, err := os.ReadFile(filepath.Join(e.WorkDir, name))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (e *Executor) Run(command string, args ...string) (*Result, error) {
	return e.runInDir(e.WorkDir, command, args...)
}

func (e *Executor) runInDir(dir, command string, args ...string) (*Result, error) {
	if blocked, reason := e.checkBlocked(command, args); blocked {
		return &Result{
			Stderr:   reason,
			ExitCode: -3,
			Duration: 0,
			WorkDir:  dir,
		}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), e.Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = dir
	cmd.Env = appleDoubleDisabledEnv()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else if ctx.Err() == context.DeadlineExceeded {
			exitCode = -1
		} else {
			exitCode = -2
		}
	}

	return &Result{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
		Duration: duration,
		WorkDir:  dir,
	}, nil
}

func (e *Executor) checkBlocked(command string, args []string) (bool, string) {
	if e.ExperimentalMode {
		return false, ""
	}

	// Check for dangerous commands regardless of git-write settings.
	fullCmd := command
	if len(args) > 0 {
		fullCmd = command + " " + strings.Join(args, " ")
	}
	if blocked, reason := checkDangerousCommand(fullCmd); blocked {
		return true, reason
	}

	if !e.BlockGitWrite {
		return false, ""
	}
	if command != "git" {
		return false, ""
	}
	if len(args) == 0 {
		return false, ""
	}
	subcmd := args[0]
	if gitWriteCommands[subcmd] {
		return true, fmt.Sprintf("BLOCKED: 'git %s' is disabled by default. Use experimental/ directory for write access.", subcmd)
	}
	if !gitReadCommands[subcmd] && !gitWriteCommands[subcmd] {
		return true, fmt.Sprintf("BLOCKED: 'git %s' is not in the allowed command list.", subcmd)
	}
	return false, ""
}

func (e *Executor) RunShell(code string, lang string) (*Result, error) {
	switch strings.ToLower(lang) {
	case "go":
		return e.runGo(code)
	case "zig":
		return e.runZig(code)
	case "typescript", "ts":
		return e.runTS(code)
	case "javascript", "js":
		return e.runJS(code)
	case "python", "py":
		return e.runPython(code)
	case "rust", "rs":
		return e.runRust(code)
	case "bash", "sh":
		return e.runBash(code)
	default:
		return nil, fmt.Errorf("unsupported language: %s", lang)
	}
}

func (e *Executor) RunProjectShell(code string, projectRoot string) (*Result, error) {
	root, err := filepath.Abs(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve project root: %w", err)
	}

	// Check shell script content for dangerous commands before executing.
	if !e.ExperimentalMode {
		if blocked, reason := checkDangerousCommand(code); blocked {
			return &Result{
				Stderr:   reason,
				ExitCode: -3,
				Duration: 0,
				WorkDir:  root,
			}, nil
		}
	}

	before, err := snapshotFiles(root)
	if err != nil {
		return nil, fmt.Errorf("snapshot project before execution: %w", err)
	}

	script, err := os.CreateTemp("", "rose-project-*.sh")
	if err != nil {
		return nil, fmt.Errorf("create project script: %w", err)
	}
	scriptPath := script.Name()
	defer os.Remove(scriptPath)

	if _, err := script.WriteString(code); err != nil {
		script.Close()
		return nil, fmt.Errorf("write project script: %w", err)
	}
	if err := script.Close(); err != nil {
		return nil, fmt.Errorf("close project script: %w", err)
	}
	if err := os.Chmod(scriptPath, 0700); err != nil {
		return nil, fmt.Errorf("chmod project script: %w", err)
	}

	result, err := e.runInDir(root, "bash", scriptPath)
	if result == nil {
		result = &Result{ExitCode: -2, WorkDir: root}
	}
	_ = cleanupAppleDouble(root)

	after, snapErr := snapshotFiles(root)
	if snapErr == nil {
		result.Changes = diffSnapshots(before, after)
	}
	result.WorkDir = root

	return result, err
}

func (e *Executor) DetectLanguage(code string) string {
	code = strings.TrimSpace(code)
	if strings.HasPrefix(code, "package main") || strings.HasPrefix(code, "package ") {
		return "go"
	}
	if strings.HasPrefix(code, "#!") {
		return "bash"
	}
	if strings.HasPrefix(code, "import") || strings.HasPrefix(code, "fn main") {
		return "rust"
	}
	if strings.HasPrefix(code, "const ") || strings.HasPrefix(code, "function") || strings.HasPrefix(code, "let ") {
		return "javascript"
	}
	if strings.HasPrefix(code, "print") || strings.HasPrefix(code, "def ") || strings.HasPrefix(code, "import ") {
		return "python"
	}
	return ""
}

func (e *Executor) runGo(code string) (*Result, error) {
	if err := e.WriteFile("main.go", code); err != nil {
		return nil, err
	}
	e.Run("go", "mod", "init", "sandbox")
	result, err := e.Run("go", "run", ".")
	return result, err
}

func (e *Executor) runZig(code string) (*Result, error) {
	if err := e.WriteFile("main.zig", code); err != nil {
		return nil, err
	}
	result, err := e.Run("zig", "run", "main.zig")
	return result, err
}

func (e *Executor) runTS(code string) (*Result, error) {
	if err := e.WriteFile("main.ts", code); err != nil {
		return nil, err
	}
	result, err := e.Run("npx", "--yes", "tsx", "main.ts")
	return result, err
}

func (e *Executor) runJS(code string) (*Result, error) {
	if err := e.WriteFile("main.js", code); err != nil {
		return nil, err
	}
	result, err := e.Run("node", "main.js")
	return result, err
}

func (e *Executor) runPython(code string) (*Result, error) {
	if err := e.WriteFile("main.py", code); err != nil {
		return nil, err
	}
	result, err := e.Run("python3", "main.py")
	return result, err
}

func (e *Executor) runRust(code string) (*Result, error) {
	srcDir := filepath.Join(e.WorkDir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		return nil, err
	}
	if err := e.WriteFile("src/main.rs", code); err != nil {
		return nil, err
	}
	cargoToml := `[package]
name = "sandbox"
version = "0.1.0"
edition = "2021"

[dependencies]
`
	if err := e.WriteFile("Cargo.toml", cargoToml); err != nil {
		return nil, err
	}
	result, err := e.Run("cargo", "run")
	return result, err
}

func (e *Executor) runBash(code string) (*Result, error) {
	if err := e.WriteFile("script.sh", code); err != nil {
		return nil, err
	}
	result, err := e.Run("bash", "script.sh")
	return result, err
}

func (e *Executor) Cleanup() error {
	if e.KeepDir {
		return nil
	}
	return os.RemoveAll(e.WorkDir)
}

type fileState struct {
	Size    int64
	Mode    os.FileMode
	ModTime time.Time
}

var snapshotSkipDirs = map[string]bool{
	".git":         true,
	".next":        true,
	"__pycache__":  true,
	"build":        true,
	"dist":         true,
	"node_modules": true,
	"target":       true,
	"vendor":       true,
}

func appleDoubleDisabledEnv() []string {
	env := os.Environ()
	env = append(env,
		"COPYFILE_DISABLE=1",
		"COPY_EXTENDED_ATTRIBUTES_DISABLE=1",
		"APPLEDOUBLE_DISABLE=1",
	)
	return env
}

func snapshotFiles(root string) (map[string]fileState, error) {
	files := make(map[string]fileState)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			if path != root && snapshotSkipDirs[entry.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if isAppleDoubleName(entry.Name()) {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		files[filepath.ToSlash(rel)] = fileState{
			Size:    info.Size(),
			Mode:    info.Mode(),
			ModTime: info.ModTime(),
		}
		return nil
	})
	return files, err
}

func isAppleDoubleName(name string) bool {
	return strings.HasPrefix(name, "._")
}

func cleanupAppleDouble(root string) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			if path != root && snapshotSkipDirs[entry.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if isAppleDoubleName(entry.Name()) {
			_ = os.Remove(path)
		}
		return nil
	})
}

func diffSnapshots(before, after map[string]fileState) []FileChange {
	var changes []FileChange

	for path, next := range after {
		prev, ok := before[path]
		if !ok {
			changes = append(changes, FileChange{Path: path, Kind: "created", Size: next.Size})
			continue
		}
		if prev.Size != next.Size || !prev.ModTime.Equal(next.ModTime) || prev.Mode != next.Mode {
			changes = append(changes, FileChange{Path: path, Kind: "modified", Size: next.Size})
		}
	}

	for path := range before {
		if _, ok := after[path]; !ok {
			changes = append(changes, FileChange{Path: path, Kind: "deleted"})
		}
	}

	sort.Slice(changes, func(i, j int) bool {
		if changes[i].Path == changes[j].Path {
			return changes[i].Kind < changes[j].Kind
		}
		return changes[i].Path < changes[j].Path
	})

	return changes
}
