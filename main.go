package main

import (
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ryanwi/rose/config"
	"github.com/ryanwi/rose/memory"
	"github.com/ryanwi/rose/sandbox"
	"github.com/ryanwi/rose/tui"
	"github.com/ryanwi/rose/workspace"
)

func main() {
	cfg := config.Default()
	if err := cfg.Load(); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	ws := workspace.Detect()

	if ws.RoseRoot != "" && cfg.RoseRoot != ws.RoseRoot {
		cfg.RoseRoot = ws.RoseRoot
		cfg.Save()
	} else if ws.RoseRoot == "" && cfg.RoseRoot != "" {
		ws.RoseRoot = cfg.RoseRoot
	}

	store, err := memory.NewStore(cfg.HistoryPath())
	if err != nil {
		log.Fatalf("Failed to open history store: %v", err)
	}
	defer store.Close()

	executor, err := sandbox.NewExecutor(cfg.SandboxTimeout)
	if err != nil {
		log.Fatalf("Failed to create sandbox executor: %v", err)
	}
	defer executor.Cleanup()

	options := []tea.ProgramOption{}
	if os.Getenv("ROSE_NO_ALT_SCREEN") != "1" {
		options = append(options, tea.WithAltScreen())
	}

	p := tea.NewProgram(
		tui.InitialModel(cfg, store, executor, ws),
		options...,
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
