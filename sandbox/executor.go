package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Duration time.Duration
}

type Executor struct {
	WorkDir  string
	Timeout  time.Duration
	KeepDir  bool
}

func NewExecutor(timeoutSec int) (*Executor, error) {
	workDir, err := os.MkdirTemp("", "rose-sandbox-*")
	if err != nil {
		return nil, fmt.Errorf("create sandbox dir: %w", err)
	}
	return &Executor{
		WorkDir:  workDir,
		Timeout:  time.Duration(timeoutSec) * time.Second,
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
	ctx, cancel := context.WithTimeout(context.Background(), e.Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = e.WorkDir

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
	}, nil
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
